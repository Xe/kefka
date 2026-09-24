package diff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pborman/getopt/v2"
	"github.com/pmezard/go-difflib/difflib"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

var nowFunc = time.Now

type format int

const (
	formatNormal format = iota
	formatContext
	formatUnified
	formatEd
	formatForwardEd
)

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("diff: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("diff")
	set.SetParameters("FILE1 FILE2")

	usage := func() {
		fmt.Fprint(stderr, "Usage: diff [OPTION]... FILE1 FILE2\n")
		fmt.Fprint(stderr, "Compare files line by line.\n\n")
		fmt.Fprint(stderr, "  -b, --ignore-space-change     ignore changes in the amount of white space\n")
		fmt.Fprint(stderr, "  -c                            output 3 lines of copied context\n")
		fmt.Fprint(stderr, "  -C NUM, --context[=NUM]       output NUM (default 3) lines of copied context\n")
		fmt.Fprint(stderr, "  -e, --ed                      output an ed script\n")
		fmt.Fprint(stderr, "  -f                            output a forward ed script\n")
		fmt.Fprint(stderr, "  -u                            output 3 lines of unified context\n")
		fmt.Fprint(stderr, "  -U NUM, --unified[=NUM]       output NUM (default 3) lines of unified context\n")
		fmt.Fprint(stderr, "  -r, --recursive               recursively compare any subdirectories found\n")
		fmt.Fprint(stderr, "  -q, --brief                   report only whether files differ\n")
		fmt.Fprint(stderr, "  -s, --report-identical-files  report when files are the same\n")
		fmt.Fprint(stderr, "  -i, --ignore-case             ignore case differences\n")
		fmt.Fprint(stderr, "      --help                    display this help and exit\n")
	}
	set.SetUsage(usage)

	unified := set.Bool('u', "output 3 lines of unified context")
	unifiedN := set.StringLong("unified", 'U', "", "output NUM lines of unified context")
	contextFlag := set.Bool('c', "output 3 lines of copied context")
	contextN := set.StringLong("context", 'C', "", "output NUM lines of copied context")
	edScript := set.BoolLong("ed", 'e', "output an ed script")
	forwardEd := set.Bool('f', "output a forward ed script")
	recursive := set.BoolLong("recursive", 'r', "recursively compare any subdirectories found")
	ignoreSpace := set.BoolLong("ignore-space-change", 'b', "ignore changes in the amount of white space")
	brief := set.BoolLong("brief", 'q', "report only whether files differ")
	reportSame := set.BoolLong("report-identical-files", 's', "report when files are the same")
	ignoreCase := set.BoolLong("ignore-case", 'i', "ignore case differences")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"diff"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "diff: %s\n", err)
		usage()
		return interp.ExitStatus(2)
	}
	if *help {
		usage()
		return nil
	}

	fmtKind := formatNormal
	contextLines := 3
	switch {
	case *edScript:
		fmtKind = formatEd
	case *forwardEd:
		fmtKind = formatForwardEd
	case *unifiedN != "":
		fmtKind = formatUnified
		n, err := strconv.Atoi(*unifiedN)
		if err != nil || n < 0 {
			fmt.Fprintf(stderr, "diff: invalid context length '%s'\n", *unifiedN)
			return interp.ExitStatus(2)
		}
		contextLines = n
	case *contextN != "":
		fmtKind = formatContext
		n, err := strconv.Atoi(*contextN)
		if err != nil || n < 0 {
			fmt.Fprintf(stderr, "diff: invalid context length '%s'\n", *contextN)
			return interp.ExitStatus(2)
		}
		contextLines = n
	case *unified:
		fmtKind = formatUnified
	case *contextFlag:
		fmtKind = formatContext
	}

	files := set.Args()
	if len(files) < 2 {
		fmt.Fprint(stderr, "diff: missing operand\n")
		return interp.ExitStatus(2)
	}

	f1, f2 := files[0], files[1]

	opts := compareOptions{
		ignoreCase:   *ignoreCase,
		ignoreSpace:  *ignoreSpace,
		brief:        *brief,
		reportSame:   *reportSame,
		recursive:    *recursive,
		fmtKind:      fmtKind,
		contextLines: contextLines,
	}

	// If both operands are directories, perform directory comparison.
	isDir1, err1 := isDirectory(ec, f1)
	isDir2, err2 := isDirectory(ec, f2)
	if err1 != nil && f1 != "-" {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f1)
		return interp.ExitStatus(2)
	}
	if err2 != nil && f2 != "-" {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f2)
		return interp.ExitStatus(2)
	}

	if isDir1 && isDir2 {
		return diffDirs(ec, stdout, stderr, f1, f2, opts)
	}
	if isDir1 || isDir2 {
		// If one is a directory, append the other's basename to find the
		// matching file under it. This is GNU's behavior.
		if isDir1 {
			f1 = path.Join(f1, path.Base(f2))
		} else {
			f2 = path.Join(f2, path.Base(f1))
		}
	}

	return diffFiles(ec, stdout, stderr, f1, f2, opts)
}

type compareOptions struct {
	ignoreCase   bool
	ignoreSpace  bool
	brief        bool
	reportSame   bool
	recursive    bool
	fmtKind      format
	contextLines int
}

func diffFiles(ec *command.ExecContext, stdout, stderr io.Writer, f1, f2 string, opts compareOptions) error {
	c1, t1stamp, err := readContent(ec, f1)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f1)
		return interp.ExitStatus(2)
	}
	c2, t2stamp, err := readContent(ec, f2)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f2)
		return interp.ExitStatus(2)
	}

	a := splitLines(c1)
	b := splitLines(c2)

	cmpA := normalizeLines(a, opts.ignoreCase, opts.ignoreSpace)
	cmpB := normalizeLines(b, opts.ignoreCase, opts.ignoreSpace)

	equal := len(cmpA) == len(cmpB)
	if equal {
		for i := range cmpA {
			if cmpA[i] != cmpB[i] {
				equal = false
				break
			}
		}
	}
	if equal {
		if opts.reportSame {
			fmt.Fprintf(stdout, "Files %s and %s are identical\n", f1, f2)
		}
		return nil
	}

	if opts.brief {
		fmt.Fprintf(stdout, "Files %s and %s differ\n", f1, f2)
		return interp.ExitStatus(1)
	}

	matcher := difflib.NewMatcher(cmpA, cmpB)
	ops := matcher.GetOpCodes()

	switch opts.fmtKind {
	case formatUnified:
		writeUnified(stdout, f1, f2, t1stamp, t2stamp, a, b, matcher.GetGroupedOpCodes(opts.contextLines))
	case formatContext:
		writeContext(stdout, f1, f2, t1stamp, t2stamp, a, b, matcher.GetGroupedOpCodes(opts.contextLines))
	case formatEd:
		writeEd(stdout, a, b, ops)
	case formatForwardEd:
		writeForwardEd(stdout, a, b, ops)
	default:
		writeNormal(stdout, a, b, ops)
	}

	return interp.ExitStatus(1)
}

// diffDirs performs a directory comparison as GNU diff does: list both
// directories, for matching names recurse (when -r) or compare files,
// and emit "Only in" lines for entries unique to one side.
func diffDirs(ec *command.ExecContext, stdout, stderr io.Writer, d1, d2 string, opts compareOptions) error {
	entries1, err := readDirNames(ec, d1)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: %s\n", d1, err)
		return interp.ExitStatus(2)
	}
	entries2, err := readDirNames(ec, d2)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: %s\n", d2, err)
		return interp.ExitStatus(2)
	}

	set1 := make(map[string]struct{}, len(entries1))
	for _, n := range entries1 {
		set1[n] = struct{}{}
	}
	set2 := make(map[string]struct{}, len(entries2))
	for _, n := range entries2 {
		set2[n] = struct{}{}
	}

	all := make(map[string]struct{}, len(entries1)+len(entries2))
	for n := range set1 {
		all[n] = struct{}{}
	}
	for n := range set2 {
		all[n] = struct{}{}
	}
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)

	differed := false
	for _, name := range names {
		_, in1 := set1[name]
		_, in2 := set2[name]
		p1 := path.Join(d1, name)
		p2 := path.Join(d2, name)

		switch {
		case in1 && !in2:
			fmt.Fprintf(stdout, "Only in %s: %s\n", d1, name)
			differed = true
		case !in1 && in2:
			fmt.Fprintf(stdout, "Only in %s: %s\n", d2, name)
			differed = true
		default:
			isDir1, _ := isDirectory(ec, p1)
			isDir2, _ := isDirectory(ec, p2)
			switch {
			case isDir1 && isDir2:
				if opts.recursive {
					if err := diffDirs(ec, stdout, stderr, p1, p2, opts); err != nil {
						if code := exitStatus(err); code == 1 {
							differed = true
						} else {
							return err
						}
					}
				} else {
					fmt.Fprintf(stdout, "Common subdirectories: %s and %s\n", p1, p2)
				}
			case isDir1 != isDir2:
				if isDir1 {
					fmt.Fprintf(stdout, "File %s is a directory while file %s is a regular file\n", p1, p2)
				} else {
					fmt.Fprintf(stdout, "File %s is a regular file while file %s is a directory\n", p1, p2)
				}
				differed = true
			default:
				// Pre-check whether files differ so we can emit the
				// "diff X Y" header GNU prints before the body.
				if !filesEqual(ec, p1, p2, opts) {
					if !opts.brief {
						fmt.Fprintf(stdout, "diff %s %s\n", p1, p2)
					}
				}
				if err := diffFiles(ec, stdout, stderr, p1, p2, opts); err != nil {
					if code := exitStatus(err); code == 1 {
						differed = true
					} else {
						return err
					}
				}
			}
		}
	}

	if differed {
		return interp.ExitStatus(1)
	}
	return nil
}

// filesEqual reports whether two files compare equal under the given options.
// Used by directory traversal to decide whether to emit a "diff X Y" header.
func filesEqual(ec *command.ExecContext, f1, f2 string, opts compareOptions) bool {
	c1, _, err := readContent(ec, f1)
	if err != nil {
		return false
	}
	c2, _, err := readContent(ec, f2)
	if err != nil {
		return false
	}
	a := splitLines(c1)
	b := splitLines(c2)
	cmpA := normalizeLines(a, opts.ignoreCase, opts.ignoreSpace)
	cmpB := normalizeLines(b, opts.ignoreCase, opts.ignoreSpace)
	if len(cmpA) != len(cmpB) {
		return false
	}
	for i := range cmpA {
		if cmpA[i] != cmpB[i] {
			return false
		}
	}
	return true
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	if es, ok := err.(interp.ExitStatus); ok {
		return int(es)
	}
	return -1
}

func isDirectory(ec *command.ExecContext, p string) (bool, error) {
	if p == "-" {
		return false, nil
	}
	if ec.FS == nil {
		return false, errors.New("no filesystem")
	}
	full := resolvePath(ec, p)
	info, err := ec.FS.Stat(full)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func readDirNames(ec *command.ExecContext, dir string) ([]string, error) {
	if ec.FS == nil {
		return nil, errors.New("no filesystem")
	}
	full := resolvePath(ec, dir)
	entries, err := ec.FS.ReadDir(full)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func normalizeLines(lines []string, ignoreCase, ignoreSpace bool) []string {
	if !ignoreCase && !ignoreSpace {
		out := make([]string, len(lines))
		copy(out, lines)
		return out
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		s := l
		if ignoreSpace {
			s = collapseSpace(s)
		}
		if ignoreCase {
			s = strings.ToLower(s)
		}
		out[i] = s
	}
	return out
}

func collapseSpace(s string) string {
	trailingNL := strings.HasSuffix(s, "\n")
	if trailingNL {
		s = s[:len(s)-1]
	}
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	if trailingNL {
		b.WriteByte('\n')
	}
	return b.String()
}

func hasNL(s string) bool { return strings.HasSuffix(s, "\n") }

// formatRange renders a 1-based inclusive line range "lo,hi" or "lo" when single.
func formatRange(lo, hi int) string {
	if lo == hi {
		return strconv.Itoa(lo)
	}
	return fmt.Sprintf("%d,%d", lo, hi)
}

func writeNormal(w io.Writer, a, b []string, ops []difflib.OpCode) {
	for _, op := range ops {
		switch op.Tag {
		case 'e':
			continue
		case 'r':
			lhs := formatRange(op.I1+1, op.I2)
			rhs := formatRange(op.J1+1, op.J2)
			fmt.Fprintf(w, "%sc%s\n", lhs, rhs)
			for i := op.I1; i < op.I2; i++ {
				writePrefixed(w, "< ", a[i])
			}
			fmt.Fprint(w, "---\n")
			for j := op.J1; j < op.J2; j++ {
				writePrefixed(w, "> ", b[j])
			}
		case 'd':
			lhs := formatRange(op.I1+1, op.I2)
			fmt.Fprintf(w, "%sd%d\n", lhs, op.J1)
			for i := op.I1; i < op.I2; i++ {
				writePrefixed(w, "< ", a[i])
			}
		case 'i':
			rhs := formatRange(op.J1+1, op.J2)
			fmt.Fprintf(w, "%da%s\n", op.I1, rhs)
			for j := op.J1; j < op.J2; j++ {
				writePrefixed(w, "> ", b[j])
			}
		}
	}
}

// writePrefixed writes prefix + line (which may or may not end in \n).
// If the line lacks a trailing newline (file with no final \n), GNU diff
// emits "\ No newline at end of file" on the next line.
func writePrefixed(w io.Writer, prefix, line string) {
	if hasNL(line) {
		io.WriteString(w, prefix)
		io.WriteString(w, line)
		return
	}
	io.WriteString(w, prefix)
	io.WriteString(w, line)
	io.WriteString(w, "\n\\ No newline at end of file\n")
}

func writeEd(w io.Writer, _, b []string, ops []difflib.OpCode) {
	type hunk struct {
		header string
		body   []string
	}
	var hunks []hunk
	for _, op := range ops {
		switch op.Tag {
		case 'e':
			continue
		case 'r':
			h := hunk{header: fmt.Sprintf("%sc\n", formatRange(op.I1+1, op.I2))}
			for j := op.J1; j < op.J2; j++ {
				h.body = append(h.body, b[j])
			}
			h.body = append(h.body, ".\n")
			hunks = append(hunks, h)
		case 'd':
			hunks = append(hunks, hunk{header: fmt.Sprintf("%sd\n", formatRange(op.I1+1, op.I2))})
		case 'i':
			h := hunk{header: fmt.Sprintf("%da\n", op.I1)}
			for j := op.J1; j < op.J2; j++ {
				h.body = append(h.body, b[j])
			}
			h.body = append(h.body, ".\n")
			hunks = append(hunks, h)
		}
	}
	for i := len(hunks) - 1; i >= 0; i-- {
		io.WriteString(w, hunks[i].header)
		for _, line := range hunks[i].body {
			s := line
			if !hasNL(s) {
				s += "\n"
			}
			io.WriteString(w, s)
		}
	}
}

// writeForwardEd emits a forward ed script (-f). Like -e but hunks appear
// in forward order and the command letter precedes the range:
//   c<range>, a<line>, d<range>
// Note: output is not directly executable by ed; it's intended for tools
// that want top-to-bottom ordering with ed-like syntax.
func writeForwardEd(w io.Writer, _, b []string, ops []difflib.OpCode) {
	for _, op := range ops {
		switch op.Tag {
		case 'e':
			continue
		case 'r':
			fmt.Fprintf(w, "c%s\n", formatRange(op.I1+1, op.I2))
			for j := op.J1; j < op.J2; j++ {
				s := b[j]
				if !hasNL(s) {
					s += "\n"
				}
				io.WriteString(w, s)
			}
			io.WriteString(w, ".\n")
		case 'd':
			fmt.Fprintf(w, "d%s\n", formatRange(op.I1+1, op.I2))
		case 'i':
			fmt.Fprintf(w, "a%d\n", op.I1)
			for j := op.J1; j < op.J2; j++ {
				s := b[j]
				if !hasNL(s) {
					s += "\n"
				}
				io.WriteString(w, s)
			}
			io.WriteString(w, ".\n")
		}
	}
}

func unifiedRange(start, count int) string {
	if count == 0 {
		return fmt.Sprintf("%d,0", start)
	}
	if count == 1 {
		return strconv.Itoa(start + 1)
	}
	return fmt.Sprintf("%d,%d", start+1, count)
}

func writeUnified(w io.Writer, f1, f2, t1, t2 string, a, b []string, groups [][]difflib.OpCode) {
	if len(groups) == 0 {
		return
	}
	fmt.Fprintf(w, "--- %s\t%s\n", f1, t1)
	fmt.Fprintf(w, "+++ %s\t%s\n", f2, t2)
	for _, g := range groups {
		first, last := g[0], g[len(g)-1]
		fmt.Fprintf(w, "@@ -%s +%s @@\n",
			unifiedRange(first.I1, last.I2-first.I1),
			unifiedRange(first.J1, last.J2-first.J1),
		)
		for _, op := range g {
			switch op.Tag {
			case 'e':
				for i := op.I1; i < op.I2; i++ {
					writePrefixed(w, " ", a[i])
				}
			case 'r':
				for i := op.I1; i < op.I2; i++ {
					writePrefixed(w, "-", a[i])
				}
				for j := op.J1; j < op.J2; j++ {
					writePrefixed(w, "+", b[j])
				}
			case 'd':
				for i := op.I1; i < op.I2; i++ {
					writePrefixed(w, "-", a[i])
				}
			case 'i':
				for j := op.J1; j < op.J2; j++ {
					writePrefixed(w, "+", b[j])
				}
			}
		}
	}
}

func contextRange(lo, hi int) string {
	if hi < lo {
		return strconv.Itoa(hi)
	}
	if lo == hi {
		return strconv.Itoa(lo)
	}
	return fmt.Sprintf("%d,%d", lo, hi)
}

func writeContext(w io.Writer, f1, f2, t1, t2 string, a, b []string, groups [][]difflib.OpCode) {
	if len(groups) == 0 {
		return
	}
	fmt.Fprintf(w, "*** %s\t%s\n", f1, t1)
	fmt.Fprintf(w, "--- %s\t%s\n", f2, t2)
	for _, g := range groups {
		fmt.Fprint(w, "***************\n")
		first, last := g[0], g[len(g)-1]

		aLo, aHi := first.I1+1, last.I2
		if last.I2 == first.I1 {
			aLo, aHi = first.I1, first.I1
		}
		bLo, bHi := first.J1+1, last.J2
		if last.J2 == first.J1 {
			bLo, bHi = first.J1, first.J1
		}

		hasADel := false
		hasBAdd := false
		for _, op := range g {
			switch op.Tag {
			case 'r':
				hasADel = true
				hasBAdd = true
			case 'd':
				hasADel = true
			case 'i':
				hasBAdd = true
			}
		}

		fmt.Fprintf(w, "*** %s ****\n", contextRange(aLo, aHi))
		if hasADel {
			for _, op := range g {
				switch op.Tag {
				case 'e':
					for i := op.I1; i < op.I2; i++ {
						writePrefixed(w, "  ", a[i])
					}
				case 'r':
					for i := op.I1; i < op.I2; i++ {
						writePrefixed(w, "! ", a[i])
					}
				case 'd':
					for i := op.I1; i < op.I2; i++ {
						writePrefixed(w, "- ", a[i])
					}
				}
			}
		}
		fmt.Fprintf(w, "--- %s ----\n", contextRange(bLo, bHi))
		if hasBAdd {
			for _, op := range g {
				switch op.Tag {
				case 'e':
					for j := op.J1; j < op.J2; j++ {
						writePrefixed(w, "  ", b[j])
					}
				case 'r':
					for j := op.J1; j < op.J2; j++ {
						writePrefixed(w, "! ", b[j])
					}
				case 'i':
					for j := op.J1; j < op.J2; j++ {
						writePrefixed(w, "+ ", b[j])
					}
				}
			}
		}
	}
}

func readContent(ec *command.ExecContext, file string) (string, string, error) {
	now := nowFunc()
	if file == "-" {
		if ec.Stdin == nil {
			return "", formatTime(now), nil
		}
		data, err := io.ReadAll(ec.Stdin)
		if err != nil {
			return "", "", err
		}
		return string(data), formatTime(now), nil
	}
	if ec.FS == nil {
		return "", "", errors.New("no filesystem")
	}
	full := resolvePath(ec, file)
	stamp := formatTime(now)
	f, err := ec.FS.Open(full)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", "", err
	}
	return string(data), stamp, nil
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return "."
		}
		return path.Clean(p)
	}
	joined := path.Join(dir, p)
	if joined == "" {
		return "."
	}
	return joined
}
