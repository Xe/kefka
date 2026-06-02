package command

import (
	"context"
	"io"
	"path"

	"github.com/go-git/go-billy/v6"
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

// GuestPWD maps the fsys-relative Dir into the absolute path a WASI guest sees,
// given the billy filesystem is mounted at "/". Returns "/" when Dir is unset
// or ".". Used to seed the guest's PWD env var so a guest that honors it (see
// command/uutils/pwd-hack.patch) resolves relative paths against the shell's
// current directory rather than the filesystem root.
func (ec *ExecContext) GuestPWD() string {
	if ec.Dir == "" {
		return "/"
	}
	return path.Join("/", ec.Dir)
}

type Execer interface {
	Exec(ctx context.Context, ec *ExecContext, args []string) error
}
