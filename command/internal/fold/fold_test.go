package fold

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
		{
			name:       "carriage return resets column",
			args:       []string{"-w", "5"},
			stdin:      "a\rb",
			wantStdout: "a\rb",
		},
		{
			name:       "carriage return after long run resets column",
			args:       []string{"-w", "3"},
			stdin:      "abcd\rxy",
			wantStdout: "abc\nd\rxy",
		},
		{
			name:       "tab at column zero exceeding width folds immediately",
			args:       []string{"-w", "4"},
			stdin:      "\tab",
			wantStdout: "\t\nab",
		},
		{
			name:       "narrow runes count as one column each",
			args:       []string{"-w", "3"},
			stdin:      "λλλ",
			wantStdout: "λλλ",
		},
		{
			name:       "east asian wide runes count as two columns",
			args:       []string{"-w", "3"},
			stdin:      "中中中",
			wantStdout: "中\n中\n中",
		},
		{
			name:       "east asian wide rune fits exactly",
			args:       []string{"-w", "4"},
			stdin:      "中中",
			wantStdout: "中中",
		},
		{
			name:       "backspace decrements column",
			args:       []string{"-w", "2"},
			stdin:      "a\bbc",
			wantStdout: "a\bbc",
		},
		{
			name:       "byte mode treats wide rune bytes individually",
			args:       []string{"-b", "-w", "3"},
			stdin:      "中中",
			wantStdout: "中\n中",
		},
		{
			// After a fold, the tab's advance is recomputed against
			// column 0 of the new segment. With width 5, a tab from
			// column 0 still wants column 8, so it fills its own
			// segment before the next char wraps again.
			name:       "tab after fold recomputes width from new segment",
			args:       []string{"-w", "5"},
			stdin:      "aaa\tb",
			wantStdout: "aaa\n\t\nb",
		},
		{
			// With width 9 the tab should land on column 8, then fit
			// b at column 9 without folding. This regression-checks
			// that the tab's advance is correct when no fold occurs.
			name:       "tab fits within width without fold",
			args:       []string{"-w", "9"},
			stdin:      "aaa\tb",
			wantStdout: "aaa\tb",
		},
		{
			name:       "carriage return at end of line keeps column zero",
			args:       []string{"-w", "3"},
			stdin:      "abc\rxyz",
			wantStdout: "abc\rxyz",
		},
		{
			// Backspace decrements the column, so col 0 + wide(2) = 2
			// fits within width 2 without folding.
			name:       "backspace before wide char that fits stays on line",
			args:       []string{"-w", "2"},
			stdin:      "a\b中",
			wantStdout: "a\b中",
		},
		{
			// Wide char after backspace exceeds width and triggers a
			// fold, exercising the "unless the following character has
			// a width greater than 1" rule.
			name:       "wide char after backspace folds when too wide",
			args:       []string{"-w", "1"},
			stdin:      "a\b中",
			wantStdout: "a\b\n中",
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
