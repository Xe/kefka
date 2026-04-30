package ls

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("ls: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("ls: ExecContext has no filesystem")
	}

	var (
		showAll       bool
		showAlmostAll bool
		longFormat    bool
		humanReadable bool
		recursive     bool
		reverse       bool
		sortBySize    bool
		classifyFiles bool
		directoryOnly bool
		sortByTime    bool
		onePerLine    bool
	)

	flagSet := pflag.NewFlagSet("ls", pflag.ContinueOnError)
	flagSet.SetOutput(ec.Stderr)
	flagSet.BoolVarP(&showAll, "all", "a", false, "do not ignore entries starting with .")
	flagSet.BoolVarP(&showAlmostAll, "almost-all", "A", false, "do not list . and ..")
	flagSet.BoolVarP(&longFormat, "long", "l", false, "use a long listing format")
	flagSet.BoolVarP(&humanReadable, "human-readable", "h", false, "with -l, print sizes like 1K 234M 2G etc.")
	flagSet.BoolVarP(&recursive, "recursive", "R", false, "list subdirectories recursively")
	flagSet.BoolVarP(&reverse, "reverse", "r", false, "reverse order while sorting")
	flagSet.BoolVarP(&sortBySize, "sort-size", "S", false, "sort by file size, largest first")
	flagSet.BoolVarP(&classifyFiles, "classify", "F", false, "append indicator (one of */=>@) to entries")
	flagSet.BoolVarP(&directoryOnly, "directory", "d", false, "list directories themselves, not their contents")
	flagSet.BoolVarP(&sortByTime, "sort-time", "t", false, "sort by time, newest first")
	flagSet.BoolVarP(&onePerLine, "one-per-line", "1", false, "list one file per line")

	if err := flagSet.Parse(args); err != nil {
		return errors.Join(err, interp.ExitStatus(2))
	}
	_ = sortByTime
	_ = onePerLine

	paths := flagSet.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	showHeader := len(paths) > 1
	exitCode := 0
	var stdoutBuf, stderrBuf strings.Builder

	for i, p := range paths {
		if i > 0 && stdoutBuf.Len() > 0 && !strings.HasSuffix(stdoutBuf.String(), "\n\n") {
			stdoutBuf.WriteString("\n")
		}

		var (
			out  string
			errS string
			code int
		)

		switch {
		case directoryOnly:
			out, errS, code = listDirectoryEntry(ec, p, longFormat, humanReadable, classifyFiles)
		case strings.ContainsAny(p, "*?["):
			out, errS, code = listGlob(ec, p, showAll, showAlmostAll, longFormat, reverse, humanReadable, sortBySize, classifyFiles)
		default:
			out, errS, code = listPath(ctx, ec, p, showAll, showAlmostAll, longFormat, recursive, showHeader, reverse, humanReadable, sortBySize, classifyFiles)
		}

		stdoutBuf.WriteString(out)
		stderrBuf.WriteString(errS)
		if code != 0 {
			exitCode = code
		}
	}

	if ec.Stdout != nil {
		io.WriteString(ec.Stdout, stdoutBuf.String())
	}
	if ec.Stderr != nil {
		io.WriteString(ec.Stderr, stderrBuf.String())
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
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

func formatHumanSize(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10)
	}
	if bytes < 1024*1024 {
		k := float64(bytes) / 1024
		if k < 10 {
			return fmt.Sprintf("%.1fK", k)
		}
		return fmt.Sprintf("%dK", int64(math.Round(k)))
	}
	if bytes < 1024*1024*1024 {
		m := float64(bytes) / (1024 * 1024)
		if m < 10 {
			return fmt.Sprintf("%.1fM", m)
		}
		return fmt.Sprintf("%dM", int64(math.Round(m)))
	}
	g := float64(bytes) / (1024 * 1024 * 1024)
	if g < 10 {
		return fmt.Sprintf("%.1fG", g)
	}
	return fmt.Sprintf("%dG", int64(math.Round(g)))
}

func formatDate(t time.Time) string {
	month := t.Month().String()[:3]
	day := fmt.Sprintf("%2d", t.Day())
	sixMonthsAgo := time.Now().Add(-180 * 24 * time.Hour)
	if t.After(sixMonthsAgo) {
		return fmt.Sprintf("%s %s %02d:%02d", month, day, t.Hour(), t.Minute())
	}
	return fmt.Sprintf("%s %s  %d", month, day, t.Year())
}

func classifySuffix(info fs.FileInfo) string {
	if info.IsDir() {
		return "/"
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return "@"
	}
	if info.Mode()&0o111 != 0 {
		return "*"
	}
	return ""
}

func lstatFS(fsys fs.FS, name string) (fs.FileInfo, error) {
	if r, ok := fsys.(fs.ReadLinkFS); ok {
		return r.Lstat(name)
	}
	return fs.Stat(fsys, name)
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

func longFormatLine(info fs.FileInfo, name, suffix string, humanReadable bool) string {
	mode := "-rw-r--r--"
	if info.IsDir() {
		mode = "drwxr-xr-x"
	}
	var sizeStr string
	if humanReadable {
		sizeStr = formatHumanSize(info.Size())
	} else {
		sizeStr = strconv.FormatInt(info.Size(), 10)
	}
	sizeStr = padLeft(sizeStr, 5)
	return fmt.Sprintf("%s 1 user user %s %s %s%s", mode, sizeStr, formatDate(info.ModTime()), name, suffix)
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func listDirectoryEntry(ec *command.ExecContext, p string, longFormat, humanReadable, classifyFiles bool) (string, string, int) {
	full := resolvePath(ec, p)
	info, err := fs.Stat(ec.FS, full)
	if err != nil {
		return "", fmt.Sprintf("ls: cannot access '%s': No such file or directory\n", p), 2
	}
	if longFormat {
		suffix := ""
		if classifyFiles {
			if li, lerr := lstatFS(ec.FS, full); lerr == nil {
				suffix = classifySuffix(li)
			}
		} else if info.IsDir() {
			suffix = "/"
		}
		return longFormatLine(info, p, suffix, humanReadable) + "\n", "", 0
	}
	suffix := ""
	if classifyFiles {
		if li, lerr := lstatFS(ec.FS, full); lerr == nil {
			suffix = classifySuffix(li)
		}
	}
	return p + suffix + "\n", "", 0
}

func listGlob(ec *command.ExecContext, pattern string, showAll, showAlmostAll, longFormat, reverse, humanReadable, sortBySize, classifyFiles bool) (string, string, int) {
	showHidden := showAll || showAlmostAll
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}

	var fsPattern string
	switch {
	case path.IsAbs(pattern):
		fsPattern = strings.TrimPrefix(pattern, "/")
		if fsPattern == "" {
			fsPattern = "."
		}
	case dir == ".":
		fsPattern = pattern
	default:
		fsPattern = path.Join(dir, pattern)
	}

	matched, err := fs.Glob(ec.FS, fsPattern)
	if err != nil || len(matched) == 0 {
		return "", fmt.Sprintf("ls: %s: No such file or directory\n", pattern), 2
	}

	displayPaths := make([]string, 0, len(matched))
	for _, m := range matched {
		display := m
		if dir != "." && strings.HasPrefix(m, dir+"/") {
			display = strings.TrimPrefix(m, dir+"/")
		}
		basename := path.Base(display)
		if !showHidden && strings.HasPrefix(basename, ".") {
			continue
		}
		displayPaths = append(displayPaths, display)
	}

	if len(displayPaths) == 0 {
		return "", fmt.Sprintf("ls: %s: No such file or directory\n", pattern), 2
	}

	if sortBySize {
		sizes := make(map[string]int64, len(displayPaths))
		for _, m := range displayPaths {
			if info, e := fs.Stat(ec.FS, resolvePath(ec, m)); e == nil {
				sizes[m] = info.Size()
			}
		}
		sort.SliceStable(displayPaths, func(i, j int) bool {
			return sizes[displayPaths[i]] > sizes[displayPaths[j]]
		})
	} else {
		sort.Strings(displayPaths)
	}
	if reverse {
		reverseStrings(displayPaths)
	}

	var out, errOut strings.Builder
	exitCode := 0

	if longFormat {
		for _, m := range displayPaths {
			full := resolvePath(ec, m)
			info, statErr := fs.Stat(ec.FS, full)
			if statErr != nil {
				fmt.Fprintf(&errOut, "ls: cannot access '%s': %v\n", m, statErr)
				exitCode = 2
				continue
			}
			suffix := ""
			if classifyFiles {
				if li, lerr := lstatFS(ec.FS, full); lerr == nil {
					suffix = classifySuffix(li)
				}
			} else if info.IsDir() {
				suffix = "/"
			}
			out.WriteString(longFormatLine(info, m, suffix, humanReadable))
			out.WriteString("\n")
		}
		return out.String(), errOut.String(), exitCode
	}

	if classifyFiles {
		classified := make([]string, len(displayPaths))
		for i, m := range displayPaths {
			full := resolvePath(ec, m)
			suffix := ""
			if li, lerr := lstatFS(ec.FS, full); lerr == nil {
				suffix = classifySuffix(li)
			}
			classified[i] = m + suffix
		}
		return strings.Join(classified, "\n") + "\n", "", 0
	}

	return strings.Join(displayPaths, "\n") + "\n", "", 0
}

func listPath(ctx context.Context, ec *command.ExecContext, p string, showAll, showAlmostAll, longFormat, recursive, showHeader, reverse, humanReadable, sortBySize, classifyFiles bool) (string, string, int) {
	showHidden := showAll || showAlmostAll
	full := resolvePath(ec, p)

	info, err := fs.Stat(ec.FS, full)
	if err != nil {
		return "", fmt.Sprintf("ls: %s: No such file or directory\n", p), 2
	}

	if !info.IsDir() {
		suffix := ""
		if classifyFiles {
			if li, lerr := lstatFS(ec.FS, full); lerr == nil {
				suffix = classifySuffix(li)
			}
		}
		if longFormat {
			return longFormatLine(info, p, suffix, humanReadable) + "\n", "", 0
		}
		return p + suffix + "\n", "", 0
	}

	dirEntries, err := fs.ReadDir(ec.FS, full)
	if err != nil {
		return "", fmt.Sprintf("ls: %s: %v\n", p, err), 2
	}

	names := make([]string, 0, len(dirEntries))
	entryByName := make(map[string]fs.DirEntry, len(dirEntries))
	for _, e := range dirEntries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
		entryByName[name] = e
	}

	if sortBySize {
		sizes := make(map[string]int64, len(names))
		for _, name := range names {
			if einfo, e := fs.Stat(ec.FS, path.Join(full, name)); e == nil {
				sizes[name] = einfo.Size()
			}
		}
		sort.SliceStable(names, func(i, j int) bool {
			return sizes[names[i]] > sizes[names[j]]
		})
	} else {
		sort.Strings(names)
	}

	if showAll {
		names = append([]string{".", ".."}, names...)
	}

	if reverse {
		reverseStrings(names)
	}

	var out, errOut strings.Builder
	exitCode := 0

	if recursive || showHeader {
		out.WriteString(p)
		out.WriteString(":\n")
	}

	switch {
	case longFormat:
		fmt.Fprintf(&out, "total %d\n", len(names))
		for _, name := range names {
			var entryPath string
			switch name {
			case ".":
				entryPath = full
			case "..":
				entryPath = path.Dir(full)
				if entryPath == "" {
					entryPath = "."
				}
			default:
				entryPath = path.Join(full, name)
			}
			einfo, errE := fs.Stat(ec.FS, entryPath)
			if errE != nil {
				fmt.Fprintf(&errOut, "ls: cannot access '%s': %v\n", name, errE)
				exitCode = 2
				continue
			}
			suffix := ""
			if classifyFiles {
				if li, lerr := lstatFS(ec.FS, entryPath); lerr == nil {
					suffix = classifySuffix(li)
				}
			} else if einfo.IsDir() {
				suffix = "/"
			}
			out.WriteString(longFormatLine(einfo, name, suffix, humanReadable))
			out.WriteString("\n")
		}

	case classifyFiles:
		classified := make([]string, 0, len(names))
		for _, name := range names {
			if name == "." || name == ".." {
				classified = append(classified, name+"/")
				continue
			}
			entryPath := path.Join(full, name)
			suffix := ""
			if li, lerr := lstatFS(ec.FS, entryPath); lerr == nil {
				suffix = classifySuffix(li)
			}
			classified = append(classified, name+suffix)
		}
		out.WriteString(strings.Join(classified, "\n"))
		if len(classified) > 0 {
			out.WriteString("\n")
		}

	default:
		out.WriteString(strings.Join(names, "\n"))
		if len(names) > 0 {
			out.WriteString("\n")
		}
	}

	if recursive {
		var subdirs []string
		for _, name := range names {
			if name == "." || name == ".." {
				continue
			}
			entry, ok := entryByName[name]
			isDir := false
			if ok {
				if entry.IsDir() {
					isDir = true
				} else if entry.Type()&fs.ModeSymlink != 0 {
					if einfo, e := fs.Stat(ec.FS, path.Join(full, name)); e == nil && einfo.IsDir() {
						isDir = true
					}
				}
			}
			if isDir {
				subdirs = append(subdirs, name)
			}
		}
		sort.Strings(subdirs)
		if reverse {
			reverseStrings(subdirs)
		}

		for _, name := range subdirs {
			var subPath string
			if p == "." {
				subPath = "./" + name
			} else {
				subPath = p + "/" + name
			}
			subOut, subErr, subCode := listPath(ctx, ec, subPath, showAll, showAlmostAll, longFormat, recursive, false, reverse, humanReadable, sortBySize, classifyFiles)
			out.WriteString("\n")
			out.WriteString(subOut)
			errOut.WriteString(subErr)
			if subCode != 0 {
				exitCode = subCode
			}
		}
	}

	return out.String(), errOut.String(), exitCode
}
