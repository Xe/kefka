# Writing a custom command

This page explains how to add a command to Kefka. It covers the contract
that every command obeys and the three kinds of command in this repo. It
also lists the tests that a new command must have.

Read `AGENTS.md` first. It lists the required skills and the house rules.

## How a command runs

Kefka parses bash with `mvdan.cc/sh/v3`. The shell does not run external
programs. Instead, every command name goes to an in-process registry. The
registry finds a Go value for that name and calls it.

The flow for `rev -0 notes.txt` is:

1. The shell parses the line and expands the words.
2. The exec handler in `cmd/kefka/main.go` passes the words to `reg.Exec`.
3. The registry splits the name (`rev`) from the arguments (`-0`, `notes.txt`).
4. The registry builds an `ExecContext` and calls `Exec` on the `rev` value.
5. The command reads and writes files through `ec.FS`, never through `os`.
6. The command returns `nil` or an exit status. The shell turns this into `$?`.

The registry also owns the working directory. The shell's own `Dir` is a host
path, so `internal/billysh.CallHandler` intercepts `cd` and `pwd` before the
shell sees them. Your command gets the working directory as `ec.Dir`.

## The contract

Every command implements one interface from `command/command.go`:

```go
type Execer interface {
	Exec(ctx context.Context, ec *ExecContext, args []string) error
}
```

`args` does not include the command name. The registry removes it before
the call.

`ExecContext` has these fields:

| Field | Type | Meaning |
|---|---|---|
| `Stdin` | `io.Reader` | Standard input. Can be `nil`. Treat `nil` as empty input. |
| `Stdout`, `Stderr` | `io.Writer` | Standard output and error. Can be `nil`. Replace `nil` with `io.Discard`. |
| `Dir` | `string` | The working directory, relative to the root of `FS`. `"."` is the root. |
| `Environ` | `expand.Environ` | The shell variables. Use `ec.Environ.Get("NAME").String()`. |
| `FS` | `billy.Filesystem` | The virtual filesystem. Can be `nil` when the caller has no filesystem. |
| `Runner` | `*interp.Runner` | The active shell. Use it only to run a child command. Can be `nil`. |

`ec.GuestPWD()` returns `Dir` as an absolute guest path (`/` for the root).
WASM commands pass this value to the guest as `PWD`.

### Return values

| Return | Exit status in the shell |
|---|---|
| `nil` | 0 |
| `interp.ExitStatus(uint8(n))` | `n` |
| Any other `error` | Halts the runner and returns the error to its caller |

Return `interp.ExitStatus` for every expected failure. Return a plain error
only for programming mistakes, for example a `nil` `ExecContext`.
The caller decides how to print a plain error and set the host exit status.

Use the exit codes that GNU coreutils uses for the same command. The common
values are:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Runtime error, for example a missing file or bad input |
| 2 | Usage error, when the GNU tool uses 2 for it |
| 127 | Command not found. Only the registry returns this. |

### Output rules

- Match GNU coreutils or `just-bash` byte for byte. Compare against the real tool.
- Write error lines to stderr as `<name>: <arg>: <reason>\n`.
- Do not print Go error values such as `*os.PathError` to the user. Write a fixed message.
- Write the `--help` text to stderr and return `nil`. Help on stdout breaks pipelines.
- Use ASCII only in source and in output text.

## Three kinds of command

| Kind | Where the code lives | Registered by | Use it when |
|---|---|---|---|
| Pure Go | `command/internal/<name>` | `command/registry/coreutils` | The logic is small and Go can do it directly. |
| WASM guest | `command/internal/<name>` with a `.wasm` blob | `command/registry/wasmprog` | An existing program compiles to WASI and rewriting it is not practical. |
| Embedder-specific | Your own module, for example `cmd/sophia/commands/<name>` | Your own `main` | The command needs state that only your program has, such as an API client. |

The GNU coreutils names (`cat`, `ls`, `wc`, and more) come from the uutils
WASM build in `command/uutils`. `command/registry/uutils` registers them.
Both `cmd/kefka` and `cmd/sophia` call `uutils.Register` last. The registry
is a map, so a later `Register` with the same name replaces an earlier one.
Do not write a pure-Go command with a uutils name. The uutils version wins.

## Write a pure-Go command

This section uses `rev` as the example. `rev` reverses the characters in
each line. The example has one flag and reads files or stdin.

### 1. Create the package

Make the directory `command/internal/rev` with two files:

```
command/internal/rev/rev.go
command/internal/rev/rev_test.go
```

The package name is the command name. If the command name is a Go keyword,
add `cmd` to the package name, for example `truecmd`. The directory name
matches the package name.

If the package name collides with a standard library package that you
import, alias the import instead. For example, `gzlib "compress/gzip"` in
`command/internal/gzip`.

### 2. Write the command

Export one zero-value type named `Impl`. The registry stores the value, so
`Impl` must not need construction.

```go
package rev

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/Xe/kefka/command"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
)

type Impl struct{}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("rev: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("rev")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: rev [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Reverse the order of characters in every line.\n\n")
		fmt.Fprint(stderr, "  -0, --zero   end each output line with NUL, not newline\n")
		fmt.Fprint(stderr, "      --help   display this help and exit\n")
	}
	set.SetUsage(usage)

	zero := set.BoolLong("zero", '0', "end each output line with NUL, not newline")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"rev"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "rev: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	sep := byte('\n')
	if *zero {
		sep = 0
	}

	files := set.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	var exit error
	for _, name := range files {
		r, closeFn, err := open(ec, name)
		if err != nil {
			fmt.Fprintf(stderr, "rev: cannot open %s: No such file or directory\n", name)
			exit = interp.ExitStatus(1)
			continue
		}
		err = reverseLines(r, stdout, sep)
		closeFn()
		if err != nil {
			fmt.Fprintf(stderr, "rev: %s: input/output error\n", name)
			exit = interp.ExitStatus(1)
		}
	}
	return exit
}

// reverseLines writes each sep-terminated line of r to w with its
// characters reversed. A final line without sep keeps no sep.
func reverseLines(r io.Reader, w io.Writer, sep byte) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	for len(data) > 0 {
		line := data
		hasSep := false
		if i := bytes.IndexByte(data, sep); i >= 0 {
			line = data[:i]
			data = data[i+1:]
			hasSep = true
		} else {
			data = nil
		}
		out := []rune(string(line))
		slices.Reverse(out)
		if _, err := io.WriteString(w, string(out)); err != nil {
			return err
		}
		if hasSep {
			if _, err := w.Write([]byte{sep}); err != nil {
				return err
			}
		}
	}
	return nil
}

// open returns a reader for name. "-" means stdin.
func open(ec *command.ExecContext, name string) (io.Reader, func(), error) {
	if name == "-" {
		if ec.Stdin == nil {
			return strings.NewReader(""), func() {}, nil
		}
		return ec.Stdin, func() {}, nil
	}
	if ec.FS == nil {
		return nil, nil, errors.New("no filesystem")
	}
	f, err := ec.FS.Open(resolvePath(ec, name))
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
	} else {
		p = path.Join(dir, p)
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "."
	}
	return p
}
```

The parts of this file that every command repeats are:

- **The nil guards.** `ec`, `ec.Stdout`, `ec.Stderr`, `ec.Stdin`, and `ec.FS` can all be `nil`.
- **The getopt block.** See the next section.
- **The `-` argument.** `-` means stdin. No file arguments also means stdin.
- **The `resolvePath` helper.** Copy it without changes. It maps shell paths to `FS` paths.
- **Root containment.** The helper clamps paths outside the filesystem root to `.`. It uses the same rule as the registry.
- **Write errors.** Check errors from both the line write and the separator write. Return a failure if either write fails.
- **The error loop.** Report each bad file, continue with the rest, and return the failure at the end.

### 3. Parse flags with getopt

Use `github.com/pborman/getopt/v2`. The rules are:

- Prefix the arguments with the command name. `set.Getopt` expects `args[0]` to be the program name.
- If a flag has no short form, pass rune `0` as the short name.
- Add `--help` yourself. getopt does not add it.
- Call `set.SetUsage(usage)` so that getopt prints your text for bad flags.
- Read positional arguments from `set.Args()`.

Flag constructors that the repo uses:

| Constructor | Result |
|---|---|
| `set.Bool('a', "help text")` | Short flag only |
| `set.BoolLong("zero", '0', "help text")` | Long flag with a short alias |
| `set.BoolLong("help", 0, "help text")` | Long flag only |
| `set.StringLong("suffix", 'S', ".gz", "help text")` | String flag with a default |
| `set.IntLong("max-depth", 0, 0, "help text")` | Integer flag with a default |
| `set.Lookup("max-depth").Seen()` | True when the user passed the flag |

Some commands cannot use getopt. `env` and `time` parse their arguments by
hand because the first positional word ends option parsing. If your command
has that grammar, copy the `parseLoop` pattern from `command/internal/env`.

### 4. Register the command

Add the import and the `Register` call to
`command/registry/coreutils/coreutils.go`:

```go
import (
	// ...
	"github.com/Xe/kefka/command/internal/pwd"
	"github.com/Xe/kefka/command/internal/rev"
	"github.com/Xe/kefka/command/internal/stat"
	// ...
)

func Register(reg *registry.Impl) {
	// ...
	reg.Register("pwd", pwd.Impl{})
	reg.Register("rev", rev.Impl{})
	reg.Register("stat", stat.Impl{})
	// ...
}
```

Keep both the import block and the `Register` body in alphabetical order.

### 5. Write the tests

Use the `go-table-driven-tests` skill. Put the test next to the code. The
test calls `Impl{}.Exec` directly with an in-memory filesystem. It does not
start a shell.

The test file has three parts: a fixture builder, a run helper, and the
table.

```go
package rev

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/interp"
)

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("hello.txt", []byte("abc\nxyz\n"))
	write("nofinalnl.txt", []byte("abc\nxyz"))
	write("empty.txt", []byte(""))
	return fs
}

func run(t *testing.T, args []string, stdin string, fs billy.Filesystem) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestRev(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "reverses lines from stdin",
			stdin:      "abc\nxyz\n",
			wantStdout: "cba\nzyx\n",
		},
		{
			name:       "reverses lines from file",
			args:       []string{"hello.txt"},
			wantStdout: "cba\nzyx\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "abc\n",
			wantStdout: "cba\n",
		},
		{
			name:       "two files concatenate",
			args:       []string{"hello.txt", "hello.txt"},
			wantStdout: "cba\nzyx\ncba\nzyx\n",
		},
		{
			name:       "no trailing newline keeps no trailing newline",
			args:       []string{"nofinalnl.txt"},
			wantStdout: "cba\nzyx",
		},
		{
			name:       "multibyte runes reverse as runes",
			stdin:      "ab\u00e9\n",
			wantStdout: "\u00e9ba\n",
		},
		{
			name:       "zero flag splits on NUL",
			args:       []string{"-0"},
			stdin:      "abc\x00xyz\x00",
			wantStdout: "cba\x00zyx\x00",
		},
		{
			name:       "empty file produces no output",
			args:       []string{"empty.txt"},
			wantStdout: "",
		},
		{
			name:       "empty stdin produces no output",
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "missing file",
			args:       []string{"nope.txt"},
			wantErrSub: "rev: cannot open nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:       "missing file does not stop later files",
			args:       []string{"nope.txt", "hello.txt"},
			wantStdout: "cba\nzyx\n",
			wantErr:    true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args, tt.stdin, newFS(t))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: rev [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct{ name, dir, input, want string }{
		{"root", ".", "/", "."},
		{"empty working directory", "", "file", "file"},
		{"relative file", "work", "file", "work/file"},
		{"absolute file", "work", "/file", "file"},
		{"parent within root", "work/sub", "../file", "work/file"},
		{"relative escape", ".", "../private", "."},
		{"absolute escape", "work", "/../private", "."},
		{"nested escape", "work", "../../private", "."},
		{"bare parent", ".", "..", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePath(&command.ExecContext{Dir: tt.dir}, tt.input)
			if got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

type failingWriter struct{ remaining int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}
func TestOutputErrors(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
	}{
		{"line write", 0}, {"separator write", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			ec := &command.ExecContext{Stdin: strings.NewReader("abc\n"), Stdout: &failingWriter{tt.remaining}, Stderr: &stderr}
			err := (Impl{}).Exec(context.Background(), ec, nil)
			if !errors.Is(err, interp.ExitStatus(1)) {
				t.Errorf("error = %v, want exit 1", err)
			}
			if !strings.Contains(stderr.String(), "input/output error") {
				t.Errorf("stderr = %q", stderr.String())
			}
		})
	}
}
```

Every command test must cover these cases:

- A file argument
- `-` as stdin
- No arguments, which reads stdin
- Empty input
- Each flag at least once
- `--help` writes the usage line to stderr and returns `nil`
- An unknown flag returns an error
- A missing file returns an error and writes the standard stderr line

If the command has an inverse, add a round trip. `gzip` and `gunzip` do this.

### 6. Verify

Run these commands from the repo root:

```bash
go build ./...
go test ./command/internal/rev/...
go run ./cmd/kefka -c 'printf "abc\n" | rev'
```

Make sure that the third command prints `cba`. The `go run` step proves the
registration. The unit test does not.

Compare against the Linux tool, not the macOS one. The example above matches
util-linux `rev`. BSD `rev` on macOS adds a trailing newline to the last
line and writes a different error message for a missing file.

`go vet ./...` reports one existing error in `wasm/billyfs`. Your change
must not add more.

## Write a WASM command

If a program already compiles to WASI, use this path. `jo`, `jq`, `rg`,
`python3`, and `qjs` work this way. The Go side is a thin wrapper around
`wazero`.

### 1. Build the guest

Build the program as a `wasm32-wasi` module. Copy the `.wasm` file into
`command/internal/<name>/<name>.wasm`.

Store the blob in plain git, not in git LFS. `go:embed` reads the file from
the module cache. LFS pointers in the module cache break the build for
every downstream user.

If the guest resolves relative paths against `PWD`, pass `ec.GuestPWD()` as
the `PWD` environment variable. `command/uutils` does this. Most guests
ignore `PWD` and use `/` as the working directory.

### 2. Write the wrapper

Copy `command/internal/jo/jo.go` and change the names:

```go
package rev

import (
	"context"
	_ "embed"
	"errors"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/wasm/billyfs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	wsys "github.com/tetratelabs/wazero/sys"
	"mvdan.cc/sh/v3/interp"
)

//go:embed rev.wasm
var revWASM []byte

var (
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
)

func init() {
	ctx := context.Background()
	runtime = wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	var err error
	compiled, err = runtime.CompileModule(ctx, revWASM)
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
		WithArgs(append([]string{"rev"}, args...)...).
		WithName("rev").
		WithEnv("PWD", ec.GuestPWD()).
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
```

The wrapper does these things:

- **Compiles once.** `init` compiles the module. `Exec` instantiates a fresh instance for each call.
- **Mounts the shell filesystem at `/`.** `wasm/billyfs.New` adapts `ec.FS` into the wazero filesystem interface.
- **Passes `argv[0]`.** The guest expects its own name first.
- **Maps the exit code.** A guest `exit(n)` arrives as `*wsys.ExitError`. Return it as `interp.ExitStatus`.

### 3. Register the command

Add the import and the `Register` call to
`command/registry/wasmprog/wasmprog.go`. Keep the entries in alphabetical
order.

### 4. Know the stdio limit

`wazero` reports stdio that is not an `*os.File` as a block device. A guest
that calls `isatty` then gets the wrong answer. `python.wasm` and `qjs.wasm`
change their behavior on this. `cmd/sophia` uses `*os.File` pipes for this
reason. A `bytes.Buffer` in a unit test is fine for guests that do not care.

## Write an embedder-specific command

Any package can implement `command.Execer`. The command does not have to
live under `command/internal`. If the command needs state that only your
program has, use this kind.

`cmd/sophia/commands/snapshot` is the example in this repo. It takes a
Tigris client on the `Impl` value:

```go
type Impl struct {
	Client *storage.Client
}

func (i Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	// ...
	bucket := cmp.Or(fs.Arg(0), ec.Environ.Get("BUCKET_NAME").String())
	// ...
}
```

Register it in your own `main` after the standard sets:

```go
reg := registry.New()
coreutils.Register(reg)
wasmprog.Register(reg)
uutils.Register(reg)
reg.Register("snapshot", snapshot.Impl{Client: client})
```

The same rules apply. Write help to stderr. Return `interp.ExitStatus`.
Read files through `ec.FS`. Read shell variables through `ec.Environ`.

`command/internal/commands` shows a second pattern. It needs the list of
registered names, so it takes a small interface instead of the concrete
registry:

```go
type Lister interface {
	Names() []string
}

type Impl struct {
	Reg Lister
}
```

The registry satisfies `Lister`. The command package does not import the
registry package. This keeps the dependency graph flat.

## Write a command with no GNU counterpart

Not every command copies a coreutil. A command can call an API, query a
database, or talk to your own service. The contract is the same. The
difference is that you define the output format, because no reference tool
exists.

This section uses `fetch` as the example. `fetch URL` sends an HTTP GET
request and writes the response body to stdout.

### Design rules

- **You own the output contract.** Write it in the `--help` text and keep it stable. Scripts will depend on it.
- **Body on stdout, diagnostics on stderr.** Write the response body and nothing else to stdout. Then `fetch URL | jq .` works.
- **Stream the body.** Use `io.Copy` from the response body to stdout. Do not read the whole body into memory.
- **Do not add a trailing newline.** The body is bytes. Write it as the server sent it.
- **Pass `ctx` to every request.** Use `http.NewRequestWithContext`. Kefka runs each script under a timeout, and `ctx` is how the timeout stops your request.
- **Read secrets from `ec.Environ`, not `os.Getenv`.** The shell environment is separate from the host environment. `cmd/kefka` builds it from a fixed list. `TOKEN=x fetch URL` puts `TOKEN` in `ec.Environ`.
- **Put the client on `Impl`.** The embedder sets it in `main`. Tests point it at an `httptest.Server`. A `nil` client falls back to `http.DefaultClient`.
- **Pick exit codes and document them.** There is no GNU tool to copy. Use `0` for success, `1` for a failed request, and `2` for a usage error. If a flag makes HTTP errors fatal, `22` matches `curl --fail`.
- **Strip Go error wrappers.** Unwrap `*url.Error` and `*os.PathError` before you print the cause. Filesystem wrappers can contain host paths.
- **Check output close errors.** S3FS uploads on close. Return a failure if the close fails.
- **Write files through `ec.FS`.** Use `ec.FS.Create` with `resolvePath` for an `--output` flag. S3FS rejects the `O_CREATE` and `O_TRUNC` flags for `OpenFile`.

Network commands must be pure Go. WASI preview 1 has no sockets, so a WASM
guest cannot open a connection.

### Where to register it

An API command is a policy decision for the shell that embeds Kefka. The
shell does not start host processes. Network access is a
separate promise that you make for your own shell. Register the command in
your own `main`, as `cmd/sophia` does with `snapshot`. If the command
belongs in Kefka itself, add it to `coreutils.Register`. That function is
the home for every pure-Go command, even though the name says coreutils.

### The command

```go
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Xe/kefka/command"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
)

// Impl sends an HTTP GET request and writes the response body to stdout.
type Impl struct {
	// Client sends the request. The embedder sets it. Tests point it at
	// an httptest server. When nil, Exec uses http.DefaultClient.
	Client *http.Client
}

func (i Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("fetch: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("fetch")
	set.SetParameters("URL")

	usage := func() {
		fmt.Fprint(stderr, "Usage: fetch [OPTION]... URL\n")
		fmt.Fprint(stderr, "Send an HTTP GET request and write the response body to standard output.\n\n")
		fmt.Fprint(stderr, "  -H, --header=LINE   add a request header, for example 'Accept: application/json'\n")
		fmt.Fprint(stderr, "  -o, --output=FILE   write the body to FILE instead of standard output\n")
		fmt.Fprint(stderr, "  -f, --fail          exit 22 when the HTTP status is 400 or more\n")
		fmt.Fprint(stderr, "      --help          display this help and exit\n\n")
		fmt.Fprint(stderr, "If FETCH_TOKEN is set in the shell, fetch sends it as a Bearer token.\n\n")
		fmt.Fprint(stderr, "Exit status is 0 on success, 1 when the request fails, 2 on a usage error.\n")
	}
	set.SetUsage(usage)

	headers := set.ListLong("header", 'H', "add a request header")
	output := set.StringLong("output", 'o', "", "write the body to FILE")
	fail := set.BoolLong("fail", 'f', "exit 22 on HTTP errors")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"fetch"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "fetch: %s\n", err)
		usage()
		return interp.ExitStatus(2)
	}
	if *help {
		usage()
		return nil
	}
	if len(set.Args()) != 1 {
		fmt.Fprint(stderr, "fetch: expected exactly one URL\n")
		usage()
		return interp.ExitStatus(2)
	}
	target := set.Args()[0]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		fmt.Fprintf(stderr, "fetch: %s: invalid URL\n", target)
		return interp.ExitStatus(2)
	}
	for _, line := range *headers {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			fmt.Fprintf(stderr, "fetch: %s: header must look like 'Name: value'\n", line)
			return interp.ExitStatus(2)
		}
		req.Header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if ec.Environ != nil {
		if token := ec.Environ.Get("FETCH_TOKEN").String(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	client := i.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "fetch: %s: %s\n", target, reason(err))
		return interp.ExitStatus(1)
	}
	defer resp.Body.Close()

	if *fail && resp.StatusCode >= 400 {
		fmt.Fprintf(stderr, "fetch: %s: HTTP %d\n", target, resp.StatusCode)
		return interp.ExitStatus(22)
	}

	dst := stdout
	var outputFile io.Closer
	if *output != "" {
		f, err := openOutput(ec, *output)
		if err != nil {
			fmt.Fprintf(stderr, "fetch: %s: cannot open for writing\n", *output)
			return interp.ExitStatus(1)
		}
		outputFile = f
		dst = f
	}

	_, copyErr := io.Copy(dst, resp.Body)
	var closeErr error
	if outputFile != nil {
		closeErr = outputFile.Close()
	}
	if err := copyErr; err != nil {
		fmt.Fprintf(stderr, "fetch: %s: %s\n", target, reason(err))
		return interp.ExitStatus(1)
	}
	if closeErr != nil {
		fmt.Fprintf(stderr, "fetch: %s: cannot close output\n", *output)
		return interp.ExitStatus(1)
	}
	return nil
}

// reason returns the cause without URL or filesystem path wrappers.
func reason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		err = uerr.Err
	}
	if perr, ok := errors.AsType[*os.PathError](err); ok {
		err = perr.Err
	}
	return err.Error()
}

func openOutput(ec *command.ExecContext, name string) (io.WriteCloser, error) {
	if ec.FS == nil {
		return nil, errors.New("no filesystem")
	}
	return ec.FS.Create(resolvePath(ec, name))
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
	} else {
		p = path.Join(dir, p)
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "."
	}
	return p
}
```

Register it from your `main` with a client that you control:

```go
reg.Register("fetch", fetch.Impl{
	Client: &http.Client{Timeout: 30 * time.Second},
})
```

### The tests

Start an `httptest.Server` in the test and pass its client on `Impl`. The
server handler routes on the path. Each table case picks a path. Check the
exit code with `errors.As`, not with a bool, because this command has more
than one failure code.

```go
package fetch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	})
	mux.HandleFunc("/echo-accept", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.Header.Get("Accept"))
	})
	mux.HandleFunc("/echo-auth", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.Header.Get("Authorization"))
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not here", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type result struct {
	stdout string
	stderr string
	exit   uint8
	err    error
}

func run(t *testing.T, srv *httptest.Server, fs billy.Filesystem, env []string, args []string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		Environ: expand.ListEnviron(env...),
		FS:      fs,
	}
	err := Impl{Client: srv.Client()}.Exec(context.Background(), ec, args)
	var exit interp.ExitStatus
	errors.As(err, &exit)
	return result{stdout: stdout.String(), stderr: stderr.String(), exit: uint8(exit), err: err}
}

func TestFetch(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		args       []string
		env        []string
		wantStdout string
		wantExit   uint8
		wantErrSub string
	}{
		{
			name:       "writes body to stdout",
			path:       "/ok",
			wantStdout: "hello",
		},
		{
			name:       "forwards a header",
			path:       "/echo-accept",
			args:       []string{"-H", "Accept: application/json"},
			wantStdout: "application/json",
		},
		{
			name:       "sends FETCH_TOKEN as a bearer token",
			path:       "/echo-auth",
			env:        []string{"FETCH_TOKEN=secret"},
			wantStdout: "Bearer secret",
		},
		{
			name:       "http error without --fail writes the body and exits 0",
			path:       "/missing",
			wantStdout: "not here\n",
		},
		{
			name:       "http error with --fail exits 22",
			path:       "/missing",
			args:       []string{"--fail"},
			wantExit:   22,
			wantErrSub: "HTTP 404",
		},
		{
			name:       "malformed header is a usage error",
			path:       "/ok",
			args:       []string{"-H", "nocolon"},
			wantExit:   2,
			wantErrSub: "header must look like",
		},
		{
			name:     "unknown flag is a usage error",
			path:     "/ok",
			args:     []string{"--nope"},
			wantExit: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t)
			args := append([]string{}, tt.args...)
			args = append(args, srv.URL+tt.path)
			got := run(t, srv, memfs.New(), tt.env, args)
			if got.exit != tt.wantExit {
				t.Errorf("exit = %d, want %d; stderr=%q", got.exit, tt.wantExit, got.stderr)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got.stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(got.stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", got.stderr, tt.wantErrSub)
			}
		})
	}
}

func TestOutputFile(t *testing.T) {
	srv := newServer(t)
	fs := createOnlyFS{memfs.New()}
	got := run(t, srv, fs, nil, []string{"-o", "out.txt", srv.URL + "/ok"})
	if got.err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", got.err, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	f, err := fs.Open("out.txt")
	if err != nil {
		t.Fatalf("open out.txt: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "hello" {
		t.Errorf("out.txt = %q, want %q", data, "hello")
	}
}

type closeErrorFS struct{ billy.Filesystem }

func (fs closeErrorFS) Create(name string) (billy.File, error) {
	f, err := fs.Filesystem.Create(name)
	if err != nil {
		return nil, err
	}
	return closeErrorFile{f}, nil
}

type closeErrorFile struct{ billy.File }

func (f closeErrorFile) Close() error {
	_ = f.File.Close()
	return errors.New("upload failed")
}
func TestOutputCloseError(t *testing.T) {
	srv := newServer(t)
	got := run(t, srv, closeErrorFS{memfs.New()}, nil, []string{"-o", "out.txt", srv.URL + "/ok"})
	if got.exit != 1 {
		t.Errorf("exit = %d, want 1", got.exit)
	}
	if !strings.Contains(got.stderr, "fetch: out.txt: cannot close output") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

func TestMissingURL(t *testing.T) {
	got := run(t, newServer(t), memfs.New(), nil, nil)
	if got.exit != 2 {
		t.Errorf("exit = %d, want 2", got.exit)
	}
	if !strings.Contains(got.stderr, "expected exactly one URL") {
		t.Errorf("stderr = %q", got.stderr)
	}
}

func TestConnectionRefused(t *testing.T) {
	srv := newServer(t)
	url := srv.URL
	srv.Close()
	got := run(t, srv, memfs.New(), nil, []string{url + "/ok"})
	if got.exit != 1 {
		t.Errorf("exit = %d, want 1; stderr=%q", got.exit, got.stderr)
	}
	if strings.Contains(got.stderr, "Get \"") {
		t.Errorf("stderr leaks the url.Error wrapper: %q", got.stderr)
	}
}

func TestHelp(t *testing.T) {
	got := run(t, newServer(t), memfs.New(), nil, []string{"--help"})
	if got.err != nil {
		t.Fatalf("unexpected error: %v", got.err)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	if !strings.Contains(got.stderr, "Usage: fetch [OPTION]... URL") {
		t.Errorf("usage line missing from stderr: %q", got.stderr)
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct{ name, dir, input, want string }{
		{"root", ".", "/", "."},
		{"empty working directory", "", "file", "file"},
		{"relative file", "work", "file", "work/file"},
		{"absolute file", "work", "/file", "file"},
		{"parent within root", "work/sub", "../file", "work/file"},
		{"relative escape", ".", "../private", "."},
		{"absolute escape", "work", "/../private", "."},
		{"nested escape", "work", "../../private", "."},
		{"bare parent", ".", "..", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePath(&command.ExecContext{Dir: tt.dir}, tt.input)
			if got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

// Model a backend that supports Create but rejects OpenFile flags.
type createOnlyFS struct{ billy.Filesystem }

func (fs createOnlyFS) OpenFile(string, int, os.FileMode) (billy.File, error) {
	return nil, errors.New("unsupported open flag")
}

type pathErrorWriter struct{}

func (pathErrorWriter) Write([]byte) (int, error) {
	return 0, &os.PathError{Op: "write", Path: "/host/private/output", Err: errors.New("disk full")}
}
func TestOutputPathError(t *testing.T) {
	srv := newServer(t)
	var stderr bytes.Buffer
	target := srv.URL + "/ok"
	ec := &command.ExecContext{Stdout: pathErrorWriter{}, Stderr: &stderr}
	err := (Impl{Client: srv.Client()}).Exec(context.Background(), ec, []string{target})
	if !errors.Is(err, interp.ExitStatus(1)) {
		t.Errorf("error = %v, want exit 1", err)
	}
	if want := "fetch: " + target + ": disk full\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}
```

The tests use only the loopback network. `srv.Client()` only reaches the test
server. This keeps `go test ./...` fast and offline.

## Run a child command

Some commands run another command. `time CMD` and `env NAME=VALUE CMD` are
the examples in this repo. Do not call the registry from your command. Use
`ec.Runner` so that the child goes through the same handler chain as a
typed command. This includes shell functions and the `cd` interception.

The pattern from `command/internal/time/time.go` is:

1. If `ec.Runner` is `nil`, write `<name>: exec not available` to stderr and return 127.
2. Quote each argument with `syntax.Quote(arg, syntax.LangBash)`.
3. Join the quoted words and parse them with `syntax.NewParser(syntax.Variant(syntax.LangBash))`.
4. Make a subshell with `ec.Runner.Subshell()`.
5. Wire stdio with `interp.StdIO(ec.Stdin, ec.Stdout, ec.Stderr)(sub)`.
6. Return `sub.Run(ctx, prog)`.

Tests for these commands need a real runner. Copy the `newRunner` helper from
`command/internal/time/time_test.go`. It builds an `interp.Runner` whose exec
handler dispatches into a `registry.Impl` that the test filled.

## Embed Kefka in your own shell

For a binary inside the Kefka module, wire the registry into `interp.New`
as `cmd/kefka/main.go` does. External modules cannot import `internal/billysh`.
External embedders must copy or implement these handlers in their own module.
The important parts are:

```go
reg := registry.New()
coreutils.Register(reg)
wasmprog.Register(reg)
uutils.Register(reg)
reg.Register("mycmd", mycmd.Impl{})

fsys := memfs.New() // or osfs.New("."), or s3fs

var sh *interp.Runner
middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		return reg.Exec(ctx, fsys, sh, args)
	}
}

sh, err := interp.New(
	interp.StdIO(os.Stdin, os.Stdout, os.Stderr),
	interp.ExecHandlers(middleware),
	interp.CallHandler(billysh.CallHandler(reg, fsys, os.Stdout, os.Stderr)),
	interp.StatHandler(billysh.FsysStatHandler(reg, fsys)),
	interp.OpenHandler(billysh.FsysOpenHandler(reg, fsys)),
	interp.ReadDirHandler2(billysh.FsysReadDirHandler(reg, fsys)),
)
```

The `billysh` handlers route the shell's own file operations (redirections,
globs, `test -f`) through `fsys`. The open handler is read-only on purpose.
The registry is the only path that writes to the filesystem. This keeps one
place to audit.

## Checklist

Before you open a pull request, make sure that:

- [ ] The package exports a zero-value `Impl` that implements `command.Execer`.
- [ ] `Exec` guards against `nil` `ec`, `Stdin`, `Stdout`, `Stderr`, and `FS`.
- [ ] Flags come from `getopt/v2`, with `--help` added by hand.
- [ ] `--help` goes to stderr and returns `nil`.
- [ ] Every stderr line has the form `<name>: <arg>: <reason>`.
- [ ] No Go error value leaks to the user.
- [ ] Every filesystem call goes through `ec.FS` with `resolvePath`.
- [ ] Failures return `interp.ExitStatus`, with the same code as GNU.
- [ ] Output matches GNU coreutils or `just-bash` byte for byte.
- [ ] The command is registered in alphabetical order.
- [ ] The test file covers the list in step 5.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Source and output text are ASCII only.

## Commit

Use the `conventional-commits` skill. Stage the files by name:

```bash
git add command/internal/rev/rev.go \
        command/internal/rev/rev_test.go \
        command/registry/coreutils/coreutils.go
git commit -s -m "feat(command): add rev"
```

Never use `git add -A`.

## Related reading

- `AGENTS.md` for the house rules and required skills
- `.claude/skills/just-bash-port/SKILL.md` for porting a command from `just-bash`
- `docs/posix2018/CONFORMANCE.md` for the POSIX audit of existing commands
- `docs/sophia.md` for the SSH server that embeds Kefka
