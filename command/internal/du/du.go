package du

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type duOptions struct {
	allFiles      bool
	humanReadable bool
	summarize     bool
	grandTotal    bool
	kilobytes     bool
	bytes         bool
	apparentSize  bool
	maxDepth      int
	maxDepthSet   bool
	followCmdLine bool
	followAll     bool
	oneFileSystem bool
	blockSize     int64
}

const maxRecursionDepth = 1000

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("du: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("du: ExecContext has no filesystem")
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
	set.SetProgram("du")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: du [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Estimate file space usage.\n\n")
		fmt.Fprint(stderr, "  -a               write counts for all files, not just directories\n")
		fmt.Fprint(stderr, "  -b, --bytes      equivalent to --apparent-size --block-size=1\n")
		fmt.Fprint(stderr, "      --apparent-size  print apparent sizes rather than disk usage\n")
		fmt.Fprint(stderr, "  -h               print sizes in human readable format\n")
		fmt.Fprint(stderr, "  -k               like --block-size=1K\n")
		fmt.Fprint(stderr, "  -s               display only a total for each argument\n")
		fmt.Fprint(stderr, "  -c               produce a grand total\n")
		fmt.Fprint(stderr, "  -H               follow symbolic links on the command line only\n")
		fmt.Fprint(stderr, "  -L               follow all symbolic links\n")
		fmt.Fprint(stderr, "  -x, --one-file-system  skip directories on different file systems\n")
		fmt.Fprint(stderr, "      --max-depth=N  print total for directory only if N or fewer levels deep\n")
		fmt.Fprint(stderr, "      --help       display this help and exit\n")
	}
	set.SetUsage(usage)

	allFiles := set.Bool('a', "write counts for all files, not just directories")
	bytesFlag := set.Bool('b', "equivalent to --apparent-size --block-size=1")
	bytesLong := set.BoolLong("bytes", 0, "equivalent to --apparent-size --block-size=1")
	apparentSize := set.BoolLong("apparent-size", 0, "print apparent sizes rather than disk usage")
	humanReadable := set.Bool('h', "print sizes in human readable format")
	kilobytes := set.Bool('k', "like --block-size=1K")
	summarize := set.Bool('s', "display only a total for each argument")
	grandTotal := set.Bool('c', "produce a grand total")
	followCmdLine := set.Bool('H', "follow symbolic links on the command line only")
	followAll := set.Bool('L', "follow all symbolic links")
	oneFileSystem := set.Bool('x', "skip directories on different file systems")
	oneFileSystemLong := set.BoolLong("one-file-system", 0, "skip directories on different file systems")
	maxDepth := set.IntLong("max-depth", 0, 0, "print total for directory only if N or fewer levels deep")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"du"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "du: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	bytesEffective := *bytesFlag || *bytesLong
	opts := duOptions{
		allFiles:      *allFiles,
		humanReadable: *humanReadable,
		summarize:     *summarize,
		grandTotal:    *grandTotal,
		kilobytes:     *kilobytes,
		bytes:         bytesEffective,
		apparentSize:  *apparentSize || bytesEffective,
		followCmdLine: *followCmdLine,
		followAll:     *followAll,
		oneFileSystem: *oneFileSystem || *oneFileSystemLong,
		blockSize:     resolveBlockSize(ec, *kilobytes, bytesEffective),
	}
	if set.Lookup("max-depth").Seen() {
		opts.maxDepth = *maxDepth
		opts.maxDepthSet = true
	}

	targets := set.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	var stdoutBuf, stderrBuf strings.Builder
	var grand int64

	for _, target := range targets {
		full := resolvePath(ec, target)
		// For top-level targets: -L or -H always follow the link given on
		// the command line. Default behavior counts the link itself, but
		// since billy's memfs Stat-on-symlink-to-dir still descends, the
		// distinction mainly matters for -H/-L semantics in real FS use.
		followTop := opts.followAll || opts.followCmdLine
		if _, err := statEntry(ec.FS, full, followTop); err != nil {
			fmt.Fprintf(&stderrBuf, "du: cannot access '%s': No such file or directory\n", target)
			continue
		}
		out, total, errOut := calculateSize(ec, full, target, opts, 0, true)
		stdoutBuf.WriteString(out)
		stderrBuf.WriteString(errOut)
		grand += total
	}

	if opts.grandTotal && len(targets) > 0 {
		fmt.Fprintf(&stdoutBuf, "%s\ttotal\n", formatSize(grand, opts.humanReadable, opts.blockSize))
	}

	io.WriteString(stdout, stdoutBuf.String())
	io.WriteString(stderr, stderrBuf.String())

	if stderrBuf.Len() > 0 {
		return interp.ExitStatus(1)
	}
	return nil
}

func calculateSize(ec *command.ExecContext, fullPath, displayPath string, opts duOptions, depth int, isCmdLine bool) (string, int64, string) {
	if depth > maxRecursionDepth {
		return "", 0, ""
	}

	// Decide whether to follow this entry if it is a symlink.
	// -L: follow always. -H: follow only at the command line. Default: don't follow.
	follow := opts.followAll || (opts.followCmdLine && isCmdLine)
	info, err := statEntry(ec.FS, fullPath, follow)
	if err != nil {
		return "", 0, fmt.Sprintf("du: cannot read directory '%s': Permission denied\n", displayPath)
	}

	// If it's a symlink we are not following, count just the link itself.
	if info.Mode()&fs.ModeSymlink != 0 && !follow {
		size := info.Size()
		if opts.allFiles || depth == 0 {
			return formatSize(size, opts.humanReadable, opts.blockSize) + "\t" + displayPath + "\n", size, ""
		}
		return "", size, ""
	}

	if !info.IsDir() {
		size := info.Size()
		if opts.allFiles || depth == 0 {
			return formatSize(size, opts.humanReadable, opts.blockSize) + "\t" + displayPath + "\n", size, ""
		}
		return "", size, ""
	}

	entries, err := ec.FS.ReadDir(fullPath)
	if err != nil {
		return "", 0, fmt.Sprintf("du: cannot read directory '%s': Permission denied\n", displayPath)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var out, errOut strings.Builder
	var dirSize int64

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// For symlinks encountered during traversal: under -L we should
		// follow them; under default/-H we count the link's own size.
		entryPath := path.Join(fullPath, e.Name())
		var entrySize int64
		if info, ierr := e.Info(); ierr == nil {
			entrySize = info.Size()
		}
		if e.Type()&fs.ModeSymlink != 0 && opts.followAll {
			if linked, err := ec.FS.Stat(entryPath); err == nil {
				if linked.IsDir() {
					// Recurse into the linked directory.
					subOut, subTotal, subErr := calculateSize(ec, entryPath, joinDisplay(displayPath, e.Name()), opts, depth+1, false)
					dirSize += subTotal
					errOut.WriteString(subErr)
					if !opts.summarize && (!opts.maxDepthSet || depth+1 <= opts.maxDepth) {
						out.WriteString(subOut)
					}
					continue
				}
				entrySize = linked.Size()
			}
		}
		dirSize += entrySize
		if opts.allFiles && !opts.summarize {
			fmt.Fprintf(&out, "%s\t%s\n", formatSize(entrySize, opts.humanReadable, opts.blockSize), joinDisplay(displayPath, e.Name()))
		}
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entryPath := path.Join(fullPath, e.Name())
		entryDisplay := joinDisplay(displayPath, e.Name())
		// -x (one-file-system) is documented as a best-effort no-op: billy
		// abstractions don't expose device IDs, so we cannot detect a
		// filesystem boundary. Accept the flag and continue traversal.
		subOut, subTotal, subErr := calculateSize(ec, entryPath, entryDisplay, opts, depth+1, false)
		dirSize += subTotal
		errOut.WriteString(subErr)
		if !opts.summarize && (!opts.maxDepthSet || depth+1 <= opts.maxDepth) {
			out.WriteString(subOut)
		}
	}

	if opts.summarize || !opts.maxDepthSet || depth <= opts.maxDepth {
		fmt.Fprintf(&out, "%s\t%s\n", formatSize(dirSize, opts.humanReadable, opts.blockSize), displayPath)
	}

	return out.String(), dirSize, errOut.String()
}

// statEntry returns FileInfo for fullPath. When follow is true it uses Stat
// (resolving symlinks). When follow is false it tries Lstat via the
// billy.Symlink capability; if the underlying filesystem doesn't expose
// Lstat, it falls back to Stat. This means on filesystems without Lstat
// support, default/-H behavior degrades to -L behavior (symlinks always
// followed). That limitation is inherent to the billy abstraction.
func statEntry(fsys billy.Filesystem, fullPath string, follow bool) (fs.FileInfo, error) {
	if follow {
		return fsys.Stat(fullPath)
	}
	if sl, ok := fsys.(billy.Symlink); ok {
		return sl.Lstat(fullPath)
	}
	return fsys.Stat(fullPath)
}

func joinDisplay(displayPath, name string) string {
	if displayPath == "." {
		return name
	}
	return displayPath + "/" + name
}

func formatSize(bytes int64, humanReadable bool, blockSize int64) string {
	if blockSize <= 0 {
		blockSize = 1024
	}
	if !humanReadable {
		v := (bytes + blockSize - 1) / blockSize
		if v <= 0 {
			v = 1
		}
		return strconv.FormatInt(v, 10)
	}
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1fG", float64(bytes)/(1024*1024*1024))
}

func resolveBlockSize(ec *command.ExecContext, kilobytes, bytes bool) int64 {
	if bytes {
		return 1
	}
	if kilobytes {
		return 1024
	}
	if ec != nil && ec.Environ != nil {
		if v := ec.Environ.Get("POSIXLY_CORRECT"); v.IsSet() && v.String() != "" {
			return 512
		}
		if v := ec.Environ.Get("BLOCKSIZE"); v.IsSet() {
			if n, err := strconv.ParseInt(v.String(), 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	return 1024
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
