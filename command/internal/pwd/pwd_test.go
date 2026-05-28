package pwd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	if err := fs.MkdirAll("home/user", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := fs.OpenFile("home/user/notes.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()
	return fs
}

func run(t *testing.T, dir string, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if dir == "" {
		dir = "."
	}
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    dir,
		FS:     newFS(t),
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestPwd(t *testing.T) {
	tests := []struct {
		name       string
		dir        string
		args       []string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "default at fsys root",
			dir:        ".",
			args:       []string{},
			wantStdout: "/\n",
		},
		{
			name:       "default in subdir",
			dir:        "home/user",
			args:       []string{},
			wantStdout: "/home/user\n",
		},
		{
			name:       "logical flag explicit",
			dir:        "home/user",
			args:       []string{"-L"},
			wantStdout: "/home/user\n",
		},
		{
			name:       "physical flag without symlinks matches logical",
			dir:        "home/user",
			args:       []string{"-P"},
			wantStdout: "/home/user\n",
		},
		{
			name:       "P then L: last wins (logical)",
			dir:        "home/user",
			args:       []string{"-P", "-L"},
			wantStdout: "/home/user\n",
		},
		{
			name:       "L then P: last wins (physical)",
			dir:        "home/user",
			args:       []string{"-L", "-P"},
			wantStdout: "/home/user\n",
		},
		{
			name:       "double-dash terminator",
			dir:        ".",
			args:       []string{"--"},
			wantStdout: "/\n",
		},
		{
			name:    "unknown flag returns error",
			dir:     ".",
			args:    []string{"-Z"},
			wantErr: true,
		},
		{
			name:    "unknown long flag returns error",
			dir:     ".",
			args:    []string{"--no-such-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.dir, tt.args)
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

func TestEmptyDir(t *testing.T) {
	stdout, _, err := run(t, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "/\n" {
		t.Errorf("stdout = %q, want %q", stdout, "/\n")
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, ".", []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: pwd") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help output: %q", stderr)
	}
}

func TestNilExecContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestPhysicalWithoutFS(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    "home/user",
		FS:     nil,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-P"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stdout.String(); got != "/home/user\n" {
		t.Errorf("stdout = %q, want %q", got, "/home/user\n")
	}
}
