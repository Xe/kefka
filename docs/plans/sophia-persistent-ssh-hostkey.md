# Sophia: persistent SSH host keys via flags + docs

## Context

`cmd/sophia` is an SSH server that drops connecting users into an isolated
shell backed by a per-session Tigris bucket fork. Today the server calls
`ssh.ListenAndServe(*bind, srv.HandleSSH)` with no host key option, so
[`gliderlabs/ssh`](https://pkg.go.dev/github.com/gliderlabs/ssh) generates a
fresh ephemeral host key on every startup. That makes the host key
"trust-on-first-use" record useless across restarts — clients see a host key
mismatch and refuse to reconnect.

We want sophia to load a persistent host key from disk, configurable via
flags or environment variables, and we want `docs/sophia.md` to explain what
sophia is and how to operate it (no docs exist yet — only POSIX utility
references live under `docs/`).

## Changes

### 1. `cmd/sophia/main.go`

**Add two flags** to the existing `var ( ... )` block at lines 36–43,
following the established `pflag.StringP` pattern used for `bind`, `bucket`,
and `timeout`:

```go
sshPrivateKey = pflag.StringP(
    "ssh-private-key", "k",
    cmp.Or(os.Getenv("SSH_PRIVATE_KEY"), "./var/ssh_host_ed25519_key"),
    "path to the SSH host private key (PEM)",
)
sshPublicKey = pflag.StringP(
    "ssh-public-key", "K",
    cmp.Or(os.Getenv("SSH_PUBLIC_KEY"), "./var/ssh_host_ed25519_key.pub"),
    "path to the SSH host public key",
)
```

Use `cmp.Or` (stdlib `cmp` package) so an empty env var falls through to the
default. Add `"cmp"` to the import block.

**Validate the files in `main`** immediately after `pflag.Parse()` (line 46),
before `run()` is called. Use `os.Stat` on each path; if either is missing or
unreadable, `log.Fatal` with a clear message naming the flag and the
expected path. This matches the project pattern (no helper layer; validation
inline in the entrypoint).

**Wire the private key into the SSH server** by passing
`ssh.HostKeyFile(*sshPrivateKey)` as a third argument to
`ssh.ListenAndServe` at line 57:

```go
return ssh.ListenAndServe(*bind, srv.HandleSSH, ssh.HostKeyFile(*sshPrivateKey))
```

The public key path is validated for operator hygiene (so deployment
mistakes surface at startup) but is not consumed by `gliderlabs/ssh`; the
library derives the public key from the private key. Logging the validated
public key path alongside `bind`/`timeout` in the existing `slog.Info`
"listening" line at line 56 keeps it visible.

### 2. `docs/sophia.md` (new)

Write a single Markdown file modelled on the repo's README tone — direct,
operator-focused. Sections:

- **What it is** — gliderlabs/ssh server that forks a Tigris bucket per
  connection and runs an interactive bash-compatible shell (`mvdan.cc/sh/v3`)
  inside that scratch bucket. Session bucket is deleted on disconnect via
  `Tigris-Force-Delete`. Reference the registry-based command system
  (`coreutils`, `wasmprog`) and that Python/qjs run as WASI guests.
- **Running it** — env vars expected (`BUCKET_NAME`, plus any Tigris auth
  picked up by `tigrisdata/storage-go`), flags table covering all five
  flags (`--bind`, `--bucket`, `--timeout`, `--ssh-private-key`,
  `--ssh-public-key`) with defaults and env var equivalents.
- **Generating host keys** — one ready-to-paste command:
  ```
  mkdir -p var
  ssh-keygen -t ed25519 -N '' -f ./var/ssh_host_ed25519_key
  ```
  Note that `ssh-keygen` writes both files (private at the given path,
  public at `<path>.pub`), which lines up with the defaults.
- **Connecting** — `ssh -p 2222 anything@localhost` and what to expect (motd,
  isolated bucket message, prompt).
- **Session lifecycle** — bucket fork on connect, cleanup on disconnect,
  `--timeout` caps any single command.

Keep it under ~150 lines. No screenshots; this is a CLI tool.

## Files to modify

- `cmd/sophia/main.go` — add imports (`cmp`), two flag vars, validation
  block in `main`, `ssh.HostKeyFile` option in `run`, extend "listening"
  log line.
- `docs/sophia.md` — new file.

## Verification

1. Build: `go build ./cmd/sophia` — must succeed.
2. Missing key path fails fast:
   ```
   ./sophia --ssh-private-key=/nonexistent/key
   ```
   should exit non-zero with a clear message before binding the port.
3. Generate keys and run with defaults:
   ```
   mkdir -p var && ssh-keygen -t ed25519 -N '' -f ./var/ssh_host_ed25519_key
   BUCKET_NAME=<some-bucket> ./sophia
   ```
   Server logs the public-key path and binds `:2222`.
4. Persistence: connect with `ssh -p 2222 -o StrictHostKeyChecking=accept-new anyone@localhost`, exit, restart sophia, reconnect — second connection must NOT prompt about a changed host key (proves the key is now stable across restarts). Disconnect the first time to confirm the per-session bucket is force-deleted (look for the `cleaned up bucket` log line).
5. Env-var override: `SSH_PRIVATE_KEY=/tmp/other_key SSH_PUBLIC_KEY=/tmp/other_key.pub ./sophia` (after generating that pair) should use those paths; missing them should fail with the same validation error.
