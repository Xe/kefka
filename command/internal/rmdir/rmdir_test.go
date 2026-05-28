package rmdir

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
	if err := fs.MkdirAll("emptydir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("a/b/c", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("populated", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("siblings/empty1", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("siblings/empty2", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := fs.OpenFile("populated/inside.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("inside\n"))
	f.Close()
	f2, err := fs.OpenFile("regular.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f2.Write([]byte("regular\n"))
	f2.Close()
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

func TestRmdir(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "remove single empty directory",
			args: []string{"emptydir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed")
				}
			},
		},
		{
			name: "remove multiple empty directories",
			args: []string{"siblings/empty1", "siblings/empty2"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "siblings/empty1") {
					t.Errorf("siblings/empty1 was not removed")
				}
				if exists(t, fs, "siblings/empty2") {
					t.Errorf("siblings/empty2 was not removed")
				}
				if !exists(t, fs, "siblings") {
					t.Errorf("siblings was unexpectedly removed")
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
			name:       "missing directory errors",
			args:       []string{"nope"},
			wantErrSub: "rmdir: failed to remove 'nope': No such file or directory",
			wantErr:    true,
		},
		{
			name:       "non-empty directory errors",
			args:       []string{"populated"},
			wantErrSub: "rmdir: failed to remove 'populated': Directory not empty",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "populated") {
					t.Errorf("populated was removed despite being non-empty")
				}
			},
		},
		{
			name:       "regular file errors as Not a directory",
			args:       []string{"regular.txt"},
			wantErrSub: "rmdir: failed to remove 'regular.txt': Not a directory",
			wantErr:    true,
		},
		{
			name: "parents flag removes ancestors",
			args: []string{"-p", "a/b/c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, p := range []string{"a/b/c", "a/b", "a"} {
					if exists(t, fs, p) {
						t.Errorf("%s was not removed", p)
					}
				}
			},
		},
		{
			name: "long parents flag",
			args: []string{"--parents", "a/b/c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "a") {
					t.Errorf("a was not removed via --parents")
				}
			},
		},
		{
			name:       "verbose prints diagnostic",
			args:       []string{"-v", "emptydir"},
			wantStdout: "rmdir: removing directory, 'emptydir'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed")
				}
			},
		},
		{
			name:       "long verbose flag",
			args:       []string{"--verbose", "emptydir"},
			wantStdout: "rmdir: removing directory, 'emptydir'\n",
		},
		{
			name:       "verbose with parents emits one line per directory",
			args:       []string{"-pv", "a/b/c"},
			wantStdout: "rmdir: removing directory, 'a/b/c'\nrmdir: removing directory, 'a/b'\nrmdir: removing directory, 'a'\n",
		},
		{
			name: "parents stops silently when parent non-empty",
			args: []string{"-p", "siblings/empty1"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "siblings/empty1") {
					t.Errorf("siblings/empty1 was not removed")
				}
				if !exists(t, fs, "siblings") {
					t.Errorf("siblings was unexpectedly removed despite empty2 sibling")
				}
				if !exists(t, fs, "siblings/empty2") {
					t.Errorf("siblings/empty2 was unexpectedly removed")
				}
			},
		},
		{
			name:       "partial success continues but reports error",
			args:       []string{"emptydir", "nope", "siblings/empty1"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed despite later failure")
				}
				if exists(t, fs, "siblings/empty1") {
					t.Errorf("siblings/empty1 was not removed despite earlier failure")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "emptydir"},
			wantErr: true,
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/emptydir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed via absolute path")
				}
			},
		},
		{
			name: "single component with parents does not traverse above cwd",
			args: []string{"-p", "emptydir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed")
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
	if !strings.Contains(stderr, "Usage: rmdir [-pv] DIRECTORY...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--parents") {
		t.Errorf("--parents flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("--verbose flag missing from help output: %q", stderr)
	}
	if !exists(t, fs, "emptydir") {
		t.Errorf("--help should not have removed any directories")
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
