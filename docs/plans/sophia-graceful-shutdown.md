# Sophia: graceful shutdown on SIGINT/SIGTERM

## Context

Sophia (`cmd/sophia/main.go`) is a long-running SSH server. Each accepted SSH
session forks a Tigris bucket (`*bucket + "-" + sessID`) and registers a
`defer` to delete that bucket when the session ends (lines 115–134). The
defer uses `context.Background()` with a 1-minute timeout, so it's already
robust against `sess.Context()` cancellation.

The problem: Sophia today calls `ssh.ListenAndServe(...)` and installs no
signal handler. When the operator presses Ctrl+C (or Kubernetes sends
SIGTERM — Sophia is now deployed via k8s manifests per commit `0854199`),
the Go runtime's default SIGINT/SIGTERM behavior terminates the process
immediately. Active SSH session goroutines never get to run their deferred
`DeleteBucket`, and the forked session buckets pile up as orphans in
Tigris.

Goal: trap SIGINT/SIGTERM, stop accepting new connections, allow in-flight
sessions a brief grace period to finish, then forcefully close remaining
connections — and in all cases wait for each session's bucket-delete
defer to actually complete before exiting.

## Approach

Replace the convenience call `ssh.ListenAndServe(...)` with an explicit
`ssh.Server` so we have access to its `Shutdown(ctx)` and `Close()`
methods. `gliderlabs/ssh@v0.3.8/server.go` confirms:

- `Shutdown(ctx)` closes the listener and blocks on the internal
  `connWg` until every connection handler (i.e. every `runKefka`
  goroutine, including its defers) has returned, or until `ctx` expires.
- `Close()` closes the listener AND every active `*ssh.ServerConn`
  immediately, which unblocks `sess.Read()` in our pump goroutine and
  `term.Terminal.ReadLine()` in the parser loop, causing each handler to
  unwind through its defers.
- `closeDoneChanLocked` (line 404) is idempotent, so calling `Shutdown`
  again after `Close` to wait on `connWg` is safe.

Tiered shutdown:

1. Receive SIGINT or SIGTERM via `signal.NotifyContext`.
2. Call `srv.Shutdown(graceCtx)` with a configurable grace period
   (default ~30 s) — gives anyone mid-command a chance to wrap up.
3. If `Shutdown` returns `context.DeadlineExceeded`, call `srv.Close()`
   to forcefully disconnect, then call `srv.Shutdown(forceCtx)` again
   with a longer timeout (~90 s) to wait for the now-unblocked handler
   goroutines to finish their `DeleteBucket` defers.
4. Then return from `run()` cleanly.

No changes are needed to `runKefka`: its existing defer already uses
`context.Background()` with its own 1-minute timeout, so the cleanup
survives `sess.Context()` cancellation. The storage client (created from
`sess.Context()` at line 100) is a stateless S3-over-HTTP client whose
context is only used for setup, so it remains usable from the defer.

Optional courtesy: write a one-line "server is shutting down, please
exit" message to each active session before the grace period. Out of
scope unless requested — it requires tracking sessions ourselves in a
`map[ssh.Session]struct{}` guarded by a mutex.

## Files to modify

- `cmd/sophia/main.go` — only file that needs changes:
  - New imports: `os/signal`, `syscall`.
  - Add CLI flags for `--shutdown-grace` (default `30s`) and
    `--shutdown-force-timeout` (default `90s`), following the existing
    `pflag` pattern at lines 38–43.
  - Rewrite `run()` (lines 68–73) to construct an `&ssh.Server{Addr:
    *bind, Handler: srv.HandleSSH}`, apply `ssh.HostKeyFile(*sshPrivateKey)`
    via `server.SetOption`, run `server.ListenAndServe()` in a
    goroutine reporting into an error channel, and implement the
    tiered-shutdown select described above.
  - Treat `ssh.ErrServerClosed` (exported by `gliderlabs/ssh`, line 16)
    as a normal exit, not an error.

No changes elsewhere. `runKefka`, `s3fs`, `snapshot`, and the registry
are unaffected.

## Verification

1. `go build ./cmd/sophia/...` — compiles.
2. `go vet ./...` — clean.
3. Manual end-to-end:
   - Start Sophia locally: `BUCKET_NAME=<test-bucket> ./sophia
     --ssh-private-key ./var/ssh_host_ed25519_key
     --ssh-public-key  ./var/ssh_host_ed25519_key.pub`
   - In another terminal: `ssh -p 2222 anything@localhost`, run a few
     commands so the session bucket is populated.
   - Before disconnecting, `kill -INT <sophia-pid>` (or Ctrl+C in the
     server terminal).
   - Expect log line `shutting down`, then after the SSH session is
     dropped, the `cleaned up bucket` log line from `runKefka`'s defer.
   - Confirm with the Tigris CLI / console that the forked
     `<test-bucket>-<uuid>` bucket is gone.
4. Stress: open 3 SSH sessions, send SIGTERM, confirm all three
   `cleaned up bucket` lines appear and `tigris bucket list` shows no
   `*-<uuid>` leftovers.
5. Force-close path: open a session, send SIGTERM, wait past the
   `--shutdown-grace` window without disconnecting the SSH client —
   server should force-close the connection, the defer should still
   fire, and the bucket should still be deleted.
