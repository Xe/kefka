package jo

import (
	"context"
	_ "embed"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/wasmcommand"
)

//go:embed jo.wasm
var joWASM []byte

var commandImpl = mustNew()

func mustNew() *wasmcommand.Impl {
	impl, err := wasmcommand.New("jo", joWASM)
	if err != nil {
		panic(err)
	}
	return impl
}

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return commandImpl.Exec(ctx, ec, args)
}
