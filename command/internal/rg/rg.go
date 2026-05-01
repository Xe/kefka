package rg

import (
	"context"
	_ "embed"
	"errors"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
	"tangled.org/xeiaso.net/kefka/wasm/billyfs"
)

//go:embed rg.wasm
var rgWASM []byte

var (
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
)

func init() {
	ctx := context.Background()
	runtime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	var err error
	compiled, err = runtime.CompileModule(ctx, rgWASM)
	if err != nil {
		panic(err)
	}
}

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	fsConfig := wazero.NewFSConfig().(sysfs.FSConfig).
		WithSysFSMount(billyfs.New(ec.FS), "/")

	config := wazero.NewModuleConfig().
		WithStdin(ec.Stdin).
		WithStdout(ec.Stdout).
		WithStderr(ec.Stderr).
		WithArgs(append([]string{"rg"}, args...)...).
		WithName("rg").
		WithFSConfig(fsConfig).
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime()

	mod, err := runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		if exitErr, ok := errors.AsType[*wsys.ExitError](err); ok {
			if code := exitErr.ExitCode(); code != 0 {
				return interp.ExitStatus(uint8(code))
			}
			return nil
		}
		return err
	}
	return mod.Close(ctx)
}
