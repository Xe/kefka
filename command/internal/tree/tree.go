package tree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

type treeOptions struct {
	showHidden      bool
	directoriesOnly bool
	fullPath        bool
	hasMaxDepth     bool
	maxDepth        int
}

type treeResult struct {
	output    strings.Builder
	stderr    strings.Builder
	dirCount  int
	fileCount int
}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("tree: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("tree: ExecContext has no filesystem")
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
	set.SetProgram("tree")
	set.SetParameters("[DIRECTORY]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: tree [OPTION]... [DIRECTORY]...\n")
		fmt.Fprint(stderr, "List contents of directories in a tree-like format.\n\n")
		fmt.Fprint(stderr, "  -a          include hidden files\n")
		fmt.Fprint(stderr, "  -d          list directories only\n")
		fmt.Fprint(stderr, "  -L LEVEL    limit depth of directory tree\n")
		fmt.Fprint(stderr, "  -f          print full path prefix for each file\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	showHidden := set.Bool('a', "include hidden files")
	directoriesOnly := set.Bool('d', "list directories only")
	fullPath := set.Bool('f', "print full path prefix for each file")
	maxDepthFlag := set.Int('L', 0, "limit depth of directory tree")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"tree"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "tree: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	opts := treeOptions{
		showHidden:      *showHidden,
		directoriesOnly: *directoriesOnly,
		fullPath:        *fullPath,
	}
	if set.Lookup('L').Seen() {
		opts.hasMaxDepth = true
		opts.maxDepth = *maxDepthFlag
	}

	dirs := set.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	var result treeResult
	for _, d := range dirs {
		walkRoot(ec, &opts, &result, d)
	}

	dirWord := "directories"
	if result.dirCount == 1 {
		dirWord = "directory"
	}
	result.output.WriteString("\n")
	fmt.Fprintf(&result.output, "%d %s", result.dirCount, dirWord)
	if !opts.directoriesOnly {
		fileWord := "files"
		if result.fileCount == 1 {
			fileWord = "file"
		}
		fmt.Fprintf(&result.output, ", %d %s", result.fileCount, fileWord)
	}
	result.output.WriteString("\n")

	io.WriteString(stdout, result.output.String())
	io.WriteString(stderr, result.stderr.String())

	if result.stderr.Len() > 0 {
		return interp.ExitStatus(1)
	}
	return nil
}

func walkRoot(ec *command.ExecContext, opts *treeOptions, result *treeResult, displayPath string) {
	fsPath := resolvePath(ec, displayPath)
	info, err := ec.FS.Stat(fsPath)
	if err != nil {
		fmt.Fprintf(&result.stderr, "tree: %s: No such file or directory\n", displayPath)
		return
	}

	result.output.WriteString(displayPath)
	result.output.WriteString("\n")

	if !info.IsDir() {
		result.fileCount++
		return
	}

	walk(ec, opts, result, displayPath, fsPath, "", 0)
}

func walk(ec *command.ExecContext, opts *treeOptions, result *treeResult,
	displayPath, fsPath, prefix string, depth int,
) {
	if opts.hasMaxDepth && depth >= opts.maxDepth {
		return
	}

	entries, err := ec.FS.ReadDir(fsPath)
	if err != nil {
		return
	}

	type entryInfo struct {
		name  string
		isDir bool
	}
	infos := make([]entryInfo, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !opts.showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := e.IsDir()
		if opts.directoriesOnly && !isDir {
			continue
		}
		infos = append(infos, entryInfo{name: name, isDir: isDir})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].name < infos[j].name
	})

	for i, entry := range infos {
		isLast := i == len(infos)-1
		var connector, childSuffix string
		if isLast {
			connector = "`-- "
			childSuffix = "    "
		} else {
			connector = "|-- "
			childSuffix = "|   "
		}

		entryFS := path.Join(fsPath, entry.name)
		entryDisplay := joinDisplay(displayPath, entry.name)

		shown := entry.name
		if opts.fullPath {
			shown = entryDisplay
		}

		result.output.WriteString(prefix)
		result.output.WriteString(connector)
		result.output.WriteString(shown)
		result.output.WriteString("\n")

		if entry.isDir {
			result.dirCount++
			walk(ec, opts, result, entryDisplay, entryFS, prefix+childSuffix, depth+1)
		} else {
			result.fileCount++
		}
	}
}

func joinDisplay(parent, name string) string {
	switch parent {
	case "", ".":
		return "./" + name
	case "/":
		return "/" + name
	default:
		return parent + "/" + name
	}
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
