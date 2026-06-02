# Delete internal command implementations now covered by uutils

## Context

A previous commit (`419998e feat: add uutils implementations of coreutils`) introduced
`command/registry/uutils/uutils.go`, which registers 38 coreutil commands backed by the
`uutils` implementation. In the `kefka` CLI, `uutils.Register` runs _after_
`coreutils.Register`, so the uutils versions already override the hand-written
implementations in `command/internal/*`. Those internal implementations are now dead
weight: redundant code + tests to maintain for commands that are served by uutils.

Goal: delete the internal implementations for the 38 commands that overlap with the
uutils list, drop their wiring from `coreutils.go`, remove the now-orphaned `checksum`
helper, and wire `uutils.Register` into `sophia` so it keeps serving those commands.

## Commands in scope (38)

These appear in BOTH `command/registry/uutils/uutils.go` and `coreutils.go`:

base64, basename, cat, cp, cut, date, dirname, expand, false (`falsecmd/`), fold, head,
join, ls, md5sum, mkdir, mv, nl, od, paste, printenv, printf, readlink, rm, rmdir, seq,
sha1sum, sha256sum, sleep, split, tail, tee, touch, tr, true (`truecmd/`), unexpand,
uniq, wc

**Kept** (no uutils overlap — leave untouched): clear, column, commands, diff, du, env,
expr, file, gunzip, gzip, hostname, pwd, stat, tac, time, tree, whoami, zcat, plus the
wasmprog commands (jq, python3, qjs, rg).

## Changes

### 1. Delete internal implementation directories

Remove these 38 directories under `command/internal/` (each contains `*.go` impl + tests):

`base64/ basename/ cat/ cp/ cut/ date/ dirname/ expand/ falsecmd/ fold/ head/ join/ ls/
md5sum/ mkdir/ mv/ nl/ od/ paste/ printenv/ printf/ readlink/ rm/ rmdir/ seq/ sha1sum/
sha256sum/ sleep/ split/ tail/ tee/ touch/ tr/ truecmd/ unexpand/ uniq/ wc/`

Also delete the orphaned helper `command/internal/checksum/` — it is imported only by
md5sum/sha1sum/sha256sum (`grep` confirms: `command/internal/{md5sum,sha1sum,sha256sum}`),
all of which are being removed.

### 2. Edit `command/registry/coreutils/coreutils.go`

- Remove the 38 corresponding `import` lines (lines 4–56 range — drop the overlapping
  packages, keep the non-overlapping ones listed above plus the `registry` import).
- Remove the 38 matching `reg.Register("<name>", <pkg>.Impl{})` lines in the
  `Register` func (lines 63–115 range).
- After editing, the file should register only the 22 kept commands.

### 3. Wire uutils into `cmd/sophia/main.go`

Sophia currently registers only `coreutils` + `wasmprog` (lines 203–204) and never
`uutils`, so without this it would lose all 38 commands.

- Add import `"tangled.org/xeiaso.net/kefka/command/registry/uutils"` (near lines 31–32).
- Add `uutils.Register(reg)` alongside the existing `coreutils.Register(reg)` /
  `wasmprog.Register(reg)` calls (after line 204), matching the ordering in
  `cmd/kefka/main.go` (coreutils → wasmprog → uutils).

### 4. No change needed in `cmd/kefka/main.go`

It already calls `coreutils.Register` then `uutils.Register`, so the deleted commands
remain available via uutils.

## Verification

1. `go build ./...` — confirms no dangling imports and both binaries compile.
2. `go vet ./...` — catch any unused-import / reference issues.
3. `go test ./...` — remaining tests pass; the deleted commands' tests are gone with them.
4. Sanity-check that both binaries still expose the commands:
   - `go run ./cmd/kefka cat <file>` (and e.g. `ls`, `wc`, `true`) — served by uutils.
   - For sophia, confirm the registry includes the names (e.g. via its `commands`
     listing) after adding `uutils.Register`.
5. Confirm a kept command still works through the internal path: `go run ./cmd/kefka tree`
   or `gzip`/`stat`.
