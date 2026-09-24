package rg

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/wasm/billyfs"
)

//go:embed rg.wasm
var rgWASM []byte

var (
	compileOnce sync.Once
	runtime     wazero.Runtime
	compiled    wazero.CompiledModule
	compileErr  error
)

// compile creates the runtime and compiles the embedded program on first
// use, so that importing this package does not pay the compile cost.
func compile() error {
	compileOnce.Do(func() {
		ctx := context.Background()
		r := wazero.NewRuntime(ctx)
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
			_ = r.Close(ctx)
			compileErr = err
			return
		}

		c, err := r.CompileModule(ctx, rgWASM)
		if err != nil {
			_ = r.Close(ctx)
			compileErr = err
			return
		}

		runtime = r
		compiled = c
	})
	return compileErr
}

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if err := compile(); err != nil {
		return fmt.Errorf("rg: can't compile wasm: %w", err)
	}

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
