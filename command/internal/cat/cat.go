package cat

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
		return errors.New("cat: nil ExecContext")
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
	set.SetProgram("cat")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: cat [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Concatenate FILE(s) to standard output.\n\n")
		fmt.Fprint(stderr, "  -n, --number      number all output lines\n")
		fmt.Fprint(stderr, "  -u                (ignored; Go writes are unbuffered)\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	number := set.BoolLong("number", 'n', "number all output lines")
	_ = set.Bool('u', "(ignored; Go writes are unbuffered)")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"cat"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "cat: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	lineNumber := 1
	for _, file := range files {
		data, err := readOne(ec, file, stderr)
		if err != nil {
			exitCode = 1
			continue
		}
		if *number {
			out, next := addLineNumbers(string(data), lineNumber)
			io.WriteString(stdout, out)
			lineNumber = next
		} else {
			stdout.Write(data)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func readOne(ec *command.ExecContext, file string, stderr io.Writer) ([]byte, error) {
	if file == "-" {
		if ec.Stdin == nil {
			return nil, nil
		}
		return io.ReadAll(ec.Stdin)
	}
	if ec.FS == nil {
		err := errors.New("no filesystem")
		fmt.Fprintf(stderr, "cat: %s: %s\n", file, err)
		return nil, err
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "cat: %s: %s\n", file, err)
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func addLineNumbers(content string, startLine int) (string, int) {
	if content == "" {
		return "", startLine
	}
	lines := strings.Split(content, "\n")
	hasTrailingNewline := strings.HasSuffix(content, "\n")
	linesToNumber := lines
	if hasTrailingNewline {
		linesToNumber = lines[:len(lines)-1]
	}
	var b strings.Builder
	for i, line := range linesToNumber {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%6d\t%s", startLine+i, line)
	}
	if hasTrailingNewline {
		b.WriteByte('\n')
	}
	return b.String(), startLine + len(linesToNumber)
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
