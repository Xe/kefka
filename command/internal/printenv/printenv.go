// Package printenv implements the printenv(1) coreutil: print all or part
// of the environment.
package printenv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("printenv: nil ExecContext")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	usage := func(w io.Writer) {
		fmt.Fprint(w, "Usage: printenv [OPTION]... [VARIABLE]...\n")
		fmt.Fprint(w, "Print the values of the specified environment VARIABLE(s).\n")
		fmt.Fprint(w, "If no VARIABLE is specified, print name and value pairs for them all.\n\n")
		fmt.Fprint(w, "      --help     display this help and exit\n")
	}

	// printenv accepts arbitrary VARIABLE names, including ones that look
	// like flags. Only --help is treated as an option; everything else
	// (including a leading dash) is taken as a variable name. This matches
	// just-bash, with the exception that we recognise --help anywhere in the
	// arg list, since GNU coreutils does the same.
	var vars []string
	for _, arg := range args {
		switch arg {
		case "--help":
			usage(stdout)
			return nil
		default:
			vars = append(vars, arg)
		}
	}

	if len(vars) == 0 {
		printAll(stdout, ec.Environ)
		return nil
	}

	exitCode := 0
	for _, name := range vars {
		if ec.Environ == nil {
			exitCode = 1
			continue
		}
		v := ec.Environ.Get(name)
		if !v.IsSet() || (v.Kind != expand.String && v.Kind != expand.NameRef) {
			exitCode = 1
			continue
		}
		fmt.Fprintln(stdout, v.String())
	}
	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// printAll writes the full exported environment, one NAME=VALUE per line,
// sorted alphabetically. Trailing newline only when output is non-empty,
// matching just-bash and GNU printenv.
func printAll(w io.Writer, parent expand.Environ) {
	if parent == nil {
		return
	}
	pairs := map[string]string{}
	parent.Each(func(name string, vr expand.Variable) bool {
		if !vr.IsSet() {
			return true
		}
		if vr.Kind != expand.String && vr.Kind != expand.NameRef {
			return true
		}
		if !vr.Exported {
			return true
		}
		pairs[name] = vr.String()
		return true
	})
	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s=%s\n", name, pairs[name])
	}
	io.WriteString(w, b.String())
}
