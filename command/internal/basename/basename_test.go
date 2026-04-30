package basename

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

func TestBasename(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string // substring expected in stderr (when set)
		wantErr    bool
	}{
		{
			name:       "single path",
			args:       []string{"/usr/local/bin/sh"},
			wantStdout: "sh\n",
		},
		{
			name:       "no slash",
			args:       []string{"foo"},
			wantStdout: "foo\n",
		},
		{
			name:       "trailing slash",
			args:       []string{"/usr/local/bin/"},
			wantStdout: "bin\n",
		},
		{
			name:       "multiple trailing slashes",
			args:       []string{"foo/bar///"},
			wantStdout: "bar\n",
		},
		{
			name:       "root only",
			args:       []string{"/"},
			wantStdout: "\n",
		},
		{
			name:       "empty string",
			args:       []string{""},
			wantStdout: "\n",
		},
		{
			name:       "second positional treated as suffix",
			args:       []string{"foo.txt", ".txt"},
			wantStdout: "foo\n",
		},
		{
			name:       "suffix only stripped at end",
			args:       []string{"a.txt.bak", ".txt"},
			wantStdout: "a.txt.bak\n",
		},
		{
			name:       "short suffix flag",
			args:       []string{"-s", ".txt", "foo.txt", "bar.txt"},
			wantStdout: "foo\nbar\n",
		},
		{
			name:       "long suffix flag with equals",
			args:       []string{"--suffix=.txt", "foo.txt", "bar.txt"},
			wantStdout: "foo\nbar\n",
		},
		{
			name:       "multiple flag short",
			args:       []string{"-a", "foo.txt", "bar.txt"},
			wantStdout: "foo.txt\nbar.txt\n",
		},
		{
			name:       "multiple flag long",
			args:       []string{"--multiple", "/a/b", "/c/d"},
			wantStdout: "b\nd\n",
		},
		{
			name:       "suffix implies multiple",
			args:       []string{"-s", ".log", "/var/log/a.log", "/var/log/b.log"},
			wantStdout: "a\nb\n",
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
			name:       "suffix equals base produces empty",
			args:       []string{"-a", "-s", ".txt", ".txt"},
			wantStdout: "\n",
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "-weird-name"},
			wantStdout: "-weird-name\n",
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
	if !strings.Contains(stderr, "Usage: basename NAME [SUFFIX]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-a, --multiple") {
		t.Errorf("multiple flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-s, --suffix=SUFFIX") {
		t.Errorf("suffix flag missing from help: %q", stderr)
	}
}
