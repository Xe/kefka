package uniq

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
	write("dups.txt", []byte("a\na\nb\nc\nc\nc\nd\n"))
	write("mixed.txt", []byte("Apple\napple\nAPPLE\nbanana\n"))
	write("part1.txt", []byte("x\nx\n"))
	write("part2.txt", []byte("x\ny\n"))
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

func TestUniq(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default collapses adjacent duplicates from stdin",
			args:       nil,
			stdin:      "a\na\nb\nc\nc\nc\nd\n",
			wantStdout: "a\nb\nc\nd\n",
		},
		{
			name:       "default collapses adjacent duplicates from file",
			args:       []string{"dups.txt"},
			wantStdout: "a\nb\nc\nd\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "a\na\nb\n",
			wantStdout: "a\nb\n",
		},
		{
			name:       "empty input produces no output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "non-adjacent duplicates are kept",
			args:       nil,
			stdin:      "a\nb\na\nb\n",
			wantStdout: "a\nb\na\nb\n",
		},
		{
			name:       "input without trailing newline",
			args:       nil,
			stdin:      "a\na\nb",
			wantStdout: "a\nb\n",
		},
		{
			name:       "count prefixes occurrences",
			args:       []string{"-c"},
			stdin:      "a\na\nb\nc\nc\nc\n",
			wantStdout: "   2 a\n   1 b\n   3 c\n",
		},
		{
			name:       "count via long flag",
			args:       []string{"--count"},
			stdin:      "a\na\n",
			wantStdout: "   2 a\n",
		},
		{
			name:       "repeated only prints duplicate runs",
			args:       []string{"-d"},
			stdin:      "a\na\nb\nc\nc\nc\nd\n",
			wantStdout: "a\nc\n",
		},
		{
			name:       "repeated long flag",
			args:       []string{"--repeated"},
			stdin:      "a\nb\nb\n",
			wantStdout: "b\n",
		},
		{
			name:       "unique only prints singletons",
			args:       []string{"-u"},
			stdin:      "a\na\nb\nc\nc\nc\nd\n",
			wantStdout: "b\nd\n",
		},
		{
			name:       "unique long flag",
			args:       []string{"--unique"},
			stdin:      "a\nb\nb\n",
			wantStdout: "a\n",
		},
		{
			name:       "ignore-case folds adjacent variants",
			args:       []string{"-i"},
			stdin:      "Apple\napple\nAPPLE\nbanana\n",
			wantStdout: "Apple\nbanana\n",
		},
		{
			name:       "ignore-case long flag from file",
			args:       []string{"--ignore-case", "mixed.txt"},
			wantStdout: "Apple\nbanana\n",
		},
		{
			name:       "count combined with ignore-case",
			args:       []string{"-c", "-i"},
			stdin:      "Apple\napple\nbanana\n",
			wantStdout: "   2 Apple\n   1 banana\n",
		},
		{
			name:       "repeated and count together",
			args:       []string{"-cd"},
			stdin:      "a\na\nb\nc\nc\nc\n",
			wantStdout: "   2 a\n   3 c\n",
		},
		{
			name:       "concatenates multiple file arguments",
			args:       []string{"part1.txt", "part2.txt"},
			wantStdout: "x\ny\n",
		},
		{
			name:       "missing file reports error",
			args:       []string{"nope.txt"},
			wantStdout: "",
			wantErrSub: "uniq: nope.txt: No such file or directory",
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
	if !strings.Contains(stderr, "Usage: uniq [OPTION]... [INPUT [OUTPUT]]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-c, --count") {
		t.Errorf("count flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-i, --ignore-case") {
		t.Errorf("ignore-case flag missing from help: %q", stderr)
	}
}
