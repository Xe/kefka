package tree

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
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("alpha.txt", []byte("a"))
	write("beta.txt", []byte("b"))
	write(".hidden", []byte("h"))
	write("sub/inner.txt", []byte("i"))
	write("sub/deeper/leaf.txt", []byte("L"))
	return fs
}

func TestExec(t *testing.T) {
	tests := []struct {
		name       string
		dir        string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name: "default tree from cwd",
			args: []string{},
			wantStdout: ".\n" +
				"|-- alpha.txt\n" +
				"|-- beta.txt\n" +
				"`-- sub\n" +
				"    |-- deeper\n" +
				"    |   `-- leaf.txt\n" +
				"    `-- inner.txt\n" +
				"\n2 directories, 4 files\n",
		},
		{
			name: "show hidden with -a",
			args: []string{"-a"},
			wantStdout: ".\n" +
				"|-- .hidden\n" +
				"|-- alpha.txt\n" +
				"|-- beta.txt\n" +
				"`-- sub\n" +
				"    |-- deeper\n" +
				"    |   `-- leaf.txt\n" +
				"    `-- inner.txt\n" +
				"\n2 directories, 5 files\n",
		},
		{
			name: "directories only with -d",
			args: []string{"-d"},
			wantStdout: ".\n" +
				"`-- sub\n" +
				"    `-- deeper\n" +
				"\n2 directories\n",
		},
		{
			name: "limit depth with -L 1",
			args: []string{"-L", "1"},
			wantStdout: ".\n" +
				"|-- alpha.txt\n" +
				"|-- beta.txt\n" +
				"`-- sub\n" +
				"\n1 directory, 2 files\n",
		},
		{
			name: "limit depth with -L 2",
			args: []string{"-L", "2"},
			wantStdout: ".\n" +
				"|-- alpha.txt\n" +
				"|-- beta.txt\n" +
				"`-- sub\n" +
				"    |-- deeper\n" +
				"    `-- inner.txt\n" +
				"\n2 directories, 3 files\n",
		},
		{
			name: "full path with -f",
			args: []string{"-f"},
			wantStdout: ".\n" +
				"|-- ./alpha.txt\n" +
				"|-- ./beta.txt\n" +
				"`-- ./sub\n" +
				"    |-- ./sub/deeper\n" +
				"    |   `-- ./sub/deeper/leaf.txt\n" +
				"    `-- ./sub/inner.txt\n" +
				"\n2 directories, 4 files\n",
		},
		{
			name: "explicit subdirectory",
			args: []string{"sub"},
			wantStdout: "sub\n" +
				"|-- deeper\n" +
				"|   `-- leaf.txt\n" +
				"`-- inner.txt\n" +
				"\n1 directory, 2 files\n",
		},
		{
			name: "single file argument",
			args: []string{"alpha.txt"},
			wantStdout: "alpha.txt\n" +
				"\n0 directories, 1 file\n",
		},
		{
			name:       "missing path reports stderr and error",
			args:       []string{"nope"},
			wantStdout: "\n0 directories, 0 files\n",
			wantStderr: "tree: nope: No such file or directory\n",
			wantErr:    true,
		},
		{
			name: "multiple roots aggregate counts",
			args: []string{"sub", "sub/deeper"},
			wantStdout: "sub\n" +
				"|-- deeper\n" +
				"|   `-- leaf.txt\n" +
				"`-- inner.txt\n" +
				"sub/deeper\n" +
				"`-- leaf.txt\n" +
				"\n1 directory, 3 files\n",
		},
		{
			name: "ec.Dir scopes the listing",
			dir:  "sub",
			args: []string{},
			wantStdout: ".\n" +
				"|-- deeper\n" +
				"|   `-- leaf.txt\n" +
				"`-- inner.txt\n" +
				"\n1 directory, 2 files\n",
		},
		{
			name: "unknown flag returns error",
			args: []string{"--no-such-flag"},
			wantStderr: "tree: unknown option: --no-such-flag\n" +
				"Usage: tree [OPTION]... [DIRECTORY]...\n" +
				"List contents of directories in a tree-like format.\n\n" +
				"  -a          include hidden files\n" +
				"  -d          list directories only\n" +
				"  -L LEVEL    limit depth of directory tree\n" +
				"  -f          print full path prefix for each file\n" +
				"      --help  display this help and exit\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			dir := tc.dir
			if dir == "" {
				dir = "."
			}
			ec := &command.ExecContext{
				Stdout: &stdout,
				Stderr: &stderr,
				Dir:    dir,
				FS:     newTestFS(t),
			}
			err := Impl{}.Exec(context.Background(), ec, tc.args)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("stdout mismatch\nwant:\n%q\ngot:\n%q", tc.wantStdout, got)
			}
			if got := stderr.String(); got != tc.wantStderr {
				t.Errorf("stderr mismatch\nwant:\n%q\ngot:\n%q", tc.wantStderr, got)
			}
		})
	}
}

func TestExec_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     memfs.New(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout output, got %q", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "Usage: tree") {
		t.Errorf("expected usage on stderr, got %q", stderr.String())
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
	if err := (Impl{}).Exec(context.Background(), ec, nil); err == nil {
		t.Fatal("expected error for missing filesystem")
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		dir, in, want string
	}{
		{"", "foo", "foo"},
		{".", "foo", "foo"},
		{".", ".", "."},
		{"sub", "foo", "sub/foo"},
		{"sub", ".", "sub"},
		{".", "/abs/path", "abs/path"},
		{".", "/", "."},
	}
	for _, tc := range tests {
		ec := &command.ExecContext{Dir: tc.dir}
		if got := resolvePath(ec, tc.in); got != tc.want {
			t.Errorf("resolvePath(dir=%q, in=%q) = %q, want %q", tc.dir, tc.in, got, tc.want)
		}
	}
}
