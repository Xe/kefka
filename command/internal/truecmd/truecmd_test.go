package truecmd

import (
	"bytes"
	"context"
	"testing"

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

func TestTrue(t *testing.T) {
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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
