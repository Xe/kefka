package cut

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
	write("tab.txt", []byte("a\tb\tc\nd\te\tf\n"))
	write("csv.txt", []byte("one,two,three\nfour,five,six\n"))
	write("mixed.txt", []byte("has,delim\nnodelim\nalso,delim\n"))
	write("chars.txt", []byte("abcdef\nghijkl\n"))
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

func TestCut(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "field tab default from stdin",
			args:       []string{"-f1"},
			stdin:      "a\tb\tc\n",
			wantStdout: "a\n",
		},
		{
			name:       "field tab default from file",
			args:       []string{"-f", "2", "tab.txt"},
			wantStdout: "b\ne\n",
		},
		{
			name:       "field with custom delimiter",
			args:       []string{"-d", ",", "-f", "2", "csv.txt"},
			wantStdout: "two\nfive\n",
		},
		{
			name:       "field comma list",
			args:       []string{"-d", ",", "-f", "1,3", "csv.txt"},
			wantStdout: "one,three\nfour,six\n",
		},
		{
			name:       "field range",
			args:       []string{"-d", ",", "-f", "1-2", "csv.txt"},
			wantStdout: "one,two\nfour,five\n",
		},
		{
			name:       "field open-ended start",
			args:       []string{"-d", ",", "-f", "2-", "csv.txt"},
			wantStdout: "two,three\nfive,six\n",
		},
		{
			name:       "field open-ended end",
			args:       []string{"-d", ",", "-f", "-2", "csv.txt"},
			wantStdout: "one,two\nfour,five\n",
		},
		{
			name:       "field out of range yields empty",
			args:       []string{"-d", ",", "-f", "5", "csv.txt"},
			wantStdout: "\n\n",
		},
		{
			name:       "char single",
			args:       []string{"-c", "1"},
			stdin:      "abcdef\n",
			wantStdout: "a\n",
		},
		{
			name:       "char list",
			args:       []string{"-c", "1,3,5"},
			stdin:      "abcdef\n",
			wantStdout: "ace\n",
		},
		{
			name:       "char range",
			args:       []string{"-c", "1-3"},
			stdin:      "abcdef\n",
			wantStdout: "abc\n",
		},
		{
			name:       "char open-ended end",
			args:       []string{"-c", "-3", "chars.txt"},
			wantStdout: "abc\nghi\n",
		},
		{
			name:       "char open-ended start",
			args:       []string{"-c", "4-", "chars.txt"},
			wantStdout: "def\njkl\n",
		},
		{
			name:       "char allows duplicate codepoints",
			args:       []string{"-c", "1,1,2"},
			stdin:      "abc\n",
			wantStdout: "aab\n",
		},
		{
			name:       "suppress lines without delimiter",
			args:       []string{"-d", ",", "-f", "1", "-s", "mixed.txt"},
			wantStdout: "has\nalso\n",
		},
		{
			name:       "without -s lines without delimiter pass through",
			args:       []string{"-d", ",", "-f", "1", "mixed.txt"},
			wantStdout: "has\nnodelim\nalso\n",
		},
		{
			name:       "long form only-delimited",
			args:       []string{"-d", ",", "-f", "1", "--only-delimited", "mixed.txt"},
			wantStdout: "has\nalso\n",
		},
		{
			name:       "stdin via dash",
			args:       []string{"-d", ",", "-f", "1", "-"},
			stdin:      "x,y\nz,w\n",
			wantStdout: "x\nz\n",
		},
		{
			name:       "concatenates multiple files",
			args:       []string{"-d", ",", "-f", "1", "csv.txt", "mixed.txt"},
			wantStdout: "one\nfour\nhas\nnodelim\nalso\n",
		},
		{
			name:       "empty stdin produces no output",
			args:       []string{"-f", "1"},
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "no trailing newline in input still emits one",
			args:       []string{"-d", ",", "-f", "1"},
			stdin:      "a,b",
			wantStdout: "a\n",
		},
		{
			name:       "duplicate field values are deduped",
			args:       []string{"-d", ",", "-f", "1-3"},
			stdin:      "a,a,b\n",
			wantStdout: "a,b\n",
		},
		{
			name:       "double dash terminator",
			args:       []string{"-d", ",", "-f", "1", "--", "csv.txt"},
			wantStdout: "one\nfour\n",
		},
		{
			name:       "missing -c and -f errors",
			args:       []string{"csv.txt"},
			wantErrSub: "you must specify",
			wantErr:    true,
		},
		{
			name:       "missing file errors",
			args:       []string{"-f", "1", "nope.txt"},
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
	if !strings.Contains(stderr, "Usage: cut [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-d DELIM") {
		t.Errorf("delimiter flag missing from help: %q", stderr)
	}
}
