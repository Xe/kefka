package nl

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
	write("hello.txt", []byte("hello\nworld\n"))
	write("blanks.txt", []byte("alpha\n\nbravo\n\ncharlie\n"))
	write("nofinalnl.txt", []byte("foo\nbar"))
	write("empty.txt", []byte(""))
	write("part1.txt", []byte("a\nb\n"))
	write("part2.txt", []byte("c\nd\n"))
	write("sections.txt", []byte("\\:\\:\\:\nh1\nh2\n\\:\\:\nb1\n\nb2\n\\:\nf1\nf2\n"))
	write("manyblanks.txt", []byte("a\n\n\n\n\nb\n\n\n\n\nc\n"))
	write("custom.txt", []byte("#@#@#@\nh1\n#@#@\nb1\n#@\nf1\n"))
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

func TestNl(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default numbers non-empty lines from stdin",
			args:       nil,
			stdin:      "alpha\nbravo\n",
			wantStdout: "     1\talpha\n     2\tbravo\n",
		},
		{
			name:       "default skips blank lines (style t)",
			args:       []string{"blanks.txt"},
			wantStdout: "     1\talpha\n       \n     2\tbravo\n       \n     3\tcharlie\n",
		},
		{
			name:       "style a numbers all lines",
			args:       []string{"-b", "a", "blanks.txt"},
			wantStdout: "     1\talpha\n     2\t\n     3\tbravo\n     4\t\n     5\tcharlie\n",
		},
		{
			name:       "style n numbers no lines",
			args:       []string{"-b", "n", "hello.txt"},
			wantStdout: "       hello\n       world\n",
		},
		{
			name:       "style attached to flag (-ba)",
			args:       []string{"-ba", "blanks.txt"},
			wantStdout: "     1\talpha\n     2\t\n     3\tbravo\n     4\t\n     5\tcharlie\n",
		},
		{
			name:       "left-justified format",
			args:       []string{"-n", "ln", "hello.txt"},
			wantStdout: "1     \thello\n2     \tworld\n",
		},
		{
			name:       "right-justified zero padding",
			args:       []string{"-n", "rz", "-w", "3", "hello.txt"},
			wantStdout: "001\thello\n002\tworld\n",
		},
		{
			name:       "custom width",
			args:       []string{"-w", "3", "hello.txt"},
			wantStdout: "  1\thello\n  2\tworld\n",
		},
		{
			name:       "custom separator",
			args:       []string{"-s", ": ", "hello.txt"},
			wantStdout: "     1: hello\n     2: world\n",
		},
		{
			name:       "starting number",
			args:       []string{"-v", "10", "hello.txt"},
			wantStdout: "    10\thello\n    11\tworld\n",
		},
		{
			name:       "increment",
			args:       []string{"-i", "5", "hello.txt"},
			wantStdout: "     1\thello\n     6\tworld\n",
		},
		{
			name:       "no trailing newline preserved",
			args:       []string{"nofinalnl.txt"},
			wantStdout: "     1\tfoo\n     2\tbar",
		},
		{
			name:       "empty file produces no output",
			args:       []string{"empty.txt"},
			wantStdout: "",
		},
		{
			name:       "empty stdin produces no output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "alpha\nbravo\n",
			wantStdout: "     1\talpha\n     2\tbravo\n",
		},
		{
			name:       "multiple files share line counter",
			args:       []string{"part1.txt", "part2.txt"},
			wantStdout: "     1\ta\n     2\tb\n     3\tc\n     4\td\n",
		},
		{
			name:       "invalid body style",
			args:       []string{"-b", "x", "hello.txt"},
			wantErrSub: "invalid body numbering style",
			wantErr:    true,
		},
		{
			name:       "invalid number format",
			args:       []string{"-n", "xx", "hello.txt"},
			wantErrSub: "invalid line numbering format",
			wantErr:    true,
		},
		{
			name:       "invalid width zero",
			args:       []string{"-w", "0", "hello.txt"},
			wantErrSub: "invalid line number field width",
			wantErr:    true,
		},
		{
			name:       "invalid width non-numeric",
			args:       []string{"-w", "abc", "hello.txt"},
			wantErrSub: "invalid line number field width",
			wantErr:    true,
		},
		{
			name:       "invalid starting number",
			args:       []string{"-v", "abc", "hello.txt"},
			wantErrSub: "invalid starting line number",
			wantErr:    true,
		},
		{
			name:       "invalid increment",
			args:       []string{"-i", "abc", "hello.txt"},
			wantErrSub: "invalid line number increment",
			wantErr:    true,
		},
		{
			name:       "missing file",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: true,
		},
		{
			name:       "header and footer numbered separately, page resets",
			args:       []string{"-ha", "-ba", "-fa", "sections.txt"},
			wantStdout: "\n     1\th1\n     2\th2\n\n     1\tb1\n     2\t\n     3\tb2\n\n     1\tf1\n     2\tf2\n",
		},
		{
			name:       "p flag keeps numbering across page breaks",
			args:       []string{"-ha", "-ba", "-fa", "-p", "-v", "10", "sections.txt"},
			wantStdout: "\n    10\th1\n    11\th2\n\n    12\tb1\n    13\t\n    14\tb2\n\n    15\tf1\n    16\tf2\n",
		},
		{
			name:       "header and footer default to n",
			args:       []string{"sections.txt"},
			wantStdout: "\n       h1\n       h2\n\n     1\tb1\n       \n     2\tb2\n\n       f1\n       f2\n",
		},
		{
			name:       "explicit -h n -b a -f n numbers only body including blanks",
			args:       []string{"-h", "n", "-b", "a", "-f", "n", "sections.txt"},
			wantStdout: "\n       h1\n       h2\n\n     1\tb1\n     2\t\n     3\tb2\n\n       f1\n       f2\n",
		},
		{
			name:       "join blank lines with l 3",
			args:       []string{"-ba", "-l", "3", "manyblanks.txt"},
			wantStdout: "     1\ta\n       \n       \n     2\t\n       \n     3\tb\n       \n       \n     4\t\n       \n     5\tc\n",
		},
		{
			name:       "unnumbered prefix is width plus sep length",
			args:       []string{"-bn", "-w", "3", "-s", ": ", "hello.txt"},
			wantStdout: "     hello\n     world\n",
		},
		{
			name:       "custom delimiter",
			args:       []string{"-d", "#@", "-ha", "-ba", "-fa", "custom.txt"},
			wantStdout: "\n     1\th1\n\n     1\tb1\n\n     1\tf1\n",
		},
		{
			name:       "single-char delim defaults second to colon",
			args:       []string{"-d", "#", "-ha", "custom.txt"},
			wantStdout: "     1\t#@#@#@\n     2\th1\n     3\t#@#@\n     4\tb1\n     5\t#@\n     6\tf1\n",
		},
		{
			name:       "pBRE numbers only matching lines",
			args:       []string{"-b", "pba", "hello.txt"},
			wantStdout: "       hello\n       world\n",
		},
		{
			name:       "pBRE matches",
			args:       []string{"-b", "p^h", "hello.txt"},
			wantStdout: "     1\thello\n       world\n",
		},
		{
			name:       "invalid header style",
			args:       []string{"-h", "x", "hello.txt"},
			wantErrSub: "invalid header numbering style",
			wantErr:    true,
		},
		{
			name:       "invalid footer style",
			args:       []string{"-f", "x", "hello.txt"},
			wantErrSub: "invalid footer numbering style",
			wantErr:    true,
		},
		{
			name:       "invalid join blanks",
			args:       []string{"-l", "0", "hello.txt"},
			wantErrSub: "invalid line group size",
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

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: nl [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-b STYLE") {
		t.Errorf("body style flag missing from help: %q", stderr)
	}
	for _, want := range []string{"-d CC", "-f STYLE", "-h STYLE", "-l NUMBER", "-p"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("flag %q missing from help: %q", want, stderr)
		}
	}
}
