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

	"github.com/gliderlabs/ssh"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/spf13/pflag"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
	"tangled.org/xeiaso.net/kefka/command/registry"
	"tangled.org/xeiaso.net/kefka/command/registry/coreutils"
	"tangled.org/xeiaso.net/kefka/command/registry/wasmprog"
	"tangled.org/xeiaso.net/kefka/internal/billysh"
)

var (
	bind    = pflag.StringP("bind", "b", ":2222", "host:port to bind SSH to")
	timeout = pflag.DurationP("timeout", "T", 5*time.Minute, "the total time a command can run for")
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
	if err := s.runKefka(sess); err != nil {
		fmt.Fprintln(sess, "internal server error:", err)
		slog.Error("error serving Kefka session", "err", err)
		return
	}
}

func (s *Server) runKefka(sess ssh.Session) error {
	tempDir, err := os.MkdirTemp("", "sophia-"+sess.RemoteAddr().String()+"-*")
	if err != nil {
		return fmt.Errorf("can't make chroot jail: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fsys := osfs.New(tempDir)

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
