package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/spf13/pflag"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
	"tangled.org/xeiaso.net/kefka/command/registry"
	"tangled.org/xeiaso.net/kefka/command/registry/coreutils"
	"tangled.org/xeiaso.net/kefka/command/registry/wasmprog"
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
	reg := registry.New()
	coreutils.Register(reg)
	wasmprog.Register(reg)

	fsys := osfs.New(".")

	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, args)
		}
	}

	sh, err := interp.New(
		interp.Interactive(true),
		interp.StdIO(os.Stdin, os.Stdout, os.Stderr),
		interp.ExecHandlers(middleware),
		interp.CallHandler(callHandler(reg, fsys, os.Stdout, os.Stderr)),
		interp.StatHandler(fsysStatHandler(reg, fsys)),
		interp.OpenHandler(fsysOpenHandler(reg, fsys)),
		interp.ReadDirHandler2(fsysReadDirHandler(reg, fsys)),
	)
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
	prog, err := syntax.NewParser(
		syntax.Variant(syntax.LangBash),
	).Parse(in, name)
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

// callHandler intercepts cd and pwd before interp's builtins handle them,
// so we can route directory state through the registry's fsys-relative pwd
// instead of interp's host-rooted Dir. Intercepted calls are replaced with
// `:` (no-op) so interp's builtin doesn't run.
func callHandler(reg *registry.Impl, fsys billy.Filesystem, stdout, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) == 0 {
			return args, nil
		}
		switch args[0] {
		case "cd":
			target := ""
			if len(args) > 1 {
				target = args[1]
			}
			if err := reg.Chdir(fsys, target); err != nil {
				fmt.Fprintln(stderr, err)
				return []string{"false"}, nil
			}
			return []string{":"}, nil
		case "pwd":
			pwd := reg.Pwd()
			if pwd == "." {
				fmt.Fprintln(stdout, "/")
			} else {
				fmt.Fprintln(stdout, "/"+pwd)
			}
			return []string{":"}, nil
		}
		return args, nil
	}
}

func fsysStatHandler(reg *registry.Impl, fsys billy.Filesystem) interp.StatHandlerFunc {
	return func(ctx context.Context, name string, followSymlinks bool) (fs.FileInfo, error) {
		resolved := reg.Resolve(name)
		if !followSymlinks {
			if r, ok := fsys.(billy.Symlink); ok {
				return r.Lstat(resolved)
			}
		}
		return fsys.Stat(resolved)
	}
}

func fsysOpenHandler(reg *registry.Impl, fsys billy.Filesystem) interp.OpenHandlerFunc {
	return func(ctx context.Context, name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC) != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
		}
		f, err := fsys.Open(reg.Resolve(name))
		if err != nil {
			return nil, err
		}
		return readOnlyFile{f}, nil
	}
}

func fsysReadDirHandler(reg *registry.Impl, fsys billy.Filesystem) interp.ReadDirHandlerFunc2 {
	return func(ctx context.Context, name string) ([]fs.DirEntry, error) {
		entries, err := fsys.ReadDir(reg.Resolve(name))
		if err != nil {
			return nil, err
		}
		out := make([]fs.DirEntry, len(entries))
		for i, e := range entries {
			out[i] = fs.FileInfoToDirEntry(e)
		}
		return out, nil
	}
}

type readOnlyFile struct{ billy.File }

func (readOnlyFile) Write([]byte) (int, error) { return 0, fs.ErrPermission }

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
