package command

import (
	"context"
	"io"
	"io/fs"

	"mvdan.cc/sh/v3/expand"
)

type ExecContext struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Dir            string
	Environ        expand.Environ
	FS             fs.FS
}

type Execer interface {
	Exec(ctx context.Context, ec *ExecContext, args []string) error
}
