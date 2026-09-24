package whoami

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("whoami: nil ExecContext")
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
	set.SetProgram("whoami")
	set.SetParameters("")

	usage := func() {
		fmt.Fprint(stderr, "Usage: whoami [OPTION]...\n")
		fmt.Fprint(stderr, "Print the user name associated with the current effective user ID.\n\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"whoami"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "whoami: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	io.WriteString(stdout, "user\n")
	return nil
}
