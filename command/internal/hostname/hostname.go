package hostname

import (
	"context"
	"io"

	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec != nil && ec.Stdout != nil {
		io.WriteString(ec.Stdout, "localhost\n")
	}
	return nil
}
