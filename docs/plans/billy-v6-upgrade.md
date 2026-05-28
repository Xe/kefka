# Upgrade kefka to go-billy v6 (full s3fs port from objgit)

## Context

kefka pins `github.com/go-git/go-billy/v5 v5.8.0`. The sibling repo **objgit** is already
on `go-billy/v6 v6.0.0-alpha.1`, and its `internal/s3fs` package is the evolved descendant
of kefka's `s3fs` (same code, comments, and an already-byte-identical `file_test.go`). The
goal is to move kefka to billy v6 and, per the chosen scope, **fully port objgit's s3fs**
(its v6 implementation plus the behavioral work it has since gained: a real `TempFile`/temp
registry, `RenameObject`-based `Rename`, HeadObject-on-open, enriched `Stat`).

kefka does **not** depend on go-git, and objgit's `s3fs` package does not import go-git, so
this upgrade is self-contained — no new top-level dependency enters kefka. `storage-go v0.6.0`
(kefka's pinned version) already exposes `RenameObject`, and kefka's `s3fs/unixmeta` is
byte-identical to objgit's, so neither needs touching.

### What actually breaks in v6 (everything else is mechanical)

`os.FileMode`/`os.FileInfo`/`os.DirEntry` are **type aliases** of the `io/fs` equivalents, so
v6's `os.*` → `fs.*` signature changes are non-breaking. Only two changes have teeth:

1. **`billy.Dir.ReadDir` returns `[]fs.DirEntry`** (was `[]os.FileInfo`). `fs.DirEntry` exposes
   only `Name()/IsDir()/Type()/Info()` — not `Size()/Mode()/ModTime()/Sys()`.
2. **`billy.File` now embeds `fs.File`** (requires `Stat() (fs.FileInfo, error)`) **and adds
   `io.WriterAt`/`io.ReaderAt`**. `Lock()/Unlock()` moved to the optional `billy.Locker`
   (keeping the methods is harmless). The s3fs full port already satisfies all of this; no other
   kefka type implements `billy.File` except via embedding memfs (safe by promotion).

## Change set

### 1. Module + dependencies

- `go.mod` line 10: `github.com/go-git/go-billy/v5 v5.8.0` → `github.com/go-git/go-billy/v6 v6.0.0-alpha.1`.
- Run `go mod tidy` to refresh `go.sum` and indirect deps (e.g. `cyphar/filepath-securejoin`).
  Everything v6 needs is already in the module cache (objgit built with it); use `GOPROXY=off`
  if the network is unavailable.

### 2. Mechanical import swap (all `.go` files)

Replace `go-git/go-billy/v5` → `go-git/go-billy/v6` everywhere (covers the root pkg plus
`/util`, `/memfs`, `/osfs`). 18 non-test + 41 test files. A blanket substitution also fixes the
stale `v5` mention in the `wasm/billyfs/billyfs.go:1` doc comment.

```
grep -rl 'go-git/go-billy/v5' --include='*.go' | xargs sed -i 's#go-git/go-billy/v5#go-git/go-billy/v6#g'
```

Representative files: `cmd/kefka/main.go` (osfs), `command/command.go`, `command/registry/registry.go`,
`internal/billysh/billysh.go`, `wasm/billyfs/billyfs.go`, the 9 command dirs that import billy
(`cp`, `du`, `file`, `ls`, `mkdir`, `pwd`, `readlink`, `rm`, `touch`), and all `*_test.go` using `memfs`.

### 3. Full s3fs port (replace kefka's with objgit's)

Overwrite each `kefka/s3fs/*.go` with the objgit `internal/s3fs/*` equivalent, rewriting only the
one import path `tangled.org/xeiaso.net/objgit/internal/s3fs/unixmeta` →
`tangled.org/xeiaso.net/kefka/s3fs/unixmeta` (present only in `file.go` and `fileinfo.go`):

- `filesystem.go` — adds `temps` registry + `tempMu` and the `key()` helper.
- `basic.go` — `key()`-based paths, temp-file read branch, `RenameObject`-based `Rename`, temp-aware `Remove`.
- `chroot.go` — carries `separator`/`unixMeta`/`temps` into the child FS (fixes a real bug).
- `file.go` — `name`/`head` fields; adds `WriteAt`, `ReadAt`, `Stat` to all four File types; `s3DirFile` gains `bucket`/`cli`; adds `ErrNotImplemented`.
- `dir.go` — `ReadDir` returns `[]fs.DirEntry` (via `fs.FileInfoToDirEntry`); real `MkdirAll`.
- `fileinfo.go` — adds `enrichedFileInfo` (type renamed `s3FileInfo`→`simpleFileInfo` to match).
- `tempfile.go` — real in-memory temp implementation (was a `nil,nil` stub).
- `symlink.go` — unchanged (already identical).
- **New file** `tempfs.go` — `tempBuffer` + temp handles + registry helpers. **Mandatory**:
  `NewS3FS` now initializes `temps`, and `key_test.go`/`tempfs_test.go` exercise it.

Tests: kefka already has the identical `file_test.go`. Add objgit's `chroot_test.go`,
`key_test.go`, `tempfs_test.go` (all `package s3fs`, no import rewrites; `objgit.git` appears
only as test-data strings). These are pure-logic — no live S3 client needed.

Public API is unchanged (`S3FS`, `Option`, `WithUnixMetadata`, `NewS3FS`, `FileStat`), so the two
in-repo consumers — `cmd/sophia/main.go` and `cmd/s3fs-test/main.go` — keep compiling against it.

### 4. ReadDir-consumer fixes (the only hand edits outside s3fs)

These call `billy.Filesystem.ReadDir`, now `[]fs.DirEntry`:

- **`command/internal/du/du.go`** (~207–209): replace `entrySize := e.Size()` with a guarded
  `Info()` (`if info, err := e.Info(); err == nil { entrySize = info.Size() }`), and
  `e.Mode()&fs.ModeSymlink` → `e.Type()&fs.ModeSymlink`. (du does not import `os`.)
- **`command/internal/ls/ls.go`**: line 741 `map[string]os.FileInfo` → `map[string]fs.DirEntry`;
  line 899 `entry.Mode()&fs.ModeSymlink` → `entry.Type()&fs.ModeSymlink` (line 897 `entry.IsDir()`
  unchanged). **Then remove the now-unused `"os"` import (line 10)** — `os.FileInfo` at 741 was its
  only real use. All other `info.Mode()/.Size()/.ModTime()/.Sys()` in ls operate on `os.FileInfo`
  from separate `ec.FS.Stat()` calls and are unaffected.
- **`wasm/billyfs/billyfs.go`**: field `dirEntries []os.FileInfo` (line 96) → `[]stdfs.DirEntry`;
  line 192 `Type: info.Mode() & stdfs.ModeType` → `Type: info.Type()`. (`os` stays used elsewhere.)
- **`internal/billysh/billysh.go`** `FsysReadDirHandler` (40–52): `ReadDir` already returns
  `[]fs.DirEntry`, so collapse the body to `return fsys.ReadDir(reg.Resolve(name))` (drop the
  `fs.FileInfoToDirEntry` loop).
- **`cmd/s3fs-test/main.go:82`**: replace `e.Size()` on the ReadDir entry with a guarded
  `e.Info()` size (e.g. `size := int64(-1); if fi, err := e.Info(); err == nil { size = fi.Size() }`).
  (`info.*` on line 71 is from `Stat()` — leave it.)

No change needed in `cp.go`, `rm.go`, `diff.go`, `gunzip.go`, `rmdir.go`, `tree.go` (entries used
only via `.Name()/.IsDir()/len()`), nor in the test wrappers `lockedFS`/`lockingFS`/`failingFS`/
`silentDropFS` (they embed billy types and delegate to memfs, which satisfies v6 by promotion).

## Verification

1. `go build ./...` — covers `cmd/kefka`, `cmd/sophia`, `cmd/s3fs-test`, `wasm/billyfs`, all commands.
2. `go vet ./...`.
3. `go test ./...` — the 41 memfs-backed command tests are the real regression gate; the 4 s3fs
   logic tests validate the port (chroot separator, `key()`, temp read-while-write, write buffer).
4. Smoke-test the CLI against the real `osfs` (v6 `osfs.New(".")` now always returns a containment-
   enforcing `BoundOS`; `registry.Resolve` already clamps `..`/escapes, so behavior should match):
   build `cmd/kefka`, then run `ls`, `cat <file>`, `cp a b`, `du`, `tree` in a scratch dir and confirm
   output is unchanged.

## Notes / risks

- **go-git is not pulled in** — verified objgit's `s3fs` package imports only billy/v6, aws-s3,
  smithy, storage-go, atomic, std, and unixmeta.
- **`RenameObject` confirmed** present in `storage-go@v0.6.0` (`client.go:91`); no dep bump.
- s3/smithy version drift (objgit slightly newer) is reconciled by `go mod tidy`; no symbol used by
  the ported files is absent at kefka's pinned versions — `go build` confirms.
- The embedded `*.wasm` guests (ripgrep/jq/python/quickjs) are prebuilt WASI binaries that use
  syscalls, not billy; they need no rebuild. `wasm/billyfs` is host-side wazero glue.
