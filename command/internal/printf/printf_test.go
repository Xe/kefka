package printf

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/expand"
	"tangled.org/xeiaso.net/kefka/command"
)

type writeEnv struct {
	vars map[string]expand.Variable
}

func newWriteEnv() *writeEnv {
	return &writeEnv{vars: map[string]expand.Variable{}}
}

func (w *writeEnv) Get(name string) expand.Variable {
	return w.vars[name]
}

func (w *writeEnv) Each(fn func(name string, vr expand.Variable) bool) {
	for k, v := range w.vars {
		if !fn(k, v) {
			return
		}
	}
}

func (w *writeEnv) Set(name string, vr expand.Variable) error {
	w.vars[name] = vr
	return nil
}

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	return runWithEnv(t, args, nil)
}

func runWithEnv(t *testing.T, args []string, env expand.Environ) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:   strings.NewReader(""),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Dir:     ".",
		Environ: env,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestPrintf(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "literal string no specifiers",
			args:       []string{"hello world"},
			wantStdout: "hello world",
		},
		{
			name:       "newline escape",
			args:       []string{"hello\\n"},
			wantStdout: "hello\n",
		},
		{
			name:       "tab escape",
			args:       []string{"a\\tb"},
			wantStdout: "a\tb",
		},
		{
			name:       "backslash escape",
			args:       []string{"a\\\\b"},
			wantStdout: "a\\b",
		},
		{
			name:       "octal escape",
			args:       []string{"\\101"},
			wantStdout: "A",
		},
		{
			name:       "hex escape ascii",
			args:       []string{"\\x41"},
			wantStdout: "A",
		},
		{
			name:       "hex escape utf-8 sequence",
			args:       []string{"\\xc3\\xa9"},
			wantStdout: "é",
		},
		{
			name:       "unicode escape",
			args:       []string{"\\u00e9"},
			wantStdout: "é",
		},
		{
			name:       "literal percent",
			args:       []string{"100%%"},
			wantStdout: "100%",
		},
		{
			name:       "format string %s",
			args:       []string{"%s\\n", "hello"},
			wantStdout: "hello\n",
		},
		{
			name:       "format string %d",
			args:       []string{"%d\\n", "42"},
			wantStdout: "42\n",
		},
		{
			name:       "format negative integer",
			args:       []string{"%d", "-7"},
			wantStdout: "-7",
		},
		{
			name:       "format hex lower",
			args:       []string{"%x", "255"},
			wantStdout: "ff",
		},
		{
			name:       "format hex upper",
			args:       []string{"%X", "255"},
			wantStdout: "FF",
		},
		{
			name:       "format hex with # flag",
			args:       []string{"%#x", "255"},
			wantStdout: "0xff",
		},
		{
			name:       "format octal",
			args:       []string{"%o", "8"},
			wantStdout: "10",
		},
		{
			name:       "format octal with # flag",
			args:       []string{"%#o", "8"},
			wantStdout: "010",
		},
		{
			name:       "format float default precision",
			args:       []string{"%f", "3.14"},
			wantStdout: "3.140000",
		},
		{
			name:       "format float with precision",
			args:       []string{"%.2f", "3.14159"},
			wantStdout: "3.14",
		},
		{
			name:       "format float with width and precision",
			args:       []string{"%8.2f", "3.14"},
			wantStdout: "    3.14",
		},
		{
			name:       "string width right justify",
			args:       []string{"%5s", "hi"},
			wantStdout: "   hi",
		},
		{
			name:       "string width left justify",
			args:       []string{"%-5s|", "hi"},
			wantStdout: "hi   |",
		},
		{
			name:       "string precision truncates",
			args:       []string{"%.3s", "hello"},
			wantStdout: "hel",
		},
		{
			name:       "integer zero pad",
			args:       []string{"%05d", "42"},
			wantStdout: "00042",
		},
		{
			name:       "integer plus flag positive",
			args:       []string{"%+d", "42"},
			wantStdout: "+42",
		},
		{
			name:       "integer plus flag negative",
			args:       []string{"%+d", "-42"},
			wantStdout: "-42",
		},
		{
			name:       "integer space flag positive",
			args:       []string{"% d", "42"},
			wantStdout: " 42",
		},
		{
			name:       "char specifier",
			args:       []string{"%c", "ABC"},
			wantStdout: "A",
		},
		{
			name:       "format reused with multiple args",
			args:       []string{"%s\\n", "a", "b", "c"},
			wantStdout: "a\nb\nc\n",
		},
		{
			name:       "format reused mixed types",
			args:       []string{"[%d]", "1", "2", "3"},
			wantStdout: "[1][2][3]",
		},
		{
			name:       "missing arg defaults to zero",
			args:       []string{"%d"},
			wantStdout: "0",
		},
		{
			name:       "missing arg defaults to empty string",
			args:       []string{"<%s>"},
			wantStdout: "<>",
		},
		{
			name:       "width from arg",
			args:       []string{"%*d", "5", "42"},
			wantStdout: "   42",
		},
		{
			name:       "precision from arg",
			args:       []string{"%.*f", "2", "3.14159"},
			wantStdout: "3.14",
		},
		{
			name:       "%b interprets escapes",
			args:       []string{"%b", "hello\\nworld"},
			wantStdout: "hello\nworld",
		},
		{
			name:       "%b with c stops output",
			args:       []string{"%b\\n", "hi\\cdropped"},
			wantStdout: "hi",
		},
		{
			name:       "hex input parsed",
			args:       []string{"%d", "0x1f"},
			wantStdout: "31",
		},
		{
			name:       "octal input parsed",
			args:       []string{"%d", "010"},
			wantStdout: "8",
		},
		{
			name:       "char notation single quote",
			args:       []string{"%d", "'A"},
			wantStdout: "65",
		},
		{
			name:       "%q empty string",
			args:       []string{"%q", ""},
			wantStdout: "''",
		},
		{
			name:       "%q safe string",
			args:       []string{"%q", "hello"},
			wantStdout: "hello",
		},
		{
			name:       "%q with space",
			args:       []string{"%q", "hello world"},
			wantStdout: "hello\\ world",
		},
		{
			name:       "%q with newline uses dollar quote",
			args:       []string{"%q", "a\nb"},
			wantStdout: "$'a\\nb'",
		},
		{
			name:       "stop at -- separator",
			args:       []string{"--", "%s", "hello"},
			wantStdout: "hello",
		},
		{
			name:       "no args returns usage error",
			args:       nil,
			wantStdout: "",
			wantErr:    true,
		},
		{
			name:       "unknown directive errors",
			args:       []string{"%z", "x"},
			wantStdout: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := run(t, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}

func TestPrintfHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: printf") {
		t.Errorf("stderr missing usage line, got %q", stderr)
	}
}

func TestPrintfMissingArg(t *testing.T) {
	_, stderr, err := run(t, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(stderr, "usage: printf format") {
		t.Errorf("stderr missing usage line, got %q", stderr)
	}
}

func TestPrintfVarAssignment(t *testing.T) {
	env := newWriteEnv()
	stdout, stderr, err := runWithEnv(t, []string{"-v", "myvar", "%s", "hello"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty when -v is used, got %q", stdout)
	}
	got := env.Get("myvar")
	if !got.IsSet() || got.String() != "hello" {
		t.Errorf("myvar = %q, want %q", got.String(), "hello")
	}
}

func TestPrintfVarInvalidIdentifier(t *testing.T) {
	env := newWriteEnv()
	_, stderr, err := runWithEnv(t, []string{"-v", "1bad", "%s", "x"}, env)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(stderr, "not a valid identifier") {
		t.Errorf("stderr missing identifier error, got %q", stderr)
	}
}

func TestPrintfVarMissingValue(t *testing.T) {
	env := newWriteEnv()
	_, stderr, err := runWithEnv(t, []string{"-v"}, env)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(stderr, "option requires an argument") {
		t.Errorf("stderr missing option-requires-arg error, got %q", stderr)
	}
}

func TestPrintfVarArraySubscript(t *testing.T) {
	env := newWriteEnv()
	_, stderr, err := runWithEnv(t, []string{"-v", "arr[key]", "%s", "value"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr)
	}
	got := env.Get("arr_key")
	if !got.IsSet() || got.String() != "value" {
		t.Errorf("arr_key = %q, want %q", got.String(), "value")
	}
}

func TestPrintfStrftime(t *testing.T) {
	// 2025-01-15T12:00:00Z = unix 1736942400
	stdout, _, err := run(t, []string{"%(%Y-%m-%d)T", "1736942400"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "2025-01-15" {
		t.Errorf("stdout = %q, want %q", stdout, "2025-01-15")
	}
}
