package base64

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
	write("hello.txt", []byte("hello"))
	write("two.txt", []byte("world"))
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

func TestEncodeStdin(t *testing.T) {
	stdout, stderr, err := run(t, nil, "hello", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if want := "aGVsbG8=\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestEncodeFile(t *testing.T) {
	stdout, _, err := run(t, []string{"hello.txt"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "aGVsbG8=\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestEncodeMultipleFiles(t *testing.T) {
	stdout, _, err := run(t, []string{"hello.txt", "two.txt"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "aGVsbG93b3JsZA==\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestEncodeStdinDash(t *testing.T) {
	stdout, _, err := run(t, []string{"-"}, "hello", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "aGVsbG8=\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestEncodeWrap(t *testing.T) {
	// 80 'a' bytes encodes to 108 chars; wrap=10 should split.
	input := strings.Repeat("a", 30)
	stdout, _, err := run(t, []string{"-w", "10"}, input, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if len(line) > 10 {
			t.Errorf("line too long (%d): %q", len(line), line)
		}
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("output should end with newline, got %q", stdout)
	}
}

func TestEncodeWrapZeroDisablesWrap(t *testing.T) {
	input := strings.Repeat("a", 30)
	stdout, _, err := run(t, []string{"-w", "0"}, input, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout, "\n") {
		t.Errorf("expected no newlines with -w 0, got %q", stdout)
	}
}

func TestEncodeEmpty(t *testing.T) {
	stdout, _, err := run(t, nil, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestDecodeStdin(t *testing.T) {
	stdout, _, err := run(t, []string{"-d"}, "aGVsbG8=", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "hello"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestDecodeStripsWhitespace(t *testing.T) {
	stdout, _, err := run(t, []string{"-d"}, "aGVs\nbG8=\n", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "hello"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, stderr, err := run(t, []string{"--decode"}, "not!valid!base64!@#", newFS(t))
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if !strings.Contains(stderr, "invalid input") {
		t.Errorf("stderr = %q, want 'invalid input'", stderr)
	}
}

func TestMissingFile(t *testing.T) {
	_, stderr, err := run(t, []string{"nope.txt"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(stderr, "No such file or directory") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	_, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "Usage: base64 [OPTION]... [FILE]") {
		t.Errorf("usage missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-d, --decode") {
		t.Errorf("decode flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-w, --wrap=COLS") {
		t.Errorf("wrap flag missing from help: %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, err := run(t, []string{"--no-such-flag"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRoundTrip(t *testing.T) {
	input := "Hello, \xff\xfe World! \x00\x01\x02"
	enc, _, err := run(t, nil, input, newFS(t))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dec, _, err := run(t, []string{"-d"}, enc, newFS(t))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dec != input {
		t.Errorf("round trip mismatch: want %q, got %q", input, dec)
	}
}
