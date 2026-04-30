package mkdir

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
		return errors.New("mkdir: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("mkdir: ExecContext has no filesystem")
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
	set.SetProgram("mkdir")
	set.SetParameters("DIRECTORY...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: mkdir [OPTION]... DIRECTORY...\n")
		fmt.Fprint(stderr, "Create the DIRECTORY(ies), if they do not already exist.\n\n")
		fmt.Fprint(stderr, "  -p, --parents   no error if existing, make parent directories as needed\n")
		fmt.Fprint(stderr, "  -v, --verbose   print a message for each created directory\n")
		fmt.Fprint(stderr, "      --help      display this help and exit\n")
	}
	set.SetUsage(usage)

	parents := set.BoolLong("parents", 'p', "no error if existing, make parent directories as needed")
	verbose := set.BoolLong("verbose", 'v', "print a message for each created directory")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"mkdir"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "mkdir: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	dirs := set.Args()
	if len(dirs) == 0 {
		fmt.Fprint(stderr, "mkdir: missing operand\n")
		return interp.ExitStatus(1)
	}

	exitCode := 0
	for _, dir := range dirs {
		full := resolvePath(ec, dir)

		if !*parents {
			if _, err := ec.FS.Stat(full); err == nil {
				fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': File exists\n", dir)
				exitCode = 1
				continue
			}
			parent := path.Dir(full)
			if parent != "." && parent != "/" && parent != "" {
				if _, err := ec.FS.Stat(parent); err != nil {
					fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': No such file or directory\n", dir)
					exitCode = 1
					continue
				}
			}
		}

		if err := ec.FS.MkdirAll(full, 0o755); err != nil {
			fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': %s\n", dir, err)
			exitCode = 1
			continue
		}

		if *verbose {
			fmt.Fprintf(stdout, "mkdir: created directory '%s'\n", dir)
		}
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
