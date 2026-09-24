// Package wasmcommand adapts a WASI WebAssembly program to a Kefka command.
package wasmcommand

import (
	"context"
	"errors"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/wasm/billyfs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
	"mvdan.cc/sh/v3/interp"
)

// Impl runs a compiled WASI program as a Kefka command.
type Impl struct {
	name     string
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

// New compiles wasm and prepares it to run as a command named name.
func New(name string, wasm []byte) (*Impl, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		_ = runtime.Close(ctx)
		return nil, err
	}

	compiled, err := runtime.CompileModule(ctx, wasm)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, err
	}

	return &Impl{name: name, runtime: runtime, compiled: compiled}, nil
}

// Exec runs the command with stdio, arguments, and filesystem from ec.
func (i *Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	fsConfig := wazero.NewFSConfig().(sysfs.FSConfig).
		WithSysFSMount(billyfs.New(ec.FS), "/")

	config := wazero.NewModuleConfig().
		WithStdin(ec.Stdin).
		WithStdout(ec.Stdout).
		WithStderr(ec.Stderr).
		WithArgs(append([]string{i.name}, args...)...).
		WithName(i.name).
		WithFSConfig(fsConfig).
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime()

	mod, err := i.runtime.InstantiateModule(ctx, i.compiled, config)
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
