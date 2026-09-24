package jq

import (
	"context"
	_ "embed"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/wasmcommand"
)

//go:embed jq.wasm
var jqWASM []byte

var commandImpl = wasmcommand.New("jq", jqWASM)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return commandImpl.Exec(ctx, ec, args)
}
