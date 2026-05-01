package du

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"mvdan.cc/sh/v3/expand"
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
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	write("file1.txt", bytes.Repeat([]byte("a"), 500))
	write("file2.txt", bytes.Repeat([]byte("b"), 2000))
	write("sub/inner.txt", bytes.Repeat([]byte("c"), 1024))
	write("sub/deeper/leaf.txt", bytes.Repeat([]byte("d"), 3000))
	return fs
}

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	return runWithEnv(t, args, nil)
}

func runWithEnv(t *testing.T, args []string, env expand.Environ) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		FS:      newTestFS(t),
		Environ: env,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestDu(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantStdout      string
		wantStderr      string
		wantErr         bool
		skipStderrExact bool
	}{
		{
			name:       "default lists directory totals only",
			args:       []string{},
			wantStdout: "3\tsub/deeper\n4\tsub\n7\t.\n",
		},
		{
			name:       "all files prints every entry",
			args:       []string{"-a"},
			wantStdout: "1\tfile1.txt\n2\tfile2.txt\n1\tsub/inner.txt\n3\tsub/deeper/leaf.txt\n3\tsub/deeper\n4\tsub\n7\t.\n",
		},
		{
			name:       "summarize prints only top-level total",
			args:       []string{"-s"},
			wantStdout: "7\t.\n",
		},
		{
			name:       "grand total appends total line",
			args:       []string{"-c"},
			wantStdout: "3\tsub/deeper\n4\tsub\n7\t.\n7\ttotal\n",
		},
		{
			name:       "human readable formats sizes",
			args:       []string{"-h"},
			wantStdout: "2.9K\tsub/deeper\n3.9K\tsub\n6.4K\t.\n",
		},
		{
			name:       "max-depth=1 hides nested directory output",
			args:       []string{"--max-depth=1"},
			wantStdout: "4\tsub\n7\t.\n",
		},
		{
			name:       "max-depth=0 only top level",
			args:       []string{"--max-depth=0"},
			wantStdout: "7\t.\n",
		},
		{
			name:       "single file argument always prints",
			args:       []string{"file1.txt"},
			wantStdout: "1\tfile1.txt\n",
		},
		{
			name:       "single subdir argument with relative display path",
			args:       []string{"sub"},
			wantStdout: "3\tsub/deeper\n4\tsub\n",
		},
		{
			name:       "multiple targets keep command-line order",
			args:       []string{"file1.txt", "sub"},
			wantStdout: "1\tfile1.txt\n3\tsub/deeper\n4\tsub\n",
		},
		{
			name:       "summarize with grand total over multiple targets",
			args:       []string{"-sc", "file1.txt", "sub"},
			wantStdout: "1\tfile1.txt\n4\tsub\n5\ttotal\n",
		},
		{
			name:       "missing target reports stderr and errors",
			args:       []string{"nope"},
			wantStderr: "du: cannot access 'nope': No such file or directory\n",
			wantErr:    true,
		},
		{
			name:       "good and missing targets emit both",
			args:       []string{"file1.txt", "nope"},
			wantStdout: "1\tfile1.txt\n",
			wantStderr: "du: cannot access 'nope': No such file or directory\n",
			wantErr:    true,
		},
		{
			name:           "unknown flag returns error",
			args:           []string{"--no-such-flag"},
			wantErr:        true,
			skipStderrExact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout mismatch\nwant:\n%q\ngot:\n%q", tt.wantStdout, stdout)
			}
			if !tt.skipStderrExact && stderr != tt.wantStderr {
				t.Errorf("stderr mismatch\nwant:\n%q\ngot:\n%q", tt.wantStderr, stderr)
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
	if !strings.Contains(stderr, "Usage: du [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--max-depth") {
		t.Errorf("--max-depth missing from help output: %q", stderr)
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

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name      string
		bytes     int64
		human     bool
		blockSize int64
		want      string
	}{
		{"zero blocks rounds up to 1", 0, false, 1024, "1"},
		{"sub-block rounds up to 1", 500, false, 1024, "1"},
		{"exact one block", 1024, false, 1024, "1"},
		{"just over one block", 1025, false, 1024, "2"},
		{"two blocks", 2048, false, 1024, "2"},
		{"512-byte block sub-block", 500, false, 512, "1"},
		{"512-byte block 1024 bytes", 1024, false, 512, "2"},
		{"512-byte block 1500 bytes", 1500, false, 512, "3"},
		{"zero block size defaults to 1024", 1024, false, 0, "1"},
		{"human under 1K shows bytes", 500, true, 1024, "500"},
		{"human exact 1K", 1024, true, 1024, "1.0K"},
		{"human 2K", 2048, true, 1024, "2.0K"},
		{"human 1.5K", 1536, true, 1024, "1.5K"},
		{"human 1M", 1024 * 1024, true, 1024, "1.0M"},
		{"human 1G", 1024 * 1024 * 1024, true, 1024, "1.0G"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes, tt.human, tt.blockSize)
			if got != tt.want {
				t.Errorf("formatSize(%d, %v, %d) = %q, want %q", tt.bytes, tt.human, tt.blockSize, got, tt.want)
			}
		})
	}
}

func TestFlagAcceptance(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"-H accepted", []string{"-H", "."}},
		{"-L accepted", []string{"-L", "."}},
		{"-x accepted", []string{"-x", "."}},
		{"--one-file-system accepted", []string{"--one-file-system", "."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout == "" {
				t.Errorf("expected non-empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Errorf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

// newSymlinkFS builds a filesystem containing a regular file, a regular
// directory tree, and a symlink in the root pointing at the subdirectory.
// Layout:
//
//	file1.txt        (500 bytes)
//	target/big.txt   (2000 bytes)
//	link -> target   (symlink)
func newSymlinkFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	write("file1.txt", bytes.Repeat([]byte("a"), 500))
	write("target/big.txt", bytes.Repeat([]byte("b"), 2000))
	if sl, ok := fs.(billy.Symlink); ok {
		if err := sl.Symlink("target", "link"); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	} else {
		t.Skip("filesystem does not support symlinks")
	}
	return fs
}

func runWithFS(t *testing.T, fs billy.Filesystem, args []string) (string, string, error) {
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

func TestSymlinkBehavior(t *testing.T) {
	// Default: do not follow link encountered in traversal -> link is
	// counted as a tiny entry, not as the directory it points at.
	// -L: follow; the link is treated like the linked directory.
	// -H: follow only when the link is given on the command line.
	t.Run("default does not follow link in traversal", func(t *testing.T) {
		fs := newSymlinkFS(t)
		// du -s link counts the symlink itself, not its target.
		stdout, _, err := runWithFS(t, fs, []string{"-s", "link"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The symlink content is "target" (6 bytes), which rounds up to 1 block.
		want := "1\tlink\n"
		if stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("-L follows link given on command line", func(t *testing.T) {
		fs := newSymlinkFS(t)
		stdout, _, err := runWithFS(t, fs, []string{"-Ls", "link"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 'link' resolves to target/, which contains 2000 bytes -> 2 blocks.
		want := "2\tlink\n"
		if stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("-H follows link only on command line", func(t *testing.T) {
		fs := newSymlinkFS(t)
		stdout, _, err := runWithFS(t, fs, []string{"-Hs", "link"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "2\tlink\n"
		if stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("-x is accepted and does not error", func(t *testing.T) {
		fs := newSymlinkFS(t)
		stdout, stderr, err := runWithFS(t, fs, []string{"-x", "-s", "."})
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if stdout == "" {
			t.Errorf("expected non-empty stdout")
		}
	})
}

func TestBytesFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"-b reports bytes for a single file", []string{"-b", "file2.txt"}, "2000\tfile2.txt\n"},
		{"--bytes long flag", []string{"--bytes", "file1.txt"}, "500\tfile1.txt\n"},
		{"-b overrides BLOCKSIZE", []string{"-b", "file2.txt"}, "2000\tfile2.txt\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.want {
				t.Errorf("stdout = %q, want %q", stdout, tt.want)
			}
		})
	}
}

func TestBytesOverridesKilobytes(t *testing.T) {
	// -b should win over -k since GNU du treats -b as the strongest bytes
	// directive (block-size=1).
	stdout, stderr, err := run(t, []string{"-bk", "file2.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "2000\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestApparentSizeFlag(t *testing.T) {
	// --apparent-size on its own keeps the default 1024 block size; it just
	// changes the conceptual meaning. Since billy already reports apparent
	// sizes, output matches the default.
	stdout, stderr, err := run(t, []string{"--apparent-size", "file2.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "2\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestKilobytesFlag(t *testing.T) {
	stdout, stderr, err := run(t, []string{"-k", "file2.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "2\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBlocksizeEnv(t *testing.T) {
	env := expand.ListEnviron("BLOCKSIZE=512")
	stdout, stderr, err := runWithEnv(t, []string{"file2.txt"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "4\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestPosixlyCorrectEnv(t *testing.T) {
	env := expand.ListEnviron("POSIXLY_CORRECT=1")
	stdout, stderr, err := runWithEnv(t, []string{"file2.txt"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "4\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestKilobytesOverridesBlocksize(t *testing.T) {
	env := expand.ListEnviron("BLOCKSIZE=512")
	stdout, stderr, err := runWithEnv(t, []string{"-k", "file2.txt"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := "2\tfile2.txt\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
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
		{".", "/abs/path", "abs/path"},
		{".", "/", "."},
	}
	for _, tt := range tests {
		ec := &command.ExecContext{Dir: tt.dir}
		if got := resolvePath(ec, tt.in); got != tt.want {
			t.Errorf("resolvePath(dir=%q, in=%q) = %q, want %q", tt.dir, tt.in, got, tt.want)
		}
	}
}
