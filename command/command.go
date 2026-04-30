package command

import (
	"context"
	"io"

	"github.com/go-git/go-billy/v5"
	"mvdan.cc/sh/v3/expand"
)

type ExecContext struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Dir            string
	Environ        expand.Environ
	FS             billy.Filesystem
}

type Execer interface {
	Exec(ctx context.Context, ec *ExecContext, args []string) error
}
