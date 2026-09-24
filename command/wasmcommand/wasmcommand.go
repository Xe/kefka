// Package wasmcommand adapts a WASI WebAssembly program to a Kefka command.
package wasmcommand

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/wasm/billyfs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
	"mvdan.cc/sh/v3/interp"
)

// Impl runs a WASI program as a Kefka command. The program is compiled
// on the first call to Exec, not when the Impl is created.
type Impl struct {
	name string
	wasm []byte

	once     sync.Once
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	err      error
}

// New prepares wasm to run as a command named name. Compilation errors are
// returned by Exec.
func New(name string, wasm []byte) *Impl {
	return &Impl{name: name, wasm: wasm}
}

// compile creates the runtime and compiles the program exactly once.
func (i *Impl) compile() error {
	i.once.Do(func() {
		ctx := context.Background()
		runtime := wazero.NewRuntime(ctx)
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
			_ = runtime.Close(ctx)
			i.err = err
			return
		}

		compiled, err := runtime.CompileModule(ctx, i.wasm)
		if err != nil {
			_ = runtime.Close(ctx)
			i.err = err
			return
		}

		i.runtime = runtime
		i.compiled = compiled
	})
	return i.err
}

// Exec runs the command with stdio, arguments, and filesystem from ec.
func (i *Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if err := i.compile(); err != nil {
		return fmt.Errorf("wasmcommand: can't compile %s: %w", i.name, err)
	}

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
