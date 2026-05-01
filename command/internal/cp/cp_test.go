package cp

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

// timedFS wraps a billy.Filesystem with persistent atime/mtime storage and
// implements billy.Change so cp -p can round-trip times. memfs alone does
// not store ModTime nor implement billy.Change.
type timedFS struct {
	billy.Filesystem
	mu     sync.Mutex
	atimes map[string]time.Time
	mtimes map[string]time.Time
}

func newTimedFS(inner billy.Filesystem) *timedFS {
	return &timedFS{
		Filesystem: inner,
		atimes:     map[string]time.Time{},
		mtimes:     map[string]time.Time{},
	}
}

func (t *timedFS) Stat(name string) (os.FileInfo, error) {
	info, err := t.Filesystem.Stat(name)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	mt, ok := t.mtimes[name]
	t.mu.Unlock()
	if !ok {
		return info, nil
	}
	return &timedInfo{FileInfo: info, mtime: mt}, nil
}

func (t *timedFS) Chmod(name string, mode os.FileMode) error {
	if c, ok := t.Filesystem.(billy.Chmod); ok {
		return c.Chmod(name, mode)
	}
	return billy.ErrNotSupported
}

func (t *timedFS) Lchown(name string, uid, gid int) error {
	return billy.ErrNotSupported
}

func (t *timedFS) Chown(name string, uid, gid int) error {
	return billy.ErrNotSupported
}

func (t *timedFS) Chtimes(name string, atime, mtime time.Time) error {
	if _, err := t.Filesystem.Stat(name); err != nil {
		return err
	}
	t.mu.Lock()
	t.atimes[name] = atime
	t.mtimes[name] = mtime
	t.mu.Unlock()
	return nil
}

type timedInfo struct {
	os.FileInfo
	mtime time.Time
}

func (t *timedInfo) ModTime() time.Time { return t.mtime }

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

func runWithStdin(t *testing.T, args []string, fs billy.Filesystem, stdin string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func run(t *testing.T, args []string, fs billy.Filesystem) (string, string, error) {
	t.Helper()
	return runWithStdin(t, args, fs, "")
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
		stdin      string
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
		{
			name:       "interactive y overwrites",
			args:       []string{"-i", "hello.txt", "existing/keep.txt"},
			stdin:      "y\n",
			wantErrSub: "overwrite 'existing/keep.txt'?",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
					t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "interactive n keeps target",
			args:       []string{"-i", "hello.txt", "existing/keep.txt"},
			stdin:      "n\n",
			wantErrSub: "overwrite 'existing/keep.txt'?",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
				}
			},
		},
		{
			name:  "i then f: force wins, no prompt",
			args:  []string{"-if", "hello.txt", "existing/keep.txt"},
			stdin: "",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
					t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
				}
			},
		},
		{
			name:       "f then i: interactive wins, prompts",
			args:       []string{"-fi", "hello.txt", "existing/keep.txt"},
			stdin:      "n\n",
			wantErrSub: "overwrite",
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
				}
			},
		},
		{
			name:       "same file diagnostic",
			args:       []string{"hello.txt", "hello.txt"},
			wantErrSub: "'hello.txt' and 'hello.txt' are the same file",
			wantErr:    true,
		},
		{
			name: "double dash with dash-prefixed source",
			args: []string{"--", "-src", "outname"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "outname"); got != "dashy\n" {
					t.Errorf("outname = %q, want %q", got, "dashy\n")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			// Seed for the dash-prefixed source case.
			if tt.name == "double dash with dash-prefixed source" {
				f, err := fs.OpenFile("-src", os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					t.Fatal(err)
				}
				f.Write([]byte("dashy\n"))
				f.Close()
			}
			stdout, stderr, err := runWithStdin(t, tt.args, fs, tt.stdin)
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

func chmod(t *testing.T, fs billy.Filesystem, name string, mode os.FileMode) {
	t.Helper()
	c, ok := fs.(billy.Chmod)
	if !ok {
		t.Fatalf("filesystem does not support Chmod")
	}
	if err := c.Chmod(name, mode); err != nil {
		t.Fatalf("chmod %s: %v", name, err)
	}
}

func TestPreserveModeAndMtime(t *testing.T) {
	mem := memfs.New()
	fs := newTimedFS(mem)

	f, err := fs.OpenFile("src.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("payload\n"))
	f.Close()

	chmod(t, mem, "src.txt", 0o600)
	srcMtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := fs.Chtimes("src.txt", srcMtime, srcMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-p", "src.txt", "dst.txt"}); err != nil {
		t.Fatalf("cp -p: %v; stderr=%q", err, stderr.String())
	}

	info, err := fs.Stat("dst.txt")
	if err != nil {
		t.Fatalf("stat dst.txt: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0o600", got)
	}
	if got := info.ModTime(); !got.Equal(srcMtime) {
		t.Errorf("mtime = %v, want %v", got, srcMtime)
	}
}

func TestCopyPreservesSourceMode(t *testing.T) {
	// Without -p, GNU cp 9.x still creates the destination using the
	// source's permission bits (subject to umask). On memfs the umask is
	// effectively 0 so the source mode round-trips exactly.
	mem := memfs.New()

	f, err := mem.OpenFile("src.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("data\n"))
	f.Close()
	chmod(t, mem, "src.txt", 0o600)

	stdout, stderr, err := run(t, []string{"src.txt", "dst.txt"}, mem)
	if err != nil {
		t.Fatalf("cp: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}

	info, err := mem.Stat("dst.txt")
	if err != nil {
		t.Fatalf("stat dst.txt: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0o600", got)
	}
}

func TestRecursivePreservesFileModes(t *testing.T) {
	mem := memfs.New()
	if err := mem.MkdirAll("d", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := mem.OpenFile("d/secret.txt", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("x\n"))
	f.Close()
	chmod(t, mem, "d/secret.txt", 0o600)

	if _, _, err := run(t, []string{"-R", "d", "d2"}, mem); err != nil {
		t.Fatalf("cp -R: %v", err)
	}

	info, err := mem.Stat("d2/secret.txt")
	if err != nil {
		t.Fatalf("stat d2/secret.txt: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want 0o600", got)
	}
}

// lockingFS wraps a billy.Filesystem and refuses to OpenFile any path in
// `locked` for writing until it has been Removed. This simulates a write-
// protected destination so we can exercise cp -f's unlink-and-retry path.
type lockingFS struct {
	billy.Filesystem
	locked map[string]bool
}

func (l *lockingFS) OpenFile(name string, flag int, perm os.FileMode) (billy.File, error) {
	if l.locked[name] && (flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0) {
		return nil, os.ErrPermission
	}
	return l.Filesystem.OpenFile(name, flag, perm)
}

func (l *lockingFS) Remove(name string) error {
	delete(l.locked, name)
	return l.Filesystem.Remove(name)
}

func TestForceUnlinksOnWriteFailure(t *testing.T) {
	mem := memfs.New()
	fs := &lockingFS{Filesystem: mem, locked: map[string]bool{"dst.txt": true}}

	write := func(name, data string) {
		f, err := mem.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(data))
		f.Close()
	}
	write("src.txt", "fresh\n")
	write("dst.txt", "stale\n")

	// Without -f, cp should fail because the destination is locked.
	stdout, stderr, err := run(t, []string{"src.txt", "dst.txt"}, fs)
	if err == nil {
		t.Fatalf("expected error without -f; stdout=%q stderr=%q", stdout, stderr)
	}
	if got := readFile(t, mem, "dst.txt"); got != "stale\n" {
		t.Errorf("dst.txt should still be stale, got %q", got)
	}

	// With -f, cp should unlink the destination and retry, succeeding.
	stdout, stderr, err = run(t, []string{"-f", "src.txt", "dst.txt"}, fs)
	if err != nil {
		t.Fatalf("cp -f failed: %v; stdout=%q stderr=%q", err, stdout, stderr)
	}
	if got := readFile(t, mem, "dst.txt"); got != "fresh\n" {
		t.Errorf("dst.txt = %q, want %q", got, "fresh\n")
	}
}

func TestSameFileNormalizedPaths(t *testing.T) {
	// path.Clean should normalize "./hello.txt" and "hello.txt" into the
	// same internal target; cp must detect this as same-file.
	fs := newFS(t)
	_, stderr, err := run(t, []string{"./hello.txt", "hello.txt"}, fs)
	if err == nil {
		t.Fatal("expected error for same-file copy")
	}
	if !strings.Contains(stderr, "are the same file") {
		t.Errorf("stderr should mention same file, got %q", stderr)
	}
}

func TestForceLastWinsOverNoClobber(t *testing.T) {
	// GNU honors the last-specified of -f / -n / -i. -nf should overwrite.
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-nf", "hello.txt", "existing/keep.txt"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
		t.Errorf("-nf should overwrite, got %q", got)
	}
}

func TestNoClobberLastWinsOverForce(t *testing.T) {
	// -fn should NOT overwrite — -n was specified last.
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-fn", "hello.txt", "existing/keep.txt"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
		t.Errorf("-fn should not overwrite, got %q", got)
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
	if !strings.Contains(stderr, "-i, --interactive") {
		t.Errorf("interactive flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-f, --force") {
		t.Errorf("force flag missing from help: %q", stderr)
	}
	for _, want := range []string{"-H", "-L, --dereference", "-P, --no-dereference"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("symlink flag %q missing from help: %q", want, stderr)
		}
	}
}

// newSymlinkFS builds a memfs containing a target file and a symlink to it.
//
//	target.txt  (content "linked\n")
//	link        -> target.txt
//	dir/inner.txt  (content "inner\n")
//	dir/dlink   -> ../target.txt
func newSymlinkFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name, data string) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(data))
		f.Close()
	}
	if err := fs.MkdirAll("dir", 0o755); err != nil {
		t.Fatal(err)
	}
	write("target.txt", "linked\n")
	write("dir/inner.txt", "inner\n")
	sl, ok := fs.(billy.Symlink)
	if !ok {
		t.Skip("memfs does not support symlinks")
	}
	if err := sl.Symlink("target.txt", "link"); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := sl.Symlink("../target.txt", "dir/dlink"); err != nil {
		t.Fatalf("symlink dir/dlink: %v", err)
	}
	return fs
}

func TestSymlinkDefaultFollows(t *testing.T) {
	// Without -R, GNU cp dereferences command-line symlinks (-L semantics).
	// "cp link out" copies the contents of target.txt to out.
	fs := newSymlinkFS(t)
	if _, _, err := run(t, []string{"link", "out.txt"}, fs); err != nil {
		t.Fatalf("cp: %v", err)
	}
	if got := readFile(t, fs, "out.txt"); got != "linked\n" {
		t.Errorf("out.txt = %q, want %q", got, "linked\n")
	}
}

func TestSymlinkP_PreservesLink(t *testing.T) {
	fs := newSymlinkFS(t)
	if _, _, err := run(t, []string{"-P", "link", "out"}, fs); err != nil {
		t.Fatalf("cp -P: %v", err)
	}
	sl := fs.(billy.Symlink)
	info, err := sl.Lstat("out")
	if err != nil {
		t.Fatalf("lstat out: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("out is not a symlink: mode=%v", info.Mode())
	}
	got, err := sl.Readlink("out")
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if got != "target.txt" {
		t.Errorf("readlink out = %q, want %q", got, "target.txt")
	}
}

func TestSymlinkRecursive_DefaultPreservesInTree(t *testing.T) {
	// With -R and no explicit symlink mode, GNU defaults to -P for in-tree
	// links (the link is recreated, not dereferenced).
	fs := newSymlinkFS(t)
	if _, _, err := run(t, []string{"-R", "dir", "dir2"}, fs); err != nil {
		t.Fatalf("cp -R: %v", err)
	}
	sl := fs.(billy.Symlink)
	info, err := sl.Lstat("dir2/dlink")
	if err != nil {
		t.Fatalf("lstat dir2/dlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("dir2/dlink should be a symlink, got mode=%v", info.Mode())
	}
	if got := readFile(t, fs, "dir2/inner.txt"); got != "inner\n" {
		t.Errorf("dir2/inner.txt = %q, want %q", got, "inner\n")
	}
}

func TestSymlinkRecursiveL_DereferencesAll(t *testing.T) {
	// -RL dereferences every symlink in the source tree.
	fs := newSymlinkFS(t)
	if _, _, err := run(t, []string{"-RL", "dir", "dir2"}, fs); err != nil {
		t.Fatalf("cp -RL: %v", err)
	}
	// dir2/dlink should be a regular file containing target.txt's contents.
	sl := fs.(billy.Symlink)
	info, err := sl.Lstat("dir2/dlink")
	if err != nil {
		t.Fatalf("lstat dir2/dlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("dir2/dlink should not be a symlink under -L: mode=%v", info.Mode())
	}
	if got := readFile(t, fs, "dir2/dlink"); got != "linked\n" {
		t.Errorf("dir2/dlink = %q, want %q", got, "linked\n")
	}
}

func TestSymlinkNotSupportedDiagnostic(t *testing.T) {
	// When the backend lacks symlink support, -P on a non-symlink still
	// works (it falls through to plain copy). The "not supported" path is
	// only reached when we actually need to recreate a symlink, which
	// requires Lstat to detect one in the first place — so on a pure
	// non-symlink FS the user gets normal copy semantics. This test
	// verifies we don't crash and produce a sensible result.
	mem := memfs.New()
	f, err := mem.OpenFile("x.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("plain\n"))
	f.Close()
	if _, _, err := run(t, []string{"-P", "x.txt", "y.txt"}, mem); err != nil {
		t.Fatalf("cp -P on regular file: %v", err)
	}
	if got := readFile(t, mem, "y.txt"); got != "plain\n" {
		t.Errorf("y.txt = %q, want %q", got, "plain\n")
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
