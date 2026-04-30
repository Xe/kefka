package nl

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
	write("hello.txt", []byte("hello\nworld\n"))
	write("blanks.txt", []byte("alpha\n\nbravo\n\ncharlie\n"))
	write("nofinalnl.txt", []byte("foo\nbar"))
	write("empty.txt", []byte(""))
	write("part1.txt", []byte("a\nb\n"))
	write("part2.txt", []byte("c\nd\n"))
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
			wantStdout: "     1\talpha\n      \t\n     2\tbravo\n      \t\n     3\tcharlie\n",
		},
		{
			name:       "style a numbers all lines",
			args:       []string{"-b", "a", "blanks.txt"},
			wantStdout: "     1\talpha\n     2\t\n     3\tbravo\n     4\t\n     5\tcharlie\n",
		},
		{
			name:       "style n numbers no lines",
			args:       []string{"-b", "n", "hello.txt"},
			wantStdout: "      \thello\n      \tworld\n",
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
}
