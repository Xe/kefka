package jo

import (
	"context"
	_ "embed"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/wasmcommand"
)

//go:embed jo.wasm
var joWASM []byte

var commandImpl = wasmcommand.New("jo", joWASM)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return commandImpl.Exec(ctx, ec, args)
}
