package rm

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
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	if err := fs.MkdirAll("emptydir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("populated/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	write("hello.txt", []byte("hello\n"))
	write("other.txt", []byte("other\n"))
	write("populated/inside.txt", []byte("inside\n"))
	write("populated/sub/deep.txt", []byte("deep\n"))
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

func exists(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	_, err := fs.Stat(name)
	return err == nil
}

func TestRm(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "remove single file",
			args: []string{"hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
			},
		},
		{
			name: "remove multiple files",
			args: []string{"hello.txt", "other.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed")
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
			name: "force suppresses missing operand",
			args: []string{"-f"},
		},
		{
			name:       "missing file errors without force",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name: "missing file silent with force",
			args: []string{"-f", "nope.txt"},
		},
		{
			name:       "directory without recursive errors",
			args:       []string{"emptydir"},
			wantErrSub: "Is a directory",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "emptydir") {
					t.Errorf("emptydir was removed despite missing -r")
				}
			},
		},
		{
			name: "recursive removes empty directory",
			args: []string{"-r", "emptydir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed")
				}
			},
		},
		{
			name: "recursive removes populated directory",
			args: []string{"-r", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, p := range []string{"populated", "populated/inside.txt", "populated/sub", "populated/sub/deep.txt"} {
					if exists(t, fs, p) {
						t.Errorf("%s was not removed", p)
					}
				}
			},
		},
		{
			name: "uppercase R is recursive",
			args: []string{"-R", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "populated") {
					t.Errorf("populated was not removed via -R")
				}
			},
		},
		{
			name: "long recursive flag",
			args: []string{"--recursive", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "populated") {
					t.Errorf("populated was not removed via --recursive")
				}
			},
		},
		{
			name:       "verbose prints removed line",
			args:       []string{"-v", "hello.txt"},
			wantStdout: "removed 'hello.txt'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
			},
		},
		{
			name:       "long verbose flag",
			args:       []string{"--verbose", "hello.txt"},
			wantStdout: "removed 'hello.txt'\n",
		},
		{
			name:       "verbose with multiple files",
			args:       []string{"-v", "hello.txt", "other.txt"},
			wantStdout: "removed 'hello.txt'\nremoved 'other.txt'\n",
		},
		{
			name:       "verbose recursive on directory",
			args:       []string{"-rv", "populated"},
			wantStdout: "removed 'populated'\n",
		},
		{
			name:       "partial success continues but reports error",
			args:       []string{"hello.txt", "nope.txt", "other.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed despite later failure")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed despite earlier failure")
				}
			},
		},
		{
			name: "force ignores missing in middle of list",
			args: []string{"-f", "hello.txt", "nope.txt", "other.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "hello.txt"},
			wantErr: true,
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed via absolute path")
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
	if !strings.Contains(stderr, "Usage: rm [OPTION]... FILE...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--recursive") {
		t.Errorf("--recursive flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("--force flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("--verbose flag missing from help output: %q", stderr)
	}
	if exists(t, fs, "hello.txt") == false {
		t.Errorf("--help should not have removed any files")
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
