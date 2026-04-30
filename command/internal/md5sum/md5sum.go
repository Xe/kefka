package md5sum

import (
	"context"

	"tangled.org/xeiaso.net/kefka/command"
	"tangled.org/xeiaso.net/kefka/command/internal/checksum"
)

type Impl struct{}

var config = checksum.Config{
	Name:      "md5sum",
	Algorithm: checksum.MD5,
	Summary:   "compute MD5 message digest",
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return config.Exec(ctx, ec, args)
}
