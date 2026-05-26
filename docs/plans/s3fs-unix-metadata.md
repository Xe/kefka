# Plan: Optional Unix-permission metadata in s3fs

## Context

`s3fs/` is a go-billy filesystem mapped onto Tigris (S3) storage, consumed by the
`sophia` SSH server (`cmd/sophia`) and `cmd/s3fs-test`. Today it never reads or
writes POSIX attributes: every file reports mode `0666`, directories `ModeDir`,
uid/gid are absent, and `PutObject` carries no user metadata
(`s3fs/fileinfo.go:21`, `s3fs/file.go:198`).

`docs/reference/how-tigris-fs-unix-metadata.md` defines a convention for storing
Unix attributes as `x-amz-meta-*` headers (uid, gid, mode, rdev, mtime,
`--symlink-target`). This change implements that convention in s3fs as an
**opt-in** feature with three session knobs: **username**, **group name**, and
**umask**.

### Constraints / assumptions (from user)

- **Off by default in sophia.** When disabled, behavior is byte-for-byte what it
  is today.
- Knobs are configured values (not derived from `sess.User()`).
- **s3fs deals in numeric uid/gid.** The package does not resolve names→IDs or
  IDs→names itself; it exposes optional helper functions and lets callers decide
  whether/how to resolve.
- Scope = **create-time write + read** (uid/gid/mode/mtime). No chmod/chown
  writeback (`billy.Change`) and no symlink/device support in this pass — those
  are noted as follow-ups.

## Approach

### 1. New package `s3fs/unixmeta`

New file `s3fs/unixmeta/unixmeta.go` implementing the doc verbatim:

- `Attrs` struct, `PosixMode(os.FileMode) uint32`, `GoFileMode(uint32) os.FileMode`,
  `Encode(Attrs) map[string]string`, `Decode(meta map[string]string, defaults Attrs) Attrs`.
- Provide optional, caller-invoked helpers (the package itself never calls them;
  callers decide whether to resolve names):
  - `LookupUID(name string) (uint32, error)` — `user.Lookup`, fall back to parsing
    `name` as a decimal uint32.
  - `LookupGID(name string) (uint32, error)` — `user.LookupGroup`, same fallback.
    No reverse (uid/gid → name) resolution is provided.
- Table-driven tests `unixmeta_test.go`: PosixMode/GoFileMode round-trip across
  file/dir/symlink/setuid/sticky; Encode→Decode round-trip; malformed-header
  tolerance; missing-key-keeps-default. Follow `go-table-driven-tests` + `xe-go-style`.

### 2. Opt-in config on `S3FS` (`s3fs/filesystem.go`)

Add a nil-able config (nil = disabled, preserving current behavior):

```go
type unixMetaConfig struct { uid, gid uint32; umask os.FileMode }

type S3FS struct {
    client *storage.Client
    bucket string
    root, separator string
    unixMeta *unixMetaConfig // nil => feature off
}

type Option func(*S3FS)
func WithUnixMetadata(uid, gid uint32, umask os.FileMode) Option

func NewS3FS(client *storage.Client, bucket string, opts ...Option) (billy.Filesystem, error)
```

Variadic options keep both existing callers (`cmd/sophia`, `cmd/s3fs-test`)
compiling unchanged.

### 3. Write path (`s3fs/file.go`)

Thread `*unixMetaConfig` into `newS3WriteFile` and `newS3MultipartUploadFile`
(plumbed from `OpenFile` in `s3fs/basic.go`). In each `Close()`:

- If config is nil → unchanged (no `Metadata`).
- Else set `PutObjectInput.Metadata = unixmeta.Encode(unixmeta.Attrs{UID, GID,
Mode: 0o666 &^ umask, Mtime: time.Now()})`. (Multipart: `Metadata` on
  `CreateMultipartUploadInput` at construction.)

### 4. Read path (`s3fs/basic.go`, `s3fs/fileinfo.go`)

- Extend `s3FileInfo` with `uid, gid uint32`; make `Sys()` return a small struct
  exposing `Uid()/Gid()` (instead of `nil`) so consumers can read the raw numeric
  ownership and resolve to names themselves if they want.
- Add `newFileInfoFromHead(name, head, cfg)` that, when `cfg != nil`, runs
  `unixmeta.Decode(head.Metadata, defaults)` with defaults `{UID: cfg.uid,
GID: cfg.gid, Mode: 0o644, Mtime: head.LastModified}` and builds a fully
  populated `s3FileInfo`. When `cfg == nil`, keep the current `0666` path.
- Wire this into `Stat` (`s3fs/basic.go:127`). `Lstat` already delegates to `Stat`.
- **ReadDir** (`s3fs/dir.go`): `ListObjectsV2` does not return user metadata, so
  list entries keep default modes; full attributes come from `Stat`. (`ls` stats
  entries for the long format.) Documented limitation; avoids an N-Head fan-out.

### 5. sophia wiring (`cmd/sophia/main.go`) — default OFF

Add flags, all defaulting to the disabled state:

- `--fs-unix-metadata` (bool, default **false**)
- `--fs-user` (string), `--fs-group` (string), `--fs-umask` (e.g. octal string, default `022`)

At `s3fs.NewS3FS` (line 136): only pass `WithUnixMetadata(...)` when
`--fs-unix-metadata` is true. sophia (the caller) resolves the `--fs-user` /
`--fs-group` strings to numeric IDs using the `unixmeta.LookupUID/LookupGID`
helpers and passes numbers into `WithUnixMetadata`. When the flag is false, call
`NewS3FS(client, sessBucket)` exactly as today.

### 6. (Optional, same PR if desired) `ls -l` ownership display

`command/internal/ls/ls.go:387-397` hardcodes owner/group to `"user"`/`"0"`.
Could read the numeric uid/gid from `info.Sys()` when present (numeric display
only; no name resolution). **Out of scope by default** — mode bits already
display correctly once `Stat` returns decoded mode.

## Files touched

- `s3fs/unixmeta/unixmeta.go` (new), `s3fs/unixmeta/unixmeta_test.go` (new)
- `s3fs/filesystem.go` — config + `Option` + `WithUnixMetadata`
- `s3fs/basic.go` — plumb config into `OpenFile`; decode in `Stat`
- `s3fs/file.go` — encode metadata in write/multipart `Close`
- `s3fs/fileinfo.go` — uid/gid fields, `Sys()`, decode constructor
- `cmd/sophia/main.go` — opt-in flags (default off)
- `cmd/s3fs-test/main.go` — optional flags to exercise the feature

## Verification

- `go test ./s3fs/...` — unixmeta round-trip + tolerance tests pass.
- `go build ./...` — both commands still compile.
- Manual (needs Tigris creds + `BUCKET_NAME`): run `cmd/s3fs-test` with the
  feature on; write a file, then `HeadObject` (via `tigris-objects` MCP / aws cli)
  and confirm `x-amz-meta-uid/gid/mode/mtime` are present and correct; read it
  back and confirm `Stat().Mode()` reflects `0666 &^ umask`.
- Confirm default path: run sophia without `--fs-unix-metadata`, write a file,
  `HeadObject` shows **no** `x-amz-meta-*` headers (regression guard).

## Deferred (not in this change)

- `billy.Change` (chmod/chown/chtimes) writeback via metadata-rewriting CopyObject.
- Symlink target / device-node (`rdev`) storage.
- Per-entry metadata in `ReadDir`.
