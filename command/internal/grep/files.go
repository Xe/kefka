package grep

import (
	"bufio"
	"context"
	"errors"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Xe/kefka/command"
	"mvdan.cc/sh/v3/pattern"
)

var errDirectory = errors.New("Is a directory")
var errMissing = errors.New("No such file or directory")

func resolvePath(ec *command.ExecContext, p string) string {
	// Root the clean operation to prevent .. from escaping the virtual filesystem.
	if !path.IsAbs(p) {
		p = path.Join(ec.Dir, p)
	}
	p = strings.TrimPrefix(path.Clean("/"+p), "/")
	if p == "" {
		return "."
	}
	return p
}

// openInput reuses stdin's reader for pattern files and search operands.
// Only filesystem inputs are closed.
func openInput(ctx context.Context, ec *command.ExecContext, name string) (*bufio.Reader, func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if name == "-" {
		if ec.Stdin == nil {
			return bufio.NewReader(strings.NewReader("")), func() error { return nil }, nil
		}
		if r, ok := ec.Stdin.(*bufio.Reader); ok {
			return r, func() error { return nil }, nil
		}
		return bufio.NewReader(contextReader{ctx, ec.Stdin}), func() error { return nil }, nil
	}
	if name == "" || ec.FS == nil {
		return nil, nil, errMissing
	}
	full := resolvePath(ec, name)
	info, err := ec.FS.Stat(full)
	if err != nil {
		return nil, nil, errMissing
	}
	if info.IsDir() {
		return nil, nil, errDirectory
	}
	f, err := ec.FS.Open(full)
	if err != nil {
		return nil, nil, errMissing
	}
	return bufio.NewReader(contextReader{ctx, f}), f.Close, nil
}

// Pattern files must be read completely before compiling the search expression.
func readInput(ctx context.Context, ec *command.ExecContext, name string) (string, error) {
	r, closeInput, err := openInput(ctx, ec, name)
	if err != nil {
		return "", err
	}
	defer closeInput()
	content, err := io.ReadAll(r)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errInput
	}
	return decodeUTF8(content), nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}

func matchesGlob(name, glob string) bool {
	if len(glob) >= 2 && (glob[0] == '\'' && glob[len(glob)-1] == '\'' || glob[0] == '"' && glob[len(glob)-1] == '"') {
		glob = glob[1 : len(glob)-1]
	}
	expr, err := pattern.Regexp(glob, 0)
	if err != nil {
		return false
	}
	re, err := regexp.Compile("^(?:" + expr + ")$")
	return err == nil && re.MatchString(name)
}
func matchesAny(name string, globs []string) bool {
	for _, glob := range globs {
		if matchesGlob(name, glob) {
			return true
		}
	}
	return false
}
func (o options) acceptFile(name string) bool {
	base := path.Base(name)
	return !matchesAny(base, o.exclude) && (len(o.include) == 0 || matchesAny(base, o.include))
}

const maxDepth = 256
const maxTraversalOperations = 100000
const maxTraversalResults = 100000

type traversalBudget struct{ operations, results int }

func (b *traversalBudget) step() error {
	b.operations++
	if b.operations > maxTraversalOperations {
		return errors.New("glob operation limit exceeded")
	}
	return nil
}
func (b *traversalBudget) add(result *[]string, name string) error {
	if b.results >= maxTraversalResults {
		return errors.New("array element limit exceeded")
	}
	b.results++
	*result = append(*result, name)
	return nil
}

func collectFiles(ctx context.Context, ec *command.ExecContext, files []string, o options) ([]string, error) {
	var result []string
	budget := new(traversalBudget)
	for _, name := range files {
		if err := budget.step(); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if name == "-" {
			if err := budget.add(&result, name); err != nil {
				return nil, err
			}
			continue
		}
		expanded := []string{name}
		if strings.ContainsAny(name, "*?[") {
			var err error
			expanded, err = expandGlob(ctx, ec, name, budget)
			if err != nil {
				return nil, err
			}
		}
		for _, entry := range expanded {
			if !o.recursive {
				if err := budget.add(&result, entry); err != nil {
					return nil, err
				}
				continue
			}
			if err := walkFiles(ctx, ec, entry, o, 0, &result, budget); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func walkFiles(ctx context.Context, ec *command.ExecContext, name string, o options, depth int, result *[]string, budget *traversalBudget) error {
	if err := budget.step(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth >= maxDepth || ec.FS == nil {
		return nil
	}
	full := resolvePath(ec, name)
	info, err := ec.FS.Stat(full)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() && o.acceptFile(name) {
			return budget.add(result, name)
		}
		return nil
	}
	if matchesAny(path.Base(name), o.excludeDir) {
		return nil
	}
	if err := budget.step(); err != nil {
		return err
	}
	entries, err := ec.FS.ReadDir(full)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if err := budget.step(); err != nil {
			return err
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := walkFiles(ctx, ec, childPath(name, e.Name()), o, depth+1, result, budget); err != nil {
			return err
		}
	}
	return nil
}

func childPath(dir, name string) string {
	if dir == "." || dir == "" {
		if name == "-" {
			return "./-"
		}
		return name
	}
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

func expandGlob(ctx context.Context, ec *command.ExecContext, glob string, budget *traversalBudget) ([]string, error) {
	if err := budget.step(); err != nil {
		return nil, err
	}
	if ec.FS == nil {
		return nil, nil
	}
	if i := strings.Index(glob, "**"); i >= 0 {
		base := strings.TrimSuffix(glob[:i], "/")
		if base == "" {
			base = "."
		}
		suffix := strings.TrimPrefix(strings.Split(glob[i+2:], "**")[0], "/")
		var result []string
		err := walkGlob(ctx, ec, base, suffix, 0, &result, budget)
		return result, err
	}
	dir, leaf := path.Split(glob)
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		if strings.HasPrefix(glob, "/") {
			dir = "/"
		} else {
			dir = "."
		}
	}
	entries, err := ec.FS.ReadDir(resolvePath(ec, dir))
	if err != nil {
		return nil, nil
	}
	var result []string
	for _, e := range entries {
		if err := budget.step(); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if matchesGlob(e.Name(), leaf) {
			if err := budget.add(&result, childPath(dir, e.Name())); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(result)
	return result, nil
}

func walkGlob(ctx context.Context, ec *command.ExecContext, name, glob string, depth int, result *[]string, budget *traversalBudget) error {
	if err := budget.step(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth >= maxDepth {
		return nil
	}
	info, err := ec.FS.Stat(resolvePath(ec, name))
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		if glob != "" && matchesGlob(path.Base(name), glob) {
			return budget.add(result, name)
		}
		return nil
	}
	entries, err := ec.FS.ReadDir(resolvePath(ec, name))
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if err := walkGlob(ctx, ec, childPath(name, e.Name()), glob, depth+1, result, budget); err != nil {
			return err
		}
	}
	return nil
}

// decodeUTF8 replaces each malformed UTF-8 subsequence, as TextDecoder does.
func decodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var out strings.Builder
	for len(data) > 0 {
		r, n := utf8.DecodeRune(data)
		if r == utf8.RuneError && n == 1 {
			// An incomplete valid prefix is one error. Invalid leading bytes remain
			// separate errors, including consecutive stray continuation bytes.
			width := 0
			switch {
			case data[0] >= 0xc2 && data[0] <= 0xdf:
				width = 2
			case data[0] >= 0xe0 && data[0] <= 0xef:
				width = 3
			case data[0] >= 0xf0 && data[0] <= 0xf4:
				width = 4
			}
			for n < width && n < len(data) {
				c := data[n]
				if c < 0x80 || c > 0xbf {
					break
				}
				if n == 1 && (data[0] == 0xe0 && c < 0xa0 || data[0] == 0xed && c > 0x9f || data[0] == 0xf0 && c < 0x90 || data[0] == 0xf4 && c > 0x8f) {
					break
				}
				n++
			}
		}
		out.WriteRune(r)
		data = data[n:]
	}
	return out.String()
}
