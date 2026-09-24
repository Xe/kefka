package gunzip

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
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
	fs.MkdirAll("subdir", 0o755)
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
	fs := newFS(t)
	stdout, stderr, err := run(t, nil, gzipData(t, []byte("hello")), fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stderr) != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if string(stdout) != "hello" {
		t.Errorf("stdout = %q, want \"hello\"", stdout)
	}
}

func TestDecompressFile(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	if string(stderr) != "" {
		t.Errorf("stderr should be empty, got %q", stderr)
	}
	f, err := fs.Open("hello.txt")
	if err != nil {
		t.Fatalf("hello.txt not created: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want \"hello world\"", data)
	}
}

func TestDecompressToStdout(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"-c", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "hello world" {
		t.Errorf("stdout = %q, want \"hello world\"", stdout)
	}
	_, err = fs.Stat("hello.txt.gz")
	if err != nil {
		t.Errorf("original .gz file should still exist")
	}
	_, err = fs.Stat("hello.txt")
	if err == nil {
		t.Errorf("hello.txt should not exist when using -c")
	}
	if string(stderr) != "" {
		t.Errorf("stderr should be empty, got %q", stderr)
	}
}

func TestKeepOriginal(t *testing.T) {
	fs := newFS(t)
	_, _, err := run(t, []string{"-k", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = fs.Stat("hello.txt.gz")
	if err != nil {
		t.Errorf("original .gz file should still exist with -k")
	}
	_, err = fs.Stat("hello.txt")
	if err != nil {
		t.Errorf("hello.txt should exist")
	}
}

func TestForceOverwrite(t *testing.T) {
	fs := newFS(t)
	f, _ := fs.OpenFile("hello.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	f.Write([]byte("old content"))
	f.Close()

	_, _, err := run(t, []string{"-f", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, err = fs.Open("hello.txt")
	if err != nil {
		t.Fatalf("hello.txt not created: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want \"hello world\"", data)
	}
}

func TestList(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"-l", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	stderrStr := string(stderr)
	if !strings.Contains(stderrStr, "compressed") || !strings.Contains(stderrStr, "uncompressed") {
		t.Errorf("list output should contain headers, got: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "hello.txt") {
		t.Errorf("list output should contain file name, got: %q", stderrStr)
	}
}

func TestTestValid(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"-t", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" || string(stderr) != "" {
		t.Errorf("output should be empty, got stdout=%q, stderr=%q", stdout, stderr)
	}
}

func TestTestInvalid(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-t", "notgz.txt"}, nil, fs)
	if err == nil {
		t.Fatal("expected error for non-gzip file")
	}
	if !strings.Contains(string(stderr), "not in gzip format") {
		t.Errorf("stderr should mention not in gzip format, got: %q", stderr)
	}
}

func TestVerbose(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"-v", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	if !strings.Contains(string(stderr), "%") {
		t.Errorf("stderr should contain ratio, got: %q", stderr)
	}
}

func TestSuffix(t *testing.T) {
	fs := memfs.New()
	gzData := gzipData(t, []byte("custom content"))
	f, _ := fs.OpenFile("file.custom", os.O_CREATE|os.O_WRONLY, 0o644)
	f.Write(gzData)
	f.Close()

	_, _, err := run(t, []string{"-S", ".custom", "file.custom"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, err = fs.Open("file")
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "custom content" {
		t.Errorf("file content = %q, want \"custom content\"", data)
	}
}

func TestEmptyGzipStdin(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, nil, gzipData(t, []byte{}), fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	if string(stderr) != "" {
		t.Errorf("stderr should be empty, got %q", stderr)
	}
}

func TestMissingFile(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"nope.gz"}, nil, fs)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(string(stderr), "No such file or directory") {
		t.Errorf("stderr should mention missing file, got: %q", stderr)
	}
}

func TestNotGzipFormat(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"notgz.txt"}, nil, fs)
	if err == nil {
		t.Fatal("expected error for non-gzip file")
	}
	stderrStr := string(stderr)
	if !strings.Contains(stderrStr, "not in gzip format") {
		t.Errorf("stderr should mention not in gzip format, got: %q", stderrStr)
	}
}

func TestUnknownSuffix(t *testing.T) {
	fs := memfs.New()
	f, _ := fs.OpenFile("file.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	f.Write(gzipData(t, []byte("test")))
	f.Close()

	stdout, stderr, err := run(t, []string{"file.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(stdout) != "" {
		t.Errorf("stdout should be empty, got %q", stdout)
	}
	if !strings.Contains(string(stderr), "unknown suffix") {
		t.Errorf("stderr should mention unknown suffix, got: %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"--help"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(stderr), "Usage: gunzip") {
		t.Errorf("help should contain usage, got: %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	fs := newFS(t)
	_, _, err := run(t, []string{"--nope"}, nil, fs)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestDirectoryWithoutRecursive(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"subdir"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(stderr), "is a directory") {
		t.Errorf("stderr should mention directory, got: %q", stderr)
	}
}

func TestRecursive(t *testing.T) {
	fs := memfs.New()
	fs.MkdirAll("subdir", 0o755)
	f, _ := fs.OpenFile("subdir/file.gz", os.O_CREATE|os.O_WRONLY, 0o644)
	f.Write(gzipData(t, []byte("recursive content")))
	f.Close()

	_, _, err := run(t, []string{"-r", "subdir"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, err = fs.Open("subdir/file")
	if err != nil {
		t.Fatalf("subdir/file not created: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "recursive content" {
		t.Errorf("file content = %q, want \"recursive content\"", data)
	}
}

// silentDropFS wraps a billy.Filesystem so any file opened for writing
// silently drops writes (Write returns 0, nil — exactly the bug that lived
// in s3fs.s3WriteFile.Write).
type silentDropFS struct {
	billy.Filesystem
}

func (s *silentDropFS) Create(filename string) (billy.File, error) {
	f, err := s.Filesystem.Create(filename)
	if err != nil {
		return nil, err
	}
	return &silentDropFile{File: f}, nil
}

func (s *silentDropFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := s.Filesystem.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		return &silentDropFile{File: f}, nil
	}
	return f, nil
}

type silentDropFile struct {
	billy.File
}

func (silentDropFile) Write(p []byte) (int, error) { return 0, nil }

// TestNoSourceRemovalOnShortWrite ensures that when the output filesystem
// silently drops writes (as s3fs once did), gunzip surfaces an error and
// leaves the source .gz file intact instead of deleting it.
func TestNoSourceRemovalOnShortWrite(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		sourcePath string
		outputPath string
		wantStderr string
	}{
		{
			name:       "default suffix",
			args:       []string{"hello.txt.gz"},
			sourcePath: "hello.txt.gz",
			outputPath: "hello.txt",
			wantStderr: "hello.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &silentDropFS{Filesystem: newFS(t)}
			_, stderr, err := run(t, tt.args, nil, fs)
			if err == nil {
				t.Fatal("expected non-nil error when writes are dropped")
			}
			if _, statErr := fs.Stat(tt.sourcePath); statErr != nil {
				t.Errorf("source %s was removed despite write failure: %v", tt.sourcePath, statErr)
			}
			if _, statErr := fs.Stat(tt.outputPath); statErr == nil {
				t.Errorf("empty %s should have been cleaned up after short write", tt.outputPath)
			}
			if !bytes.Contains(stderr, []byte(tt.wantStderr)) {
				t.Errorf("stderr = %q, want a message mentioning %q", string(stderr), tt.wantStderr)
			}
		})
	}
}
