package cp

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
	if err := fs.MkdirAll("src/inner", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("existing", 0o755); err != nil {
		t.Fatal(err)
	}
	write("hello.txt", []byte("hello\n"))
	write("two.txt", []byte("world\n"))
	write("existing/keep.txt", []byte("keep\n"))
	write("src/a.txt", []byte("a\n"))
	write("src/inner/b.txt", []byte("b\n"))
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

func TestCp(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "single file to new file",
			args: []string{"hello.txt", "copy.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "copy.txt"); got != "hello\n" {
					t.Errorf("copy.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "single file into existing directory",
			args: []string{"hello.txt", "dir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "dir/hello.txt"); got != "hello\n" {
					t.Errorf("dir/hello.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "multiple sources into directory",
			args: []string{"hello.txt", "two.txt", "dir"},
			check: func(t *testing.T, fs billy.Filesystem) {
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
			wantErrSub: "is not a directory",
			wantErr:    true,
		},
		{
			name:       "missing source",
			args:       []string{"nope.txt", "out.txt"},
			wantErrSub: "cannot stat 'nope.txt'",
			wantErr:    true,
		},
		{
			name:       "directory without recursive flag",
			args:       []string{"src", "dest"},
			wantErrSub: "-r not specified; omitting directory 'src'",
			wantErr:    true,
		},
		{
			name: "recursive short flag copies directory",
			args: []string{"-r", "src", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "newdir/a.txt"); got != "a\n" {
					t.Errorf("newdir/a.txt = %q, want %q", got, "a\n")
				}
				if got := readFile(t, fs, "newdir/inner/b.txt"); got != "b\n" {
					t.Errorf("newdir/inner/b.txt = %q, want %q", got, "b\n")
				}
			},
		},
		{
			name: "recursive uppercase R flag",
			args: []string{"-R", "src", "uppdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "uppdir/inner/b.txt"); got != "b\n" {
					t.Errorf("uppdir/inner/b.txt = %q, want %q", got, "b\n")
				}
			},
		},
		{
			name: "recursive long flag into existing dir",
			args: []string{"--recursive", "src", "existing"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/src/a.txt"); got != "a\n" {
					t.Errorf("existing/src/a.txt = %q, want %q", got, "a\n")
				}
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt should be untouched, got %q", got)
				}
			},
		},
		{
			name: "no-clobber skips existing target",
			args: []string{"-n", "hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
				}
			},
		},
		{
			name: "no-clobber long flag still copies missing target",
			args: []string{"--no-clobber", "hello.txt", "fresh.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "fresh.txt"); got != "hello\n" {
					t.Errorf("fresh.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "verbose short flag prints copy lines",
			args:       []string{"-v", "hello.txt", "v1.txt"},
			wantStdout: "'hello.txt' -> 'v1.txt'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "v1.txt"); got != "hello\n" {
					t.Errorf("v1.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "verbose long flag with directory destination",
			args:       []string{"--verbose", "hello.txt", "dir"},
			wantStdout: "'hello.txt' -> 'dir/hello.txt'\n",
		},
		{
			name:       "verbose multiple sources",
			args:       []string{"-v", "hello.txt", "two.txt", "dir"},
			wantStdout: "'hello.txt' -> 'dir/hello.txt'\n'two.txt' -> 'dir/two.txt'\n",
		},
		{
			name: "preserve flag accepted",
			args: []string{"-p", "hello.txt", "p.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "p.txt"); got != "hello\n" {
					t.Errorf("p.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name: "overwrite existing file by default",
			args: []string{"hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
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
	if !strings.Contains(stderr, "Usage: cp [OPTION]... SOURCE... DEST") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-r, -R, --recursive") {
		t.Errorf("recursive flag missing from help: %q", stderr)
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
