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

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/util"
	"github.com/pborman/getopt/v2"
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

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("ls")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: ls [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "list directory contents\n\n")
		fmt.Fprint(stderr, "  -a, --all            do not ignore entries starting with .\n")
		fmt.Fprint(stderr, "  -A, --almost-all     do not list . and ..\n")
		fmt.Fprint(stderr, "  -c                   sort by ctime (best-effort; falls back to mtime)\n")
		fmt.Fprint(stderr, "  -d, --directory      list directories themselves, not their contents\n")
		fmt.Fprint(stderr, "  -f                   do not sort, enable -a, disable -l\n")
		fmt.Fprint(stderr, "  -F, --classify       append indicator (one of */=>@) to entries\n")
		fmt.Fprint(stderr, "  -h, --human-readable with -l, print sizes like 1K 234M 2G etc.\n")
		fmt.Fprint(stderr, "  -i, --inode          print the index number of each file (always '?' here)\n")
		fmt.Fprint(stderr, "  -k                   with -s, use 1024-byte blocks\n")
		fmt.Fprint(stderr, "  -l                   use a long listing format\n")
		fmt.Fprint(stderr, "  -m                   fill width with a comma separated list of entries\n")
		fmt.Fprint(stderr, "  -n, --numeric-uid-gid like -l but list numeric user and group IDs\n")
		fmt.Fprint(stderr, "  -o                   like -l, but do not list group information\n")
		fmt.Fprint(stderr, "  -p                   append / indicator to directories\n")
		fmt.Fprint(stderr, "  -q, --hide-control-chars print ? instead of nongraphic characters\n")
		fmt.Fprint(stderr, "  -r, --reverse        reverse order while sorting\n")
		fmt.Fprint(stderr, "  -R, --recursive      list subdirectories recursively\n")
		fmt.Fprint(stderr, "  -s, --size           print the allocated size of each file, in blocks\n")
		fmt.Fprint(stderr, "  -S                   sort by file size, largest first\n")
		fmt.Fprint(stderr, "  -t                   sort by time, newest first\n")
		fmt.Fprint(stderr, "  -u                   sort by atime (best-effort; falls back to mtime)\n")
		fmt.Fprint(stderr, "  -1                   list one file per line\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	showAll := set.BoolLong("all", 'a', "do not ignore entries starting with .")
	showAlmostAll := set.BoolLong("almost-all", 'A', "do not list . and ..")
	useCtime := set.Bool('c', "sort by ctime (best-effort; falls back to mtime)")
	directoryOnly := set.BoolLong("directory", 'd', "list directories themselves, not their contents")
	noSort := set.Bool('f', "do not sort, enable -a, disable -l")
	classifyFiles := set.BoolLong("classify", 'F', "append indicator (one of */=>@) to entries")
	humanReadable := set.BoolLong("human-readable", 'h', "with -l, print sizes like 1K 234M 2G etc.")
	showInode := set.BoolLong("inode", 'i', "print the index number of each file")
	kBytes := set.Bool('k', "with -s, use 1024-byte blocks")
	longFormat := set.Bool('l', "use a long listing format")
	commaList := set.Bool('m', "fill width with a comma separated list of entries")
	numericIDs := set.BoolLong("numeric-uid-gid", 'n', "like -l but list numeric user and group IDs")
	longNoGroup := set.Bool('o', "like -l, but do not list group information")
	slashDirs := set.Bool('p', "append / indicator to directories")
	hideControl := set.BoolLong("hide-control-chars", 'q', "print ? instead of nongraphic characters")
	reverse := set.BoolLong("reverse", 'r', "reverse order while sorting")
	recursive := set.BoolLong("recursive", 'R', "list subdirectories recursively")
	showBlocks := set.BoolLong("size", 's', "print the allocated size of each file, in blocks")
	sortBySize := set.Bool('S', "sort by file size, largest first")
	sortByTime := set.Bool('t', "sort by time, newest first")
	useAtime := set.Bool('u', "sort by atime (best-effort; falls back to mtime)")
	onePerLine := set.Bool('1', "list one file per line")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"ls"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "ls: %s\n", err)
		usage()
		return interp.ExitStatus(2)
	}
	if *help {
		usage()
		return nil
	}

	// -n, -o imply -l (long format).
	effLong := *longFormat || *numericIDs || *longNoGroup
	// -m disables long format.
	if *commaList {
		effLong = false
	}
	// -1 disables -m.
	if *onePerLine {
		*commaList = false
	}
	// -f: disable sorting, force -a, suppress long format. Per GNU ls,
	// -f also implies --color=none and disables -l/-s indirectly via the
	// "do not access file metadata" rule, but kefka is non-tty and we
	// already approximate metadata, so the visible effect here is the
	// sort/-a override.
	effShowAll := *showAll
	effSortByTime := *sortByTime
	effSortBySize := *sortBySize
	if *noSort {
		effShowAll = true
		effLong = false
		effSortByTime = false
		effSortBySize = false
	}

	opts := lsOptions{
		showAll:       effShowAll,
		showAlmostAll: *showAlmostAll,
		directoryOnly: *directoryOnly,
		classifyFiles: *classifyFiles,
		humanReadable: *humanReadable,
		showInode:     *showInode,
		kBytes:        *kBytes,
		longFormat:    effLong,
		commaList:     *commaList,
		numericIDs:    *numericIDs,
		longNoGroup:   *longNoGroup,
		slashDirs:     *slashDirs,
		hideControl:   *hideControl,
		reverse:       *reverse,
		recursive:     *recursive,
		showBlocks:    *showBlocks,
		sortBySize:    effSortBySize,
		sortByTime:    effSortByTime,
		noSort:        *noSort,
		useCtime:      *useCtime,
		useAtime:      *useAtime,
	}
	// -1 is the default for non-tty output, which kefka always is. The
	// flag is accepted for compatibility but has no observable effect on
	// the default per-line output beyond overriding -m above.
	_ = onePerLine

	paths := set.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	opts.showHeader = len(paths) > 1
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
		case opts.directoryOnly:
			out, errS, code = listDirectoryEntry(ec, p, opts)
		case strings.ContainsAny(p, "*?["):
			out, errS, code = listGlob(ec, p, opts)
		default:
			out, errS, code = listPath(ctx, ec, p, opts)
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

func realFormatDate(t time.Time) string {
	month := t.Month().String()[:3]
	day := fmt.Sprintf("%2d", t.Day())
	sixMonthsAgo := time.Now().Add(-180 * 24 * time.Hour)
	if t.After(sixMonthsAgo) {
		return fmt.Sprintf("%s %s %02d:%02d", month, day, t.Hour(), t.Minute())
	}
	return fmt.Sprintf("%s %s  %d", month, day, t.Year())
}

var formatDate = realFormatDate

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

func lstatFS(fsys billy.Filesystem, name string) (fs.FileInfo, error) {
	if r, ok := fsys.(billy.Symlink); ok {
		return r.Lstat(name)
	}
	return fsys.Stat(name)
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

// lsOptions bundles the flag values that downstream helpers need so the
// signatures don't grow unbounded as we add more flags.
type lsOptions struct {
	showAll       bool
	showAlmostAll bool
	directoryOnly bool
	classifyFiles bool
	humanReadable bool
	showInode     bool
	kBytes        bool
	longFormat    bool
	commaList     bool
	numericIDs    bool
	longNoGroup   bool
	slashDirs     bool
	hideControl   bool
	reverse       bool
	recursive     bool
	showBlocks    bool
	sortBySize    bool
	sortByTime    bool
	noSort        bool
	useCtime      bool
	useAtime      bool
	showHeader    bool
}

// blockSize returns the unit, in bytes, used for the -s block column and
// the "total N" line. POSIX defaults to 512; -k forces 1024.
func (o lsOptions) blockSize() int64 {
	if o.kBytes {
		return 1024
	}
	return 512
}

// blocksFor reports the number of blockSize-byte blocks consumed by a file
// of the given byte size, rounded up. Real ls uses st_blocks, but billy
// doesn't expose that, so we approximate from logical size.
func (o lsOptions) blocksFor(size int64) int64 {
	bs := o.blockSize()
	return (size + bs - 1) / bs
}

// hideControlChars replaces non-printable bytes (and tab) in name with '?',
// per ls -q. We operate on bytes since billy paths are byte strings.
func hideControlChars(name string) string {
	out := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		// Treat anything outside printable ASCII (and tab) as non-printable.
		// This is intentionally conservative: utf-8 multibyte chars get '?'
		// per byte. Real GNU ls does locale-aware printable detection, which
		// kefka does not have access to.
		if c < 0x20 || c == 0x7f {
			out[i] = '?'
		} else {
			out[i] = c
		}
	}
	return string(out)
}

// nlinkOf returns the link count for an fs.FileInfo if the underlying type
// implements Sys() with a *syscall.Stat_t (real OS); otherwise 1.
func nlinkOf(info fs.FileInfo) uint64 {
	if info == nil {
		return 1
	}
	if st, ok := info.Sys().(interface{ Nlink() uint64 }); ok {
		return st.Nlink()
	}
	return 1
}

// timeFor picks the timestamp used for sorting and long-format display
// based on -c (ctime), -u (atime), or default (mtime). billy.FileInfo only
// exposes ModTime(), so -c and -u silently fall back to mtime. This is
// documented in the help text and in the audit notes.
func timeFor(info fs.FileInfo, opts lsOptions) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}

func longFormatLine(info fs.FileInfo, name, suffix string, opts lsOptions) string {
	mode := formatMode(info.Mode())
	var sizeStr string
	if opts.humanReadable {
		sizeStr = formatHumanSize(info.Size())
	} else {
		sizeStr = strconv.FormatInt(info.Size(), 10)
	}
	sizeStr = padLeft(sizeStr, 5)

	var ownerField, groupField string
	if opts.numericIDs {
		// billy doesn't expose UID/GID; fall back to 0.
		ownerField = "0"
		groupField = "0"
	} else {
		ownerField = "user"
		// Match historical kefka output: same as owner. Real ls would
		// resolve via getgrgid; billy doesn't expose gid.
		groupField = "user"
	}

	displayName := name
	if opts.hideControl {
		displayName = hideControlChars(displayName)
	}

	nlink := nlinkOf(info)
	t := timeFor(info, opts)

	if opts.longNoGroup {
		// -o: omit group column.
		return fmt.Sprintf("%s %d %s %s %s %s%s", mode, nlink, ownerField, sizeStr, formatDate(t), displayName, suffix)
	}
	return fmt.Sprintf("%s %d %s %s %s %s %s%s", mode, nlink, ownerField, groupField, sizeStr, formatDate(t), displayName, suffix)
}

// inodePrefix returns the inode column for -i. billy doesn't expose inodes,
// so we use '?' to be honest about that. Width 1, single space separator.
func inodePrefix(opts lsOptions) string {
	if !opts.showInode {
		return ""
	}
	return "? "
}

// blockPrefix returns the -s block-count column. billy doesn't track
// st_blocks, so we approximate from logical size, rounded up to blockSize.
func blockPrefix(info fs.FileInfo, opts lsOptions) string {
	if !opts.showBlocks || info == nil {
		return ""
	}
	return strconv.FormatInt(opts.blocksFor(info.Size()), 10) + " "
}

// totalBlocks sums the rounded block counts for the given paths.
func totalBlocks(ec *command.ExecContext, dir string, names []string, opts lsOptions) int64 {
	var total int64
	for _, name := range names {
		var p string
		switch name {
		case ".":
			p = dir
		case "..":
			p = path.Dir(dir)
			if p == "" {
				p = "."
			}
		default:
			p = path.Join(dir, name)
		}
		info, err := ec.FS.Stat(p)
		if err != nil {
			continue
		}
		total += opts.blocksFor(info.Size())
	}
	return total
}

// shortSuffix computes the trailing indicator for a non-long entry given the
// active flags. -F wins over -p; -p only adds slashes for directories.
func shortSuffix(info fs.FileInfo, opts lsOptions) string {
	if info == nil {
		return ""
	}
	if opts.classifyFiles {
		return classifySuffix(info)
	}
	if opts.slashDirs && info.IsDir() {
		return "/"
	}
	return ""
}

// formatNameForOutput applies -q hiding and -F/-p suffixing to a bare entry
// name, producing the string ls should write for non-long modes.
func formatNameForOutput(name string, info fs.FileInfo, opts lsOptions) string {
	display := name
	if opts.hideControl {
		display = hideControlChars(display)
	}
	return display + shortSuffix(info, opts)
}

// formatMode renders an os.FileMode as a 10-char string like "drwxrwxrwt",
// matching GNU ls 9.x: type prefix + perms with suid/sgid/sticky overlays.
func formatMode(m fs.FileMode) string {
	b := []byte("----------")
	switch {
	case m&fs.ModeDir != 0:
		b[0] = 'd'
	case m&fs.ModeSymlink != 0:
		b[0] = 'l'
	case m&fs.ModeNamedPipe != 0:
		b[0] = 'p'
	case m&fs.ModeSocket != 0:
		b[0] = 's'
	case m&fs.ModeDevice != 0:
		if m&fs.ModeCharDevice != 0 {
			b[0] = 'c'
		} else {
			b[0] = 'b'
		}
	case m&fs.ModeCharDevice != 0:
		b[0] = 'c'
	default:
		b[0] = '-'
	}

	perm := m.Perm()
	const rwx = "rwxrwxrwx"
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			b[i+1] = rwx[i]
		}
	}

	if m&fs.ModeSetuid != 0 {
		if b[3] == 'x' {
			b[3] = 's'
		} else {
			b[3] = 'S'
		}
	}
	if m&fs.ModeSetgid != 0 {
		if b[6] == 'x' {
			b[6] = 's'
		} else {
			b[6] = 'S'
		}
	}
	if m&fs.ModeSticky != 0 {
		if b[9] == 'x' {
			b[9] = 't'
		} else {
			b[9] = 'T'
		}
	}
	return string(b)
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func listDirectoryEntry(ec *command.ExecContext, p string, opts lsOptions) (string, string, int) {
	full := resolvePath(ec, p)
	info, err := ec.FS.Stat(full)
	if err != nil {
		return "", fmt.Sprintf("ls: cannot access '%s': No such file or directory\n", p), 2
	}
	li, _ := lstatFS(ec.FS, full)
	if li == nil {
		li = info
	}

	if opts.longFormat {
		suffix := ""
		if opts.classifyFiles {
			suffix = classifySuffix(li)
		} else if info.IsDir() {
			suffix = "/"
		}
		var prefix strings.Builder
		prefix.WriteString(inodePrefix(opts))
		prefix.WriteString(blockPrefix(info, opts))
		return prefix.String() + longFormatLine(info, p, suffix, opts) + "\n", "", 0
	}

	displayName := formatNameForOutput(p, li, opts)
	var line strings.Builder
	line.WriteString(inodePrefix(opts))
	line.WriteString(blockPrefix(info, opts))
	line.WriteString(displayName)
	line.WriteString("\n")
	return line.String(), "", 0
}

func listGlob(ec *command.ExecContext, pattern string, opts lsOptions) (string, string, int) {
	showHidden := opts.showAll || opts.showAlmostAll
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

	matched, err := util.Glob(ec.FS, fsPattern)
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

	switch {
	case opts.noSort:
		// -f: leave glob order as returned by the matcher.
	case opts.sortByTime:
		mtimes := make(map[string]time.Time, len(displayPaths))
		for _, m := range displayPaths {
			if info, e := ec.FS.Stat(resolvePath(ec, m)); e == nil {
				mtimes[m] = info.ModTime()
			}
		}
		sort.SliceStable(displayPaths, func(i, j int) bool {
			return mtimes[displayPaths[i]].After(mtimes[displayPaths[j]])
		})
	case opts.sortBySize:
		sizes := make(map[string]int64, len(displayPaths))
		for _, m := range displayPaths {
			if info, e := ec.FS.Stat(resolvePath(ec, m)); e == nil {
				sizes[m] = info.Size()
			}
		}
		sort.SliceStable(displayPaths, func(i, j int) bool {
			return sizes[displayPaths[i]] > sizes[displayPaths[j]]
		})
	default:
		sort.Strings(displayPaths)
	}
	if opts.reverse {
		reverseStrings(displayPaths)
	}

	var out, errOut strings.Builder
	exitCode := 0

	if opts.longFormat {
		for _, m := range displayPaths {
			full := resolvePath(ec, m)
			info, statErr := ec.FS.Stat(full)
			if statErr != nil {
				fmt.Fprintf(&errOut, "ls: cannot access '%s': %v\n", m, statErr)
				exitCode = 2
				continue
			}
			suffix := ""
			if opts.classifyFiles {
				if li, lerr := lstatFS(ec.FS, full); lerr == nil {
					suffix = classifySuffix(li)
				}
			} else if opts.slashDirs && info.IsDir() {
				suffix = "/"
			} else if info.IsDir() {
				suffix = "/"
			}
			out.WriteString(inodePrefix(opts))
			out.WriteString(blockPrefix(info, opts))
			out.WriteString(longFormatLine(info, m, suffix, opts))
			out.WriteString("\n")
		}
		return out.String(), errOut.String(), exitCode
	}

	// Build display lines: each entry gets optional inode/block prefix and
	// classify/slash suffix.
	formatted := make([]string, 0, len(displayPaths))
	for _, m := range displayPaths {
		full := resolvePath(ec, m)
		info, _ := ec.FS.Stat(full)
		li, _ := lstatFS(ec.FS, full)
		if li == nil {
			li = info
		}
		var line strings.Builder
		line.WriteString(inodePrefix(opts))
		line.WriteString(blockPrefix(info, opts))
		line.WriteString(formatNameForOutput(m, li, opts))
		formatted = append(formatted, line.String())
	}

	if opts.commaList {
		return strings.Join(formatted, ", ") + "\n", "", 0
	}
	return strings.Join(formatted, "\n") + "\n", "", 0
}

func listPath(ctx context.Context, ec *command.ExecContext, p string, opts lsOptions) (string, string, int) {
	showHidden := opts.showAll || opts.showAlmostAll
	full := resolvePath(ec, p)

	info, err := ec.FS.Stat(full)
	if err != nil {
		return "", fmt.Sprintf("ls: %s: No such file or directory\n", p), 2
	}

	if !info.IsDir() {
		li, _ := lstatFS(ec.FS, full)
		if li == nil {
			li = info
		}
		if opts.longFormat {
			suffix := ""
			if opts.classifyFiles {
				suffix = classifySuffix(li)
			}
			var prefix strings.Builder
			prefix.WriteString(inodePrefix(opts))
			prefix.WriteString(blockPrefix(info, opts))
			return prefix.String() + longFormatLine(info, p, suffix, opts) + "\n", "", 0
		}
		var line strings.Builder
		line.WriteString(inodePrefix(opts))
		line.WriteString(blockPrefix(info, opts))
		line.WriteString(formatNameForOutput(p, li, opts))
		line.WriteString("\n")
		return line.String(), "", 0
	}

	dirEntries, err := ec.FS.ReadDir(full)
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

	switch {
	case opts.noSort:
		// -f: leave directory order as returned by the filesystem.
	case opts.sortByTime:
		mtimes := make(map[string]time.Time, len(names))
		for _, name := range names {
			if einfo, e := ec.FS.Stat(path.Join(full, name)); e == nil {
				mtimes[name] = einfo.ModTime()
			}
		}
		sort.SliceStable(names, func(i, j int) bool {
			return mtimes[names[i]].After(mtimes[names[j]])
		})
	case opts.sortBySize:
		sizes := make(map[string]int64, len(names))
		for _, name := range names {
			if einfo, e := ec.FS.Stat(path.Join(full, name)); e == nil {
				sizes[name] = einfo.Size()
			}
		}
		sort.SliceStable(names, func(i, j int) bool {
			return sizes[names[i]] > sizes[names[j]]
		})
	default:
		sort.Strings(names)
	}

	if opts.showAll {
		names = append([]string{".", ".."}, names...)
	}

	if opts.reverse {
		reverseStrings(names)
	}

	var out, errOut strings.Builder
	exitCode := 0

	if opts.recursive || opts.showHeader {
		out.WriteString(p)
		out.WriteString(":\n")
	}

	switch {
	case opts.longFormat:
		// Long format prints "total N" header where N is the sum of blocks
		// for the listed entries, in 512-byte (or 1024 with -k) units.
		total := totalBlocks(ec, full, names, opts)
		fmt.Fprintf(&out, "total %d\n", total)
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
			einfo, errE := ec.FS.Stat(entryPath)
			if errE != nil {
				fmt.Fprintf(&errOut, "ls: cannot access '%s': %v\n", name, errE)
				exitCode = 2
				continue
			}
			suffix := ""
			if opts.classifyFiles {
				if li, lerr := lstatFS(ec.FS, entryPath); lerr == nil {
					suffix = classifySuffix(li)
				}
			} else if einfo.IsDir() {
				suffix = "/"
			}
			out.WriteString(inodePrefix(opts))
			out.WriteString(blockPrefix(einfo, opts))
			out.WriteString(longFormatLine(einfo, name, suffix, opts))
			out.WriteString("\n")
		}

	default:
		formatted := make([]string, 0, len(names))
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
			einfo, _ := ec.FS.Stat(entryPath)
			li, _ := lstatFS(ec.FS, entryPath)
			if li == nil {
				li = einfo
			}
			// "." and ".." are always directories.
			if name == "." || name == ".." {
				display := name
				if opts.hideControl {
					display = hideControlChars(display)
				}
				suffix := ""
				if opts.classifyFiles || opts.slashDirs {
					suffix = "/"
				}
				var line strings.Builder
				line.WriteString(inodePrefix(opts))
				line.WriteString(blockPrefix(einfo, opts))
				line.WriteString(display)
				line.WriteString(suffix)
				formatted = append(formatted, line.String())
				continue
			}
			var line strings.Builder
			line.WriteString(inodePrefix(opts))
			line.WriteString(blockPrefix(einfo, opts))
			line.WriteString(formatNameForOutput(name, li, opts))
			formatted = append(formatted, line.String())
		}

		separator := "\n"
		if opts.commaList {
			separator = ", "
		}
		out.WriteString(strings.Join(formatted, separator))
		if len(formatted) > 0 {
			out.WriteString("\n")
		}
	}

	if opts.recursive {
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
					if einfo, e := ec.FS.Stat(path.Join(full, name)); e == nil && einfo.IsDir() {
						isDir = true
					}
				}
			}
			if isDir {
				subdirs = append(subdirs, name)
			}
		}
		sort.Strings(subdirs)
		if opts.reverse {
			reverseStrings(subdirs)
		}

		subOpts := opts
		subOpts.showHeader = false
		for _, name := range subdirs {
			var subPath string
			if p == "." {
				subPath = "./" + name
			} else {
				subPath = p + "/" + name
			}
			subOut, subErr, subCode := listPath(ctx, ec, subPath, subOpts)
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
