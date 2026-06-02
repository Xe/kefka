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

## Follow-up: pass the shell's working directory to uutils

### Context

After the migration above, `cd <dir>` followed by a relative-path uutils command did not
work: `cd sub; cat hello.txt` looked for `/hello.txt` and failed. The shell tracks its
current directory in `registry.Impl.pwd` (the `cd`/`pwd` builtins are intercepted in
`internal/billysh/billysh.go` and routed through `registry.Chdir`) and propagates it as
`ExecContext.Dir` (`command/registry/registry.go`), but the WASI guest never received it.

A WASI guest resolves its own working directory before requesting files — wazero provides
no host-side cwd, and `/` vs `.` both collapse to a single root mount. Worse, the embedded
`coreutils.wasm` (from `419998e`) had **no working-directory support at all**: its `getcwd`
returned "operation not supported" and it ignored `PWD`. That binary was built before
`pwd-hack.patch` existed, so the chdir-at-startup hook was never compiled in.

Approaches that don't work and why:

- **`WithEnv("PWD", …)` alone** — the unpatched binary ignores `PWD` entirely.
- **chroot the mount to `ec.Dir`** — makes relative paths work but breaks absolute paths:
  `/sub/x` doubles to `/sub/sub/x` and `/` can no longer reach the fsys root. Also diverges
  from the shell's own redirection handling, which resolves against the true root.

### Changes

#### 1. Rebuild `coreutils.wasm` with the cwd patch

`command/uutils/pwd-hack.patch` adds an `.init_array` constructor to `src/bin/coreutils.rs`
that calls `std::env::set_current_dir($PWD)` at startup. This works because Rust's
`wasm32-wasip1` `chdir` maps to wasi-libc `chdir`, which sets the cwd used for relative
path resolution. Fix `command/uutils/regen-coreutils.sh` to install the artifact (it built
into `target/…` but never copied it back — leaving `coreutils.wasm` stale):

```sh
cp target/wasm32-wasip1/release/coreutils.wasm ../../coreutils.wasm
```

Regenerate: `cd command/uutils && ./regen-coreutils.sh` (clones uutils 0.9.0, applies the
patch, builds for `wasm32-wasip1`, installs the embedded `coreutils.wasm`).

#### 2. Hand the cwd to the guest as `PWD`

- `command/command.go` — add `ExecContext.GuestPWD()`, mapping the fsys-relative `Dir` to
  the absolute path the guest expects (`.` → `/`, `home/xe` → `/home/xe`), mirroring the
  `pwd` builtin's formatting.
- `command/uutils/uutils.go` — add `.WithEnv("PWD", ec.GuestPWD())` to the wazero
  `ModuleConfig` so the patched binary chdir's into the shell's current directory at
  startup. The mount stays at the true root `/`, so absolute paths remain correct.

#### 3. Scope

This covers the uutils coreutils binary only. The other WASM commands (`jq`, `rg`, `qjs`,
`python3`) are separate upstream binaries without an equivalent chdir hook, so they still
resolve relative paths against the root. Covering them needs per-binary build-side work.

### Verification

1. `go build ./...`, `go vet ./command/...`, `go test ./command/uutils/ ./command/...`.
2. Regression test `command/uutils/uutils_test.go` (uses `osfs`, the production FS):
   asserts `pwd` reflects `Dir`, relative paths resolve against cwd, absolute paths reach
   the fsys root, and `/sub/x` does not double.
3. End-to-end through the real shell:

   ```text
   cd <tmp-with-sub/greeting.txt> && go run ./cmd/kefka <<'EOF'
   cd sub
   pwd            # -> /sub
   cat greeting.txt   # -> file contents (relative honors cwd)
   cat /rootfile.txt  # -> reads fsys-root file (absolute still works)
   EOF
   ```
