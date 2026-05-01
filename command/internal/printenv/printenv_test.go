package printenv

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type runResult struct {
	stdout string
	stderr string
	err    error
}

func run(t *testing.T, args []string, env expand.Environ) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		Environ: env,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return runResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		err:    err,
	}
}

func TestPrintenv(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      expand.Environ
		want     string
		wantCode uint8 // 0 = success
	}{
		{
			name: "no args prints sorted env",
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "BAZ=qux\nFOO=bar\n",
		},
		{
			name: "empty env produces no output",
			env:  expand.ListEnviron(),
			want: "",
		},
		{
			name: "nil environ produces no output",
			env:  nil,
			want: "",
		},
		{
			name: "single existing variable prints value",
			args: []string{"FOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "bar\n",
		},
		{
			name: "multiple existing variables print values in order",
			args: []string{"BAZ", "FOO"},
			env:  expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want: "qux\nbar\n",
		},
		{
			name:     "missing variable exits 1",
			args:     []string{"NOPE"},
			env:      expand.ListEnviron("FOO=bar"),
			want:     "",
			wantCode: 1,
		},
		{
			name:     "mix of present and missing prints the present ones, exits 1",
			args:     []string{"FOO", "NOPE", "BAZ"},
			env:      expand.ListEnviron("FOO=bar", "BAZ=qux"),
			want:     "bar\nqux\n",
			wantCode: 1,
		},
		{
			name: "value containing equals is preserved in print-all",
			env:  expand.ListEnviron("K=a=b=c"),
			want: "K=a=b=c\n",
		},
		{
			name: "value with newline is printed verbatim",
			args: []string{"MULTI"},
			env:  expand.ListEnviron("MULTI=line1\nline2"),
			want: "line1\nline2\n",
		},
		{
			name: "leading-dash arg is a variable name, not a flag",
			args: []string{"-FOO"},
			env:  expand.ListEnviron("-FOO=dash-name"),
			want: "dash-name\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, tt.args, tt.env)
			if tt.wantCode == 0 {
				if r.err != nil {
					t.Fatalf("unexpected err: %v", r.err)
				}
			} else {
				var status interp.ExitStatus
				if !errors.As(r.err, &status) || uint8(status) != tt.wantCode {
					t.Errorf("err = %v, want ExitStatus(%d)", r.err, tt.wantCode)
				}
			}
			if r.stdout != tt.want {
				t.Errorf("stdout = %q, want %q", r.stdout, tt.want)
			}
		})
	}
}

func TestPrintenv_Help(t *testing.T) {
	r := run(t, []string{"--help"}, expand.ListEnviron("FOO=bar"))
	if r.err != nil {
		t.Fatalf("unexpected err: %v", r.err)
	}
	if !strings.Contains(r.stdout, "Usage: printenv") {
		t.Errorf("stdout missing usage; got: %q", r.stdout)
	}
	// Help must not print the environment.
	if strings.Contains(r.stdout, "FOO=bar") {
		t.Errorf("stdout leaked env into help: %q", r.stdout)
	}
}

func TestPrintenv_NilExecContext(t *testing.T) {
	err := Impl{}.Exec(context.Background(), nil, []string{"FOO"})
	if err == nil || !strings.Contains(err.Error(), "nil ExecContext") {
		t.Errorf("err = %v, want nil ExecContext error", err)
	}
}
