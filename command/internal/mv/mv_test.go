package mv

import (
	"bytes"
	"context"
	"io"
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
	return runWithStdin(t, args, fs, "")
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
			name: "no-clobber wins when last over force",
			args: []string{"-f", "-n", "hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
					t.Errorf("existing/keep.txt = %q, want %q (untouched, -n wins because last)", got, "keep\n")
				}
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should still exist (move was skipped)")
				}
			},
		},
		{
			name: "force wins when last over no-clobber",
			args: []string{"-n", "-f", "hello.txt", "existing/keep.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
					t.Errorf("existing/keep.txt = %q, want %q (overwritten, -f wins because last)", got, "hello\n")
				}
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should be gone after move")
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
	if !strings.Contains(stderr, "-i, --interactive") {
		t.Errorf("interactive flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-n, --no-clobber") {
		t.Errorf("no-clobber flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-v, --verbose") {
		t.Errorf("verbose flag missing from help: %q", stderr)
	}
}

func TestInteractiveYes(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := runWithStdin(t, []string{"-i", "hello.txt", "existing/keep.txt"}, fs, "y\n")
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "overwrite 'existing/keep.txt'?") {
		t.Errorf("stderr should contain prompt, got %q", stderr)
	}
	if exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should be gone")
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
		t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
	}
}

func TestInteractiveYesUppercase(t *testing.T) {
	fs := newFS(t)
	_, _, err := runWithStdin(t, []string{"-i", "hello.txt", "existing/keep.txt"}, fs, "Yes\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
		t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
	}
}

func TestInteractiveNo(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-i", "hello.txt", "existing/keep.txt"}, fs, "n\n")
	if err != nil {
		t.Fatalf("decline should not be an error, got: %v", err)
	}
	if !strings.Contains(stderr, "overwrite 'existing/keep.txt'?") {
		t.Errorf("stderr should contain prompt, got %q", stderr)
	}
	if !exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should still exist (declined)")
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
		t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
	}
}

func TestInteractiveEmptyLine(t *testing.T) {
	fs := newFS(t)
	_, _, err := runWithStdin(t, []string{"-i", "hello.txt", "existing/keep.txt"}, fs, "")
	if err != nil {
		t.Fatalf("empty stdin should not be an error, got: %v", err)
	}
	if !exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should still exist (declined)")
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
		t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
	}
}

func TestInteractiveTargetMissing(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-i", "hello.txt", "fresh.txt"}, fs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stderr, "overwrite") {
		t.Errorf("no prompt should appear when destination doesn't exist; stderr=%q", stderr)
	}
	if got := readFile(t, fs, "fresh.txt"); got != "hello\n" {
		t.Errorf("fresh.txt = %q, want %q", got, "hello\n")
	}
}

func TestForceWinsWhenLast(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-i", "-f", "hello.txt", "existing/keep.txt"}, fs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if strings.Contains(stderr, "overwrite") {
		t.Errorf("-f after -i should suppress prompt; stderr=%q", stderr)
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
		t.Errorf("existing/keep.txt = %q, want %q (overwritten)", got, "hello\n")
	}
}

func TestInteractiveWinsWhenLast(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-f", "-i", "hello.txt", "existing/keep.txt"}, fs, "y\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "overwrite 'existing/keep.txt'?") {
		t.Errorf("-i after -f should still prompt; stderr=%q", stderr)
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "hello\n" {
		t.Errorf("existing/keep.txt = %q, want %q", got, "hello\n")
	}
}

func TestSameFile(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"hello.txt", "hello.txt"}, fs)
	if err == nil {
		t.Fatalf("expected error for same-file move")
	}
	if !strings.Contains(stderr, "are the same file") {
		t.Errorf("stderr should mention same file; got %q", stderr)
	}
	if !exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should still exist after same-file refusal")
	}
}

func TestSameFileAmongMany(t *testing.T) {
	fs := newFS(t)
	// move hello.txt, two.txt, and existing/keep.txt into existing/
	// existing/keep.txt -> existing/keep.txt is the same file; others should succeed
	_, stderr, err := run(t, []string{"hello.txt", "two.txt", "existing/keep.txt", "existing"}, fs)
	if err == nil {
		t.Fatalf("expected error because one src is the same as a dst file")
	}
	if !strings.Contains(stderr, "are the same file") {
		t.Errorf("stderr should mention same file; got %q", stderr)
	}
	if exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should have been moved")
	}
	if exists(t, fs, "two.txt") {
		t.Errorf("two.txt should have been moved")
	}
	if got := readFile(t, fs, "existing/hello.txt"); got != "hello\n" {
		t.Errorf("existing/hello.txt = %q, want %q", got, "hello\n")
	}
	if got := readFile(t, fs, "existing/two.txt"); got != "world\n" {
		t.Errorf("existing/two.txt = %q, want %q", got, "world\n")
	}
	if got := readFile(t, fs, "existing/keep.txt"); got != "keep\n" {
		t.Errorf("existing/keep.txt = %q, want %q (untouched)", got, "keep\n")
	}
}

func TestDashDashWithDashFile(t *testing.T) {
	fs := newFS(t)
	// create a file whose name starts with a dash
	f, err := fs.OpenFile("-src", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("dash\n"))
	f.Close()

	_, stderr, err := run(t, []string{"--", "-src", "dst.txt"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if exists(t, fs, "-src") {
		t.Errorf("-src should be gone after move")
	}
	if got := readFile(t, fs, "dst.txt"); got != "dash\n" {
		t.Errorf("dst.txt = %q, want %q", got, "dash\n")
	}
}

func TestTrailingSlashOnNonDirDest(t *testing.T) {
	fs := newFS(t)
	// POSIX: single-source mv where dest ends with slash and isn't an
	// existing directory must fail without processing the source.
	_, stderr, err := run(t, []string{"hello.txt", "newpath/"}, fs)
	if err == nil {
		t.Fatalf("expected error for trailing-slash on non-dir target")
	}
	if !strings.Contains(stderr, "Not a directory") {
		t.Errorf("stderr should mention Not a directory; got %q", stderr)
	}
	if !exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should still exist after refusal")
	}
	if exists(t, fs, "newpath") {
		t.Errorf("newpath should not have been created")
	}
}

func TestTrailingSlashOnExistingDir(t *testing.T) {
	fs := newFS(t)
	// Trailing slash on an existing directory is fine and should move
	// the source into that directory.
	_, stderr, err := run(t, []string{"hello.txt", "dir/"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if exists(t, fs, "hello.txt") {
		t.Errorf("hello.txt should be gone after move")
	}
	if got := readFile(t, fs, "dir/hello.txt"); got != "hello\n" {
		t.Errorf("dir/hello.txt = %q, want %q", got, "hello\n")
	}
}

func TestTrailingSlashVerboseDisplay(t *testing.T) {
	fs := newFS(t)
	stdout, _, err := run(t, []string{"-v", "hello.txt", "dir/"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verbose output should join cleanly without producing "dir//hello.txt"
	want := "renamed 'hello.txt' -> 'dir/hello.txt'\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
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
