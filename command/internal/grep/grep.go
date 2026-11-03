// Package grep ports just-bash's grep and its egrep/fgrep aliases.
package grep

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Xe/kefka/command"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
)

type Impl struct{}
type Egrep struct{}
type Fgrep struct{}

func (Egrep) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return Impl{}.Exec(ctx, ec, append([]string{"-E"}, args...))
}
func (Fgrep) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return Impl{}.Exec(ctx, ec, append([]string{"-F"}, args...))
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("grep: nil ExecContext")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Keep buffered stdin local to this invocation.
	local := *ec
	ec = &local
	if ec.Stdin != nil {
		ec.Stdin = bufio.NewReader(contextReader{ctx, ec.Stdin})
	}
	budget := new(searchBudget)
	stdout, stderr := ec.Stdout, ec.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	o, files, help, err := parseOptions(args, stderr)
	if err != nil {
		var optionErr *getopt.Error
		if errors.As(err, &optionErr) {
			switch optionErr.ErrorCode {
			case getopt.UnknownOption:
				if strings.HasPrefix(optionErr.Name, "--") {
					fmt.Fprintf(stderr, "grep: unrecognized option '%s'\n", optionErr.Name)
				} else {
					fmt.Fprintf(stderr, "grep: invalid option -- '%s'\n", strings.TrimPrefix(optionErr.Name, "-"))
				}
				return interp.ExitStatus(1)
			case getopt.MissingParameter:
				if strings.HasPrefix(optionErr.Name, "--") {
					fmt.Fprintf(stderr, "grep: option '%s' requires an argument\n", optionErr.Name)
				} else {
					fmt.Fprintf(stderr, "grep: option requires an argument -- '%s'\n", strings.TrimPrefix(optionErr.Name, "-"))
				}
				return interp.ExitStatus(2)
			}
		}
		fmt.Fprintf(stderr, "grep: %s\n", err)
		return interp.ExitStatus(2)
	}
	if help {
		return nil
	}
	for _, name := range o.patternFiles {
		content, err := readInput(ctx, ec, name)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(stderr, "grep: %s: %s\n", name, err)
			return interp.ExitStatus(2)
		}
		o.patterns = append(o.patterns, splitLines(content)...)
	}
	if len(o.patterns) == 0 && !o.invert && !o.listWithout {
		return interp.ExitStatus(1)
	}
	matcher, err := compilePatterns(o)
	if err != nil {
		fmt.Fprintf(stderr, "grep: %s\n", err)
		return interp.ExitStatus(2)
	}
	if len(files) == 0 {
		if ec.Stdin == nil {
			fmt.Fprintln(stderr, "grep: no input files")
			return interp.ExitStatus(2)
		}
		input, closeInput, err := openInput(ctx, ec, "-")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(stderr, "grep: (standard input): %s\n", err)
			return interp.ExitStatus(2)
		}
		// Upstream's implicit stdin branch bypasses file-listing modes.
		searchOpts := o
		searchOpts.list = false
		searchOpts.listWithout = false
		output, matched, err := search(ctx, input, matcher, searchOpts, "", budget)
		closeInput()
		if err != nil {
			return searchError(ctx, stderr, err)
		}
		if !o.quiet {
			if _, err := io.WriteString(stdout, output); err != nil {
				return err
			}
		}
		if !matched {
			return interp.ExitStatus(1)
		}
		return nil
	}
	entries, err := collectFiles(ctx, ec, files, o)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintf(stderr, "grep: %s\n", err)
		return interp.ExitStatus(2)
	}
	hasFile := false
	for _, f := range files {
		if f != "-" {
			hasFile = true
		}
	}
	showFilename := !o.noFilename && (len(entries) > 1 || o.recursive && hasFile)
	anyMatch, anyError := false, false
	stdinClaimed := false
	var diagnostics strings.Builder
	for _, name := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if name != "-" && !o.recursive && !o.acceptFile(name) {
			continue
		}
		display := name
		if name == "-" {
			display = "(standard input)"
		}
		input, closeInput, err := openInput(ctx, ec, name)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(&diagnostics, "grep: %s: %s\n", display, err)
			// Preserve upstream: a directory diagnostic alone does not set status 2.
			if !errors.Is(err, errDirectory) {
				anyError = true
			}
			continue
		}
		if name == "-" {
			// Upstream consumes stdin only once, even when -m stops early.
			// Claim it without draining a pipe that may still be open.
			if stdinClaimed {
				input = bufio.NewReader(strings.NewReader(""))
			}
			stdinClaimed = true
		}
		prefix := ""
		if showFilename {
			prefix = display
		}
		output, matched, err := search(ctx, input, matcher, o, prefix, budget)
		closeInput()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, errInput) {
				fmt.Fprintf(&diagnostics, "grep: %s: %s\n", display, err)
				anyError = true
				continue
			}
			return searchError(ctx, stderr, err)
		}
		if matched {
			anyMatch = true
			if o.quiet {
				return nil
			}
			if o.list {
				output = display + "\n"
			} else if o.listWithout {
				output = ""
			}
		} else {
			if o.listWithout {
				output = display + "\n"
			} else if !o.count || o.list {
				output = ""
			}
		}
		if !o.quiet && (o.list || o.listWithout) {
			var listing strings.Builder
			if err := budget.write(&listing, output); err != nil {
				return searchError(ctx, stderr, err)
			}
		}
		if !o.quiet {
			if _, err := io.WriteString(stdout, output); err != nil {
				return err
			}
		}
	}
	if !o.quiet {
		if _, err := io.WriteString(stderr, diagnostics.String()); err != nil {
			return err
		}
	}
	if anyError {
		return interp.ExitStatus(2)
	}
	if !anyMatch {
		return interp.ExitStatus(1)
	}
	return nil
}

func searchError(ctx context.Context, stderr io.Writer, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	switch {
	case errors.Is(err, errWorkLimit), errors.Is(err, errResultLimit), errors.Is(err, errOutputLimit), errors.Is(err, errLineLimit), errors.Is(err, errContextLimit), errors.Is(err, errInput):
		fmt.Fprintf(stderr, "grep: %s\n", err)
		return interp.ExitStatus(2)
	}
	// Engine errors may include the input. Keep guest data out of diagnostics.
	fmt.Fprintln(stderr, "grep: regular expression execution limit exceeded")
	return interp.ExitStatus(2)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
