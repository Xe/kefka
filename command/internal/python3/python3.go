package python3

import (
	"context"
	_ "embed"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"tangled.org/xeiaso.net/kefka/command"
)

var (
	//go:embed python.wasm
	pyWASM []byte

	r    wazero.Runtime
	code wazero.CompiledModule
)

func init() {
	ctx := context.Background()
	r = wazero.NewRuntime(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	var err error
	code, err = r.CompileModule(ctx, pyWASM)
	if err != nil {
		panic(err)
	}
}

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	fsConfig := wazero.NewFSConfig().
		WithFSMount(ec.FS, "/")

	config := wazero.NewModuleConfig().
		// stdio
		WithStdin(ec.Stdin).
		WithStdout(ec.Stdout).
		WithStderr(ec.Stderr).
		// argv
		WithArgs(append([]string{"python3"}, args...)...).
		WithName("python3").
		// filesystem
		WithFSConfig(fsConfig).
		// time
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime()

	mod, err := r.InstantiateModule(ctx, code, config)
	if err != nil {
		return err
	}

	defer mod.Close(ctx)

	return nil
}
