package tee

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("tee: nil ExecContext")
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
	set.SetProgram("tee")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: tee [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Copy standard input to each FILE, and also to standard output.\n\n")
		fmt.Fprint(stderr, "  -a, --append              append to the given FILEs, do not overwrite\n")
		fmt.Fprint(stderr, "  -i, --ignore-interrupts   ignore interrupt signals\n")
		fmt.Fprint(stderr, "      --help                display this help and exit\n")
	}
	set.SetUsage(usage)

	appendMode := set.BoolLong("append", 'a', "append to the given FILEs, do not overwrite")
	// -i is accepted for GNU coreutils 9.x compatibility but is a no-op:
	// kefka runs in-process under mvdan.cc/sh, so signal handling is the
	// host shell's responsibility, not ours.
	_ = set.BoolLong("ignore-interrupts", 'i', "ignore interrupt signals")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"tee"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "tee: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}
	files := set.Args()

	var content []byte
	if ec.Stdin != nil {
		data, err := io.ReadAll(ec.Stdin)
		if err != nil {
			fmt.Fprintf(stderr, "tee: %v\n", err)
			return interp.ExitStatus(1)
		}
		content = data
	}

	// GNU tee continues writing to remaining files (and to stdout) when one
	// file fails; it just exits non-zero at the end.
	exitCode := 0
	for _, file := range files {
		if err := writeFile(ec, file, content, *appendMode); err != nil {
			fmt.Fprintf(stderr, "tee: %s: %s\n", file, errMessage(err))
			exitCode = 1
		}
	}

	if _, err := stdout.Write(content); err != nil {
		fmt.Fprintf(stderr, "tee: %v\n", err)
		return interp.ExitStatus(1)
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// errMessage extracts a GNU-tee-style short error message from err. Billy
// returns wrapped *PathError values for most failures; the underlying error's
// Error() is what GNU coreutils prints (e.g. "Permission denied"). When we
// can't pull a more specific cause out, we default to "No such file or
// directory" — billy's memfs does not always wrap missing-parent errors as
// *PathError, and this matches what users hit most often.
func errMessage(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return "No such file or directory"
}

func writeFile(ec *command.ExecContext, file string, content []byte, appendMode bool) error {
	if ec.FS == nil {
		return errors.New("no filesystem")
	}
	full := resolvePath(ec, file)
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if appendMode {
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := ec.FS.OpenFile(full, flag, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		return err
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
