package zcat

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	gzlib "compress/gzip"
	"github.com/Xe/kefka/command"
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
	write("hello.txt.gz", gzipData(t, []byte("hello world")))
	write("notgz.txt", []byte("not compressed"))
	return fs
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func run(t *testing.T, args []string, stdin []byte, fs billy.Filesystem) (stdout, stderr []byte, err error) {
	t.Helper()
	var outb, errb bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  bytes.NewReader(stdin),
		Stdout: &outb,
		Stderr: &errb,
		Dir:    ".",
		FS:     fs,
	}
	err = Impl{}.Exec(context.Background(), ec, args)
	return outb.Bytes(), errb.Bytes(), err
}

func TestDecompressStdin(t *testing.T) {
	stdout, stderr, err := run(t, nil, gzipData(t, []byte("hello")), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if want := "hello"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestDecompressFile(t *testing.T) {
	stdout, stderr, err := run(t, []string{"hello.txt.gz"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if want := "hello world"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	fs := newFS(t)
	_, err = fs.Stat("hello.txt.gz")
	if err != nil {
		t.Fatalf("original file should still exist: %v", err)
	}
}

func TestDecompressMultipleFiles(t *testing.T) {
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, _ := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		f.Write(data)
		f.Close()
	}
	write("first.gz", gzipData(t, []byte("foo")))
	write("second.gz", gzipData(t, []byte("bar")))

	stdout, stderr, err := run(t, []string{"first.gz", "second.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if want := "foobar"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestDashForStdin(t *testing.T) {
	stdout, stderr, err := run(t, []string{"-"}, gzipData(t, []byte("hello world")), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if want := "hello world"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestListMode(t *testing.T) {
	stdout, stderr, err := run(t, []string{"-l", "hello.txt.gz"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stderrStr := string(stderr)
	if !contains(stderrStr, "compressed") {
		t.Errorf("stderr should contain header: %q", stderrStr)
	}
	if !contains(stderrStr, "hello.txt.gz") {
		t.Errorf("stderr should contain filename: %q", stderrStr)
	}
	if len(stdout) > 0 {
		t.Errorf("stdout should be empty in list mode, got: %q", stdout)
	}
}

func TestTestValid(t *testing.T) {
	stdout, _, err := run(t, []string{"-t", "hello.txt.gz"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stdout) > 0 {
		t.Errorf("stdout should be empty in test mode, got: %q", stdout)
	}
}

func TestTestInvalid(t *testing.T) {
	_, _, err := run(t, []string{"-t", "notgz.txt"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for non-gzip file")
	}
}

func TestVerbose(t *testing.T) {
	stdout, stderr, err := run(t, []string{"-v", "hello.txt.gz"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stderrStr := string(stderr)
	if !contains(stderrStr, "hello.txt.gz") || !contains(stderrStr, "OK") {
		t.Errorf("stderr should contain verbose info: %q", stderrStr)
	}
	if want := "hello world"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestEmptyGzipStdin(t *testing.T) {
	stdout, stderr, err := run(t, nil, gzipData(t, []byte{}), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout should be empty, got: %q", stdout)
	}
}

func TestMissingFile(t *testing.T) {
	_, stderr, err := run(t, []string{"nope.gz"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !contains(string(stderr), "No such file or directory") {
		t.Errorf("stderr should mention 'No such file or directory': %q", stderr)
	}
}

func TestNotGzip(t *testing.T) {
	_, stderr, err := run(t, []string{"notgz.txt"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for non-gzip file")
	}
	if !contains(string(stderr), "not in gzip format") {
		t.Errorf("stderr should mention 'not in gzip format': %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	_, stderr, err := run(t, []string{"--help"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stderrStr := string(stderr)
	if !contains(stderrStr, "Usage: zcat") {
		t.Errorf("stderr should contain usage: %q", stderrStr)
	}
	if !contains(stderrStr, "--force") {
		t.Errorf("stderr should contain --help flag info: %q", stderrStr)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, err := run(t, []string{"--nope"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestConcatenatedRoundTrip(t *testing.T) {
	foo := gzipData(t, []byte("foo"))
	bar := gzipData(t, []byte("bar"))
	combined := append(foo, bar...)

	stdout, stderr, err := run(t, nil, combined, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}

	if want := "foo"; string(stdout[:3]) != want {
		t.Errorf("first part stdout = %q, want %q", stdout[:3], want)
	}
	if want := "bar"; string(stdout[3:]) != want {
		t.Errorf("second part stdout = %q, want %q", stdout[3:], want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
