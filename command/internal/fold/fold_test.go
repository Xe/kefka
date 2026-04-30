package fold

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
	write("hello.txt", []byte("hello world\n"))
	write("long.txt", []byte("abcdefghij\n"))
	write("multi.txt", []byte("foo\nbar\n"))
	write("words.txt", []byte("the quick brown fox\n"))
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

func TestFold(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default width passes short input through",
			args:       nil,
			stdin:      "hello world\n",
			wantStdout: "hello world\n",
		},
		{
			name:       "wraps at width from stdin",
			args:       []string{"-w", "5"},
			stdin:      "abcdefghij\n",
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "long flag width",
			args:       []string{"--width=5"},
			stdin:      "abcdefghij\n",
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "short flag with attached value",
			args:       []string{"-w5"},
			stdin:      "abcdefghij\n",
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "break at spaces",
			args:       []string{"-s", "-w", "10"},
			stdin:      "the quick brown fox\n",
			wantStdout: "the quick \nbrown fox\n",
		},
		{
			name:       "combined short flags -sw",
			args:       []string{"-sw", "10"},
			stdin:      "the quick brown fox\n",
			wantStdout: "the quick \nbrown fox\n",
		},
		{
			name:       "no space to break on falls back to width",
			args:       []string{"-s", "-w", "5"},
			stdin:      "abcdefghij\n",
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "empty input produces empty output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "no trailing newline preserved",
			args:       []string{"-w", "5"},
			stdin:      "abcdefghij",
			wantStdout: "abcde\nfghij",
		},
		{
			name:       "multiple lines folded independently",
			args:       []string{"-w", "3"},
			stdin:      "abcdef\nxyz\n",
			wantStdout: "abc\ndef\nxyz\n",
		},
		{
			name:       "tab expands to next column boundary",
			args:       []string{"-w", "9"},
			stdin:      "a\tb\n",
			wantStdout: "a\tb\n",
		},
		{
			name:       "tab pushes line over width and wraps",
			args:       []string{"-w", "5"},
			stdin:      "a\tbc\n",
			wantStdout: "a\n\t\nbc\n",
		},
		{
			name:       "byte mode counts bytes not columns",
			args:       []string{"-b", "-w", "3"},
			stdin:      "abcdef\n",
			wantStdout: "abc\ndef\n",
		},
		{
			name:       "byte mode treats tab as one byte",
			args:       []string{"-b", "-w", "3"},
			stdin:      "a\tbc\n",
			wantStdout: "a\tb\nc\n",
		},
		{
			name:       "fold from file",
			args:       []string{"-w", "5", "long.txt"},
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "concatenates multiple files",
			args:       []string{"-w", "3", "long.txt", "multi.txt"},
			wantStdout: "abc\ndef\nghi\nj\nfoo\nbar\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-w", "5", "-"},
			stdin:      "abcdefghij\n",
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "double dash terminator",
			args:       []string{"-w", "5", "--", "long.txt"},
			wantStdout: "abcde\nfghij\n",
		},
		{
			name:       "invalid width errors",
			args:       []string{"-w", "abc"},
			stdin:      "hello\n",
			wantErrSub: "invalid number of columns",
			wantErr:    true,
		},
		{
			name:       "zero width errors",
			args:       []string{"-w", "0"},
			stdin:      "hello\n",
			wantErrSub: "invalid number of columns",
			wantErr:    true,
		},
		{
			name:       "missing file errors",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--no-such-flag"},
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
	if !strings.Contains(stderr, "Usage: fold [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-w WIDTH") {
		t.Errorf("width flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-s") {
		t.Errorf("spaces flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-b") {
		t.Errorf("bytes flag missing from help: %q", stderr)
	}
}
