package wc

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
	write("multi.txt", []byte("one two three\nfour five\nsix\n"))
	write("empty.txt", []byte(""))
	write("nolf.txt", []byte("trailing"))
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

func TestWc(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default counts from stdin",
			args:       nil,
			stdin:      "hello world\n",
			wantStdout: "1 2 12\n",
		},
		{
			name:       "default counts from file",
			args:       []string{"hello.txt"},
			wantStdout: " 1  2 12 hello.txt\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "hello world\n",
			wantStdout: " 1  2 12 -\n",
		},
		{
			name:       "empty stdin yields zeros",
			args:       nil,
			stdin:      "",
			wantStdout: "0 0 0\n",
		},
		{
			name:       "empty file yields zeros with filename",
			args:       []string{"empty.txt"},
			wantStdout: "0 0 0 empty.txt\n",
		},
		{
			name:       "lines flag only",
			args:       []string{"-l"},
			stdin:      "a\nb\nc\n",
			wantStdout: "3\n",
		},
		{
			name:       "words flag only",
			args:       []string{"-w"},
			stdin:      "alpha beta gamma\n",
			wantStdout: "3\n",
		},
		{
			name:       "bytes flag only",
			args:       []string{"-c"},
			stdin:      "abcde",
			wantStdout: "5\n",
		},
		{
			name:       "chars flag treats bytes the same as -c",
			args:       []string{"-m"},
			stdin:      "abcde",
			wantStdout: "5\n",
		},
		{
			name:       "long lines flag",
			args:       []string{"--lines"},
			stdin:      "a\nb\n",
			wantStdout: "2\n",
		},
		{
			name:       "trailing word with no newline still counts",
			args:       []string{"-lw"},
			stdin:      "trailing",
			wantStdout: "0 1\n",
		},
		{
			name:       "tab and cr split words",
			args:       []string{"-w"},
			stdin:      "a\tb\rc d",
			wantStdout: "4\n",
		},
		{
			name:       "multi file totals",
			args:       []string{"hello.txt", "multi.txt"},
			wantStdout: "  1   2  12 hello.txt\n  3   6  28 multi.txt\n  4   8  40 total\n",
		},
		{
			name:       "multi file with -l flag",
			args:       []string{"-l", "hello.txt", "multi.txt"},
			wantStdout: "  1 hello.txt\n  3 multi.txt\n  4 total\n",
		},
		{
			name:       "missing file reports error",
			args:       []string{"nope.txt"},
			wantStdout: "",
			wantErrSub: "wc: nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:       "missing file in list still counts present files",
			args:       []string{"hello.txt", "nope.txt"},
			wantStdout: "  1   2  12 hello.txt\n  1   2  12 total\n",
			wantErrSub: "wc: nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag returns error",
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
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, tt.wantStdout)
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
	if !strings.Contains(stderr, "Usage: wc [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-l, --lines") {
		t.Errorf("lines flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-w, --words") {
		t.Errorf("words flag missing from help: %q", stderr)
	}
}

func TestNilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestCountStats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  stats
	}{
		{"empty", "", stats{lines: 0, words: 0, chars: 0}},
		{"single word no newline", "hello", stats{lines: 0, words: 1, chars: 5}},
		{"single line with newline", "hello\n", stats{lines: 1, words: 1, chars: 6}},
		{"multiple words", "a b c", stats{lines: 0, words: 3, chars: 5}},
		{"tab separated", "a\tb\tc", stats{lines: 0, words: 3, chars: 5}},
		{"mixed whitespace", "  a  b  ", stats{lines: 0, words: 2, chars: 8}},
		{"three lines", "one\ntwo\nthree\n", stats{lines: 3, words: 3, chars: 14}},
		{"only newlines", "\n\n\n", stats{lines: 3, words: 0, chars: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countStats([]byte(tt.input))
			if got != tt.want {
				t.Errorf("countStats(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}
