package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
)

type fakeLister struct {
	names []string
}

func (f fakeLister) Names() []string { return f.names }

func run(t *testing.T, lister Lister, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{Reg: lister}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestCommandsLists(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		wantStdout string
	}{
		{
			name:       "empty registry prints nothing",
			registered: []string{},
			wantStdout: "",
		},
		{
			name:       "single command",
			registered: []string{"cat"},
			wantStdout: "- `cat`\n",
		},
		{
			name:       "sorted output regardless of insertion order",
			registered: []string{"wc", "cat", "ls", "base64"},
			wantStdout: "- `base64`\n- `cat`\n- `ls`\n- `wc`\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, fakeLister{names: tt.registered}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}

func TestHelpFlag(t *testing.T) {
	stdout, stderr, err := run(t, fakeLister{names: []string{"cat"}}, []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: commands [OPTION]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, err := run(t, fakeLister{}, []string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}

func TestNilRegistry(t *testing.T) {
	_, stderr, err := run(t, nil, nil)
	if err == nil {
		t.Fatal("expected error with nil registry, got nil")
	}
	if !strings.Contains(stderr, "no command registry available") {
		t.Errorf("expected diagnostic in stderr, got %q", stderr)
	}
}

func TestNilExecContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error with nil ExecContext, got nil")
	}
}
