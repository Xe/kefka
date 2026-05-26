package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"github.com/tigrisdata/storage-go"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
	"tangled.org/xeiaso.net/kefka/cmd/sophia/commands/snapshot"
	"tangled.org/xeiaso.net/kefka/command/registry"
	"tangled.org/xeiaso.net/kefka/command/registry/coreutils"
	"tangled.org/xeiaso.net/kefka/command/registry/wasmprog"
	"tangled.org/xeiaso.net/kefka/internal/billysh"
	"tangled.org/xeiaso.net/kefka/s3fs"
	"tangled.org/xeiaso.net/kefka/s3fs/unixmeta"

	_ "embed"

	_ "github.com/joho/godotenv/autoload"
)

var (
	bind                 = pflag.StringP("bind", "b", ":2222", "host:port to bind SSH to")
	bucket               = pflag.StringP("bucket", "B", os.Getenv("BUCKET_NAME"), "the bucket name to constrain sessions to")
	timeout              = pflag.DurationP("timeout", "T", 5*time.Minute, "the total time a command can run for")
	sshPrivateKey        = pflag.String("ssh-private-key", cmp.Or(os.Getenv("SSH_PRIVATE_KEY"), "./var/ssh_host_ed25519_key"), "path to the SSH host private key (PEM)")
	sshPublicKey         = pflag.String("ssh-public-key", cmp.Or(os.Getenv("SSH_PUBLIC_KEY"), "./var/ssh_host_ed25519_key.pub"), "path to the SSH host public key")
	shutdownGrace        = pflag.Duration("shutdown-grace", 30*time.Second, "how long to wait for in-flight SSH sessions to finish after a shutdown signal before forcing them closed")
	shutdownForceTimeout = pflag.Duration("shutdown-force-timeout", 90*time.Second, "how long to wait, after force-closing connections, for per-session bucket cleanup to finish")

	fsUnixMetadata = pflag.Bool("fs-unix-metadata", false, "store POSIX file attributes (uid/gid/mode/mtime) as S3 user metadata")
	fsUser         = pflag.String("fs-user", "0", "owner (name or numeric uid) recorded on written files when --fs-unix-metadata is set")
	fsGroup        = pflag.String("fs-group", "0", "group (name or numeric gid) recorded on written files when --fs-unix-metadata is set")
	fsUmask        = pflag.String("fs-umask", "022", "octal umask applied to new files when --fs-unix-metadata is set")

	//go:embed static/motd
	motd []byte
)

func main() {
	pflag.Parse()

	for _, k := range []struct {
		flag, path string
	}{
		{"--ssh-private-key", *sshPrivateKey},
		{"--ssh-public-key", *sshPublicKey},
	} {
		if _, err := os.Stat(k.path); err != nil {
			log.Fatalf("%s %q: %v", k.flag, k.path, err)
		}
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	srv := New()

	fsOpts, err := buildFSOptions()
	if err != nil {
		return err
	}
	srv.fsOpts = fsOpts

	server := &ssh.Server{
		Addr:    *bind,
		Handler: srv.HandleSSH,
	}
	if err := server.SetOption(ssh.HostKeyFile(*sshPrivateKey)); err != nil {
		return fmt.Errorf("can't set SSH host key: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("listening", "bind", *bind, "timeout", *timeout, "sshPrivateKey", *sshPrivateKey, "sshPublicKey", *sshPublicKey, "unixMetadata", *fsUnixMetadata)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down", "grace", *shutdownGrace, "force_timeout", *shutdownForceTimeout)

	graceCtx, cancelGrace := context.WithTimeout(context.Background(), *shutdownGrace)
	defer cancelGrace()
	if err := server.Shutdown(graceCtx); err != nil {
		slog.Warn("graceful shutdown exceeded grace period, forcing close", "err", err)
		if cerr := server.Close(); cerr != nil {
			slog.Error("force close failed", "err", cerr)
		}
	}

	// gliderlabs/ssh's connWg (which Shutdown waits on) decrements when the
	// connection loop exits, not when our HandleSSH returns. Wait on our own
	// session WaitGroup so each runKefka's deferred DeleteBucket actually runs
	// to completion before main exits.
	done := make(chan struct{})
	go func() {
		srv.sessions.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(*shutdownForceTimeout):
		slog.Error("session cleanup did not complete in force timeout; some buckets may be orphaned")
	}

	if err := <-errCh; err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}

// buildFSOptions resolves the --fs-* flags into s3fs options. When
// --fs-unix-metadata is off it returns nil, so the filesystem behaves exactly as
// it did before the feature existed.
func buildFSOptions() ([]s3fs.Option, error) {
	if !*fsUnixMetadata {
		return nil, nil
	}

	uid, err := unixmeta.LookupUID(*fsUser)
	if err != nil {
		return nil, fmt.Errorf("--fs-user %q: %w", *fsUser, err)
	}
	gid, err := unixmeta.LookupGID(*fsGroup)
	if err != nil {
		return nil, fmt.Errorf("--fs-group %q: %w", *fsGroup, err)
	}
	umask, err := strconv.ParseUint(*fsUmask, 8, 32)
	if err != nil {
		return nil, fmt.Errorf("--fs-umask %q: must be octal: %w", *fsUmask, err)
	}

	return []s3fs.Option{s3fs.WithUnixMetadata(uid, gid, os.FileMode(umask))}, nil
}

type Server struct {
	// sessions tracks in-flight HandleSSH invocations so shutdown can wait
	// for each runKefka's deferred bucket cleanup to complete. gliderlabs/ssh's
	// own connWg counts connection-loop goroutines, which return before the
	// session handler goroutine does, so Server.Shutdown alone is not enough.
	sessions sync.WaitGroup
	fsOpts   []s3fs.Option
}

func New() *Server {
	return &Server{}
}

func (s *Server) HandleSSH(sess ssh.Session) {
	s.sessions.Add(1)
	defer s.sessions.Done()

	sess.Write(motd)

	lg := slog.With("remoteAddr", sess.RemoteAddr().String(), "user", sess.User())
	lg.Info("got connection")

	if err := s.runKefka(sess, lg); err != nil {
		fmt.Fprintln(sess, "internal server error:", err)
		slog.Error("error serving Kefka session", "err", err, "remoteAddr", sess.RemoteAddr().String())
		return
	}
}

func (s *Server) runKefka(sess ssh.Session, lg *slog.Logger) error {
	reg := registry.New()
	coreutils.Register(reg)
	wasmprog.Register(reg)

	client, err := storage.New(sess.Context())
	if err != nil {
		return fmt.Errorf("can't make storage client: %w", err)
	}

	reg.Register("snapshot", snapshot.Impl{Client: client})

	sessID := uuid.Must(uuid.NewV7()).String()
	sessBucket := *bucket + "-" + sessID
	lg = lg.With("sessionBucket", sessBucket)

	fmt.Fprintln(sess)
	fmt.Fprintf(sess, "You are isolated to the bucket %s, which was automatically forked from %s on connection.\n", sessBucket, *bucket)
	fmt.Fprintln(sess)

	if _, err := client.CreateBucketFork(sess.Context(), *bucket, sessBucket); err != nil {
		return fmt.Errorf("can't create per-session bucket fork: %w", err)
	}
	lg.Info("made bucket fork", "source", *bucket)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		withForce := func(opts *s3.Options) {
			opts.APIOptions = append(opts.APIOptions, smithyhttp.AddHeaderValue("Tigris-Force-Delete", "true"))
		}

		if _, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: new(sessBucket),
		}, withForce); err != nil {
			lg.Error("can't delete session bucket", "err", err)
		}
		lg.Info("cleaned up bucket")
	}()

	fsys, err := s3fs.NewS3FS(client, sessBucket, s.fsOpts...)
	if err != nil {
		return fmt.Errorf("can't setup s3fs: %w", err)
	}

	// Wire stdio for the shell through real *os.File pipes. The kefka CLI
	// gets *os.File via os.Stdin/Stdout/Stderr; sophia previously handed
	// the shell strings.NewReader("") plus a *term.Terminal, which forces
	// wazero down its non-*os.File path. That path reports stdio as
	// FILETYPE_BLOCK_DEVICE to WASI guests and trips up wasi-libc's
	// isatty/buffering detection in python.wasm and qjs.wasm.
	//
	// Two pipes for input: a long-lived prompt pipe that term.Terminal
	// reads from, and a per-command pipe (rotated each sh.Run) so we can
	// close it on Ctrl-D to deliver EOF to the foreground command without
	// taking down the shell prompt.
	promptR, promptW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("can't open prompt pipe: %w", err)
	}
	defer promptR.Close()
	defer promptW.Close()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("can't open stdout pipe: %w", err)
	}
	defer stdoutR.Close()
	defer stdoutW.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("can't open stderr pipe: %w", err)
	}
	defer stderrR.Close()
	defer stderrW.Close()

	// commandActive is true while the foreground command (sh.Run) is
	// executing. It gates the input-side line discipline below: term.Terminal
	// already echoes during the prompt, so we only echo during command mode.
	var commandActive atomic.Bool

	// cmdStdinW is the write end of the foreground command's stdin pipe.
	// It rotates each sh.Run; the pump writes typed bytes here while
	// commandActive is true, and closes it (signalling EOF to the wasm
	// guest) when the user presses Ctrl-D.
	var (
		cmdMu     sync.Mutex
		cmdStdinW *os.File
	)

	// Pump SSH client bytes into either the prompt pipe (when idle) or the
	// foreground command's stdin pipe (while a command runs).
	//
	// Three pieces of line discipline that a kernel PTY would normally do
	// for us, done here in software:
	//   - ICRNL: translate \r (the Enter key on a raw SSH channel) to \n,
	//     so line-mode WASI readers like Python's fgets recognize Enter.
	//     term.Terminal accepts either, so the prompt is unaffected.
	//   - ECHO: while a command is running, echo typed bytes back to the
	//     SSH client so REPLs (qjs, python -i) aren't typing blind.
	//   - VEOF (Ctrl-D, 0x04): forward bytes up to the Ctrl-D and then
	//     close the foreground command's stdin so its next fd_read returns
	//     EOF.
	go func() {
		defer promptW.Close()
		defer func() {
			cmdMu.Lock()
			if cmdStdinW != nil {
				cmdStdinW.Close()
				cmdStdinW = nil
			}
			cmdMu.Unlock()
		}()
		buf := make([]byte, 4096)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				for i := range buf[:n] {
					if buf[i] == '\r' {
						buf[i] = '\n'
					}
				}

				if commandActive.Load() {
					eofAt := -1
					for i, b := range buf[:n] {
						if b == 0x04 {
							eofAt = i
							break
						}
					}
					end := n
					if eofAt >= 0 {
						end = eofAt
					}
					if end > 0 {
						echoLineDiscipline(sess, buf[:end])
					}
					cmdMu.Lock()
					if cmdStdinW != nil {
						if end > 0 {
							cmdStdinW.Write(buf[:end])
						}
						if eofAt >= 0 {
							cmdStdinW.Close()
							cmdStdinW = nil
						}
					}
					cmdMu.Unlock()
				} else {
					if _, werr := promptW.Write(buf[:n]); werr != nil {
						return
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Drain guest stdout/stderr into the terminal, which handles \n→\r\n
	// translation in writeWithCRLF.
	t := term.NewTerminal(sessRW{r: promptR, w: sess}, "$ ")
	go io.Copy(t, stdoutR)
	go io.Copy(t, stderrR)

	var sh *interp.Runner

	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, sh, args)
		}
	}

	env := expand.ListEnviron(
		"HOME=/",
		"IFS=\n",
		"MACHTYPE=x86_64-pc-linux-gnu",
		"HOSTTYPE=x86_64",
		"HOSTNAME=localhost",
		"PWD=/",
		"OLDPWD=/",
		"OPTIND=1",
		"KEFKA=1",
		"PATH=/usr/bin:/bin",
		"BUCKET_NAME="+sessBucket,
	)

	sh, err = interp.New(
		interp.Interactive(true),
		interp.Env(env),
		interp.StdIO(nil, stdoutW, stderrW),
		interp.ExecHandlers(middleware),
		interp.CallHandler(billysh.CallHandler(reg, fsys, os.Stdout, os.Stderr)),
		interp.StatHandler(billysh.FsysStatHandler(reg, fsys)),
		interp.OpenHandler(billysh.FsysOpenHandler(reg, fsys)),
		interp.ReadDirHandler2(billysh.FsysReadDirHandler(reg, fsys)),
	)
	if err != nil {
		return fmt.Errorf("can't make shell: %w", err)
	}

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	reader := &termLineReader{t: t}
	for stmts, err := range parser.InteractiveSeq(reader) {
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			fmt.Fprintln(t, "parse error:", err)
			t.SetPrompt("$ ")
			continue
		}

		if parser.Incomplete() {
			t.SetPrompt("> ")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		for _, stmt := range stmts {
			cmdR, cmdW, perr := os.Pipe()
			if perr != nil {
				fmt.Fprintln(t, "stdin pipe:", perr)
				continue
			}
			cmdMu.Lock()
			cmdStdinW = cmdW
			cmdMu.Unlock()
			interp.StdIO(cmdR, stdoutW, stderrW)(sh)

			commandActive.Store(true)
			runErr := sh.Run(ctx, stmt)
			commandActive.Store(false)

			cmdMu.Lock()
			if cmdStdinW != nil {
				cmdStdinW.Close()
				cmdStdinW = nil
			}
			cmdMu.Unlock()
			cmdR.Close()

			if sh.Exited() {
				cancel()
				return runErr
			}
			if runErr != nil {
				fmt.Fprintln(t, runErr)
			}
		}
		cancel()

		t.SetPrompt("$ ")
	}

	return nil
}

// echoLineDiscipline writes buf to w, expanding \n to \r\n so the terminal
// returns to column 0 after a line. Used to echo typed bytes back to the
// SSH client during command mode (where no kernel PTY does it for us).
func echoLineDiscipline(w io.Writer, buf []byte) {
	start := 0
	for i, b := range buf {
		if b == '\n' {
			if i > start {
				w.Write(buf[start:i])
			}
			w.Write([]byte{'\r', '\n'})
			start = i + 1
		}
	}
	if start < len(buf) {
		w.Write(buf[start:])
	}
}

// sessRW wires term.Terminal's input to a separately-fed reader (typically
// a pipe driven by a goroutine copying from the SSH session) while keeping
// writes going to the SSH session directly. This lets us share a single
// stdin source between the prompt and any running command.
type sessRW struct {
	r io.Reader
	w io.Writer
}

func (s sessRW) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (s sessRW) Write(p []byte) (int, error) { return s.w.Write(p) }

// termLineReader adapts term.Terminal.ReadLine into an io.Reader that emits
// one line (with a trailing '\n') per ReadLine call, so the bash parser can
// consume input that's already been through the terminal's line discipline.
type termLineReader struct {
	t   *term.Terminal
	buf []byte
}

func (r *termLineReader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		line, err := r.t.ReadLine()
		if err != nil {
			return 0, err
		}
		r.buf = append([]byte(line), '\n')
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}
