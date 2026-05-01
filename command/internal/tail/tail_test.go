package tail

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

// syncBuffer is a goroutine-safe bytes.Buffer wrapper for capturing
// follow-mode output written from one goroutine while the test goroutine
// reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lockedFS wraps a billy.Filesystem and serialises Open/OpenFile/Stat with
// any writes made through writeAppend below. The memfs implementation has
// internal races between fileInfo.Size() and content.WriteAt; this wrapper
// exists so the -f follow-mode tests can run cleanly under -race.
type lockedFS struct {
	billy.Filesystem
	mu sync.Mutex
}

func (l *lockedFS) Open(name string) (billy.File, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := l.Filesystem.Open(name)
	if err != nil {
		return nil, err
	}
	return &lockedFile{File: f, mu: &l.mu}, nil
}

func (l *lockedFS) OpenFile(name string, flag int, perm os.FileMode) (billy.File, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := l.Filesystem.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &lockedFile{File: f, mu: &l.mu}, nil
}

func (l *lockedFS) Stat(name string) (os.FileInfo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Filesystem.Stat(name)
}

// lockedFile serialises Read/Write/Seek through the same lock as the
// surrounding lockedFS, so concurrent appends and stats don't race on
// memfs internal state.
type lockedFile struct {
	billy.File
	mu *sync.Mutex
}

func (f *lockedFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.Read(p)
}

func (f *lockedFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.Write(p)
}

func (f *lockedFile) Seek(off int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.Seek(off, whence)
}

func (f *lockedFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.File.Close()
}

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
	write("twelve.txt", []byte("L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n"))
	write("five.txt", []byte("a\nb\nc\nd\ne\n"))
	write("nofinalnl.txt", []byte("x1\nx2\nx3"))
	write("empty.txt", []byte(""))
	write("bytes.txt", []byte("abcdefghij"))
	return fs
}

func run(t *testing.T, args []string, stdin string, fs billy.Filesystem) (string, string, error) {
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

func TestTail(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default 10 lines from stdin",
			stdin:      "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
			wantStdout: "L3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
		},
		{
			name:       "default 10 lines from file",
			args:       []string{"twelve.txt"},
			wantStdout: "L3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
		},
		{
			name:       "fewer lines than requested",
			args:       []string{"five.txt"},
			wantStdout: "a\nb\nc\nd\ne\n",
		},
		{
			name:       "short -n flag",
			args:       []string{"-n", "3", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "short -n no space",
			args:       []string{"-n3", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "long --lines=N",
			args:       []string{"--lines=3", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "GNU shorthand -N",
			args:       []string{"-3", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "from line +N space form",
			args:       []string{"-n", "+10", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "from line +N no space",
			args:       []string{"-n+10", "twelve.txt"},
			wantStdout: "L10\nL11\nL12\n",
		},
		{
			name:       "from line +1 prints all",
			args:       []string{"-n", "+1", "five.txt"},
			wantStdout: "a\nb\nc\nd\ne\n",
		},
		{
			name:       "lines zero produces nothing",
			args:       []string{"-n", "0", "twelve.txt"},
			wantStdout: "",
		},
		{
			name:       "byte mode",
			args:       []string{"-c", "3", "bytes.txt"},
			wantStdout: "hij",
		},
		{
			name:       "byte mode no space",
			args:       []string{"-c3", "bytes.txt"},
			wantStdout: "hij",
		},
		{
			name:       "long --bytes=N",
			args:       []string{"--bytes=3", "bytes.txt"},
			wantStdout: "hij",
		},
		{
			name:       "byte mode beyond eof returns full content",
			args:       []string{"-c", "100", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "fromByte +N space form starts at byte N (1-based)",
			args:       []string{"-c", "+5", "bytes.txt"},
			wantStdout: "efghij",
		},
		{
			name:       "fromByte +N no space",
			args:       []string{"-c+5", "bytes.txt"},
			wantStdout: "efghij",
		},
		{
			name:       "fromByte +1 prints all",
			args:       []string{"-c", "+1", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "fromByte beyond eof prints nothing",
			args:       []string{"-c", "+100", "bytes.txt"},
			wantStdout: "",
		},
		{
			name:       "fromByte +N from stdin preserves no trailing newline",
			args:       []string{"-c", "+2"},
			stdin:      "a\nb\nc",
			wantStdout: "\nb\nc",
		},
		{
			name:       "missing final newline is preserved",
			args:       []string{"-n", "2", "nofinalnl.txt"},
			wantStdout: "x2\nx3",
		},
		{
			name:       "stdin missing final newline single line preserved",
			args:       []string{"-n", "1"},
			stdin:      "a\nb\nc",
			wantStdout: "c",
		},
		{
			name:       "stdin with final newline single line preserved",
			args:       []string{"-n", "1"},
			stdin:      "a\nb\nc\n",
			wantStdout: "c\n",
		},
		{
			name:       "fewer lines than requested no final newline preserved",
			args:       []string{"-n", "5"},
			stdin:      "a\nb\nc",
			wantStdout: "a\nb\nc",
		},
		{
			name:       "fromLine no final newline preserved",
			args:       []string{"-n", "+2"},
			stdin:      "a\nb\nc",
			wantStdout: "b\nc",
		},
		{
			name:       "fromLine with final newline preserved",
			args:       []string{"-n", "+1"},
			stdin:      "a\nb\nc\n",
			wantStdout: "a\nb\nc\n",
		},
		{
			name:       "empty file produces empty output",
			args:       []string{"empty.txt"},
			wantStdout: "",
		},
		{
			name:       "stdin via dash",
			args:       []string{"-n", "2", "-"},
			stdin:      "p\nq\nr\n",
			wantStdout: "q\nr\n",
		},
		{
			name:       "multiple files prepend headers",
			args:       []string{"-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "==> five.txt <==\ne\n\n==> twelve.txt <==\nL12\n",
		},
		{
			name:       "quiet suppresses headers across files",
			args:       []string{"-q", "-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "e\nL12\n",
		},
		{
			name:       "silent alias suppresses headers",
			args:       []string{"--silent", "-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "e\nL12\n",
		},
		{
			name:       "verbose forces header on single file",
			args:       []string{"-v", "-n", "1", "five.txt"},
			wantStdout: "==> five.txt <==\ne\n",
		},
		{
			name:       "missing file errors but other files still processed",
			args:       []string{"-n", "1", "nope.txt", "five.txt"},
			wantStdout: "==> five.txt <==\ne\n",
			wantErrSub: "tail: nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:       "negative lines is invalid",
			args:       []string{"-n", "-5", "five.txt"},
			wantErrSub: "invalid number of lines",
			wantErr:    true,
		},
		{
			name:       "negative bytes is invalid",
			args:       []string{"-c", "-5", "five.txt"},
			wantErrSub: "invalid number of bytes",
			wantErr:    true,
		},
		{
			name:       "non-numeric lines is invalid",
			args:       []string{"-n", "abc", "five.txt"},
			wantErrSub: "invalid number of lines",
			wantErr:    true,
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--nope", "five.txt"},
			wantErr: true,
		},
		{
			name:       "byte mode with K suffix",
			args:       []string{"-c", "1K", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
		},
		{
			name:       "byte mode with b suffix is 512",
			args:       []string{"-c", "1b", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
		},
		{
			name:       "lines with K suffix",
			args:       []string{"-n", "1K", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\n",
		},
		{
			name:       "byte mode with kB (decimal) suffix",
			args:       []string{"-c", "1kB", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "invalid suffix is rejected",
			args:       []string{"-c", "1Q", "bytes.txt"},
			wantErrSub: "invalid number of bytes",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args, tt.stdin, newFS(t))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

// TestFollow exercises -f against a billy memfs file. The test starts
// tail in a goroutine, appends bytes after a short delay, lets the
// follow loop pick them up, then cancels via context.
//
// Determinism is achieved by:
//   - shrinking followPollInterval to a tiny value for the duration of the test;
//   - using a syncBuffer so the test goroutine can read output without races;
//   - polling the captured output for the expected suffix before asserting,
//     within a generous wall-clock budget.
func TestFollow(t *testing.T) {
	prev := followPollInterval
	followPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { followPollInterval = prev })

	fs := &lockedFS{Filesystem: memfs.New()}
	// Seed the file with three lines so the synchronous tail emits "c\n".
	f, err := fs.OpenFile("growing.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("a\nb\nc\n")); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out := &syncBuffer{}
	var stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(""),
		Stdout: out,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- Impl{}.Exec(ctx, ec, []string{"-f", "-n", "1", "growing.txt"})
	}()

	// Wait for the synchronous tail to land in the buffer.
	waitFor(t, time.Second, func() bool {
		return strings.Contains(out.String(), "c\n")
	}, "initial tail not observed; got %q", out)

	// Append more bytes; follow loop should observe and emit them.
	g, err := fs.OpenFile("growing.txt", os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Write([]byte("d\ne\n")); err != nil {
		t.Fatal(err)
	}
	g.Close()

	waitFor(t, time.Second, func() bool {
		s := out.String()
		return strings.Contains(s, "d\ne\n")
	}, "appended bytes not observed; got %q", out)

	// Append once more to confirm the loop keeps polling.
	g2, err := fs.OpenFile("growing.txt", os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g2.Write([]byte("f\n")); err != nil {
		t.Fatal(err)
	}
	g2.Close()

	waitFor(t, time.Second, func() bool {
		return strings.Contains(out.String(), "f\n")
	}, "second append not observed; got %q", out)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tail -f did not return after context cancel")
	}

	// Final stitched output must be: synchronous tail then appended chunks.
	got := out.String()
	want := "c\nd\ne\nf\n"
	if got != want {
		t.Errorf("follow output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestFollowStdinIsNoop verifies that -f without a file operand does not
// hang when stdin is a regular Reader.
func TestFollowStdinIsNoop(t *testing.T) {
	prev := followPollInterval
	followPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { followPollInterval = prev })

	stdout, stderr, err := run(t, []string{"-f", "-n", "1"}, "x\ny\nz\n", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if stdout != "z\n" {
		t.Errorf("stdout = %q, want %q", stdout, "z\n")
	}
}

// waitFor polls cond until it returns true or deadline elapses. The
// failure message is formatted with t.Fatalf if the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, format string, args ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf(format, args...)
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: tail [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--lines=") {
		t.Errorf("lines flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "+NUM") {
		t.Errorf("from-line +NUM doc missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "--follow") {
		t.Errorf("--follow flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "multiplier suffix") {
		t.Errorf("size suffix doc missing from help: %q", stderr)
	}
}
