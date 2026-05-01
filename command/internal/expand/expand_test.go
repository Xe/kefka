package expand

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
	write("tabs.txt", []byte("a\tb\tc\n"))
	write("leading.txt", []byte("\thello\tworld\n"))
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

func TestExpand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default 8-space tab from stdin",
			args:       nil,
			stdin:      "a\tb\n",
			wantStdout: "a       b\n",
		},
		{
			name:       "default 8-space tab at column 0",
			args:       nil,
			stdin:      "\tx\n",
			wantStdout: "        x\n",
		},
		{
			name:       "tab width 4 short flag",
			args:       []string{"-t", "4"},
			stdin:      "a\tb\n",
			wantStdout: "a   b\n",
		},
		{
			name:       "tab width 4 short flag combined",
			args:       []string{"-t4"},
			stdin:      "a\tb\n",
			wantStdout: "a   b\n",
		},
		{
			name:       "tab width 4 long flag",
			args:       []string{"--tabs=4"},
			stdin:      "a\tb\n",
			wantStdout: "a   b\n",
		},
		{
			name:       "explicit tab stops",
			args:       []string{"-t", "4,8,12"},
			stdin:      "\t\t\tend\n",
			wantStdout: "            end\n",
		},
		{
			name:       "explicit tab stops past last become single space",
			args:       []string{"-t", "4,8"},
			stdin:      "a\t\t\tx\n",
			wantStdout: "a" + strings.Repeat(" ", 8) + "x\n",
		},
		{
			name:       "multi-stop several tabs past last each one space",
			args:       []string{"-t", "4,8"},
			stdin:      "12345678\t\t\tx\n",
			wantStdout: "12345678   x\n",
		},
		{
			name:       "narrow rune counts as one column",
			args:       nil,
			stdin:      "λ\tx\n",
			wantStdout: "λ" + strings.Repeat(" ", 7) + "x\n",
		},
		{
			name:       "wide rune counts as two columns",
			args:       nil,
			stdin:      "中\tx\n",
			wantStdout: "中" + strings.Repeat(" ", 6) + "x\n",
		},
		{
			name:       "wide rune 日 counts as two columns",
			args:       nil,
			stdin:      "日\tx\n",
			wantStdout: "日" + strings.Repeat(" ", 6) + "x\n",
		},
		{
			name:       "mixed narrow and wide runes",
			args:       nil,
			stdin:      "a中\tb\n",
			wantStdout: "a中" + strings.Repeat(" ", 5) + "b\n",
		},
		{
			name:       "leading-only kefka extension preserves interior tabs after text",
			args:       []string{"-i"},
			stdin:      "leading\tword\ttrailing\n",
			wantStdout: "leading\tword\ttrailing\n",
		},
		{
			name:       "leading-only short flag preserves interior tabs",
			args:       []string{"-i"},
			stdin:      "\thi\tworld\n",
			wantStdout: "        hi\tworld\n",
		},
		{
			name:       "leading-only long flag",
			args:       []string{"--initial"},
			stdin:      "\thi\tworld\n",
			wantStdout: "        hi\tworld\n",
		},
		{
			name:       "leading-only with mixed leading whitespace",
			args:       []string{"-i", "-t", "4"},
			stdin:      " \tfoo\tbar\n",
			wantStdout: "    foo\tbar\n",
		},
		{
			name:       "leading-only with space-then-tab leading and interior tab",
			args:       []string{"-i"},
			stdin:      " \tword\ttrailing\n",
			wantStdout: "        word\ttrailing\n",
		},
		{
			name:       "expand from file",
			args:       []string{"-t", "4", "tabs.txt"},
			wantStdout: "a   b   c\n",
		},
		{
			name:       "concatenates multiple files",
			args:       []string{"-t", "4", "tabs.txt", "multi.txt"},
			wantStdout: "a   b   c\nfoo\nbar\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-t", "4", "-"},
			stdin:      "a\tb\n",
			wantStdout: "a   b\n",
		},
		{
			name:       "double dash terminator",
			args:       []string{"-t", "4", "--", "tabs.txt"},
			wantStdout: "a   b   c\n",
		},
		{
			name:       "empty stdin produces empty output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "no trailing newline preserved",
			args:       []string{"-t", "4"},
			stdin:      "a\tb",
			wantStdout: "a   b",
		},
		{
			name:       "non-tab characters pass through",
			args:       nil,
			stdin:      "no tabs here\n",
			wantStdout: "no tabs here\n",
		},
		{
			name:       "tab after text uses column position",
			args:       []string{"-t", "4"},
			stdin:      "ab\tc\n",
			wantStdout: "ab  c\n",
		},
		{
			name:       "invalid tab size errors",
			args:       []string{"-t", "abc"},
			stdin:      "a\tb\n",
			wantErrSub: "invalid tab size",
			wantErr:    true,
		},
		{
			name:       "zero tab size errors",
			args:       []string{"-t", "0"},
			stdin:      "a\tb\n",
			wantErrSub: "invalid tab size",
			wantErr:    true,
		},
		{
			name:       "non-ascending tab stops error",
			args:       []string{"-t", "4,3"},
			stdin:      "a\tb\n",
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
	if !strings.Contains(stderr, "Usage: expand [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-t N") {
		t.Errorf("tab-size flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-i") {
		t.Errorf("leading-only flag missing from help: %q", stderr)
	}
}
