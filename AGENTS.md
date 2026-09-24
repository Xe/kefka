# AI Agents

This file provides guidance to AI agents when working with code in this repository.

## Required skills

- Go code: **xe-go-style**
- Tests: **go-table-driven-tests**
- Commits: **conventional-commits**
- Porting from `vercel-labs/just-bash`: **just-bash-port**

## What this repo is

Kefka is a virtual shell. `mvdan.cc/sh/v3` parses bash, but every external
command is dispatched through an in-process registry against a
`billy.Filesystem` (memfs, osfs, or s3fs). Nothing ever `exec`s into the
host; the only guest processes are the WASM modules under
`command/internal/{jq,python3,qjs,rg}`.

Will eventually become coreutils for [yeet](https://github.com/TecharoHQ/yeet)'s
Windows port. Experimental; POSIX audit is in `docs/posix2018/CONFORMANCE.md`.

## Build, test, run

```bash
go build ./...
go test ./...
go test ./command/internal/cat/...             # one command
go test -run TestCat ./command/internal/cat/   # one test

go run ./cmd/kefka -c 'ls; pwd'                # one-shot
go run ./cmd/kefka testdata/basic.sh           # script
go run ./cmd/sophia -b :2222 -B my-bucket      # SSH server (needs Tigris creds)
```

`sophia` reads Tigris creds and `BUCKET_NAME` from env or `.env`. Each SSH
session forks the bucket via `client.CreateBucketFork` and deletes the fork
on disconnect.

## Architecture

**`command.Execer`** (`command/command.go`) is the one interface everything
implements. `ExecContext` carries stdio, a fsys-relative `Dir`, the
`billy.Filesystem`, and the active `*interp.Runner` (so commands like `time`
can re-enter the shell via `Runner.Subshell()`). Return exit codes as
`interp.ExitStatus(uint8(n))`.

**Registry** (`command/registry`) owns the `name -> Execer` map and the
fsys-relative `pwd`. Pwd is registry state, not interp state: interp's `Dir`
is host-rooted and unsafe here, so `billysh.CallHandler` intercepts
`cd` and `pwd` before interp's builtins. `reg.Resolve(p)` clamps paths to
the fsys root.

**Two register entry points** under `command/registry/`:

- `coreutils.Register` — pure-Go commands. One package per command under
  `command/internal/<name>`, exporting a zero-value `Impl`.
- `wasmprog.Register` — WASM commands. Each embeds `.wasm` via `//go:embed`,
  compiles once in `init()`, instantiates per call. Guest fs is `ec.FS`
  adapted through `wasm/billyfs.New`.

Both are alphabetised; keep new entries in order.

**Entry binaries**:

- `cmd/kefka` — local CLI over `osfs.New(".")`. The billysh `Open` handler
  is read-only on purpose so the registry stays the only path for mutation.
- `cmd/sophia` — SSH server over per-session forked Tigris buckets. SSH
  gives a raw byte channel, not a PTY, so sophia does manual line discipline
  (CR->LF, software echo, Ctrl-D closes the command's stdin pipe). Stdio
  must be `*os.File` pipes; wazero reports non-`*os.File` stdio as
  `FILETYPE_BLOCK_DEVICE`, breaking `isatty` in `python.wasm` and `qjs.wasm`.

**Filesystems**: `billy.Filesystem` everywhere. `s3fs/` (exposed) wraps
Tigris via `tigrisdata/storage-go`. `wasm/billyfs/` adapts billy into
wazero's `experimental/sys.FS`.

## Conventions

- Tests sit next to code; `command/internal/cat/cat_test.go` is canonical.
  Cover: file arg, `-` stdin, no-arg stdin, empty input, each flag,
  `--help`, unknown flag, missing file.
- Match GNU coreutils / just-bash byte-for-byte. Stderr is
  `<name>: <arg>: <reason>\n`. Do not leak `%v` of `*os.PathError`.
- ASCII only. Stage files explicitly; never `git add -A`.
- If you add or update a `.wasm` blob, run `wasm/shrink.sh` on it before
  you commit. The script needs binaryen, wabt, and wasm-tools.
