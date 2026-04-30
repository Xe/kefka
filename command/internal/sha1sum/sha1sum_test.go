package sha1sum

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
	f, err := fs.OpenFile("hello.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("hello"))
	f.Close()
	return fs
}

func run(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newFS(t),
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestHashStdin(t *testing.T) {
	// SHA1("hello") = aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	stdout, _, err := run(t, nil, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d  -\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHashFile(t *testing.T) {
	stdout, _, err := run(t, []string{"hello.txt"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d  hello.txt\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestCheckOK(t *testing.T) {
	stdout, _, err := run(t, []string{"-c"},
		"aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d  hello.txt\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "hello.txt: OK\n" {
		t.Errorf("stdout = %q, want OK", stdout)
	}
}

func TestHelp(t *testing.T) {
	_, stderr, err := run(t, []string{"--help"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "Usage: sha1sum [OPTION]... [FILE]...") {
		t.Errorf("usage missing: %q", stderr)
	}
	if !strings.Contains(stderr, "compute SHA1 message digest") {
		t.Errorf("summary missing: %q", stderr)
	}
}
