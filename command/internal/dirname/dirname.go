package dirname

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("dirname: nil ExecContext")
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
	set.SetProgram("dirname")
	set.SetParameters("NAME...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: dirname [OPTION] NAME...\n")
		fmt.Fprint(stderr, "Strip last component from file name.\n\n")
		fmt.Fprint(stderr, "      --help       display this help and exit\n")
	}
	set.SetUsage(usage)

	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"dirname"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "dirname: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	names := set.Args()
	if len(names) == 0 {
		fmt.Fprint(stderr, "dirname: missing operand\n")
		return interp.ExitStatus(1)
	}

	results := make([]string, 0, len(names))
	for _, name := range names {
		results = append(results, dirnameOf(name))
	}

	io.WriteString(stdout, strings.Join(results, "\n"))
	io.WriteString(stdout, "\n")
	return nil
}

func dirnameOf(name string) string {
	clean := strings.TrimRight(name, "/")
	idx := strings.LastIndex(clean, "/")
	switch idx {
	case -1:
		return "."
	case 0:
		return "/"
	default:
		return clean[:idx]
	}
}
