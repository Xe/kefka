package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

var (
	command = pflag.StringP("command", "c", "", "if set, run this command")
	timeout = pflag.DurationP("timeout", "T", 5*time.Minute, "the total time a command can run for")

	ErrInteractiveNotImplementedYet = errors.New("kefka: interactive mode not implemented yet")
)

func main() {
	pflag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "can't run shell:", err)
	}
}

func run(ctx context.Context) error {
	sh, err := interp.New(interp.Interactive(true), interp.StdIO(os.Stdin, os.Stdout, os.Stderr))
	if err != nil {
		return fmt.Errorf("can't make shell: %w", err)
	}

	if *command != "" {
		return runReader(ctx, sh, strings.NewReader(*command), "<argument>")
	}

	if pflag.NArg() == 1 {
		return runFile(ctx, sh, pflag.Arg(0))
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		return runInteractive(ctx, sh, os.Stdin, os.Stdout, os.Stderr)
	}

	return runReader(ctx, sh, os.Stdin, "<stdin>")
}

func runReader(ctx context.Context, sh *interp.Runner, in io.Reader, name string) error {
	prog, err := syntax.NewParser().Parse(in, name)
	if err != nil {
		return err
	}
	sh.Reset()
	return sh.Run(ctx, prog)
}

func runFile(ctx context.Context, sh *interp.Runner, fname string) error {
	fin, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fin.Close()
	return runReader(ctx, sh, fin, fname)
}

func runInteractive(ctx context.Context, sh *interp.Runner, stdin io.Reader, stdout, stderr io.Writer) error {
	parser := syntax.NewParser()
	fmt.Fprintf(stdout, "$ ")
	for stmts, err := range parser.InteractiveSeq(stdin) {
		if err != nil {
			return err
		}

		if parser.Incomplete() {
			fmt.Fprintf(stdout, "> ")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		for _, stmt := range stmts {
			err := sh.Run(ctx, stmt)
			if sh.Exited() {
				return err
			}
		}

		fmt.Fprintf(stdout, "$ ")
	}

	return nil
}
