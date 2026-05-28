package sha256sum

import (
	"bytes"
	"context"
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
	// SHA256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	stdout, _, err := run(t, nil, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  -\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHashFile(t *testing.T) {
	stdout, _, err := run(t, []string{"hello.txt"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  hello.txt\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestCheckOK(t *testing.T) {
	stdout, _, err := run(t, []string{"-c"},
		"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  hello.txt\n")
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
	if !strings.Contains(stderr, "Usage: sha256sum [OPTION]... [FILE]...") {
		t.Errorf("usage missing: %q", stderr)
	}
	if !strings.Contains(stderr, "compute SHA256 message digest") {
		t.Errorf("summary missing: %q", stderr)
	}
}
