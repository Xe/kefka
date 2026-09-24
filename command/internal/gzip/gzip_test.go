package gzip

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	gzlib "compress/gzip"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/Xe/kefka/command"
)

// silentDropFS wraps a billy.Filesystem so any file opened via Create or
// OpenFile silently drops writes (Write returns 0, nil — exactly the bug
// that lived in s3fs.s3WriteFile.Write).
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
	write("hello.txt", []byte("hello world"))
	return fs
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

func TestCompressStdin(t *testing.T) {
	stdout, stderr, err := run(t, nil, []byte("hello"), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", string(stderr))
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
}

func TestCompressFile(t *testing.T) {
	fs := newFS(t)
	_, _, err := run(t, []string{"hello.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, err := fs.Open("hello.txt.gz")
	if err != nil {
		t.Fatalf("hello.txt.gz not created: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	r, err := gzlib.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
	if _, err := fs.Stat("hello.txt"); err == nil {
		t.Error("original file should be removed")
	}
}

func TestDecompressStdin(t *testing.T) {
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write([]byte("hello"))
	w.Close()
	compressed := buf.Bytes()

	stdout, stderr, err := run(t, []string{"-d"}, compressed, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Fatalf("unexpected stderr: %q", string(stderr))
	}
	if string(stdout) != "hello" {
		t.Errorf("stdout = %q, want %q", string(stdout), "hello")
	}
}

func TestDecompressFile(t *testing.T) {
	fs := memfs.New()
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write([]byte("hello world"))
	w.Close()

	f, _ := fs.Create("hello.txt.gz")
	f.Write(buf.Bytes())
	f.Close()

	_, _, err := run(t, []string{"-d", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	df, err := fs.Open("hello.txt")
	if err != nil {
		t.Fatalf("hello.txt not created: %v", err)
	}
	defer df.Close()
	data, _ := io.ReadAll(df)
	if string(data) != "hello world" {
		t.Errorf("decompressed = %q, want %q", string(data), "hello world")
	}
	if _, err := fs.Stat("hello.txt.gz"); err == nil {
		t.Error("original .gz file should be removed")
	}
}

func TestRoundTrip(t *testing.T) {
	original := []byte("hello world\nthis is a test\xff\xfe")
	fs := newFS(t)

	var compressedBuf bytes.Buffer
	w := gzlib.NewWriter(&compressedBuf)
	w.Write(original)
	w.Close()
	f, _ := fs.Create("test.txt.gz")
	f.Write(compressedBuf.Bytes())
	f.Close()

	_, _, err := run(t, []string{"-d", "test.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}

	df, _ := fs.Open("test.txt")
	decompressed, _ := io.ReadAll(df)
	df.Close()

	if !bytes.Equal(decompressed, original) {
		t.Errorf("round trip mismatch: got %q, want %q", string(decompressed), string(original))
	}
}

func TestStdoutMode(t *testing.T) {
	fs := newFS(t)
	stdout, _, err := run(t, []string{"-c", "hello.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
	if _, err := fs.Stat("hello.txt"); err != nil {
		t.Error("original file should still exist with -c")
	}
	if _, err := fs.Stat("hello.txt.gz"); err == nil {
		t.Error(".gz file should not be created with -c")
	}
}

func TestKeepMode(t *testing.T) {
	fs := newFS(t)
	_, _, err := run(t, []string{"-k", "hello.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := fs.Stat("hello.txt"); err != nil {
		t.Error("original file should still exist with -k")
	}
	if _, err := fs.Stat("hello.txt.gz"); err != nil {
		t.Error(".gz file should be created with -k")
	}
}

func TestListMode(t *testing.T) {
	fs := newFS(t)
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write([]byte("hello world"))
	w.Close()
	f, _ := fs.Create("hello.txt.gz")
	f.Write(buf.Bytes())
	f.Close()

	stdout, stderr, err := run(t, []string{"-l", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stdout) > 0 {
		t.Errorf("unexpected stdout: %q", string(stdout))
	}
	output := string(stderr)
	if !bytes.Contains([]byte(output), []byte("compressed")) {
		t.Error("output missing header")
	}
	if !bytes.Contains(stderr, []byte("hello.txt")) {
		t.Error("output missing filename")
	}
}

func TestTestValid(t *testing.T) {
	fs := newFS(t)
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write([]byte("hello world"))
	w.Close()
	f, _ := fs.Create("hello.txt.gz")
	f.Write(buf.Bytes())
	f.Close()

	_, stderr, err := run(t, []string{"-t", "hello.txt.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stderr) > 0 {
		t.Errorf("unexpected stderr: %q", string(stderr))
	}
}

func TestTestInvalid(t *testing.T) {
	_, stderr, err := run(t, []string{"-t", "hello.txt"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for non-gzip file")
	}
	if len(stderr) == 0 {
		t.Error("expected error message on stderr")
	}
}

func TestVerbose(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"-c", "-v", "hello.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
	if len(stderr) == 0 {
		t.Error("expected verbose output on stderr")
	}
	if !bytes.Contains(stderr, []byte("%")) {
		t.Error("verbose output missing ratio")
	}
}

func TestCustomSuffix(t *testing.T) {
	fs := newFS(t)
	_, _, err := run(t, []string{"-S", ".custom", "hello.txt"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := fs.Stat("hello.txt.custom"); err != nil {
		t.Error("hello.txt.custom not created")
	}
	if _, err := fs.Stat("hello.txt.gz"); err == nil {
		t.Error("hello.txt.gz should not be created with custom suffix")
	}
}

func TestFastCompression(t *testing.T) {
	stdout, _, err := run(t, []string{"-1"}, []byte("hello"), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
}

func TestBestCompression(t *testing.T) {
	stdout, _, err := run(t, []string{"-9"}, []byte("hello"), newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
}

func TestEmptyInput(t *testing.T) {
	stdout, _, err := run(t, nil, []byte{}, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := gzlib.NewReader(bytes.NewReader(stdout))
	if err != nil {
		t.Fatalf("output is not valid gzip: %v", err)
	}
	r.Close()
}

func TestMissingFile(t *testing.T) {
	_, stderr, err := run(t, []string{"nope.txt"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !bytes.Contains(stderr, []byte("No such file or directory")) {
		t.Errorf("stderr = %q, want 'No such file or directory'", string(stderr))
	}
}

func TestHelp(t *testing.T) {
	_, stderr, err := run(t, []string{"--help"}, nil, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stderr, []byte("Usage: gzip")) {
		t.Error("usage missing from stderr")
	}
	if !bytes.Contains(stderr, []byte("--stdout")) {
		t.Error("--stdout flag missing from help")
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, err := run(t, []string{"--nope"}, nil, newFS(t))
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestDirectoryWithoutRecursive(t *testing.T) {
	fs := newFS(t)
	fs.MkdirAll("mydir", 0o755)
	_, stderr, err := run(t, []string{"mydir"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stderr, []byte("is a directory")) {
		t.Errorf("stderr = %q, want 'is a directory'", string(stderr))
	}
}

func TestAlreadyHasSuffix(t *testing.T) {
	fs := newFS(t)
	var buf bytes.Buffer
	w := gzlib.NewWriter(&buf)
	w.Write([]byte("hello"))
	w.Close()
	f, _ := fs.Create("file.gz")
	f.Write(buf.Bytes())
	f.Close()

	_, stderr, err := run(t, []string{"file.gz"}, nil, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stderr, []byte("already has .gz suffix")) {
		t.Errorf("stderr = %q, want 'already has .gz suffix'", string(stderr))
	}
}

// TestNoSourceRemovalOnShortWrite simulates the s3fs Write stub (returns 0,
// nil): the output bytes never reach storage, so gzip must surface an error
// and leave the source intact for both compress and decompress paths. The
// original bug deleted the source and persisted an empty file instead.
func TestNoSourceRemovalOnShortWrite(t *testing.T) {
	tests := []struct {
		name       string
		setupFS    func(t *testing.T) billy.Filesystem
		args       []string
		sourcePath string
		outputPath string
		wantStderr string
	}{
		{
			name:       "compress",
			setupFS:    func(t *testing.T) billy.Filesystem { return newFS(t) },
			args:       []string{"hello.txt"},
			sourcePath: "hello.txt",
			outputPath: "hello.txt.gz",
			wantStderr: "hello.txt",
		},
		{
			name: "decompress",
			setupFS: func(t *testing.T) billy.Filesystem {
				fs := memfs.New()
				var buf bytes.Buffer
				w := gzlib.NewWriter(&buf)
				w.Write([]byte("hello world"))
				w.Close()
				f, _ := fs.Create("hello.txt.gz")
				f.Write(buf.Bytes())
				f.Close()
				return fs
			},
			args:       []string{"-d", "hello.txt.gz"},
			sourcePath: "hello.txt.gz",
			outputPath: "hello.txt",
			wantStderr: "hello.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &silentDropFS{Filesystem: tt.setupFS(t)}
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
