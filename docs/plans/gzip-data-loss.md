# Fix gzip/gunzip data loss on s3fs

## Context

When `gzip <file>` is run inside sophia (which wires up `s3fs.S3FS` as `ec.FS`), the source file is deleted but the `.gz` output ends up empty in the bucket. The user observes this as "gzip deletes the file." There are two cooperating bugs:

1. **Root cause (`s3fs/file.go:162-164`)** — `s3WriteFile.Write` is a stub: `return 0, nil`. It accepts the data, claims success, and never appends to its `bytes.Buffer`. On `Close()` an empty body is uploaded to S3. There are zero tests in `s3fs/` (verified — no `*_test.go` files), so this regressed unnoticed.

2. **Blast amplifier (`command/internal/gzip/gzip.go` and `command/internal/gunzip/gunzip.go`)** — both commands write the output and then delete the source while ignoring every error/return value from `Write` and `Close`. So when the underlying FS lies about a successful write (as s3fs does today, or as any other backend might in the future), data is destroyed silently.

Sophia (`cmd/sophia/main.go:136`) constructs `s3fs.NewS3FS(client, sessBucket)` directly with no wrapper, so this hits every shell session backed by Tigris/S3. The bundled `memfs` used in `gzip_test.go` does honor `Write`, which is why the existing tests pass.

The fix is to (a) make `s3WriteFile.Write` actually write, (b) cover it with a round-trip test so the regression can't return, and (c) harden gzip and gunzip so they refuse to delete the source when the output write didn't go through end-to-end.

## Changes

### 1. Fix `s3fs/file.go` — `s3WriteFile.Write`

Replace the stub at lines 162-164 with a real buffer write:

```go
func (f *s3WriteFile) Write(p []byte) (n int, err error) {
    if f.closed {
        return 0, ErrFileClosed
    }
    return f.buf.Write(p)
}
```

Guarding on `f.closed` matches the precedent already set by `Close()` (line 183) and the read file's `Close()` (line 102).

### 2. Add `s3fs/file_test.go` — round-trip coverage

Create a small test file that exercises `Create` → `Write` → `Close` → `Open` → `Read` against a real-ish S3 surface. The most pragmatic options are:

- **Preferred**: use the `tigrisdata/storage-go` test fakes if they exist, or
- spin up an in-memory `httptest.Server` that handles the small set of S3 operations s3fs hits (`PutObject`, `GetObject`, `HeadObject`, `DeleteObject`) backed by a `map[string][]byte`.

The test must assert that the bytes written via `Write` are recoverable via `Open`/`Read` — that's the invariant the stub was silently breaking. A second case for "Write after Close returns error" locks in the closed-file guard.

If a real S3 fake turns out to be too heavy to land in the same PR, fall back to a unit test that constructs `s3WriteFile` directly with a stub `storage.Client` interface and asserts that the buffer accumulates correctly before `Close()` is invoked. This still catches the regression.

### 3. Harden `command/internal/gzip/gzip.go` — `compressMode` and `decompressMode`

Both functions currently end each per-file branch with the same risky sequence:

```go
outF, err := ec.FS.Create(outputFull)
// ...
outF.Write(compressed)
outF.Close()
if !keep {
    ec.FS.Remove(full)
}
```

Replace each occurrence (lines ~330-350 for compress, ~458-478 for decompress) with the following invariant:

- Capture `n, err := outF.Write(payload)`. Treat both `err != nil` AND `n != len(payload)` as a short write failure.
- Capture `closeErr := outF.Close()`.
- If either step failed: emit `gzip: %s: %v\n` to stderr (unless `quiet`), best-effort `ec.FS.Remove(outputFull)` to delete the corrupt output, set `exitStatus = 1`, and `continue` — **do not** remove the source.
- Only call `ec.FS.Remove(full)` when `!keep` AND both write and close succeeded.

`compressMode` already returns `nil` on a per-file failure today; switch it to the same pattern `listMode`/`testMode` use — accumulate an `exitStatus` and return `interp.ExitStatus(exitStatus)` at the end. That preserves "process every input" behavior while surfacing the failure to the shell.

Apply the same shape to the stdin branch (`outF` is `ec.Stdout` there — still check `n` and the error, just don't try to Remove anything).

### 4. Harden `command/internal/gunzip/gunzip.go` — same pattern

Lines 231-251 have the identical bug. Apply the same fix: capture and check `Write`'s `(n, err)` and `Close`'s error, remove the bogus output on failure, and only `ec.FS.Remove(source)` when both succeeded and `!keep`.

### 5. New tests in `gzip_test.go` and `gunzip_test.go`

Add a "failing filesystem" test that wraps `memfs` and returns a `Write` stub which behaves like the s3fs bug (returns `0, nil`). Assert that after running `gzip hello.txt`:

- `hello.txt` still exists (was NOT removed).
- The exit status is non-zero.
- Stderr contains an error mentioning the file.

Mirror the same for gunzip. This is the regression test that proves the hardening works regardless of what the FS does.

## Critical files

- `s3fs/file.go` — fix `s3WriteFile.Write` stub (lines 162-164).
- `s3fs/file_test.go` — new file, round-trip + closed-guard tests.
- `command/internal/gzip/gzip.go` — harden `compressMode` (~330-350) and `decompressMode` (~458-478).
- `command/internal/gzip/gzip_test.go` — add failing-FS regression test.
- `command/internal/gunzip/gunzip.go` — harden the write-then-remove block (~231-251).
- `command/internal/gunzip/gunzip_test.go` — add failing-FS regression test.

## Verification

1. `go test ./s3fs/...` — new round-trip test passes; would fail with the old stub.
2. `go test ./command/internal/gzip/... ./command/internal/gunzip/...` — existing tests stay green, new failing-FS tests pass.
3. `go build ./...` — no compile regressions across the tree.
4. Manual smoke against a real bucket (or `cmd/s3fs-test`):
   - Upload a small text object.
   - In a sophia shell, run `gzip <name>`; verify the `.gz` object is non-empty and `gunzip -t` reports OK.
   - Run `gzip <name>` again with a filesystem deliberately broken (or just trust the regression test) and confirm the source object is preserved.
