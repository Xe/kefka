package readlink

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

func newTestFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		f.Write(data)
		f.Close()
	}
	write("target.txt", []byte("hello"))
	write("sub/inner.txt", []byte("inner"))

	sym, ok := fs.(billy.Symlink)
	if !ok {
		t.Fatalf("memfs does not implement billy.Symlink")
	}
	if err := sym.Symlink("target.txt", "link.txt"); err != nil {
		t.Fatalf("symlink link.txt: %v", err)
	}
	if err := sym.Symlink("/target.txt", "abs-link.txt"); err != nil {
		t.Fatalf("symlink abs-link.txt: %v", err)
	}
	if err := sym.Symlink("link.txt", "chain.txt"); err != nil {
		t.Fatalf("symlink chain.txt: %v", err)
	}
	if err := sym.Symlink("../target.txt", "sub/up.txt"); err != nil {
		t.Fatalf("symlink sub/up.txt: %v", err)
	}
	if err := sym.Symlink("loop-b", "loop-a"); err != nil {
		t.Fatalf("symlink loop-a: %v", err)
	}
	if err := sym.Symlink("loop-a", "loop-b"); err != nil {
		t.Fatalf("symlink loop-b: %v", err)
	}
	return fs
}

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newTestFS(t),
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestReadlink(t *testing.T) {
	tests := []struct {
		name       string
		dir        string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "plain symlink prints target",
			args:       []string{"link.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "absolute target prints as stored",
			args:       []string{"abs-link.txt"},
			wantStdout: "/target.txt\n",
		},
		{
			name:       "non-symlink without -f errors",
			args:       []string{"target.txt"},
			wantErr:    true,
		},
		{
			name:       "missing file without -f errors silently",
			args:       []string{"nope"},
			wantErr:    true,
		},
		{
			name:       "canonicalize follows single symlink",
			args:       []string{"-f", "link.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "canonicalize follows chain",
			args:       []string{"-f", "chain.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "canonicalize on non-symlink prints resolved path",
			args:       []string{"-f", "target.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "canonicalize on missing file prints resolved path",
			args:       []string{"-f", "nope"},
			wantStdout: "nope\n",
		},
		{
			name:       "canonicalize handles circular symlinks",
			args:       []string{"-f", "loop-a"},
			wantStdout: "loop-a\n",
		},
		{
			name:       "canonicalize resolves relative target against link parent",
			args:       []string{"-f", "sub/up.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "long form --canonicalize",
			args:       []string{"--canonicalize", "link.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "multiple files prints each on its own line",
			args:       []string{"link.txt", "abs-link.txt"},
			wantStdout: "target.txt\n/target.txt\n",
		},
		{
			name:       "partial failure still prints successes and errors",
			args:       []string{"link.txt", "nope"},
			wantStdout: "target.txt\n",
			wantErr:    true,
		},
		{
			name:       "ec.Dir scopes relative paths",
			dir:        "sub",
			args:       []string{"-f", "up.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "absolute input path",
			args:       []string{"/link.txt"},
			wantStdout: "target.txt\n",
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantStderr: "readlink: missing operand\n",
			wantErr:    true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag"},
			wantErr: true,
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "link.txt"},
			wantStdout: "target.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			dir := tt.dir
			if dir == "" {
				dir = "."
			}
			ec := &command.ExecContext{
				Stdout: &stdout,
				Stderr: &stderr,
				Dir:    dir,
				FS:     newTestFS(t),
			}
			err := Impl{}.Exec(context.Background(), ec, tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr.String())
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if tt.wantStderr != "" && stderr.String() != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
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
	if !strings.Contains(stderr, "Usage: readlink [OPTIONS] FILE...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "-f, --canonicalize") {
		t.Errorf("canonicalize flag missing from help output: %q", stderr)
	}
}

func TestExec_NilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestExec_NoFS(t *testing.T) {
	ec := &command.ExecContext{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"link.txt"}); err == nil {
		t.Fatal("expected error for missing filesystem")
	}
}
