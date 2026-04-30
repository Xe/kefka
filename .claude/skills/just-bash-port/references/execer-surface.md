# kefka Execer surface area

Everything a port needs to know about the runtime contract a command
plugs into. Read once per port; do not duplicate this content into
SKILL.md.

## The interface

`tangled.org/xeiaso.net/kefka/command/command.go`:

```go
type ExecContext struct {
    Stdin          io.Reader
    Stdout, Stderr io.Writer
    Dir            string          // fsys-relative pwd
    Environ        expand.Environ  // mvdan.cc/sh/v3/expand
    FS             billy.Filesystem
}

type Execer interface {
    Exec(ctx context.Context, ec *ExecContext, args []string) error
}
```

`args` is **already stripped of `argv[0]`** — the registry has split
the command name off before calling `Exec`. Never assume `args[0]` is
the command name.

## Filesystem

`ec.FS` is a `github.com/go-git/go-billy/v5` filesystem, not
`io/fs.FS`. Differences from stdlib:

- `Open(name) (billy.File, error)` — note the return type
- `Stat(name) (os.FileInfo, error)` — yes, `os.FileInfo`, billy still
  uses the legacy alias
- `OpenFile`, `ReadDir`, `Stat`, `Symlink` (for the `billy.Symlink`
  capability check) all live on the interface
- For tests, use `github.com/go-git/go-billy/v5/memfs.New()`

A nil `ec.FS` is possible (some shell paths don't pass one). Guard:

```go
if ec.FS == nil {
    return errors.New("<name>: ExecContext has no filesystem")
}
```

…or, if the command is fine without one (e.g. reads only stdin), just
gate the file-reading branch.

## Path resolution

Every command that touches `ec.FS` needs the same `resolvePath`
helper. Copy it verbatim from `command/internal/ls/ls.go`:

```go
func resolvePath(ec *command.ExecContext, p string) string {
    dir := ec.Dir
    if dir == "" {
        dir = "."
    }
    if path.IsAbs(p) {
        p = strings.TrimPrefix(p, "/")
        if p == "" {
            return "."
        }
        return path.Clean(p)
    }
    joined := path.Join(dir, p)
    if joined == "" {
        return "."
    }
    return joined
}
```

Why: shell-absolute paths like `/foo` map to fsys-relative `foo`, and
relative paths join against `ec.Dir`. The registry's `Resolve` does
similar work but on a different layer (pwd state) — it does **not**
rewrite paths before they reach `Exec`.

## Exit codes

Return one of:

- `nil` — exit code 0
- `interp.ExitStatus(uint8(code))` from `mvdan.cc/sh/v3/interp` —
  arbitrary nonzero
- a Go `error` — propagated as exit code 1 with the error printed by
  the shell

Convention used by `ls` and matched by GNU coreutils:

| Code | Meaning |
|------|---------|
| 0    | success |
| 1    | runtime error (invalid input, bad data) |
| 2    | usage error (unknown flag, missing file) |
| 127  | command not found (registry, not commands) |

To pair an error with an exit code, wrap with `errors.Join`:

```go
return errors.Join(err, interp.ExitStatus(2))
```

## I/O patterns

- **`stdout` and `stderr` may be nil.** Defend with `io.Discard`:

  ```go
  stdout := ec.Stdout
  if stdout == nil { stdout = io.Discard }
  ```

- **`stdin` may be nil too.** Treat nil stdin as empty input, not as
  an error.

- Write bytes with `io.Writer.Write([]byte)` for binary-safe output;
  use `io.WriteString` or `fmt.Fprint`/`fmt.Fprintf` for text.

- Do **not** add trailing newlines unconditionally. Match the
  just-bash command's exact behavior — many emit a trailing newline
  only when output is non-empty.

## Reference implementations to mirror

| File | Why read it |
|------|-------------|
| `command/internal/hostname/hostname.go` | Smallest possible Execer; shows nil-guard pattern. |
| `command/internal/truecmd/truecmd.go` | Returns nil on success. |
| `command/internal/falsecmd/falsecmd.go` | Returns `interp.ExitStatus(1)`; package-name workaround for Go reserved word. |
| `command/internal/ls/ls.go` | Full pattern: flag parsing, path resolution, billy filesystem use, multi-mode output, exit-status reporting. |
| `command/internal/ls/ls_test.go` | Canonical table-driven test layout for a command. |

## Registration

Every command is registered exactly once in
`command/registry/coreutils/coreutils.go`. Both the imports and the
`reg.Register` calls are alphabetised — keep that.

```go
import (
    "tangled.org/xeiaso.net/kefka/command/internal/base64"
    "tangled.org/xeiaso.net/kefka/command/internal/falsecmd"
    // ...
)

func Register(reg *registry.Impl) {
    reg.Register("base64", base64.Impl{})
    reg.Register("false", falsecmd.Impl{})
    // ...
}
```

The registry calls `Exec` with an `ExecContext` it builds from the
shell's `interp.HandlerCtx(ctx)` — so the contract above is what the
real runtime delivers.
