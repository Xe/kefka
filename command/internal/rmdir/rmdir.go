package rmdir

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("rmdir: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("rmdir: ExecContext has no filesystem")
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
	set.SetProgram("rmdir")
	set.SetParameters("DIRECTORY...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: rmdir [-pv] DIRECTORY...\n")
		fmt.Fprint(stderr, "Remove empty directories.\n\n")
		fmt.Fprint(stderr, "  -p, --parents   Remove DIRECTORY and its ancestors\n")
		fmt.Fprint(stderr, "  -v, --verbose   Output a diagnostic for every directory processed\n")
		fmt.Fprint(stderr, "      --help      display this help and exit\n")
	}
	set.SetUsage(usage)

	parents := set.BoolLong("parents", 'p', "Remove DIRECTORY and its ancestors")
	verbose := set.BoolLong("verbose", 'v', "Output a diagnostic for every directory processed")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"rmdir"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "rmdir: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	dirs := set.Args()
	if len(dirs) == 0 {
		fmt.Fprint(stderr, "rmdir: missing operand\n")
		return interp.ExitStatus(1)
	}

	exitCode := 0
	for _, dir := range dirs {
		full := resolvePath(ec, dir)
		if code := removeSingleDir(ec, full, dir, *verbose, stdout, stderr); code != 0 {
			exitCode = code
			continue
		}

		if !*parents {
			continue
		}

		currentPath := full
		currentDir := dir
		for {
			parentPath := path.Dir(currentPath)
			parentDir := path.Dir(currentDir)
			if parentPath == currentPath ||
				parentPath == "." || parentPath == "/" ||
				parentDir == "." || parentDir == "" {
				break
			}
			if removeSingleDir(ec, parentPath, parentDir, *verbose, stdout, io.Discard) != 0 {
				break
			}
			currentPath = parentPath
			currentDir = parentDir
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func removeSingleDir(ec *command.ExecContext, full, display string, verbose bool, stdout, stderr io.Writer) int {
	info, err := ec.FS.Stat(full)
	if err != nil {
		fmt.Fprintf(stderr, "rmdir: failed to remove '%s': No such file or directory\n", display)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "rmdir: failed to remove '%s': Not a directory\n", display)
		return 1
	}

	entries, err := ec.FS.ReadDir(full)
	if err != nil {
		fmt.Fprintf(stderr, "rmdir: failed to remove '%s': %s\n", display, err)
		return 1
	}
	if len(entries) > 0 {
		fmt.Fprintf(stderr, "rmdir: failed to remove '%s': Directory not empty\n", display)
		return 1
	}

	if err := ec.FS.Remove(full); err != nil {
		fmt.Fprintf(stderr, "rmdir: failed to remove '%s': %s\n", display, err)
		return 1
	}

	if verbose {
		fmt.Fprintf(stdout, "rmdir: removing directory, '%s'\n", display)
	}
	return 0
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
