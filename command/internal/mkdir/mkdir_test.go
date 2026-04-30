package mkdir

import (
	"bytes"
	"context"
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
	if err := fs.MkdirAll("existing", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("parent", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := fs.OpenFile("file.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("hello\n"))
	f.Close()
	return fs
}

func run(t *testing.T, args []string, fs billy.Filesystem) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func isDir(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	info, err := fs.Stat(name)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func TestMkdir(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "create single directory",
			args: []string{"newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created")
				}
			},
		},
		{
			name: "create multiple directories",
			args: []string{"a", "b", "c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"a", "b", "c"} {
					if !isDir(t, fs, d) {
						t.Errorf("%s was not created", d)
					}
				}
			},
		},
		{
			name: "create inside existing parent",
			args: []string{"parent/child"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "parent/child") {
					t.Errorf("parent/child was not created")
				}
			},
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantErrSub: "missing operand",
			wantErr:    true,
		},
		{
			name:       "fails when directory exists",
			args:       []string{"existing"},
			wantErrSub: "File exists",
			wantErr:    true,
		},
		{
			name:       "fails when parent does not exist",
			args:       []string{"nope/child"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name: "parents flag silent on existing",
			args: []string{"-p", "existing"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "existing") {
					t.Errorf("existing was removed")
				}
			},
		},
		{
			name: "parents flag creates intermediate directories",
			args: []string{"-p", "a/b/c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"a", "a/b", "a/b/c"} {
					if !isDir(t, fs, d) {
						t.Errorf("%s was not created", d)
					}
				}
			},
		},
		{
			name: "long parents flag",
			args: []string{"--parents", "deep/path/here"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "deep/path/here") {
					t.Errorf("deep/path/here was not created")
				}
			},
		},
		{
			name:       "verbose prints to stdout",
			args:       []string{"-v", "newdir"},
			wantStdout: "mkdir: created directory 'newdir'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created")
				}
			},
		},
		{
			name:       "long verbose flag",
			args:       []string{"--verbose", "newdir"},
			wantStdout: "mkdir: created directory 'newdir'\n",
		},
		{
			name:       "verbose with multiple",
			args:       []string{"-pv", "a/b", "c"},
			wantStdout: "mkdir: created directory 'a/b'\nmkdir: created directory 'c'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "a/b") {
					t.Errorf("a/b was not created")
				}
				if !isDir(t, fs, "c") {
					t.Errorf("c was not created")
				}
			},
		},
		{
			name:       "partial success returns error and continues",
			args:       []string{"existing", "newdir"},
			wantErrSub: "File exists",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created after earlier failure")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "foo"},
			wantErr: true,
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/abs"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "abs") {
					t.Errorf("abs was not created")
				}
			},
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
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"--help"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: mkdir [OPTION]... DIRECTORY...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--parents") {
		t.Errorf("--parents flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("--verbose flag missing from help output: %q", stderr)
	}
}

func TestNoFilesystem(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"foo"})
	if err == nil {
		t.Fatal("expected error when ExecContext.FS is nil")
	}
}
