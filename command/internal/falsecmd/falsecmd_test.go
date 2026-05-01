package falsecmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestFalse(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no args", args: nil},
		{name: "ignored args", args: []string{"foo", "bar", "baz"}},
		{name: "looks-like-flag args", args: []string{"--help", "-x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if err == nil {
				t.Fatalf("expected non-nil error (exit 1), got nil")
			}
			var status interp.ExitStatus
			if !errors.As(err, &status) {
				t.Fatalf("expected interp.ExitStatus, got %T: %v", err, err)
			}
			if status != 1 {
				t.Errorf("exit status = %d, want 1", status)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}
