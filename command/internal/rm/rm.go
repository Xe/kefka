package rm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-billy/v5/util"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("rm: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("rm: ExecContext has no filesystem")
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
	set.SetProgram("rm")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: rm [OPTION]... FILE...\n")
		fmt.Fprint(stderr, "Remove (unlink) the FILE(s).\n\n")
		fmt.Fprint(stderr, "  -f, --force       ignore nonexistent files and arguments, never prompt\n")
		fmt.Fprint(stderr, "  -r, -R, --recursive   remove directories and their contents recursively\n")
		fmt.Fprint(stderr, "  -v, --verbose     explain what is being done\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	recursive := set.BoolLong("recursive", 'r', "remove directories and their contents recursively")
	recursiveUpper := set.Bool('R', "remove directories and their contents recursively")
	force := set.BoolLong("force", 'f', "ignore nonexistent files and arguments, never prompt")
	verbose := set.BoolLong("verbose", 'v', "explain what is being done")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"rm"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "rm: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	recurse := *recursive || *recursiveUpper
	paths := set.Args()

	if len(paths) == 0 {
		if *force {
			return nil
		}
		fmt.Fprint(stderr, "rm: missing operand\n")
		return interp.ExitStatus(1)
	}

	exitCode := 0
	for _, p := range paths {
		full := resolvePath(ec, p)
		info, err := ec.FS.Stat(full)
		if err != nil {
			if !*force {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintf(stderr, "rm: cannot remove '%s': No such file or directory\n", p)
				} else {
					fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", p, err)
				}
				exitCode = 1
			}
			continue
		}

		if info.IsDir() && !recurse {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': Is a directory\n", p)
			exitCode = 1
			continue
		}

		var rmErr error
		if recurse {
			rmErr = util.RemoveAll(ec.FS, full)
		} else {
			rmErr = ec.FS.Remove(full)
		}
		if rmErr != nil {
			if !*force {
				switch {
				case errors.Is(rmErr, os.ErrNotExist):
					fmt.Fprintf(stderr, "rm: cannot remove '%s': No such file or directory\n", p)
				case isNotEmpty(rmErr):
					fmt.Fprintf(stderr, "rm: cannot remove '%s': Directory not empty\n", p)
				default:
					fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", p, rmErr)
				}
				exitCode = 1
			}
			continue
		}

		if *verbose {
			fmt.Fprintf(stdout, "removed '%s'\n", p)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func isNotEmpty(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not empty") || strings.Contains(msg, "ENOTEMPTY")
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
