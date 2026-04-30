package sha256sum

import (
	"context"

	"tangled.org/xeiaso.net/kefka/command"
	"tangled.org/xeiaso.net/kefka/command/internal/checksum"
)

type Impl struct{}

var config = checksum.Config{
	Name:      "sha256sum",
	Algorithm: checksum.SHA256,
	Summary:   "compute SHA256 message digest",
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return config.Exec(ctx, ec, args)
}
