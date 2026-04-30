package tac

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

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("tac: nil ExecContext")
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
	set.SetProgram("tac")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: tac [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Write each FILE to standard output, last line first.\n")
		fmt.Fprint(stderr, "With no FILE, or when FILE is -, read standard input.\n\n")
		fmt.Fprint(stderr, "      --help   display this help and exit\n")
	}
	set.SetUsage(usage)

	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"tac"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "tac: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	positional := set.Args()

	var content string
	if len(positional) > 0 && positional[0] != "-" {
		data, err := readFile(ec, positional[0], stderr)
		if err != nil {
			return err
		}
		content = data
	} else {
		data, err := readStdin(ec)
		if err != nil {
			return err
		}
		content = data
	}

	io.WriteString(stdout, reverseLines(content))
	return nil
}

func reverseLines(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	var out strings.Builder
	for i := len(lines) - 1; i >= 0; i-- {
		out.WriteString(lines[i])
		out.WriteByte('\n')
	}
	return out.String()
}

func readStdin(ec *command.ExecContext) (string, error) {
	if ec.Stdin == nil {
		return "", nil
	}
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
}

func readFile(ec *command.ExecContext, file string, stderr io.Writer) (string, error) {
	if ec.FS == nil {
		fmt.Fprintf(stderr, "tac: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "tac: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "tac: %s: %v\n", file, err)
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
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
