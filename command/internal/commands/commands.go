package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

// Lister reports the names of registered commands. The registry satisfies
// this interface; using it here keeps commands decoupled from the concrete type.
type Lister interface {
	Names() []string
}

type Impl struct {
	Reg Lister
}

func (i Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("commands: nil ExecContext")
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
	set.SetProgram("commands")
	set.SetParameters("")

	usage := func() {
		fmt.Fprint(stderr, "Usage: commands [OPTION]...\n")
		fmt.Fprint(stderr, "List all built-in commands as a sorted Markdown list.\n\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"commands"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "commands: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	if i.Reg == nil {
		fmt.Fprint(stderr, "commands: no command registry available\n")
		return interp.ExitStatus(1)
	}

	names := i.Reg.Names()
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(stdout, "- `%s`\n", name)
	}
	return nil
}
