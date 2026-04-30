package expr

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var status interp.ExitStatus
	if errors.As(err, &status) {
		return int(status)
	}
	return -1
}

func TestExpr(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantExit   int
	}{
		{
			name:       "single operand passes through",
			args:       []string{"hello"},
			wantStdout: "hello\n",
		},
		{
			name:       "single zero is falsy",
			args:       []string{"0"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "single empty string is falsy",
			args:       []string{""},
			wantStdout: "\n",
			wantExit:   1,
		},
		{
			name:       "addition",
			args:       []string{"1", "+", "2"},
			wantStdout: "3\n",
		},
		{
			name:       "subtraction yields negative",
			args:       []string{"1", "-", "5"},
			wantStdout: "-4\n",
		},
		{
			name:       "multiplication",
			args:       []string{"3", "*", "4"},
			wantStdout: "12\n",
		},
		{
			name:       "division truncates toward zero",
			args:       []string{"7", "/", "2"},
			wantStdout: "3\n",
		},
		{
			name:       "negative division truncates toward zero",
			args:       []string{"-7", "/", "2"},
			wantStdout: "-3\n",
		},
		{
			name:       "modulo",
			args:       []string{"10", "%", "3"},
			wantStdout: "1\n",
		},
		{
			name:       "operator precedence",
			args:       []string{"1", "+", "2", "*", "3"},
			wantStdout: "7\n",
		},
		{
			name:       "parentheses override precedence",
			args:       []string{"(", "1", "+", "2", ")", "*", "3"},
			wantStdout: "9\n",
		},
		{
			name:       "result that evaluates to zero exits 1",
			args:       []string{"3", "-", "3"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "division by zero",
			args:       []string{"1", "/", "0"},
			wantStderr: "expr: division by zero\n",
			wantExit:   2,
		},
		{
			name:       "modulo by zero",
			args:       []string{"5", "%", "0"},
			wantStderr: "expr: division by zero\n",
			wantExit:   2,
		},
		{
			name:       "non-integer addition",
			args:       []string{"abc", "+", "1"},
			wantStderr: "expr: non-integer argument\n",
			wantExit:   2,
		},
		{
			name:       "numeric equality true",
			args:       []string{"3", "=", "3"},
			wantStdout: "1\n",
		},
		{
			name:       "numeric equality false",
			args:       []string{"3", "=", "4"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "string equality",
			args:       []string{"foo", "=", "foo"},
			wantStdout: "1\n",
		},
		{
			name:       "string inequality",
			args:       []string{"foo", "!=", "bar"},
			wantStdout: "1\n",
		},
		{
			name:       "less-than numeric",
			args:       []string{"2", "<", "10"},
			wantStdout: "1\n",
		},
		{
			name:       "less-than string lexicographic",
			args:       []string{"apple", "<", "banana"},
			wantStdout: "1\n",
		},
		{
			name:       "greater-equal",
			args:       []string{"5", ">=", "5"},
			wantStdout: "1\n",
		},
		{
			name:       "logical or returns first truthy",
			args:       []string{"foo", "|", "bar"},
			wantStdout: "foo\n",
		},
		{
			name:       "logical or falls through to second",
			args:       []string{"0", "|", "bar"},
			wantStdout: "bar\n",
		},
		{
			name:       "logical or both falsy",
			args:       []string{"0", "|", "0"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "logical and returns left when both truthy",
			args:       []string{"foo", "&", "bar"},
			wantStdout: "foo\n",
		},
		{
			name:       "logical and zero short-circuits",
			args:       []string{"0", "&", "bar"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "match anchored colon returns length",
			args:       []string{"abcdef", ":", "abc"},
			wantStdout: "3\n",
		},
		{
			name:       "match anchored colon no match returns 0",
			args:       []string{"xyz", ":", "abc"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "match anchored capture group",
			args:       []string{"abc123", ":", "abc([0-9]+)"},
			wantStdout: "123\n",
		},
		{
			name:       "match function unanchored returns length",
			args:       []string{"match", "abcdef", "cd"},
			wantStdout: "2\n",
		},
		{
			name:       "match function with capture",
			args:       []string{"match", "hello world", "(w[a-z]+)"},
			wantStdout: "world\n",
		},
		{
			name:       "match function no match",
			args:       []string{"match", "abc", "xyz"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "substr basic",
			args:       []string{"substr", "abcdef", "2", "3"},
			wantStdout: "bcd\n",
		},
		{
			name:       "substr clamps past end",
			args:       []string{"substr", "abc", "2", "10"},
			wantStdout: "bc\n",
		},
		{
			name:       "substr beyond string returns empty",
			args:       []string{"substr", "abc", "5", "1"},
			wantStdout: "\n",
			wantExit:   1,
		},
		{
			name:       "index finds first match",
			args:       []string{"index", "hello", "el"},
			wantStdout: "2\n",
		},
		{
			name:       "index no match returns zero",
			args:       []string{"index", "hello", "xyz"},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "length basic",
			args:       []string{"length", "abcdef"},
			wantStdout: "6\n",
		},
		{
			name:       "length empty",
			args:       []string{"length", ""},
			wantStdout: "0\n",
			wantExit:   1,
		},
		{
			name:       "length counts runes not bytes",
			args:       []string{"length", "café"},
			wantStdout: "4\n",
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantStderr: "expr: missing operand\n",
			wantExit:   2,
		},
		{
			name:       "syntax error on unbalanced paren",
			args:       []string{"(", "1", "+", "2"},
			wantStderr: "expr: syntax error\n",
			wantExit:   2,
		},
		{
			name:       "double dash terminator allows leading dash literal",
			args:       []string{"--", "-1", "+", "2"},
			wantStdout: "1\n",
		},
		{
			name:       "negative literal as standalone operand",
			args:       []string{"-5"},
			wantStdout: "-5\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if got := exitCode(err); got != tt.wantExit {
				t.Errorf("exit code = %d, want %d (err=%v)", got, tt.wantExit, err)
			}
			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantStderr != "" && stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: expr EXPRESSION") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help output: %q", stderr)
	}
}
