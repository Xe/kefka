package env

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/registry"
)

// envEchoImpl prints its env (via ec.Environ) one NAME=VALUE per line, sorted.
// Used to verify env's environment plumbing actually reaches the inner cmd.
type envEchoImpl struct{}

func (envEchoImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil || ec.Stdout == nil {
		return nil
	}
	if ec.Environ == nil {
		return nil
	}
	pairs := map[string]string{}
	ec.Environ.Each(func(name string, vr expand.Variable) bool {
		if !vr.IsSet() {
			return true
		}
		if vr.Kind != expand.String && vr.Kind != expand.NameRef {
			return true
		}
		if !vr.Exported {
			return true
		}
		pairs[name] = vr.String()
		return true
	})
	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		io.WriteString(ec.Stdout, name+"="+pairs[name]+"\n")
	}
	return nil
}

// argEchoImpl writes args joined by spaces and a trailing newline; used to
// confirm that command/args reach the inner Execer intact.
type argEchoImpl struct{}

func (argEchoImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec.Stdout != nil {
		io.WriteString(ec.Stdout, strings.Join(args, " "))
		io.WriteString(ec.Stdout, "\n")
	}
	return nil
}

// failImpl exits with a fixed non-zero status; used to verify env propagates
// the inner command's exit code.
type failImpl struct{}

func (failImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	return interp.ExitStatus(3)
}

func newRegistry(t *testing.T) *registry.Impl {
	t.Helper()
	reg := registry.New()
	reg.Register("env-echo", envEchoImpl{})
	reg.Register("arg-echo", argEchoImpl{})
	reg.Register("fail", failImpl{})
	return reg
}

func newRunner(t *testing.T, reg *registry.Impl, fsys billy.Filesystem, env expand.Environ) *interp.Runner {
	t.Helper()
	var sh *interp.Runner
	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, sh, args)
		}
	}
	// Use a fixed Environ so the subshell does not pick up the host's
	// os.Environ() — that would mask the env the test set up via ec.Environ.
	if env == nil {
		env = expand.ListEnviron()
	}
	var err error
	sh, err = interp.New(interp.ExecHandlers(middleware), interp.Env(env))
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

func run(t *testing.T, args []string, env expand.Environ) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	fsys := memfs.New()
	reg := newRegistry(t)
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		FS:      fsys,
		Environ: env,
		Runner:  newRunner(t, reg, fsys, env),
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return runResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func TestEnv_PrintEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     expand.Environ
		want    string
		wantErr bool
	}{
		{
			name: "no args prints sorted env",
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\nFOO=bar\n",
		},
		{
			name: "empty env produces no output and no trailing newline",
			env:  expand.ListEnviron(),
			want: "",
		},
		{
			name: "nil environ produces no output",
			env:  nil,
			want: "",
		},
		{
			name: "ignore-environment drops parent env",
			args: []string{"-i"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "",
		},
		{
			name: "long ignore-environment drops parent env",
			args: []string{"--ignore-environment"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "",
		},
		{
			name: "lone dash means -i",
			args: []string{"-"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "",
		},
		{
			name: "set adds NAME=VALUE",
			args: []string{"NEW=val"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "FOO=bar\nNEW=val\n",
		},
		{
			name: "set overrides existing var",
			args: []string{"FOO=replaced"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "FOO=replaced\n",
		},
		{
			name: "unset removes a variable",
			args: []string{"-u", "FOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\n",
		},
		{
			name: "long unset with equals removes a variable",
			args: []string{"--unset=FOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\n",
		},
		{
			name: "long unset with separate value removes a variable",
			args: []string{"--unset", "FOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\n",
		},
		{
			name: "short unset attached form removes a variable",
			args: []string{"-uFOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\n",
		},
		{
			name: "ignore plus set yields just the set vars",
			args: []string{"-i", "ONLY=this"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "ONLY=this\n",
		},
		{
			name: "double dash terminates options before printing",
			args: []string{"--"},
			env:  expand.ListEnviron("FOO=bar"),
			want: "FOO=bar\n",
		},
		{
			name: "value containing equals is preserved",
			args: []string{"K=a=b=c"},
			env:  expand.ListEnviron(),
			want: "K=a=b=c\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.args, tt.env)
			if tt.wantErr && r.err == nil {
				t.Fatalf("err = nil, want non-nil")
			}
			if !tt.wantErr && r.err != nil {
				t.Fatalf("unexpected err: %v", r.err)
			}
			if r.stdout != tt.want {
				t.Errorf("stdout = %q, want %q", r.stdout, tt.want)
			}
		})
	}
}

func TestEnv_RunCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        expand.Environ
		wantStdout string
	}{
		{
			name:       "passes parent env through",
			args:       []string{"env-echo"},
			env:        expand.ListEnviron("FOO=bar"),
			wantStdout: "FOO=bar\n",
		},
		{
			name:       "adds NAME=VALUE to inner env",
			args:       []string{"NEW=hi", "env-echo"},
			env:        expand.ListEnviron("FOO=bar"),
			wantStdout: "FOO=bar\nNEW=hi\n",
		},
		{
			name:       "ignore-environment hides parent vars",
			args:       []string{"-i", "ONLY=this", "env-echo"},
			env:        expand.ListEnviron("FOO=bar"),
			wantStdout: "ONLY=this\n",
		},
		{
			name:       "unset removes parent var",
			args:       []string{"-u", "FOO", "env-echo"},
			env:        expand.ListEnviron("FOO=bar", "KEEP=ok"),
			wantStdout: "KEEP=ok\n",
		},
		{
			name:       "passes args to inner command",
			args:       []string{"arg-echo", "one", "two"},
			env:        expand.ListEnviron(),
			wantStdout: "one two\n",
		},
		{
			name:       "double dash separates env options from inner command",
			args:       []string{"--", "arg-echo", "-i"},
			env:        expand.ListEnviron(),
			wantStdout: "-i\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.args, tt.env)
			if r.err != nil {
				t.Fatalf("unexpected err: %v\nstderr: %s", r.err, r.stderr)
			}
			if r.stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", r.stdout, tt.wantStdout)
			}
		})
	}
}

func TestEnv_ExitCodes(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		env       expand.Environ
		wantCode  uint8
		stderrSub string
	}{
		{
			name:      "unknown short option exits 125",
			args:      []string{"-Z"},
			wantCode:  125,
			stderrSub: "invalid option",
		},
		{
			name:      "unknown long option exits 125",
			args:      []string{"--bogus"},
			wantCode:  125,
			stderrSub: "unrecognized option",
		},
		{
			name:      "missing -u argument exits 125",
			args:      []string{"-u"},
			wantCode:  125,
			stderrSub: "option requires an argument",
		},
		{
			name:      "command not found exits 127",
			args:      []string{"nope-not-here"},
			wantCode:  127,
			stderrSub: "command not found",
		},
		{
			name:     "inner command's exit code propagates",
			args:     []string{"fail"},
			wantCode: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.args, tt.env)
			var status interp.ExitStatus
			if !errors.As(r.err, &status) {
				t.Fatalf("err = %v, want ExitStatus", r.err)
			}
			if uint8(status) != tt.wantCode {
				t.Errorf("exit = %d, want %d", uint8(status), tt.wantCode)
			}
			if tt.stderrSub != "" && !strings.Contains(r.stderr, tt.stderrSub) {
				t.Errorf("stderr missing %q; got: %q", tt.stderrSub, r.stderr)
			}
		})
	}
}

func TestEnv_Help(t *testing.T) {
	r := run(t, []string{"--help"}, expand.ListEnviron("FOO=bar"))
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	// --help goes to stdout per GNU coreutils convention.
	if !strings.Contains(r.stdout, "Usage: env") {
		t.Errorf("stdout missing usage; got: %q", r.stdout)
	}
	// Help must not print the environment.
	if strings.Contains(r.stdout, "FOO=bar") {
		t.Errorf("stdout leaked env into help: %q", r.stdout)
	}
}

func TestEnv_NilExecContext(t *testing.T) {
	err := Impl{}.Exec(context.Background(), nil, []string{"FOO=bar"})
	if err == nil || !strings.Contains(err.Error(), "nil ExecContext") {
		t.Errorf("err = %v, want nil ExecContext error", err)
	}
}

func TestEnv_NilRunner(t *testing.T) {
	// With a command but no Runner, env can't dispatch.
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		FS:      memfs.New(),
		Environ: expand.ListEnviron("FOO=bar"),
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"arg-echo"})
	var status interp.ExitStatus
	if !errors.As(err, &status) || uint8(status) != 127 {
		t.Errorf("err = %v, want ExitStatus(127)", err)
	}
	if !strings.Contains(stderr.String(), "exec not available") {
		t.Errorf("stderr missing 'exec not available': %q", stderr.String())
	}
}

func TestEnv_NilRunner_NoCommand(t *testing.T) {
	// Without a command, Runner is unused — should still print env.
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		Environ: expand.ListEnviron("FOO=bar"),
	}
	err := Impl{}.Exec(context.Background(), ec, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if stdout.String() != "FOO=bar\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "FOO=bar\n")
	}
}

// TestEnv_IgnoreEnv_ProductionRunner exercises the production runner setup
// (no interp.Env override, so the runner pulls in os.Environ()) to make sure
// `env -i` actually clears the host environment from the inner command and
// does not error trying to unset readonly shell vars (EUID, UID, GID).
func TestEnv_IgnoreEnv_ProductionRunner(t *testing.T) {
	t.Setenv("KEFKA_TEST_LEAKY", "leak-me")

	reg := newRegistry(t)
	fsys := memfs.New()
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

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		FS:      fsys,
		Environ: sh.Env,
		Runner:  sh,
	}
	err = Impl{}.Exec(context.Background(), ec, []string{"-i", "ONLY=this", "env-echo"})
	if err != nil {
		t.Fatalf("err = %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "readonly variable") {
		t.Errorf("stderr leaked readonly-variable errors: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "KEFKA_TEST_LEAKY") {
		t.Errorf("env -i leaked host env into inner command: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ONLY=this") {
		t.Errorf("inner env missing ONLY=this: %q", stdout.String())
	}
}

func TestEnv_LastSetWins(t *testing.T) {
	// Repeated NAME=VALUE: the last one wins, but the position is the first
	// occurrence — verifies setOrder dedup.
	r := run(t, []string{"X=1", "Y=a", "X=2"}, expand.ListEnviron())
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if r.stdout != "X=2\nY=a\n" {
		t.Errorf("stdout = %q, want %q", r.stdout, "X=2\nY=a\n")
	}
}
