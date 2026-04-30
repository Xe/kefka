package pwd

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
		return errors.New("pwd: nil ExecContext")
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
	set.SetProgram("pwd")

	usage := func() {
		fmt.Fprint(stderr, "Usage: pwd [OPTION]...\n")
		fmt.Fprint(stderr, "Print the full filename of the current working directory.\n\n")
		fmt.Fprint(stderr, "  -L  use PWD from environment, even if it contains symlinks (default)\n")
		fmt.Fprint(stderr, "  -P  avoid all symlinks\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	set.Bool('P', "avoid all symlinks")
	set.Bool('L', "use logical path even if it contains symlinks")
	help := set.BoolLong("help", 0, "display this help and exit")

	physical := false
	parseErr := set.Getopt(append([]string{"pwd"}, args...), func(opt getopt.Option) bool {
		switch opt.ShortName() {
		case "P":
			physical = true
		case "L":
			physical = false
		}
		return true
	})
	if parseErr != nil {
		fmt.Fprintf(stderr, "pwd: %s\n", parseErr)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	logical := absolutize(ec.Dir)
	out := logical

	if physical {
		if ec.FS != nil {
			if resolved, err := realpath(ec.FS, ec.Dir); err == nil {
				out = absolutize(resolved)
			}
		}
	}

	io.WriteString(stdout, out)
	io.WriteString(stdout, "\n")
	return nil
}

// absolutize renders an fsys-relative path as a shell-absolute path with a
// leading slash. The fsys root maps to "/".
func absolutize(dir string) string {
	dir = strings.TrimPrefix(dir, "/")
	if dir == "" || dir == "." {
		return "/"
	}
	return "/" + dir
}

// realpath resolves any symlinks along p using the billy.Symlink capability,
// if available. Returns an error if any path component does not exist.
// Filesystems without Symlink support return p unchanged.
func realpath(fsys billy.Filesystem, p string) (string, error) {
	if _, err := fsys.Stat(p); err != nil {
		return "", err
	}
	sym, ok := fsys.(billy.Symlink)
	if !ok {
		return path.Clean(p), nil
	}

	resolved := ""
	parts := strings.Split(path.Clean(p), "/")
	for steps := 0; len(parts) > 0 && steps < 64; steps++ {
		part := parts[0]
		parts = parts[1:]
		if part == "" || part == "." {
			continue
		}
		candidate := path.Join(resolved, part)
		if candidate == "" {
			candidate = "."
		}
		target, err := sym.Readlink(candidate)
		if err != nil {
			resolved = candidate
			continue
		}
		if path.IsAbs(target) {
			resolved = ""
			target = strings.TrimPrefix(target, "/")
		}
		parts = append(strings.Split(path.Clean(target), "/"), parts...)
	}
	if resolved == "" {
		return ".", nil
	}
	return resolved, nil
}
