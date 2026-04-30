package readlink

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("readlink: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("readlink: ExecContext has no filesystem")
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
	set.SetProgram("readlink")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: readlink [OPTIONS] FILE...\n")
		fmt.Fprint(stderr, "Print resolved symbolic links or canonical file names.\n\n")
		fmt.Fprint(stderr, "  -f, --canonicalize   canonicalize by following every symlink in every component of the given name recursively\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	canonicalize := set.BoolLong("canonicalize", 'f', "canonicalize by following every symlink in every component of the given name recursively")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"readlink"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "readlink: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		fmt.Fprint(stderr, "readlink: missing operand\n")
		return interp.ExitStatus(1)
	}

	sym, hasSymlink := ec.FS.(billy.Symlink)

	anyError := false
	for _, file := range files {
		filePath := resolvePath(ec, file)

		if *canonicalize {
			current := filePath
			seen := make(map[string]struct{})
			for {
				if _, ok := seen[current]; ok {
					break
				}
				seen[current] = struct{}{}

				if !hasSymlink {
					break
				}
				target, err := sym.Readlink(current)
				if err != nil {
					break
				}
				if path.IsAbs(target) {
					trimmed := strings.TrimPrefix(target, "/")
					if trimmed == "" {
						current = "."
					} else {
						current = path.Clean(trimmed)
					}
				} else {
					dir := path.Dir(current)
					current = path.Join(dir, target)
				}
			}
			io.WriteString(stdout, current)
			io.WriteString(stdout, "\n")
			continue
		}

		if !hasSymlink {
			anyError = true
			continue
		}
		target, err := sym.Readlink(filePath)
		if err != nil {
			anyError = true
			continue
		}
		io.WriteString(stdout, target)
		io.WriteString(stdout, "\n")
	}

	if anyError {
		return interp.ExitStatus(1)
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
