package clear

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
)

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestClear(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "no args writes ANSI clear sequence",
			args:       []string{},
			wantStdout: "\x1b[2J\x1b[H",
		},
		{
			name:       "extra positional args are ignored",
			args:       []string{"foo", "bar"},
			wantStdout: "\x1b[2J\x1b[H",
		},
		{
			name:    "unknown flag returns error",
			args:    []string{"--no-such-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: clear [OPTIONS]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help text: %q", stderr)
	}
}

func TestNilStdout(t *testing.T) {
	ec := &command.ExecContext{Dir: "."}
	if err := (Impl{}).Exec(context.Background(), ec, nil); err != nil {
		t.Fatalf("unexpected error with nil stdout/stderr: %v", err)
	}
}
