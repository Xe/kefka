package tr

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestTr(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "translate lowercase to uppercase via range",
			args:       []string{"a-z", "A-Z"},
			stdin:      "hello world",
			wantStdout: "HELLO WORLD",
		},
		{
			name:       "translate using POSIX class",
			args:       []string{"[:lower:]", "[:upper:]"},
			stdin:      "Hello World",
			wantStdout: "HELLO WORLD",
		},
		{
			name:       "set2 shorter than set1 reuses last char",
			args:       []string{"abcde", "X"},
			stdin:      "abcdef",
			wantStdout: "XXXXXf",
		},
		{
			name:       "delete characters in set1",
			args:       []string{"-d", "aeiou"},
			stdin:      "hello world",
			wantStdout: "hll wrld",
		},
		{
			name:       "delete with --delete long form",
			args:       []string{"--delete", "0-9"},
			stdin:      "abc123def456",
			wantStdout: "abcdef",
		},
		{
			name:       "squeeze repeated characters in set1",
			args:       []string{"-s", " "},
			stdin:      "hello   world  foo",
			wantStdout: "hello world foo",
		},
		{
			name:       "squeeze with --squeeze-repeats long form",
			args:       []string{"--squeeze-repeats", "a"},
			stdin:      "aaabbbaaaccc",
			wantStdout: "abbbaccc",
		},
		{
			name:       "complement with -c deletes everything except set1",
			args:       []string{"-cd", "0-9"},
			stdin:      "abc123def",
			wantStdout: "123",
		},
		{
			name:       "complement with -C is alias for -c",
			args:       []string{"-Cd", "0-9"},
			stdin:      "abc123def",
			wantStdout: "123",
		},
		{
			name:       "complement with --complement long form",
			args:       []string{"--complement", "-d", "a-z"},
			stdin:      "Hello World",
			wantStdout: "elloorld",
		},
		{
			name:       "translate with squeeze collapses repeated set2 chars",
			args:       []string{"-s", "ab", "xx"},
			stdin:      "aabbab",
			wantStdout: "x",
		},
		{
			name:       "complement translate maps to last char of set2",
			args:       []string{"-c", "0-9", "*"},
			stdin:      "abc123def",
			wantStdout: "***123***",
		},
		{
			name:       "escape sequence \\n in set",
			args:       []string{"\\n", " "},
			stdin:      "a\nb\nc",
			wantStdout: "a b c",
		},
		{
			name:       "escape sequence \\t in set",
			args:       []string{"\\t", " "},
			stdin:      "a\tb\tc",
			wantStdout: "a b c",
		},
		{
			name:       "POSIX class [:digit:] deletion",
			args:       []string{"-d", "[:digit:]"},
			stdin:      "abc123def456",
			wantStdout: "abcdef",
		},
		{
			name:       "rot13 round trip",
			args:       []string{"a-zA-Z", "n-za-mN-ZA-M"},
			stdin:      "Hello, World!",
			wantStdout: "Uryyb, Jbeyq!",
		},
		{
			name:       "empty input produces empty output",
			args:       []string{"a", "b"},
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "characters not in set1 pass through",
			args:       []string{"abc", "xyz"},
			stdin:      "abcdef",
			wantStdout: "xyzdef",
		},
		{
			name:       "missing operand",
			args:       nil,
			wantErrSub: "tr: missing operand",
			wantErr:    true,
		},
		{
			name:       "missing operand after SET1",
			args:       []string{"abc"},
			stdin:      "abc",
			wantErrSub: "tr: missing operand after SET1",
			wantErr:    true,
		},
		{
			name:       "delete with one set is allowed",
			args:       []string{"-d", "a"},
			stdin:      "banana",
			wantStdout: "bnn",
		},
		{
			name:       "squeeze with one set is allowed",
			args:       []string{"-s", "a"},
			stdin:      "baaanaaa",
			wantStdout: "bana",
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope", "a", "b"},
			wantErr: true,
		},
		{
			name:       "octal escape \\101 maps A",
			args:       []string{"\\101", "a"},
			stdin:      "A",
			wantStdout: "a",
		},
		{
			name:       "octal escape \\060 maps 0",
			args:       []string{"\\060", "X"},
			stdin:      "0a0",
			wantStdout: "XaX",
		},
		{
			name:       "backslash \\\\ is literal backslash",
			args:       []string{"\\\\", "X"},
			stdin:      "a\\b",
			wantStdout: "aXb",
		},
		{
			name:       "backslash \\a maps BEL",
			args:       []string{"\\a", "X"},
			stdin:      "a\ab",
			wantStdout: "aXb",
		},
		{
			name:       "backslash \\b maps backspace",
			args:       []string{"\\b", "X"},
			stdin:      "a\bb",
			wantStdout: "aXb",
		},
		{
			name:       "backslash \\f maps form feed",
			args:       []string{"\\f", "X"},
			stdin:      "a\fb",
			wantStdout: "aXb",
		},
		{
			name:       "backslash \\v maps vertical tab",
			args:       []string{"\\v", "X"},
			stdin:      "a\vb",
			wantStdout: "aXb",
		},
		{
			name:       "backslash \\r maps carriage return",
			args:       []string{"\\r", "X"},
			stdin:      "a\rb",
			wantStdout: "aXb",
		},
		{
			name:       "unknown backslash escape silently strips backslash",
			args:       []string{"\\Z", "X"},
			stdin:      "aZb",
			wantStdout: "aXb",
		},
		{
			name:       "POSIX class lower to upper",
			args:       []string{"[:lower:]", "[:upper:]"},
			stdin:      "hello",
			wantStdout: "HELLO",
		},
		{
			name:       "POSIX class upper to lower",
			args:       []string{"[:upper:]", "[:lower:]"},
			stdin:      "WORLD",
			wantStdout: "world",
		},
		{
			name:       "delete digit class",
			args:       []string{"-d", "[:digit:]"},
			stdin:      "a1b2c3",
			wantStdout: "abc",
		},
		{
			name:       "delete space class",
			args:       []string{"-d", "[:space:]"},
			stdin:      "a b\tc",
			wantStdout: "abc",
		},
		{
			name:       "delete blank class",
			args:       []string{"-d", "[:blank:]"},
			stdin:      "a b\tc d",
			wantStdout: "abcd",
		},
		{
			name:       "equivalence class is single char",
			args:       []string{"[=e=]", "X"},
			stdin:      "hello",
			wantStdout: "hXllo",
		},
		{
			name:       "set2 padded with last char to length of set1",
			args:       []string{"a-c", "X"},
			stdin:      "abc",
			wantStdout: "XXX",
		},
		{
			name:       "set2 [X*] pads to length of set1",
			args:       []string{"a-c", "[X*]"},
			stdin:      "abcdef",
			wantStdout: "XXXdef",
		},
		{
			name:       "set2 [X*N] explicit count",
			args:       []string{"XYZ", "[a*3]"},
			stdin:      "aXbYcZ",
			wantStdout: "aabaca",
		},
		{
			name:       "set2 mixed segment then pad",
			args:       []string{"a-e", "Y[Z*]"},
			stdin:      "abcde",
			wantStdout: "YZZZZ",
		},
		{
			name:       "set2 octal repeat count",
			args:       []string{"abcd", "[X*04]"},
			stdin:      "abcd",
			wantStdout: "XXXX",
		},
		{
			name:       "set1 repeat construct rejected",
			args:       []string{"[X*]", "a"},
			stdin:      "X",
			wantErr:    true,
			wantErrSub: "may not appear in string1",
		},
		{
			name:       "invalid character class",
			args:       []string{"[:foo:]", "X"},
			stdin:      "abc",
			wantErr:    true,
			wantErrSub: "invalid character class",
		},
		{
			name:       "octal escape NUL deletion",
			args:       []string{"-d", "\\000"},
			stdin:      "a\x00b\x00c",
			wantStdout: "abc",
		},
		{
			name:       "newline escape with actual newline input",
			args:       []string{"\\n", " "},
			stdin:      "a\nb",
			wantStdout: "a b",
		},
		{
			name:       "POSIX example word splitter",
			args:       []string{"-cs", "[:alpha:]", "[\\n*]"},
			stdin:      "hello, world! foo bar\n",
			wantStdout: "hello\nworld\nfoo\nbar\n",
		},
		{
			name:       "translate digit class to single char pads",
			args:       []string{"[:digit:]", "#"},
			stdin:      "ab12cd",
			wantStdout: "ab##cd",
		},
		{
			name:       "translate range to range",
			args:       []string{"a-z", "A-Z"},
			stdin:      "hello",
			wantStdout: "HELLO",
		},
		{
			name:       "complement -c with single replacement char pads",
			args:       []string{"-c", "a-z", "X"},
			stdin:      "Hello, World!",
			wantStdout: "XelloXXXorldX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args, tt.stdin)
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
	stdout, stderr, err := run(t, []string{"--help"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: tr [OPTION]... SET1 [SET2]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
}
