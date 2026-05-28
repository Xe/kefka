package uniq

import (
	"bytes"
	"context"
	"io"
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
			name:       "skip-fields collapses lines with same suffix",
			args:       []string{"-f", "1"},
			stdin:      "x foo\ny foo\n",
			wantStdout: "x foo\n",
		},
		{
			name:       "skip-fields keeps lines with differing suffix",
			args:       []string{"-f", "1"},
			stdin:      "a x\na y\n",
			wantStdout: "a x\na y\n",
		},
		{
			name:       "skip-fields multiple fields",
			args:       []string{"-f", "2"},
			stdin:      "a a foo\nb b foo\n",
			wantStdout: "a a foo\n",
		},
		{
			name:       "skip-fields beyond available collapses",
			args:       []string{"-f", "1"},
			stdin:      "foo\nbar\n",
			wantStdout: "foo\n",
		},
		{
			name:       "skip-fields long flag",
			args:       []string{"--skip-fields=1"},
			stdin:      "x foo\ny foo\n",
			wantStdout: "x foo\n",
		},
		{
			name:       "skip-chars collapses with same suffix",
			args:       []string{"-s", "2"},
			stdin:      "AAfoo\nBBfoo\n",
			wantStdout: "AAfoo\n",
		},
		{
			name:       "skip-chars keeps differing suffix",
			args:       []string{"-s", "2"},
			stdin:      "aabar\naabaz\n",
			wantStdout: "aabar\naabaz\n",
		},
		{
			name:       "skip-chars long flag",
			args:       []string{"--skip-chars=2"},
			stdin:      "AAfoo\nBBfoo\n",
			wantStdout: "AAfoo\n",
		},
		{
			name:       "skip-fields combined with skip-chars",
			args:       []string{"-f", "1", "-s", "1"},
			stdin:      "aa Xfoo\nbb Xfoo\n",
			wantStdout: "aa Xfoo\n",
		},
		{
			name:       "check-chars limits comparison length",
			args:       []string{"-w", "3"},
			stdin:      "fooaaa\nfoobbb\nbarccc\n",
			wantStdout: "fooaaa\nbarccc\n",
		},
		{
			name:       "check-chars zero collapses everything",
			args:       []string{"-w", "0"},
			stdin:      "abc\ndef\nghi\n",
			wantStdout: "abc\n",
		},
		{
			name:       "check-chars long flag",
			args:       []string{"--check-chars=2"},
			stdin:      "abxxx\nabyyy\nczzzz\n",
			wantStdout: "abxxx\nczzzz\n",
		},
		{
			name:       "check-chars exceeding line length keeps distinct",
			args:       []string{"-w", "10"},
			stdin:      "ab\nac\n",
			wantStdout: "ab\nac\n",
		},
		{
			name:       "skip-fields skip-chars and check-chars combined",
			args:       []string{"-f", "1", "-s", "1", "-w", "3"},
			stdin:      "a Xfoozzz\nb Xfooqqq\nc Xbarppp\n",
			wantStdout: "a Xfoozzz\nc Xbarppp\n",
		},
		{
			name:       "check-chars with ignore-case",
			args:       []string{"-w", "3", "-i"},
			stdin:      "FOOaaa\nfooBBB\n",
			wantStdout: "FOOaaa\n",
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

func TestOutputFile(t *testing.T) {
	t.Run("two positionals writes file", func(t *testing.T) {
		fs := newFS(t)
		stdout, stderr, err := run(t, []string{"dups.txt", "out.txt"}, "", fs)
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if stdout != "" {
			t.Errorf("expected empty stdout, got %q", stdout)
		}
		f, err := fs.Open("out.txt")
		if err != nil {
			t.Fatalf("output file not created: %v", err)
		}
		defer f.Close()
		got, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		want := "a\nb\nc\nd\n"
		if string(got) != want {
			t.Errorf("output mismatch\n got: %q\nwant: %q", string(got), want)
		}
	})

	t.Run("stdin to file via dash", func(t *testing.T) {
		fs := newFS(t)
		stdout, stderr, err := run(t, []string{"-", "out.txt"}, "x\nx\ny\n", fs)
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if stdout != "" {
			t.Errorf("expected empty stdout, got %q", stdout)
		}
		f, err := fs.Open("out.txt")
		if err != nil {
			t.Fatalf("output file not created: %v", err)
		}
		defer f.Close()
		got, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		want := "x\ny\n"
		if string(got) != want {
			t.Errorf("output mismatch\n got: %q\nwant: %q", string(got), want)
		}
	})

	t.Run("output dash writes stdout", func(t *testing.T) {
		fs := newFS(t)
		stdout, stderr, err := run(t, []string{"dups.txt", "-"}, "", fs)
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		want := "a\nb\nc\nd\n"
		if stdout != want {
			t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, want)
		}
	})
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
	if !strings.Contains(stderr, "-f, --skip-fields") {
		t.Errorf("skip-fields flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-s, --skip-chars") {
		t.Errorf("skip-chars flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-w, --check-chars") {
		t.Errorf("check-chars flag missing from help: %q", stderr)
	}
}
