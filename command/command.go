package command

import (
	"context"
	"io"

	"github.com/go-git/go-billy/v5"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

type ExecContext struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Dir            string
	Environ        expand.Environ
	FS             billy.Filesystem
	// Runner is the active shell runner. Commands that need to dispatch a
	// child command (for example, `time CMD`) should call Runner.Subshell()
	// and re-enter the shell so the call goes through the same exec handler
	// chain instead of poking at the registry directly. May be nil in
	// embedders or tests that have not wired up a runner.
	Runner *interp.Runner
}

type Execer interface {
	Exec(ctx context.Context, ec *ExecContext, args []string) error
}
