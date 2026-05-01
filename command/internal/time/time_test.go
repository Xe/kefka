package time

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

// echoImpl is a minimal Execer used to verify time dispatches to the inner
// command. Writes its joined args to stdout, returns nil.
type echoImpl struct{}

func (echoImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec.Stdout != nil {
		io.WriteString(ec.Stdout, strings.Join(args, " "))
		io.WriteString(ec.Stdout, "\n")
	}
	return nil
}

// failImpl returns a non-zero ExitStatus, used to verify time propagates
// inner failure exit codes.
type failImpl struct{}

func (failImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return interp.ExitStatus(3)
}

// trueImpl always succeeds (mirrors GNU `true`).
type trueImpl struct{}

func (trueImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return nil
}

// falseImpl always exits with status 1 (mirrors GNU `false`).
type falseImpl struct{}

func (falseImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return interp.ExitStatus(1)
}

// stdinEchoImpl reads from stdin and writes it to stdout, used to verify
// time passes the outer stdin through to the inner command.
type stdinEchoImpl struct{}

func (stdinEchoImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec.Stdin == nil || ec.Stdout == nil {
		return nil
	}
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		return err
	}
	ec.Stdout.Write(data)
	return nil
}

func newRegistry(t *testing.T) *registry.Impl {
	t.Helper()
	reg := registry.New()
	reg.Register("echo", echoImpl{})
	reg.Register("fail", failImpl{})
	reg.Register("stdin-echo", stdinEchoImpl{})
	reg.Register("true", trueImpl{})
	reg.Register("false", falseImpl{})
	return reg
}

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	return memfs.New()
}

// newRunner builds an interp.Runner whose exec handler dispatches into reg.
// This is what threads the registered Execers through Runner.Subshell().Run.
func newRunner(t *testing.T, reg *registry.Impl, fsys billy.Filesystem) *interp.Runner {
	t.Helper()
	var sh *interp.Runner
	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, sh, args)
		}
	}
	var err error
	sh, err = interp.New(interp.ExecHandlers(middleware))
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}
	return sh
}

type runResult struct {
	stdout string
	stderr string
	err    error
}

func run(t *testing.T, args []string, opts ...func(*command.ExecContext)) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	fsys := newFS(t)
	reg := newRegistry(t)
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fsys,
		Runner: newRunner(t, reg, fsys),
	}
	for _, opt := range opts {
		opt(ec)
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return runResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func TestTime_NoCommand(t *testing.T) {
	r := run(t, nil)
	var status interp.ExitStatus
	if !errors.As(r.err, &status) || uint8(status) != 1 {
		t.Errorf("err = %v, want ExitStatus(1)", r.err)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if !strings.Contains(r.stderr, "Usage: time") {
		t.Errorf("stderr missing usage; got: %q", r.stderr)
	}
}

func TestTime_True(t *testing.T) {
	// `time true` succeeds (exit 0) and writes timing info to stderr.
	r := run(t, []string{"true"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.stderr == "" {
		t.Errorf("expected timing on stderr, got empty")
	}
}

func TestTime_False(t *testing.T) {
	// `time false` propagates the inner exit status of 1.
	r := run(t, []string{"false"})
	var status interp.ExitStatus
	if !errors.As(r.err, &status) || uint8(status) != 1 {
		t.Errorf("err = %v, want ExitStatus(1)", r.err)
	}
	if r.stderr == "" {
		t.Errorf("expected timing on stderr even when inner command fails")
	}
}

func TestTime_PortableFormat_True(t *testing.T) {
	// `time -p true` emits the three-line POSIX portable format on stderr.
	r := run(t, []string{"-p", "true"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	lines := strings.Split(strings.TrimRight(r.stderr, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), r.stderr)
	}
	if !strings.HasPrefix(lines[0], "real ") {
		t.Errorf("line 0 = %q, want prefix %q", lines[0], "real ")
	}
	if !strings.HasPrefix(lines[1], "user ") {
		t.Errorf("line 1 = %q, want prefix %q", lines[1], "user ")
	}
	if !strings.HasPrefix(lines[2], "sys ") {
		t.Errorf("line 2 = %q, want prefix %q", lines[2], "sys ")
	}
}

func TestTime_FormatElapsedFlag(t *testing.T) {
	// `time -f '%e' true` writes only the elapsed-time substitution.
	r := run(t, []string{"-f", "%e", "true"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	got := strings.TrimRight(r.stderr, "\n")
	// %e expands to "%.2f" — verify a simple decimal pattern.
	if got == "" {
		t.Fatalf("stderr empty, want elapsed time")
	}
	if !strings.Contains(got, ".") {
		t.Errorf("stderr = %q, want decimal elapsed time", got)
	}
}

func TestTime_Help(t *testing.T) {
	r := run(t, []string{"--help"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	wantLines := []string{
		"Usage: time [OPTION]... COMMAND [ARGUMENT]...",
		"-f, --format=FORMAT",
		"--help",
		"Format specifiers:",
	}
	for _, line := range wantLines {
		if !strings.Contains(r.stderr, line) {
			t.Errorf("stderr missing %q; got: %q", line, r.stderr)
		}
	}
}

func TestTime_DefaultFormat(t *testing.T) {
	r := run(t, []string{"echo", "hello"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "hello\n")
	}
	// Default format is "%e %M" → "<seconds> 0\n"
	if !strings.HasSuffix(r.stderr, " 0\n") {
		t.Errorf("stderr = %q, want suffix %q", r.stderr, " 0\n")
	}
}

func TestTime_PortableFormat(t *testing.T) {
	r := run(t, []string{"-p", "echo", "hi"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.stdout != "hi\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "hi\n")
	}
	for _, want := range []string{"real ", "user 0.00\n", "sys 0.00\n"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr missing %q; got: %q", want, r.stderr)
		}
	}
}

func TestTime_VerboseFormat(t *testing.T) {
	r := run(t, []string{"-v", "echo", "hi"})
	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	wantLines := []string{
		"Command being timed: echo hi",
		"Elapsed (wall clock) time:",
		"Maximum resident set size (kbytes): 0",
	}
	for _, line := range wantLines {
		if !strings.Contains(r.stderr, line) {
			t.Errorf("stderr missing %q; got: %q", line, r.stderr)
		}
	}
}

func TestTime_FlagParsing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOut string
		wantErr []string // substrings stderr must contain
		errCode uint8    // 0 = no error
	}{
		{
			name:    "short format with separate value",
			args:    []string{"-f", "elapsed=%e", "echo", "x"},
			wantOut: "x\n",
			wantErr: []string{"elapsed="},
		},
		{
			name:    "long format with equals",
			args:    []string{"--format=cmd=%C", "echo", "x"},
			wantOut: "x\n",
			wantErr: []string{"cmd=echo x"},
		},
		{
			name:    "long format with separate value",
			args:    []string{"--format", "cmd=%C", "echo", "x"},
			wantOut: "x\n",
			wantErr: []string{"cmd=echo x"},
		},
		{
			name:    "double-dash ends parsing",
			args:    []string{"--", "echo", "-p"},
			wantOut: "-p\n",
		},
		{
			name:    "unknown flag is permissively skipped",
			args:    []string{"--unknown", "echo", "x"},
			wantOut: "x\n",
		},
		{
			name:    "missing argument to -f",
			args:    []string{"-f"},
			wantErr: []string{"missing argument to '-f'"},
			errCode: 1,
		},
		{
			name:    "missing argument to -o",
			args:    []string{"-o"},
			wantErr: []string{"missing argument to '-o'"},
			errCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.args)
			if tt.errCode != 0 {
				var status interp.ExitStatus
				if !errors.As(r.err, &status) || uint8(status) != tt.errCode {
					t.Errorf("err = %v, want ExitStatus(%d)", r.err, tt.errCode)
				}
			} else if r.err != nil {
				t.Fatalf("unexpected error: %v", r.err)
			}
			if tt.wantOut != "" && r.stdout != tt.wantOut {
				t.Errorf("stdout = %q, want %q", r.stdout, tt.wantOut)
			}
			for _, sub := range tt.wantErr {
				if !strings.Contains(r.stderr, sub) {
					t.Errorf("stderr missing %q; got: %q", sub, r.stderr)
				}
			}
		})
	}
}

func TestTime_FormatSpecifiers(t *testing.T) {
	// Validate that the static-value specifiers (%M, %S, %U, %P) match GNU
	// time conventions even though we don't have real metrics.
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"max RSS placeholder", "%M", "0\n"},
		{"system CPU placeholder", "%S", "0.00\n"},
		{"user CPU placeholder", "%U", "0.00\n"},
		{"CPU percentage placeholder", "%P", "0%\n"},
		{"command substitution", "%C", "echo hello\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, []string{"-f", tt.format, "echo", "hello"})
			if r.err != nil {
				t.Fatalf("unexpected error: %v", r.err)
			}
			if !strings.HasSuffix(r.stderr, tt.want) {
				t.Errorf("stderr = %q, want suffix %q", r.stderr, tt.want)
			}
		})
	}
}

func TestTime_FormatElapsedTime(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"sub-minute", 1.5, "0:01.50"},
		{"minute boundary", 60, "1:00.00"},
		{"hour boundary", 3600, "1:00:00.00"},
		{"complex", 3661.25, "1:01:01.25"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatElapsedTime(tt.seconds)
			if got != tt.want {
				t.Errorf("formatElapsedTime(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestTime_OutputToFile(t *testing.T) {
	fs := memfs.New()
	reg := newRegistry(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
		Runner: newRunner(t, reg, fs),
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"-o", "out.log", "-f", "fixed", "echo", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// stderr should not contain timing — it went to the file instead.
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty (timing should have gone to file)", stderr.String())
	}
	got := readFile(t, fs, "out.log")
	if got != "fixed\n" {
		t.Errorf("file content = %q, want %q", got, "fixed\n")
	}
}

func TestTime_AppendToFile(t *testing.T) {
	fs := memfs.New()
	// Pre-seed the file so we can verify append vs overwrite.
	f, err := fs.OpenFile("out.log", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(f, "previous\n")
	f.Close()

	reg := newRegistry(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
		Runner: newRunner(t, reg, fs),
	}
	err = Impl{}.Exec(context.Background(), ec, []string{"-o", "out.log", "-a", "-f", "appended", "echo", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, fs, "out.log")
	want := "previous\nappended\n"
	if got != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
}

func TestTime_OverwriteFile(t *testing.T) {
	fs := memfs.New()
	f, err := fs.OpenFile("out.log", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(f, "previous\n")
	f.Close()

	reg := newRegistry(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
		Runner: newRunner(t, reg, fs),
	}
	err = Impl{}.Exec(context.Background(), ec, []string{"-o", "out.log", "-f", "fresh", "echo", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := readFile(t, fs, "out.log")
	if got != "fresh\n" {
		t.Errorf("file content = %q, want %q (no append → overwrite)", got, "fresh\n")
	}
}

func TestTime_PropagatesInnerExitCode(t *testing.T) {
	r := run(t, []string{"fail"})
	var status interp.ExitStatus
	if !errors.As(r.err, &status) || uint8(status) != 3 {
		t.Errorf("err = %v, want ExitStatus(3)", r.err)
	}
	// Timing should still be written even though the inner command failed.
	if r.stderr == "" {
		t.Errorf("expected timing on stderr even on inner failure")
	}
}

func TestTime_CommandNotFound(t *testing.T) {
	r := run(t, []string{"does-not-exist"})
	var status interp.ExitStatus
	if !errors.As(r.err, &status) || uint8(status) != 127 {
		t.Errorf("err = %v, want ExitStatus(127)", r.err)
	}
	if !strings.Contains(r.stderr, "command not found") ||
		!strings.Contains(r.stderr, "does-not-exist") {
		t.Errorf("stderr missing not-found message: %q", r.stderr)
	}
}

func TestTime_NilRunner(t *testing.T) {
	// Without a runner, time has no way to dispatch the inner command, so
	// it short-circuits to a 127 with the same diagnostic the registry
	// path used to produce.
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     memfs.New(),
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"echo", "hi"})
	var status interp.ExitStatus
	if !errors.As(err, &status) || uint8(status) != 127 {
		t.Errorf("err = %v, want ExitStatus(127)", err)
	}
	if !strings.Contains(stderr.String(), "exec not available") {
		t.Errorf("stderr missing 'exec not available': %q", stderr.String())
	}
}

func TestTime_StdinPassthrough(t *testing.T) {
	fs := memfs.New()
	reg := newRegistry(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader("piped through"),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
		Runner: newRunner(t, reg, fs),
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"stdin-echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "piped through" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "piped through")
	}
}

func TestTime_NilExecContext(t *testing.T) {
	err := Impl{}.Exec(context.Background(), nil, []string{"echo", "hi"})
	if err == nil || !strings.Contains(err.Error(), "nil ExecContext") {
		t.Errorf("err = %v, want nil ExecContext error", err)
	}
}

func readFile(t *testing.T, fs billy.Filesystem, name string) string {
	t.Helper()
	f, err := fs.Open(name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
