package dirname

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tangled.org/xeiaso.net/kefka/command"
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

func TestDirname(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "single path",
			args:       []string{"/usr/local/bin/sh"},
			wantStdout: "/usr/local/bin\n",
		},
		{
			name:       "no slash",
			args:       []string{"foo"},
			wantStdout: ".\n",
		},
		{
			name:       "trailing slash",
			args:       []string{"/usr/local/bin/"},
			wantStdout: "/usr/local\n",
		},
		{
			name:       "multiple trailing slashes",
			args:       []string{"foo/bar///"},
			wantStdout: "foo\n",
		},
		{
			name:       "root only",
			args:       []string{"/"},
			wantStdout: ".\n",
		},
		{
			name:       "single component under root",
			args:       []string{"/foo"},
			wantStdout: "/\n",
		},
		{
			name:       "single component under root with trailing slash",
			args:       []string{"/foo/"},
			wantStdout: "/\n",
		},
		{
			name:       "empty string",
			args:       []string{""},
			wantStdout: ".\n",
		},
		{
			name:       "relative two-component path",
			args:       []string{"foo/bar"},
			wantStdout: "foo\n",
		},
		{
			name:       "multiple operands",
			args:       []string{"/a/b", "/c/d", "e"},
			wantStdout: "/a\n/c\n.\n",
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantErrSub: "missing operand",
			wantErr:    true,
		},
		{
			name:       "unknown flag",
			args:       []string{"--no-such-flag", "foo"},
			wantErr:    true,
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "-weird-name"},
			wantStdout: ".\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
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
	if !strings.Contains(stderr, "Usage: dirname [OPTION] NAME...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help output: %q", stderr)
	}
}
