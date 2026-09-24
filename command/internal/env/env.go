// Package env implements the env(1) coreutil: run a program in a modified
// environment, or print the current environment when no command is given.
//
// Argument parsing intentionally does not use getopt because env's grammar
// is special: NAME=VALUE assignments and the inner command share the
// positional slot, and the first positional that is neither a flag nor an
// assignment terminates option parsing. Any subsequent NAME=VALUE-shaped
// argument is part of the inner command, not an env assignment. getopt/v2
// has no way to express that boundary.
package env

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("env: nil ExecContext")
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
		fmt.Fprint(w, "Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]\n")
		fmt.Fprint(w, "Set each NAME to VALUE in the environment and run COMMAND.\n\n")
		fmt.Fprint(w, "  -i, --ignore-environment  start with an empty environment\n")
		fmt.Fprint(w, "  -u, --unset=NAME          remove variable from the environment\n")
		fmt.Fprint(w, "      --help                display this help and exit\n\n")
		fmt.Fprint(w, "A mere - implies -i. If no COMMAND, print the resulting environment.\n")
	}

	var (
		ignoreEnv bool
		unsetVars []string
		setOrder  []string
		setVars   = map[string]string{}
	)

	i := 0
parseLoop:
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-i" || arg == "--ignore-environment" || arg == "-":
			ignoreEnv = true
			i++
		case arg == "-u" || arg == "--unset":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "env: option requires an argument -- '%s'\n", strings.TrimLeft(arg, "-"))
				usage(stderr)
				return interp.ExitStatus(125)
			}
			unsetVars = append(unsetVars, args[i])
			i++
		case strings.HasPrefix(arg, "--unset="):
			unsetVars = append(unsetVars, strings.TrimPrefix(arg, "--unset="))
			i++
		case strings.HasPrefix(arg, "-u") && len(arg) > 2:
			unsetVars = append(unsetVars, arg[2:])
			i++
		case arg == "--help":
			usage(stdout)
			return nil
		case arg == "--":
			i++
			break parseLoop
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(stderr, "env: unrecognized option '%s'\n", arg)
			usage(stderr)
			return interp.ExitStatus(125)
		case strings.HasPrefix(arg, "-") && arg != "-":
			fmt.Fprintf(stderr, "env: invalid option -- '%s'\n", arg[1:])
			usage(stderr)
			return interp.ExitStatus(125)
		case strings.Contains(arg, "="):
			name, val, _ := strings.Cut(arg, "=")
			if _, exists := setVars[name]; !exists {
				setOrder = append(setOrder, name)
			}
			setVars[name] = val
			i++
		default:
			break parseLoop
		}
	}

	commandArgs := args[i:]

	if len(commandArgs) == 0 {
		printEnvironment(stdout, ec.Environ, ignoreEnv, unsetVars, setOrder, setVars)
		return nil
	}

	if ec.Runner == nil {
		fmt.Fprint(stderr, "env: exec not available\n")
		return interp.ExitStatus(127)
	}

	return runWithEnv(ctx, ec, ignoreEnv, unsetVars, setOrder, setVars, commandArgs)
}

// printEnvironment writes the resulting environment, one NAME=VALUE per line,
// matching just-bash's "join with \n + trailing \n if non-empty" rule. Output
// is sorted alphabetically for deterministic behaviour; GNU env preserves
// environ order, but inside kefka the parent Environ has no stable order.
func printEnvironment(
	w io.Writer,
	parent expand.Environ,
	ignoreEnv bool,
	unsetVars []string,
	setOrder []string,
	setVars map[string]string,
) {
	newEnv := map[string]string{}
	if !ignoreEnv && parent != nil {
		parent.Each(func(name string, vr expand.Variable) bool {
			if !exportableString(vr) {
				return true
			}
			newEnv[name] = vr.String()
			return true
		})
	}
	for _, name := range unsetVars {
		delete(newEnv, name)
	}
	for _, name := range setOrder {
		newEnv[name] = setVars[name]
	}

	names := make([]string, 0, len(newEnv))
	for name := range newEnv {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "%s=%s\n", name, newEnv[name])
	}
}

// runWithEnv synthesises a small bash script that mutates the subshell's
// environment and execs the inner command. The script:
//
//  1. Optionally `unset`s every currently-set parent var (for -i).
//  2. `unset`s each name passed via -u/--unset.
//  3. Runs `command -- CMD ARGS` with the requested NAME=VALUE assignment
//     prefix. The `command` builtin bypasses shell functions and keywords
//     (notably `time`), matching what GNU env does — it execvp's, so shell
//     functions are invisible.
//
// The script flows through ec.Runner.Subshell() so the inner command is
// dispatched through the same exec-handler chain (registered builtins, shell
// functions, PATH binaries) that the user would have hit by typing it.
func runWithEnv(
	ctx context.Context,
	ec *command.ExecContext,
	ignoreEnv bool,
	unsetVars []string,
	setOrder []string,
	setVars map[string]string,
	commandArgs []string,
) error {
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	var b strings.Builder

	if ignoreEnv && ec.Environ != nil {
		var existing []string
		ec.Environ.Each(func(name string, vr expand.Variable) bool {
			// Only unset variables that would actually propagate to the
			// inner command — i.e. exported scalars. Skip readonly vars
			// (EUID, UID, GID, ...) since `unset` on those is a runtime
			// error that mvdan reports to stderr.
			if !vr.IsSet() || vr.ReadOnly || !vr.Exported {
				return true
			}
			if vr.Kind != expand.String && vr.Kind != expand.NameRef {
				return true
			}
			existing = append(existing, name)
			return true
		})
		sort.Strings(existing)
		for _, name := range existing {
			quoted, err := syntax.Quote(name, syntax.LangBash)
			if err != nil {
				continue
			}
			b.WriteString("unset ")
			b.WriteString(quoted)
			b.WriteByte('\n')
		}
	}

	for _, name := range unsetVars {
		quoted, err := syntax.Quote(name, syntax.LangBash)
		if err != nil {
			continue
		}
		b.WriteString("unset ")
		b.WriteString(quoted)
		b.WriteByte('\n')
	}

	for _, name := range setOrder {
		b.WriteString(name)
		b.WriteByte('=')
		quoted, err := syntax.Quote(setVars[name], syntax.LangBash)
		if err != nil {
			fmt.Fprintf(stderr, "env: cannot quote value for %s: %v\n", name, err)
			return interp.ExitStatus(125)
		}
		b.WriteString(quoted)
		b.WriteByte(' ')
	}

	b.WriteString("command --")
	for _, a := range commandArgs {
		quoted, err := syntax.Quote(a, syntax.LangBash)
		if err != nil {
			fmt.Fprintf(stderr, "env: cannot quote argument %q: %v\n", a, err)
			return interp.ExitStatus(125)
		}
		b.WriteByte(' ')
		b.WriteString(quoted)
	}

	prog, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).
		Parse(strings.NewReader(b.String()), "<env>")
	if err != nil {
		fmt.Fprintf(stderr, "env: cannot parse command: %v\n", err)
		return interp.ExitStatus(125)
	}

	sub := ec.Runner.Subshell()
	if err := interp.StdIO(ec.Stdin, ec.Stdout, ec.Stderr)(sub); err != nil {
		fmt.Fprintf(stderr, "env: cannot configure subshell: %v\n", err)
		return interp.ExitStatus(125)
	}
	return sub.Run(ctx, prog)
}

// exportableString reports whether vr should appear in the printed environment
// (and is the kind of variable env can carry into an exec). Only set, scalar,
// exported variables qualify — that mirrors what a real env would see in its
// environ block.
func exportableString(vr expand.Variable) bool {
	if !vr.IsSet() {
		return false
	}
	if vr.Kind != expand.String && vr.Kind != expand.NameRef {
		return false
	}
	return vr.Exported
}
