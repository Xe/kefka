package grep

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/interp"
)

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	for name, content := range map[string]string{
		"a.txt": "apple\nBanana\napple pie\ncherry\n",
		"b.txt": "banana\ncherry\n", "empty": "", "patterns": "apple\ncherry\n",
		"blank-pattern": "\n", "bad-pattern": "[\n", "tree/a.txt": "apple\n",
		"tree/b.log": "apple\n", "tree/sub/c.txt": "apple\n", "tree/.hidden": "apple\n",
		"tree/.git/data": "apple\n", "tree/vendor/d.txt": "apple\n", "-n": "apple\n",
	} {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, content); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return fs
}

func TestGrep(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantOut  string
		wantErr  string
		wantCode int
	}{
		{"stdin", []string{"apple"}, "apple\npear\napple pie", "apple\napple pie\n", "", 0},
		{"file", []string{"apple", "a.txt"}, "ignored", "apple\napple pie\n", "", 0},
		{"dash", []string{"apple", "-"}, "apple\npear\n", "apple\n", "", 0},
		{"empty input", []string{""}, "", "", "", 1},
		{"no match", []string{"pear", "a.txt"}, "", "", "", 1},
		{"empty pattern", []string{"", "b.txt"}, "", "banana\ncherry\n", "", 0},
		{"blank line", []string{"^$"}, "one\n\ntwo\n", "\n", "", 0},
		{"ignore case", []string{"-i", "banana", "a.txt"}, "", "Banana\n", "", 0},
		{"invert", []string{"-v", "apple", "a.txt"}, "", "Banana\ncherry\n", "", 0},
		{"word", []string{"-w", "cat"}, "cat\ncategory\na cat!\ncat_\n", "cat\na cat!\n", "", 0},
		{"line", []string{"-x", "apple", "a.txt"}, "", "apple\n", "", 0},
		{"count", []string{"-c", "apple", "a.txt"}, "", "2\n", "", 0},
		{"count no matches", []string{"-c", "pear", "a.txt"}, "", "0\n", "", 1},
		{"count occurrences upstream", []string{"-co", "a"}, "aaa\na\n", "4\n", "", 0},
		{"count max lines", []string{"-co", "-m1", "a"}, "aaa\na\n", "3\n", "", 0},
		{"filenames", []string{"-l", "apple", "a.txt", "b.txt"}, "", "a.txt\n", "", 0},
		{"without matches", []string{"-L", "apple", "a.txt", "b.txt"}, "", "b.txt\n", "", 0},
		{"without matches status one", []string{"-L", "pear", "a.txt"}, "", "a.txt\n", "", 1},
		{"without matches status zero", []string{"-L", "apple", "a.txt"}, "", "", "", 0},
		{"max count", []string{"-m", "1", "apple", "a.txt"}, "", "apple\n", "", 0},
		{"max zero upstream", []string{"-m0", "apple", "a.txt"}, "", "apple\napple pie\n", "", 0},
		{"line numbers", []string{"-n", "apple", "a.txt"}, "", "1:apple\n3:apple pie\n", "", 0},
		{"multiple files", []string{"cherry", "a.txt", "b.txt"}, "", "a.txt:cherry\nb.txt:cherry\n", "", 0},
		{"no filename", []string{"-h", "cherry", "a.txt", "b.txt"}, "", "cherry\ncherry\n", "", 0},
		{"only matching", []string{"-o", "a."}, "apple banana\n", "ap\nan\nan\n", "", 0},
		{"only inverted", []string{"-ov", "apple", "a.txt"}, "", "", "", 0},
		{"zero width upstream", []string{"-o", "^"}, "abc\n", "\n", "", 0},
		{"quiet", []string{"-q", "apple", "a.txt"}, "", "", "", 0},
		{"quiet no matches", []string{"-q", "pear", "a.txt"}, "", "", "", 1},
		{"quiet suppress errors", []string{"-q", "x", "missing"}, "", "", "", 2},
		{"quiet match overrides error", []string{"-q", "apple", "missing", "a.txt"}, "", "", "", 0},
		{"after context", []string{"-nA1", "apple", "a.txt"}, "", "1:apple\n2-Banana\n3:apple pie\n4-cherry\n", "", 0},
		{"before context", []string{"-nB1", "cherry", "a.txt"}, "", "3-apple pie\n4:cherry\n", "", 0},
		{"context groups", []string{"-nC1", "hit"}, "a\nhit\nb\nc\nd\nhit\ne\n", "1-a\n2:hit\n3-b\n--\n5-d\n6:hit\n7-e\n", "", 0},
		{"context overlap", []string{"-nC1", "hit"}, "hit\nhit\nx\n", "1:hit\n2:hit\n3-x\n", "", 0},
		{"max trailing context", []string{"-nm1", "-A2", "hit"}, "hit\nhit\nx\nhit\n", "1:hit\n2-hit\n3-x\n", "", 0},
		{"context option order", []string{"-C2", "-B0", "hit"}, "before\nhit\nafter\n", "hit\nafter\n", "", 0},
		{"only matching context", []string{"-onA1", "apple", "a.txt"}, "", "1:apple\n2-Banana\n3:apple\n4-cherry\n", "", 0},
		{"explicit patterns", []string{"-e", "apple", "-e", "cherry", "a.txt"}, "", "apple\napple pie\ncherry\n", "", 0},
		{"newline pattern", []string{"apple\ncherry", "a.txt"}, "", "apple\napple pie\ncherry\n", "", 0},
		{"trailing newline pattern", []string{"apple\n", "a.txt"}, "", "apple\nBanana\napple pie\ncherry\n", "", 0},
		{"patterns file", []string{"-f", "patterns", "a.txt"}, "", "apple\napple pie\ncherry\n", "", 0},
		{"attached pattern file", []string{"-vxfpatterns", "a.txt"}, "", "Banana\napple pie\n", "", 0},
		{"pattern file and explicit", []string{"-fpatterns", "-e", "Banana", "a.txt"}, "", "apple\nBanana\napple pie\ncherry\n", "", 0},
		{"empty pattern file", []string{"-fempty", "-c", "missing"}, "", "", "", 1},
		{"empty pattern file inverted", []string{"-vfempty", "b.txt"}, "", "banana\ncherry\n", "", 0},
		{"empty pattern file list", []string{"-Lfempty", "b.txt"}, "", "b.txt\n", "", 1},
		{"blank pattern file", []string{"-fblank-pattern", "b.txt"}, "", "banana\ncherry\n", "", 0},
		{"stdin pattern file", []string{"-f", "-", "a.txt"}, "apple\n", "apple\napple pie\n", "", 0},
		{"pattern drains stdin", []string{"-f", "-"}, "apple\n", "", "", 1},
		{"pattern drains dash operand", []string{"-f", "-", "-"}, "apple\n", "", "", 1},
		{"repeated stdin", []string{"-c", "apple", "-", "-"}, "apple\n", "(standard input):1\n(standard input):0\n", "", 0},
		{"mixed stdin files", []string{"cherry", "-", "b.txt"}, "cherry\n", "(standard input):cherry\nb.txt:cherry\n", "", 0},
		{"stdin list", []string{"-l", "apple", "-"}, "apple\n", "(standard input)\n", "", 0},
		{"implicit stdin list upstream", []string{"-l", "apple"}, "apple\n", "apple\n", "", 0},
		{"options after pattern", []string{"apple", "-n", "a.txt"}, "", "1:apple\n3:apple pie\n", "", 0},
		{"option terminator", []string{"--", "apple", "-n"}, "", "apple\n", "", 0},
		{"missing pattern", nil, "", "", "grep: missing pattern\n", 2},
		{"missing file", []string{"x", "missing"}, "", "", "grep: missing: No such file or directory\n", 2},
		{"directory upstream status", []string{"x", "tree"}, "", "", "grep: tree: Is a directory\n", 1},
		{"missing pattern file", []string{"-fmissing", "a.txt"}, "", "", "grep: missing: No such file or directory\n", 2},
		{"directory pattern file", []string{"-ftree", "a.txt"}, "", "", "grep: tree: Is a directory\n", 2},
		{"empty filename patterns", []string{"--file=", "a.txt"}, "", "", "grep: : No such file or directory\n", 2},
		{"bad regex", []string{"[", "a.txt"}, "", "", "grep: invalid regular expression: [\n", 2},
		{"bad pattern before another", []string{"-e", "a\\", "-e", "apple", "a.txt"}, "", "", "grep: invalid regular expression: a\\\n", 2},
		{"extended", []string{"-E", "apple|cherry", "a.txt"}, "", "apple\napple pie\ncherry\n", "", 0},
		{"basic literal operators", []string{"a+b"}, "ab\na+b\naaab\n", "a+b\n", "", 0},
		{"basic alternation", []string{`apple\|cherry`, "a.txt"}, "", "apple\napple pie\ncherry\n", "", 0},
		{"basic grouping backref", []string{`\(ab\)\1`}, "abab\nab\n", "abab\n", "", 0},
		{"basic repetition", []string{`a\{2,3\}`}, "a\naa\naaa\n", "aa\naaa\n", "", 0},
		{"basic literal anchors", []string{`a^b$c`}, "a^b$c\nabc\n", "a^b$c\n", "", 0},
		{"basic leading star", []string{`*x`}, "*x\nx\n", "*x\n", "", 0},
		{"posix class", []string{"[[:digit:]]"}, "a\n1\n", "1\n", "", 0},
		{"fixed", []string{"-F", "a.b"}, "a.b\naxb\n", "a.b\n", "", 0},
		{"fixed multiple", []string{"-Fx", "-e", "a.b", "-e", "c+d"}, "a.b\nc+d\naxb\n", "a.b\nc+d\n", "", 0},
		{"fixed wins", []string{"-PEF", "a.b"}, "axb\na.b\n", "a.b\n", "", 0},
		{"whole line alternatives", []string{"-Ex", "a|b"}, "a\nb\nax\nxb\n", "a\nb\n", "", 0},
		{"perl lookahead", []string{"-Po", `foo(?=bar)`}, "foobar foo\n", "foo\n", "", 0},
		{"perl lookbehind", []string{"-Po", `(?<=foo)bar`}, "foobar bar\n", "bar\n", "", 0},
		{"perl reset", []string{"-Po", `foo\Kbar`}, "foobar\n", "bar\n", "", 0},
		{"perl reset capture", []string{"-Po", `(foo)\K(bar)`}, "foobar\n", "bar\n", "", 0},
		{"perl reset at end", []string{"-Po", `foo\K`}, "foo\n", "\n", "", 0},
		{"perl quote", []string{"-P", `\Qa.b+\E`}, "a.b+\naxb\n", "a.b+\n", "", 0},
		{"perl literal hyphen in class", []string{"-Po", `[a\x{2d}z]`}, "b\n-\n", "-\n", "", 0},
		{"perl reset in lookahead", []string{"-Po", `foo(?=bar\K)`}, "foobarbaz\n", "", "grep: invalid regular expression: foo(?=bar\\K)\n", 2},
		{"perl reset in lookbehind", []string{"-Po", `(?<=foo\Kbar)baz`}, "foobarbaz\n", "", "grep: invalid regular expression: (?<=foo\\Kbar)baz\n", 2},
		{"invalid utf8 bytes", []string{"-o", "."}, "\xff\xff\n", "\ufffd\n\ufffd\n", "", 0},
		{"truncated utf8 sequence", []string{"-o", "."}, "\xe2\x82\n", "\ufffd\n", "", 0},
		{"perl unicode", []string{"-Po", `\x{1F600}`}, "x\U0001f600y\n", "\U0001f600\n", "", 0},
		{"perl modifier", []string{"-P", `(?i:hello)World`}, "HELLOWorld\nHELLOWORLD\n", "HELLOWorld\n", "", 0},
		{"perl named group", []string{"-Po", `(?P<word>\w+)`}, "hello world\n", "hello\nworld\n", "", 0},
		{"perl multi rejected", []string{"-P", "-e", "a", "-e", "b"}, "", "", "grep: the -P option only supports a single pattern\n", 2},
		{"perl duplicate accepted", []string{"-P", "-e", "a", "-e", "a"}, "a\n", "a\n", "", 0},
		{"utf8 only match", []string{"-o", "."}, "\u00e9\u4e2d\U0001f600\n", "\u00e9\n\u4e2d\n\U0001f600\n", "", 0},
		{"binary text", []string{"b"}, "a\x00b\n", "a\x00b\n", "", 0},
		{"recursive filters", []string{"-r", "--include=*.txt", "--exclude-dir=vendor", "apple", "tree"}, "", "tree/a.txt:apple\ntree/sub/c.txt:apple\n", "", 0},
		{"recursive upper", []string{"-R", "--exclude=*.log", "--exclude-dir=sub", "--exclude-dir=vendor", "apple", "tree"}, "", "tree/a.txt:apple\n", "", 0},
		{"glob", []string{"cherry", "*.txt"}, "", "a.txt:cherry\nb.txt:cherry\n", "", 0},
		{"recursive glob", []string{"--exclude=d.txt", "apple", "tree/**/*.txt"}, "", "tree/a.txt:apple\ntree/sub/c.txt:apple\n", "", 0},
		{"include operands", []string{"--include=a.*", "cherry", "a.txt", "b.txt"}, "", "a.txt:cherry\n", "", 0},
		{"exclude operands", []string{"--exclude=a.*", "cherry", "a.txt", "b.txt"}, "", "b.txt:cherry\n", "", 0},
		{"stdin bypasses filters", []string{"-r", "--exclude=*", "--include=*.txt", "apple", "-"}, "apple\n", "apple\n", "", 0},
		{"recursive missing upstream", []string{"-r", "x", "missing"}, "", "", "", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			ec := &command.ExecContext{Stdin: strings.NewReader(tt.stdin), Stdout: &stdout, Stderr: &stderr, FS: newFS(t), Dir: "."}
			err := Impl{}.Exec(context.Background(), ec, tt.args)
			code := 0
			if err != nil {
				var status interp.ExitStatus
				if !errors.As(err, &status) {
					t.Fatalf("unexpected error: %v", err)
				}
				code = int(status)
			}
			if stdout.String() != tt.wantOut {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantOut)
			}
			if stderr.String() != tt.wantErr {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantErr)
			}
			if code != tt.wantCode {
				t.Errorf("exit = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		code    int
	}{
		{"unknown short", []string{"-z"}, "grep: invalid option -- 'z'\n", 1},
		{"unknown long", []string{"--bad"}, "grep: unrecognized option '--bad'\n", 1},
		{"missing short file", []string{"-f"}, "grep: option requires an argument -- 'f'\n", 2},
		{"missing long file", []string{"--file"}, "grep: option '--file' requires an argument\n", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdout: &out, Stderr: &stderr}, tt.args)
			if !errors.Is(err, interp.ExitStatus(tt.code)) || stderr.String() != tt.wantErr || out.Len() != 0 {
				t.Fatalf("got (%q, %q, %v), want (%q, %d)", out.String(), stderr.String(), err, tt.wantErr, tt.code)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	var out, stderr bytes.Buffer
	err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdout: &out, Stderr: &stderr}, []string{"--help"})
	if err != nil || out.Len() != 0 || !strings.Contains(stderr.String(), "Usage: grep [OPTION]... PATTERN [FILE]...") {
		t.Fatalf("help = (%q, %q, %v)", out.String(), stderr.String(), err)
	}
}

func TestTraversalLimit(t *testing.T) {
	// Repeated operands can fill the traversal result list before -q gets to search.
	names := make([]string, 100001)
	for i := range names {
		names[i] = "a.txt"
	}
	_, err := collectFiles(context.Background(), &command.ExecContext{FS: newFS(t)}, names, options{})
	if err == nil || !strings.Contains(err.Error(), "limit exceeded") {
		t.Fatalf("expected traversal limit, got %v", err)
	}
}

func TestLongOptions(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
		code int
	}{
		{"ignore case", []string{"--ignore-case", "banana", "a.txt"}, "Banana\n", 0},
		{"invert", []string{"--invert-match", "apple", "a.txt"}, "Banana\ncherry\n", 0},
		{"word", []string{"--word-regexp", "app", "a.txt"}, "", 1},
		{"line", []string{"--line-regexp", "apple", "a.txt"}, "apple\n", 0},
		{"extended", []string{"--extended-regexp", "apple|cherry", "a.txt"}, "apple\napple pie\ncherry\n", 0},
		{"perl", []string{"--perl-regexp", `apple(?= pie)`, "a.txt"}, "apple pie\n", 0},
		{"fixed", []string{"--fixed-strings", ".", "a.txt"}, "", 1},
		{"count", []string{"--count", "apple", "a.txt"}, "2\n", 0},
		{"list", []string{"--files-with-matches", "apple", "a.txt"}, "a.txt\n", 0},
		{"list without", []string{"--files-without-match", "apple", "b.txt"}, "b.txt\n", 1},
		{"numbers", []string{"--line-number", "cherry", "a.txt"}, "4:cherry\n", 0},
		{"no filename", []string{"--no-filename", "cherry", "a.txt", "b.txt"}, "cherry\ncherry\n", 0},
		{"only", []string{"--only-matching", "apple", "a.txt"}, "apple\napple\n", 0},
		{"quiet", []string{"--quiet", "apple", "a.txt"}, "", 0},
		{"silent", []string{"--silent", "apple", "a.txt"}, "", 0},
		{"max", []string{"--max-count=1", "apple", "a.txt"}, "apple\n", 0},
		{"recursive", []string{"--recursive", "apple", "tree/a.txt"}, "tree/a.txt:apple\n", 0},
		{"pattern file", []string{"--file=patterns", "b.txt"}, "cherry\n", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{FS: newFS(t), Stdout: &out, Stderr: &stderr}, tt.args)
			var wantErr error
			if tt.code != 0 {
				wantErr = interp.ExitStatus(tt.code)
			}
			if !errors.Is(err, wantErr) || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got (%q, %q, %v), want (%q, %v)", out.String(), stderr.String(), err, tt.want, wantErr)
			}
		})
	}
}

func TestPaths(t *testing.T) {
	for _, tt := range []struct {
		name, dir string
		args      []string
		want      string
	}{
		{"relative", "tree", []string{"apple", "a.txt"}, "apple\n"},
		{"absolute", "tree", []string{"cherry", "/b.txt"}, "cherry\n"},
		{"parent", "tree/sub", []string{"apple", "../a.txt"}, "apple\n"},
		{"clamp to root", "tree", []string{"cherry", "../../b.txt"}, "cherry\n"},
		{"relative glob", "tree", []string{"apple", "*.txt"}, "apple\n"},
		{"explicit dot prefix", ".", []string{"-r", "apple", "./tree/a.txt"}, "./tree/a.txt:apple\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{FS: newFS(t), Dir: tt.dir, Stdout: &out, Stderr: &stderr}, tt.args)
			if err != nil || stderr.Len() != 0 || out.String() != tt.want {
				t.Fatalf("got (%q, %q, %v), want %q", out.String(), stderr.String(), err, tt.want)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Impl{}.Exec(ctx, &command.ExecContext{}, []string{"x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
}

func TestRegexTimeout(t *testing.T) {
	var stderr bytes.Buffer
	err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader(strings.Repeat("a", 1000) + "!\n"), Stderr: &stderr}, []string{"-P", `(?:(a|aa)+)+$`})
	if !errors.Is(err, interp.ExitStatus(2)) || stderr.String() != "grep: regular expression execution limit exceeded\n" {
		t.Fatalf("got (%q, %v), want regex limit", stderr.String(), err)
	}
}

type cancelReader struct{ cancel context.CancelFunc }

func (r cancelReader) Read([]byte) (int, error) { r.cancel(); return 0, context.Canceled }

func TestCancellationDuringRead(t *testing.T) {
	for _, args := range [][]string{{"x"}, {"-f", "-"}, {"x", "-"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := Impl{}.Exec(ctx, &command.ExecContext{Stdin: cancelReader{cancel}}, args)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want cancellation", err)
			}
		})
	}
}
