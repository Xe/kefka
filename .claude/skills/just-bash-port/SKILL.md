---
name: just-bash-port
description: Port a TypeScript command from vercel-labs/just-bash to a Go command.Execer in the kefka repo. Use when the user asks to "port" a just-bash command, references a just-bash source URL (raw.githubusercontent.com/vercel-labs/just-bash/...), or wants to add a new coreutil-style command (base64, cat, head, wc, etc.) to kefka.
---

# Porting a just-bash command to kefka

This skill ports a single TypeScript command from
`vercel-labs/just-bash` (Node-based busybox-like shell) into a Go
implementation that satisfies `tangled.org/xeiaso.net/kefka/command.Execer`.
Flag parsing uses `github.com/pborman/getopt/v2` (already an indirect
dependency of kefka).

## When to invoke

The user request typically looks like one of:

- "Port command https://raw.githubusercontent.com/vercel-labs/just-bash/.../<name>.ts ..."
- "Add a `<name>` command to kefka based on just-bash"
- "Reimplement the `<name>` coreutil in kefka"

If the user does not specify a flag-parsing library, default to
`github.com/pborman/getopt/v2`.

## Required reading before coding

Read these references **once** at the start of the port. They are short
and contain the surface-area knowledge needed to make safe choices:

- [references/execer-surface.md](references/execer-surface.md) — `command.Execer`
  interface, `ExecContext` fields, exit-code conventions, the billy filesystem,
  and the path-resolution helper used across kefka commands.
- [references/getopt-v2.md](references/getopt-v2.md) — how to wire
  `pborman/getopt/v2`, how to set the usage line, how to expose `--help`,
  and how to map TypeScript `parseArgs` flag definitions onto getopt
  primitives.

## Port workflow

Follow these steps in order. Do not skip steps unless the user has said
the work is already done.

### 1. Fetch the source

```bash
curl -s https://raw.githubusercontent.com/vercel-labs/just-bash/refs/heads/main/packages/just-bash/src/commands/<name>/<name>.ts
```

Read it end-to-end. Note:

- the help block (name, summary, usage, options) — copy this verbatim into the Go usage func
- the `argDefs` object — each entry maps to one getopt flag
- positional args (`parsed.result.positional`) — accessed via `set.Args()` in Go
- whether stdin/files are read as binary (`readFileBuffer`) or as text
- the error/exit-code paths (`exitCode: 1`, `exitCode: 2`)

### 2. Survey the kefka conventions

Before writing code, skim two existing commands so the new one matches
the house style:

- `command/internal/hostname/hostname.go` — minimal `Execer`
- `command/internal/ls/ls.go` — full-featured command with flag
  parsing, `resolvePath`, billy filesystem use, and exit-status
  reporting

The reference file [references/execer-surface.md](references/execer-surface.md)
summarises both, but reading the actual files anchors the port to
current code.

### 3. Create the package

Layout:

```
command/internal/<name>/<name>.go
command/internal/<name>/<name>_test.go
```

Package naming rules:

- Use the command name (`base64`, `cat`, `wc`, ...) as the package name when it isn't a Go reserved word.
- For Go reserved words (`true`, `false`, `for`, ...), append `cmd` — e.g. `truecmd`, `falsecmd`. The directory name follows the package name.
- If the package name collides with a stdlib import you need (e.g. `encoding/base64`), import the stdlib under an alias (`enc "encoding/base64"`) rather than renaming the package — keeps the registry call site readable.

The file exports a single zero-value `Impl` type:

```go
type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error { ... }
```

### 4. Wire flag parsing with getopt/v2

See [references/getopt-v2.md](references/getopt-v2.md) for the full
recipe. The minimum for any port:

```go
set := getopt.New()
set.SetProgram("<name>")
set.SetParameters("[FILE]") // or whatever positional shape the help block declares

usage := func() {
    fmt.Fprint(stderr, "Usage: <name> [OPTION]... [FILE]\n")
    // ... copy the just-bash help block verbatim, one Fprint per line
}
set.SetUsage(usage)

decode := set.BoolLong("decode", 'd', "decode data")
help   := set.BoolLong("help", 0, "display this help and exit")

if err := set.Getopt(append([]string{"<name>"}, args...), nil); err != nil {
    fmt.Fprintf(stderr, "<name>: %s\n", err)
    usage()
    return interp.ExitStatus(1)
}
if *help {
    usage()
    return nil
}
```

Key gotchas:

- `set.Getopt` expects `args[0]` to be the program name. **Always** prefix
  the user's args with the command name.
- Use rune `0` for "no short alias" (`BoolLong("help", 0, ...)`).
- `--help` is **not** automatic. Add it explicitly and check the bool
  after parsing.
- The default usage prints to stderr; do not print help to stdout — that
  breaks pipelines and matches GNU coreutils behavior.

### 5. Read input correctly

If the just-bash command reads files or stdin (most do), implement a
small helper that:

1. Reads `ec.Stdin` when `len(files) == 0` or `files == ["-"]`.
2. Otherwise concatenates each file via `ec.FS.Open(resolvePath(ec, file))`.
3. Treats `-` inside a file list as stdin.
4. Returns `interp.ExitStatus(1)` with a stderr message
   (`"<name>: <file>: No such file or directory\n"`) on missing files —
   match the just-bash exact string when possible.

The `resolvePath` helper is identical across kefka commands; copy it
verbatim from `command/internal/ls/ls.go`. It is small enough to keep
local rather than extracting to a shared package.

### 6. Match the just-bash output **byte-for-byte**

Subtle parity points to check against the TypeScript:

- **Trailing newline**: many just-bash commands emit a trailing newline
  only when output is non-empty. Mirror this exactly.
- **Wrap behavior** (e.g. `base64 -w COLS`): wrap=0 disables wrapping
  *and* suppresses the trailing newline.
- **Error format**: `<name>: <arg>: <reason>\n` is the standard. Do not
  embed Go error wrapping (`%v` of an `*os.PathError`) into user-facing
  stderr; substitute a fixed message that matches the just-bash output.
- **Exit codes**: `0` success, `1` invalid input or runtime error, `2`
  usage error or missing file (matches GNU coreutils, which is what
  just-bash imitates). Return via `interp.ExitStatus(uint8(code))`.

### 7. Register the command

Add the package to `command/registry/coreutils/coreutils.go`:

```go
import (
    "tangled.org/xeiaso.net/kefka/command/internal/<name>"
    // ... other imports, alphabetical
)

func Register(reg *registry.Impl) {
    reg.Register("<name>", <pkg>.Impl{})
    // ... other registrations, alphabetical
}
```

Keep both the import block and the `Register` body alphabetised — the
existing file does, and diffs stay clean that way.

### 8. Write tests

**Use the `go-table-driven-tests` skill for the test file.** Invoke it
explicitly before writing the test — it encodes the kefka test
conventions (struct table with `name`, `t.Run`, `t.Helper()` in
fixtures, `bytes.Buffer` for stdout/stderr capture) and saves you from
reinventing the layout.

Bare minimum coverage for any port:

- Encode/decode (or whatever the command's primary operation is) via
  stdin
- The same operation via a file argument
- Empty input
- The `-` argument explicitly meaning stdin
- A round trip if the command has an inverse mode (e.g. encode+decode)
- Each declared flag exercised at least once
- `--help` prints the usage line to stderr and returns `nil`
- Unknown flag returns an error
- Missing file returns an error and writes the standard "No such file
  or directory" stderr line

Build the in-memory billy filesystem with `memfs.New()` and seed it
with small fixture files. Pattern:

```go
func newFS(t *testing.T) billy.Filesystem {
    t.Helper()
    fs := memfs.New()
    write := func(name string, data []byte) {
        f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
        if err != nil { t.Fatal(err) }
        f.Write(data)
        f.Close()
    }
    write("hello.txt", []byte("hello"))
    return fs
}
```

`command/internal/ls/ls_test.go` is the canonical example to mirror.

### 9. Verify

```bash
go mod tidy            # promote getopt/v2 from indirect → direct if needed
go build ./...
go test ./command/internal/<name>/...
```

Pre-existing `go vet` failures elsewhere in the repo (e.g.
`wasm/billyfs`) are not introduced by the port — note them but do not
fix them as part of this task.

### 10. Report

Summarise to the user:

- the new package path and its registry entry
- which flags were implemented and any TS behaviour intentionally
  omitted (e.g. browser fallbacks via `atob`/`btoa`, `Buffer`
  detection — Go has neither concern)
- which tests were added
- any pre-existing repo issues observed but not addressed

## Things to deliberately drop

The just-bash sources contain Node/browser-specific scaffolding that
does **not** belong in the Go port:

- `if (typeof Buffer !== "undefined")` branches — Go has one bytes
  type, use it
- `atob` / `btoa` / `String.fromCharCode` — use `encoding/base64`,
  `bytes`, etc.
- `flagsForFuzzing` exports — kefka has no fuzz harness wired up for
  these
- `stdoutEncoding: "binary"` markers — `io.Writer.Write([]byte)` is
  already binary-safe

## Things to keep faithful to

- Help text wording
- Flag short/long names and defaults
- Exit codes and stderr format
- Trailing-newline rules
- Whether files concatenate (most do) or only the first is honoured
