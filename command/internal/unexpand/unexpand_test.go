package unexpand

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
	write("spaces.txt", []byte("        hello\n"))
	write("middle.txt", []byte("a        b        c\n"))
	write("multi.txt", []byte("foo\nbar\n"))
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

func TestUnexpand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default 8-space leading run becomes a tab",
			args:       nil,
			stdin:      "        hello\n",
			wantStdout: "\thello\n",
		},
		{
			name:       "interior spaces left alone by default",
			args:       nil,
			stdin:      "a        b\n",
			wantStdout: "a        b\n",
		},
		{
			name:       "all-blanks short flag converts interior runs",
			args:       []string{"-a"},
			stdin:      "a       b\n",
			wantStdout: "a\tb\n",
		},
		{
			name:       "all-blanks long flag",
			args:       []string{"--all"},
			stdin:      "a       b\n",
			wantStdout: "a\tb\n",
		},
		{
			name:       "tab width 4 short flag",
			args:       []string{"-t", "4"},
			stdin:      "    hello\n",
			wantStdout: "\thello\n",
		},
		{
			name:       "tab width 4 long flag",
			args:       []string{"--tabs=4"},
			stdin:      "    hello\n",
			wantStdout: "\thello\n",
		},
		{
			name:       "explicit tab stops",
			args:       []string{"-t", "4,8"},
			stdin:      "        end\n",
			wantStdout: "\t\tend\n",
		},
		{
			name:       "partial space run not at tab stop preserved",
			args:       []string{"-t", "4"},
			stdin:      "  hi\n",
			wantStdout: "  hi\n",
		},
		{
			name:       "leading run with extra spaces tabs the multiple of width and keeps remainder",
			args:       []string{"-t", "4"},
			stdin:      "      hi\n",
			wantStdout: "\t  hi\n",
		},
		{
			name:       "tab in input passes through and advances column",
			args:       []string{"-a", "-t", "4"},
			stdin:      "a\t   b\n",
			wantStdout: "a\t   b\n",
		},
		{
			name:       "round trip with expand-style 8-space input",
			args:       nil,
			stdin:      "        hi\n",
			wantStdout: "\thi\n",
		},
		{
			name:       "unexpand from file",
			args:       []string{"spaces.txt"},
			wantStdout: "\thello\n",
		},
		{
			name:       "concatenates multiple files",
			args:       []string{"spaces.txt", "multi.txt"},
			wantStdout: "\thello\nfoo\nbar\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "        hi\n",
			wantStdout: "\thi\n",
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "spaces.txt"},
			wantStdout: "\thello\n",
		},
		{
			name:       "empty stdin produces empty output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "no trailing newline preserved",
			args:       nil,
			stdin:      "        hi",
			wantStdout: "\thi",
		},
		{
			name:       "no spaces stays unchanged",
			args:       nil,
			stdin:      "no spaces here\n",
			wantStdout: "no spaces here\n",
		},
		{
			name:       "invalid tab size errors",
			args:       []string{"-t", "abc"},
			stdin:      "    hi\n",
			wantErrSub: "invalid tab size",
			wantErr:    true,
		},
		{
			name:       "zero tab size errors",
			args:       []string{"-t", "0"},
			stdin:      "    hi\n",
			wantErrSub: "invalid tab size",
			wantErr:    true,
		},
		{
			name:       "non-ascending tab stops error",
			args:       []string{"-t", "4,3"},
			stdin:      "    hi\n",
			wantErrSub: "invalid tab size",
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
			name:       "-t implies -a (interior runs converted)",
			args:       []string{"-t", "4"},
			stdin:      "    a    b    c\n",
			wantStdout: "\ta\t b\t  c\n",
		},
		{
			name:       "-t implies -a long flag",
			args:       []string{"--tabs=4"},
			stdin:      "a   b\n",
			wantStdout: "a\tb\n",
		},
		{
			name:       "multi tab stops do not convert past last stop",
			args:       []string{"-a", "-t", "4,8"},
			stdin:      "            x\n",
			wantStdout: "\t\t    x\n",
		},
		{
			name:       "multi tab stops boundary at last stop",
			args:       []string{"-a", "-t", "4,8"},
			stdin:      "        x\n",
			wantStdout: "\t\tx\n",
		},
		{
			name:       "multi tab stops past last stop after non-blank",
			args:       []string{"-a", "-t", "4,8"},
			stdin:      "a       b   c\n",
			wantStdout: "a\t\tb   c\n",
		},
		{
			name:       "leading backspaces do not crash and disable leading conversion",
			args:       nil,
			stdin:      "\b\b\b   a\n",
			wantStdout: "\b\b\b   a\n",
		},
		{
			name:       "backspace decrements column for tab calculation",
			args:       []string{"-a", "-t", "4"},
			stdin:      "   \b   x\n",
			wantStdout: "   \b\t x\n",
		},
		{
			name:       "many backspaces do not underflow column",
			args:       []string{"-a"},
			stdin:      "a\b\b\b\b\b\b\b\b\b\b        b\n",
			wantStdout: "a\b\b\b\b\b\b\b\b\b\b\tb\n",
		},
		{
			// Audit example: -t implies -a even after non-blanks.
			name:       "audit example: -t 4 converts every quad on whole line",
			args:       []string{"-t", "4"},
			stdin:      "    a    b    c\n",
			wantStdout: "\ta\t b\t  c\n",
		},
		{
			// Audit example: beyond-last-stop trailing spaces left untouched.
			name:       "audit example: beyond_last_stop spaces past 4",
			args:       []string{"-a", "-t", "4"},
			stdin:      "beyond_last_stop spaces past 4\n",
			wantStdout: "beyond_last_stop spaces past 4\n",
		},
		{
			// With single tab spec, conversions continue to repeat past the
			// "last" stop (the tab spec defines a repeating interval).
			name:       "single tab spec keeps converting past nominal last stop",
			args:       []string{"-a", "-t", "4"},
			stdin:      "a   b   c\n",
			wantStdout: "a\tb\tc\n",
		},
		{
			// Backspace at column 1 must not decrement below 1 (GNU rule).
			name:       "backspace at start does not underflow",
			args:       []string{"-a", "-t", "4"},
			stdin:      "\b    x\n",
			wantStdout: "\b\tx\n",
		},
		{
			// Multibyte runes: GNU unexpand is byte-based; we count one
			// column per rune, which is consistent for ASCII input. A leading
			// quad of spaces followed by a multibyte char still tabifies.
			name:       "multibyte rune after leading tab-quad",
			args:       []string{"-t", "4"},
			stdin:      "    é\n",
			wantStdout: "\té\n",
		},
		{
			// Multibyte rune occupies one column for tab calculations: a
			// rune followed by 3 spaces fills column 4, so converts to tab.
			name:       "multibyte rune counts as one column",
			args:       []string{"-a", "-t", "4"},
			stdin:      "é   x\n",
			wantStdout: "é\tx\n",
		},
		{
			// Per GNU/POSIX: non-leading single space never converts
			// to tab even when it lands on a tab stop.
			name:       "interior single space at tab stop is not converted",
			args:       []string{"-a", "-t", "4"},
			stdin:      "aaa b\n",
			wantStdout: "aaa b\n",
		},
		{
			// Two interior spaces ending on a tab stop do convert.
			name:       "interior two-space run ending on tab stop converts",
			args:       []string{"-a", "-t", "4"},
			stdin:      "aa  b\n",
			wantStdout: "aa\tb\n",
		},
		{
			// East Asian wide chars occupy two columns: a wide rune
			// fills cols 1-2, then 2 spaces at cols 3-4 hit tab stop 4.
			name:       "wide character counts as two columns",
			args:       []string{"-a", "-t", "4"},
			stdin:      "中  x\n",
			wantStdout: "中\tx\n",
		},
		{
			// Two wide chars fill cols 1-4, hitting tab stop 4 already;
			// leading 4 spaces convert to a tab independently.
			name:       "leading tab then wide chars",
			args:       []string{"-t", "4"},
			stdin:      "    中文\n",
			wantStdout: "\t中文\n",
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
	if !strings.Contains(stderr, "Usage: unexpand [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-t N") {
		t.Errorf("tab-size flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-a") {
		t.Errorf("all-blanks flag missing from help: %q", stderr)
	}
}
