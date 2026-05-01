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
			name:       "chars flag on ASCII matches byte count",
			args:       []string{"-m"},
			stdin:      "abcde",
			wantStdout: "5\n",
		},
		{
			name:       "chars flag counts runes for multibyte input",
			args:       []string{"-m"},
			stdin:      "λλλ",
			wantStdout: "3\n",
		},
		{
			name:       "bytes flag counts bytes for multibyte input",
			args:       []string{"-c"},
			stdin:      "λλλ",
			wantStdout: "6\n",
		},
		{
			name:       "bytes and chars together show both columns",
			args:       []string{"-c", "-m"},
			stdin:      "λλλ",
			wantStdout: "3 6\n",
		},
		{
			name:       "lines words chars bytes ordering",
			args:       []string{"-lwmc"},
			stdin:      "λλλ\n",
			wantStdout: "1 1 4 7\n",
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
			name:       "nbsp splits words",
			args:       []string{"-w"},
			stdin:      "alpha beta gamma",
			wantStdout: "3\n",
		},
		{
			name:       "ascii word splitting unchanged",
			args:       []string{"-w"},
			stdin:      "  one two\tthree\nfour  ",
			wantStdout: "4\n",
		},
		{
			name:       "max line length flag",
			args:       []string{"-L"},
			stdin:      "short\nverylonger\nmid\n",
			wantStdout: "10\n",
		},
		{
			name:       "max line length long form",
			args:       []string{"--max-line-length"},
			stdin:      "abc\ndefgh\n",
			wantStdout: "5\n",
		},
		{
			name:       "max line length expands tabs to next stop",
			args:       []string{"-L"},
			stdin:      "a\tb\n",
			wantStdout: "9\n",
		},
		{
			name:       "max line length counts wide chars as width 2",
			args:       []string{"-L"},
			stdin:      "日本\n",
			wantStdout: "4\n",
		},
		{
			name:       "max line length combines with other flags in fixed order",
			args:       []string{"-lwL"},
			stdin:      "abc def\nhi\n",
			wantStdout: "2 3 7\n",
		},
		{
			name:       "max line length on file shows length",
			args:       []string{"-L", "multi.txt"},
			stdin:      "",
			wantStdout: "13 multi.txt\n",
		},
		{
			name:       "max line length multi-file uses max not sum",
			args:       []string{"-L", "hello.txt", "multi.txt"},
			wantStdout: " 11 hello.txt\n 13 multi.txt\n 13 total\n",
		},
		{
			name:       "no trailing newline reports zero lines",
			args:       []string{"-l"},
			stdin:      "no newline here",
			wantStdout: "0\n",
		},
		{
			name:       "single line with trailing newline reports one line",
			args:       []string{"-l"},
			stdin:      "with newline\n",
			wantStdout: "1\n",
		},
		{
			name:       "no trailing newline file reports zero lines",
			args:       []string{"-l", "nolf.txt"},
			wantStdout: "0 nolf.txt\n",
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
			wantErrSub: "wc: nope.txt: ",
			wantErr:    true,
		},
		{
			name:       "missing file in list still counts present files",
			args:       []string{"hello.txt", "nope.txt"},
			wantStdout: "  1   2  12 hello.txt\n  1   2  12 total\n",
			wantErrSub: "wc: nope.txt: ",
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

func TestMissingFileDiagnosticSurfacesErrno(t *testing.T) {
	_, stderr, err := run(t, []string{"nope.txt"}, "", newFS(t))
	if err == nil {
		t.Fatalf("expected error, got nil; stderr=%q", stderr)
	}
	if !strings.HasPrefix(stderr, "wc: nope.txt: ") {
		t.Errorf("stderr should start with %q, got %q", "wc: nope.txt: ", stderr)
	}
	if strings.Contains(stderr, "No such file or directory") {
		t.Errorf("stderr should not hardcode 'No such file or directory'; got %q", stderr)
	}
	suffix := strings.TrimPrefix(strings.TrimRight(stderr, "\n"), "wc: nope.txt: ")
	if suffix == "" {
		t.Errorf("expected an underlying error message after %q, got %q", "wc: nope.txt: ", stderr)
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
		{"empty", "", stats{}},
		{"single word no newline", "hello", stats{lines: 0, words: 1, chars: 5, bytes: 5, maxLine: 5}},
		{"single line with newline", "hello\n", stats{lines: 1, words: 1, chars: 6, bytes: 6, maxLine: 5}},
		{"multiple words", "a b c", stats{lines: 0, words: 3, chars: 5, bytes: 5, maxLine: 5}},
		{"tab separated", "a\tb\tc", stats{lines: 0, words: 3, chars: 5, bytes: 5, maxLine: 17}},
		{"mixed whitespace", "  a  b  ", stats{lines: 0, words: 2, chars: 8, bytes: 8, maxLine: 8}},
		{"three lines", "one\ntwo\nthree\n", stats{lines: 3, words: 3, chars: 14, bytes: 14, maxLine: 5}},
		{"only newlines", "\n\n\n", stats{lines: 3, words: 0, chars: 3, bytes: 3, maxLine: 0}},
		{"multibyte chars vs bytes", "λλλ", stats{lines: 0, words: 1, chars: 3, bytes: 6, maxLine: 3}},
		{"nbsp word separator", "a b", stats{lines: 0, words: 2, chars: 3, bytes: 4, maxLine: 3}},
		{"cr resets column", "aa\rbbbbb\n", stats{lines: 1, words: 2, chars: 9, bytes: 9, maxLine: 5}},
		{"east asian wide width", "日本\n", stats{lines: 1, words: 1, chars: 3, bytes: 7, maxLine: 4}},
		{"tab to column 9 with letter", "a\tb\n", stats{lines: 1, words: 2, chars: 4, bytes: 4, maxLine: 9}},
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
