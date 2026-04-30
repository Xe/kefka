package du

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

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
	maxDepth      int
	maxDepthSet   bool
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
		fmt.Fprint(stderr, "  -h               print sizes in human readable format\n")
		fmt.Fprint(stderr, "  -s               display only a total for each argument\n")
		fmt.Fprint(stderr, "  -c               produce a grand total\n")
		fmt.Fprint(stderr, "      --max-depth=N  print total for directory only if N or fewer levels deep\n")
		fmt.Fprint(stderr, "      --help       display this help and exit\n")
	}
	set.SetUsage(usage)

	allFiles := set.Bool('a', "write counts for all files, not just directories")
	humanReadable := set.Bool('h', "print sizes in human readable format")
	summarize := set.Bool('s', "display only a total for each argument")
	grandTotal := set.Bool('c', "produce a grand total")
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

	opts := duOptions{
		allFiles:      *allFiles,
		humanReadable: *humanReadable,
		summarize:     *summarize,
		grandTotal:    *grandTotal,
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
		if _, err := ec.FS.Stat(full); err != nil {
			fmt.Fprintf(&stderrBuf, "du: cannot access '%s': No such file or directory\n", target)
			continue
		}
		out, total, errOut := calculateSize(ec, full, target, opts, 0)
		stdoutBuf.WriteString(out)
		stderrBuf.WriteString(errOut)
		grand += total
	}

	if opts.grandTotal && len(targets) > 0 {
		fmt.Fprintf(&stdoutBuf, "%s\ttotal\n", formatSize(grand, opts.humanReadable))
	}

	io.WriteString(stdout, stdoutBuf.String())
	io.WriteString(stderr, stderrBuf.String())

	if stderrBuf.Len() > 0 {
		return interp.ExitStatus(1)
	}
	return nil
}

func calculateSize(ec *command.ExecContext, fullPath, displayPath string, opts duOptions, depth int) (string, int64, string) {
	if depth > maxRecursionDepth {
		return "", 0, ""
	}

	info, err := ec.FS.Stat(fullPath)
	if err != nil {
		return "", 0, fmt.Sprintf("du: cannot read directory '%s': Permission denied\n", displayPath)
	}

	if !info.IsDir() {
		size := info.Size()
		if opts.allFiles || depth == 0 {
			return formatSize(size, opts.humanReadable) + "\t" + displayPath + "\n", size, ""
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
		size := e.Size()
		dirSize += size
		if opts.allFiles && !opts.summarize {
			fmt.Fprintf(&out, "%s\t%s\n", formatSize(size, opts.humanReadable), joinDisplay(displayPath, e.Name()))
		}
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		entryPath := path.Join(fullPath, e.Name())
		entryDisplay := joinDisplay(displayPath, e.Name())
		subOut, subTotal, subErr := calculateSize(ec, entryPath, entryDisplay, opts, depth+1)
		dirSize += subTotal
		errOut.WriteString(subErr)
		if !opts.summarize && (!opts.maxDepthSet || depth+1 <= opts.maxDepth) {
			out.WriteString(subOut)
		}
	}

	if opts.summarize || !opts.maxDepthSet || depth <= opts.maxDepth {
		fmt.Fprintf(&out, "%s\t%s\n", formatSize(dirSize, opts.humanReadable), displayPath)
	}

	return out.String(), dirSize, errOut.String()
}

func joinDisplay(displayPath, name string) string {
	if displayPath == "." {
		return name
	}
	return displayPath + "/" + name
}

func formatSize(bytes int64, humanReadable bool) string {
	if !humanReadable {
		v := (bytes + 1023) / 1024
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
