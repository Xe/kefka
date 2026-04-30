package column

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
	write("words.txt", []byte("alpha beta gamma\n"))
	write("table.txt", []byte("name age city\nalice 30 nyc\nbob 25 sf\n"))
	write("csv.txt", []byte("a,b,c\nd,ee,f\n"))
	write("blank.txt", []byte("   \n\n\t\n"))
	write("dupes.txt", []byte("a,,b\n"))
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

func TestColumn(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "fill mode stdin three short items",
			args:       nil,
			stdin:      "alpha beta gamma\n",
			wantStdout: "alpha  beta   gamma\n",
		},
		{
			name:       "fill mode stdin via dash",
			args:       []string{"-"},
			stdin:      "alpha beta gamma\n",
			wantStdout: "alpha  beta   gamma\n",
		},
		{
			name:       "fill mode reads single file",
			args:       []string{"words.txt"},
			wantStdout: "alpha  beta   gamma\n",
		},
		{
			name:       "fill mode narrow width forces multiple rows",
			args:       []string{"-c", "8"},
			stdin:      "a b c d e\n",
			wantStdout: "a  c  e\nb  d\n",
		},
		{
			name:       "fill mode custom output separator",
			args:       []string{"-o", ", "},
			stdin:      "a b c\n",
			wantStdout: "a, b, c\n",
		},
		{
			name:       "table mode whitespace input aligns columns",
			args:       []string{"-t"},
			stdin:      "name age city\nalice 30 nyc\nbob 25 sf\n",
			wantStdout: "name   age  city\nalice  30   nyc\nbob    25   sf\n",
		},
		{
			name:       "table mode reads from file",
			args:       []string{"-t", "table.txt"},
			wantStdout: "name   age  city\nalice  30   nyc\nbob    25   sf\n",
		},
		{
			name:       "table mode with comma separator",
			args:       []string{"-t", "-s", ",", "csv.txt"},
			wantStdout: "a  b   c\nd  ee  f\n",
		},
		{
			name:       "merge consecutive separators by default",
			args:       []string{"-t", "-s", ",", "dupes.txt"},
			wantStdout: "a  b\n",
		},
		{
			name:       "no merge preserves empty fields",
			args:       []string{"-t", "-s", ",", "-n", "dupes.txt"},
			wantStdout: "a    b\n",
		},
		{
			name:       "long table flag",
			args:       []string{"--table"},
			stdin:      "a b\nccc d\n",
			wantStdout: "a    b\nccc  d\n",
		},
		{
			name:       "concatenates multiple files",
			args:       []string{"words.txt", "words.txt"},
			wantStdout: "alpha  beta   gamma  alpha  beta   gamma\n",
		},
		{
			name:       "empty stdin produces empty output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "whitespace only input produces empty output",
			args:       []string{"blank.txt"},
			wantStdout: "",
		},
		{
			name:       "blank lines between rows are skipped in table mode",
			args:       []string{"-t"},
			stdin:      "a b\n\nccc d\n",
			wantStdout: "a    b\nccc  d\n",
		},
		{
			name:       "missing file reports error",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
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
	if !strings.Contains(stderr, "Usage: column [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-t, --table") {
		t.Errorf("table flag missing from help: %q", stderr)
	}
}

func TestNilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestSplitFields(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		sep     string
		noMerge bool
		want    []string
	}{
		{"whitespace default", "a b  c", "", false, []string{"a", "b", "c"}},
		{"whitespace no merge", "a b  c", "", true, []string{"a", "b", "", "c"}},
		{"comma default", "a,,b", ",", false, []string{"a", "b"}},
		{"comma no merge", "a,,b", ",", true, []string{"a", "", "b"}},
		{"tab whitespace", "a\tb", "", false, []string{"a", "b"}},
		{"empty line whitespace", "", "", false, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitFields(tt.line, tt.sep, tt.noMerge)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (got=%q)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
