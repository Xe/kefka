package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
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
	"tangled.org/xeiaso.net/kefka/command/registry"
	"tangled.org/xeiaso.net/kefka/command/registry/coreutils"
	"tangled.org/xeiaso.net/kefka/command/registry/wasmprog"
	"tangled.org/xeiaso.net/kefka/internal/billysh"
	"tangled.org/xeiaso.net/kefka/internal/s3fs"

	_ "embed"

	_ "github.com/joho/godotenv/autoload"
)

var (
	bind    = pflag.StringP("bind", "b", ":2222", "host:port to bind SSH to")
	bucket  = pflag.StringP("bucket", "B", os.Getenv("BUCKET_NAME"), "the bucket name to constrain sessions to")
	timeout = pflag.DurationP("timeout", "T", 5*time.Minute, "the total time a command can run for")

	//go:embed static/motd
	motd []byte
)

func main() {
	pflag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	srv := New()

	slog.Info("listening", "bind", *bind, "timeout", *timeout)
	return ssh.ListenAndServe(*bind, srv.HandleSSH)
}

type Server struct {
	reg *registry.Impl
}

func New() *Server {
	reg := registry.New()
	coreutils.Register(reg)
	wasmprog.Register(reg)

	return &Server{
		reg: reg,
	}
}

func (s *Server) HandleSSH(sess ssh.Session) {
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
	client, err := storage.New(sess.Context())
	if err != nil {
		return fmt.Errorf("can't make storage client: %w", err)
	}

	sessID := uuid.Must(uuid.NewV7()).String()
	sessBucket := *bucket + "-" + sessID
	lg = lg.With("sessionBucket", sessBucket)

	fmt.Fprintln(sess)
	fmt.Fprintf(sess, "You are isolated to the bucket %s, which was automatically forked from %s on connection.\n", sessBucket, *bucket)
	fmt.Fprintln(sess)

	if _, err := client.CreateBucketFork(sess.Context(), *bucket, sessBucket); err != nil {
		return fmt.Errorf("can't create per-session bucket fork: %w", err)
	}
	lg.Info("made bucket fork", "source", *bucket, "dest", sessBucket)

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
	}()

	fsys, err := s3fs.NewS3FS(client.Client, sessBucket)
	if err != nil {
		return fmt.Errorf("can't setup s3fs: %w", err)
	}

	t := term.NewTerminal(sess, "$ ")

	var sh *interp.Runner

	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return s.reg.Exec(ctx, fsys, sh, args)
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
	)

	sh, err = interp.New(
		interp.Interactive(true),
		interp.Env(env),
		interp.StdIO(strings.NewReader(""), t, t),
		interp.ExecHandlers(middleware),
		interp.CallHandler(billysh.CallHandler(s.reg, fsys, os.Stdout, os.Stderr)),
		interp.StatHandler(billysh.FsysStatHandler(s.reg, fsys)),
		interp.OpenHandler(billysh.FsysOpenHandler(s.reg, fsys)),
		interp.ReadDirHandler2(billysh.FsysReadDirHandler(s.reg, fsys)),
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
			runErr := sh.Run(ctx, stmt)
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
