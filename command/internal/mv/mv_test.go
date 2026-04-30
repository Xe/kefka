package mv

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	if err := fs.MkdirAll("dir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("existing", 0o755); err != nil {
		t.Fatal(err)
	}
	write("hello.txt", []byte("hello\n"))
	write("two.txt", []byte("world\n"))
	write("existing/keep.txt", []byte("keep\n"))
	return fs
}

func run(t *testing.T, args []string, fs billy.Filesystem) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func readFile(t *testing.T, fs billy.Filesystem, name string) string {
	t.Helper()
	f, err := fs.Open(name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func exists(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	_, err := fs.Stat(name)
	return err == nil
}

func TestMv(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "rename single file",
			args: []string{"hello.txt", "renamed.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after rename")
				}
				if got := readFile(t, fs, "renamed.txt"); got != "hello\n" {
					t.Errorf("renamed.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "move single file into directory",
			args: []string{"hello.txt", "dir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after move")
				}
				if got := readFile(t, fs, "dir/hello.txt"); got != "hello\n" {
					t.Errorf("dir/hello.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "multiple sources into directory",
			args: []string{"hello.txt", "two.txt", "dir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after move")
				}
				if exists(t, fs, "two.txt") {
					t.Errorf("two.txt should be gone after move")
				}
				if got := readFile(t, fs, "dir/hello.txt"); got != "hello\n" {
					t.Errorf("dir/hello.txt = %q, want %q", got, "hello\n")
				}
				if got := readFile(t, fs, "dir/two.txt"); got != "world\n" {
					t.Errorf("dir/two.txt = %q, want %q", got, "world\n")
				}
			},
		},
		{
			name:       "missing destination operand",
			args:       []string{"hello.txt"},
			wantErrSub: "missing destination file operand",
			wantErr:    true,
		},
		{
			name:       "no args",
			args:       nil,
			wantErrSub: "missing destination file operand",
			wantErr:    true,
		},
		{
			name:       "multiple sources but dest is not directory",
			args:       []string{"hello.txt", "two.txt", "newfile"},
			wantErrSub: "target 'newfile' is not a directory",
			wantErr:    true,
		},
		{
			name:       "missing source",
			args:       []string{"nope.txt", "out.txt"},
			wantErrSub: "cannot stat 'nope.txt': No such file or directory",
			wantErr:    true,
		},
		{
			name: "no-clobber skips existing target",
			args: []string{"-n", "hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should still exist (move was skipped)")
				}
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
				}
			},
		},
		{
			name: "no-clobber long flag still moves missing target",
			args: []string{"--no-clobber", "hello.txt", "fresh.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after move")
				}
				if got := readFile(t, fs, "fresh.txt"); got != "hello\n" {
					t.Errorf("fresh.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "no-clobber overrides force",
			args: []string{"-f", "-n", "hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched, -n wins over -f)", got, "keep\n")
				}
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should still exist (move was skipped)")
				}
			},
		},
		{
			name: "force flag accepted",
			args: []string{"-f", "hello.txt", "forced.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "forced.txt"); got != "hello\n" {
					t.Errorf("forced.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "verbose short flag prints rename line",
			args:       []string{"-v", "hello.txt", "v1.txt"},
			wantStdout: "renamed 'hello.txt' -> 'v1.txt'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "v1.txt"); got != "hello\n" {
					t.Errorf("v1.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "verbose long flag with directory destination",
			args:       []string{"--verbose", "hello.txt", "dir"},
			wantStdout: "renamed 'hello.txt' -> 'dir/hello.txt'\n",
		},
		{
			name:       "verbose multiple sources into directory",
			args:       []string{"-v", "hello.txt", "two.txt", "dir"},
			wantStdout: "renamed 'hello.txt' -> 'dir/hello.txt'\nrenamed 'two.txt' -> 'dir/two.txt'\n",
		},
		{
			name: "overwrite existing file by default",
			args: []string{"hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after move")
				}
				if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
					t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "double dash terminator",
			args: []string{"--", "hello.txt", "term.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "term.txt"); got != "hello\n" {
					t.Errorf("term.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "hello.txt", "out.txt"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			stdout, stderr, err := run(t, tt.args, fs)
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
			if tt.check != nil {
				tt.check(t, fs)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: mv [OPTION]... SOURCE... DEST") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-f, --force") {
		t.Errorf("force flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-n, --no-clobber") {
		t.Errorf("no-clobber flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-v, --verbose") {
		t.Errorf("verbose flag missing from help: %q", stderr)
	}
}

func TestNilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestNilFilesystem(t *testing.T) {
	ec := &command.ExecContext{Dir: "."}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"a", "b"}); err == nil {
		t.Fatal("expected error when filesystem is nil")
	}
}
