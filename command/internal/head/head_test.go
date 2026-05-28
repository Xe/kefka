package head

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
	// File literally named "-n2" — used to verify "--" terminates option parsing.
	write("-n2", []byte("hello\nworld\n"))
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

func TestHead(t *testing.T) {
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
			wantStdout: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\n",
		},
		{
			name:       "default 10 lines from file",
			args:       []string{"twelve.txt"},
			wantStdout: "L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\n",
		},
		{
			name:       "fewer lines than requested",
			args:       []string{"five.txt"},
			wantStdout: "a\nb\nc\nd\ne\n",
		},
		{
			name:       "short -n flag",
			args:       []string{"-n", "3", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\n",
		},
		{
			name:       "short -n no space",
			args:       []string{"-n3", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\n",
		},
		{
			name:       "long --lines=N",
			args:       []string{"--lines=3", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\n",
		},
		{
			name:       "GNU shorthand -N",
			args:       []string{"-3", "twelve.txt"},
			wantStdout: "L1\nL2\nL3\n",
		},
		{
			name:       "lines zero produces nothing",
			args:       []string{"-n", "0", "twelve.txt"},
			wantStdout: "",
		},
		{
			name:       "byte mode",
			args:       []string{"-c", "5", "bytes.txt"},
			wantStdout: "abcde",
		},
		{
			name:       "byte mode no space",
			args:       []string{"-c5", "bytes.txt"},
			wantStdout: "abcde",
		},
		{
			name:       "long --bytes=N",
			args:       []string{"--bytes=5", "bytes.txt"},
			wantStdout: "abcde",
		},
		{
			name:       "byte mode beyond eof returns full content",
			args:       []string{"-c", "100", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "no trailing newline preserved",
			args:       []string{"-n", "10", "nofinalnl.txt"},
			wantStdout: "x1\nx2\nx3",
		},
		{
			name:       "stdin without trailing newline preserved when truncated",
			args:       []string{"-n", "2"},
			stdin:      "a\nb\nc",
			wantStdout: "a\nb\n",
		},
		{
			name:       "stdin with trailing newline keeps newline",
			args:       []string{"-n", "1"},
			stdin:      "a\nb\n",
			wantStdout: "a\n",
		},
		{
			name:       "stdin shorter than n preserves missing final newline",
			args:       []string{"-n", "5"},
			stdin:      "a\nb",
			wantStdout: "a\nb",
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
			wantStdout: "p\nq\n",
		},
		{
			name:       "multiple files prepend headers",
			args:       []string{"-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "==> five.txt <==\na\n\n==> twelve.txt <==\nL1\n",
		},
		{
			name:       "quiet suppresses headers across files",
			args:       []string{"-q", "-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "a\nL1\n",
		},
		{
			name:       "silent alias suppresses headers",
			args:       []string{"--silent", "-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "a\nL1\n",
		},
		{
			name:       "verbose forces header on single file",
			args:       []string{"-v", "-n", "1", "five.txt"},
			wantStdout: "==> five.txt <==\na\n",
		},
		{
			name:       "verbose with multiple files prints headers",
			args:       []string{"-v", "-n", "1", "five.txt", "twelve.txt"},
			wantStdout: "==> five.txt <==\na\n\n==> twelve.txt <==\nL1\n",
		},
		{
			name:       "double-dash treats -n2 as filename",
			args:       []string{"--", "-n2"},
			wantStdout: "hello\nworld\n",
		},
		{
			name:       "double-dash with -n flag preceding still parses option",
			args:       []string{"-n", "1", "--", "-n2"},
			wantStdout: "hello\n",
		},
		{
			name:       "missing file errors but other files still processed",
			args:       []string{"-n", "1", "nope.txt", "five.txt"},
			wantStdout: "==> five.txt <==\na\n",
			wantErrSub: "head: nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:       "negative lines drops trailing lines",
			args:       []string{"-n", "-2", "five.txt"},
			wantStdout: "a\nb\nc\n",
		},
		{
			name:       "negative lines larger than file produces empty",
			args:       []string{"-n", "-99", "five.txt"},
			wantStdout: "",
		},
		{
			name:       "negative lines with no trailing newline",
			args:       []string{"-n", "-1", "nofinalnl.txt"},
			wantStdout: "x1\nx2\n",
		},
		{
			name:       "negative bytes drops trailing bytes",
			args:       []string{"-c", "-3", "bytes.txt"},
			wantStdout: "abcdefg",
		},
		{
			name:       "negative bytes larger than file produces empty",
			args:       []string{"-c", "-99", "bytes.txt"},
			wantStdout: "",
		},
		{
			name:       "size suffix K bytes",
			args:       []string{"-c", "1K", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "size suffix b bytes (512)",
			args:       []string{"-c", "1b", "bytes.txt"},
			wantStdout: "abcdefghij",
		},
		{
			name:       "size suffix kB decimal",
			args:       []string{"-c", "1kB", "bytes.txt"},
			wantStdout: "abcdefghij",
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

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: head [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--lines=") {
		t.Errorf("lines flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "--bytes=") {
		t.Errorf("bytes flag missing from help: %q", stderr)
	}
}
