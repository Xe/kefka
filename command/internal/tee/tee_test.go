package tee

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("existing.txt", []byte("old contents\n"))
	return fs
}

func readFile(t *testing.T, fs billy.Filesystem, name string) string {
	t.Helper()
	f, err := fs.Open(name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
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

func TestTee(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		stdin       string
		wantStdout  string
		wantErrSub  string
		wantErr     bool
		wantFiles   map[string]string
		preexisting map[string]string
	}{
		{
			name:       "no files passes stdin to stdout",
			args:       nil,
			stdin:      "hello\n",
			wantStdout: "hello\n",
		},
		{
			name:       "single file gets stdin and stdout passes through",
			args:       []string{"out.txt"},
			stdin:      "hello\n",
			wantStdout: "hello\n",
			wantFiles:  map[string]string{"out.txt": "hello\n"},
		},
		{
			name:       "multiple files all receive stdin",
			args:       []string{"a.txt", "b.txt", "c.txt"},
			stdin:      "shared\n",
			wantStdout: "shared\n",
			wantFiles: map[string]string{
				"a.txt": "shared\n",
				"b.txt": "shared\n",
				"c.txt": "shared\n",
			},
		},
		{
			name:       "default mode overwrites existing file",
			args:       []string{"existing.txt"},
			stdin:      "new\n",
			wantStdout: "new\n",
			wantFiles:  map[string]string{"existing.txt": "new\n"},
		},
		{
			name:       "append short flag preserves existing content",
			args:       []string{"-a", "existing.txt"},
			stdin:      "added\n",
			wantStdout: "added\n",
			wantFiles:  map[string]string{"existing.txt": "old contents\nadded\n"},
		},
		{
			name:       "append long flag preserves existing content",
			args:       []string{"--append", "existing.txt"},
			stdin:      "added\n",
			wantStdout: "added\n",
			wantFiles:  map[string]string{"existing.txt": "old contents\nadded\n"},
		},
		{
			name:       "empty stdin writes empty file",
			args:       []string{"empty.txt"},
			stdin:      "",
			wantStdout: "",
			wantFiles:  map[string]string{"empty.txt": ""},
		},
		{
			name:       "binary safe stdin propagates verbatim",
			args:       []string{"bin.dat"},
			stdin:      "a\x00b\xffc",
			wantStdout: "a\x00b\xffc",
			wantFiles:  map[string]string{"bin.dat": "a\x00b\xffc"},
		},
		{
			name:       "ignore-interrupts short flag is accepted as no-op",
			args:       []string{"-i", "out.txt"},
			stdin:      "hello",
			wantStdout: "hello",
			wantFiles:  map[string]string{"out.txt": "hello"},
		},
		{
			name:       "ignore-interrupts long flag is accepted as no-op",
			args:       []string{"--ignore-interrupts", "out.txt"},
			stdin:      "hello",
			wantStdout: "hello",
			wantFiles:  map[string]string{"out.txt": "hello"},
		},
		{
			name:       "append plus ignore-interrupts combined short flags",
			args:       []string{"-ai", "existing.txt"},
			stdin:      "added\n",
			wantStdout: "added\n",
			wantFiles:  map[string]string{"existing.txt": "old contents\nadded\n"},
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			stdout, stderr, err := run(t, tt.args, tt.stdin, fs)
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
			for name, want := range tt.wantFiles {
				if got := readFile(t, fs, name); got != want {
					t.Errorf("file %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestNilFSWithFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader("hi\n"),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     nil,
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"out.txt"})
	if err == nil {
		t.Fatalf("expected error when FS is nil and files are given")
	}
	if stdout.String() != "hi\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hi\n")
	}
	if !strings.Contains(stderr.String(), "tee: out.txt: No such file or directory") {
		t.Errorf("stderr missing expected message: %q", stderr.String())
	}
}

// failingFS wraps a billy.Filesystem and forces OpenFile to fail with errFail
// for any path equal to failPath. Every other call delegates to the inner FS.
type failingFS struct {
	billy.Filesystem
	failPath string
	errFail  error
}

func (f *failingFS) OpenFile(name string, flag int, perm os.FileMode) (billy.File, error) {
	if name == f.failPath {
		return nil, &os.PathError{Op: "open", Path: name, Err: f.errFail}
	}
	return f.Filesystem.OpenFile(name, flag, perm)
}

func (f *failingFS) Create(name string) (billy.File, error) {
	if name == f.failPath {
		return nil, &os.PathError{Op: "create", Path: name, Err: f.errFail}
	}
	return f.Filesystem.Create(name)
}

func TestFailureOnOneFileKeepsWritingOthers(t *testing.T) {
	inner := memfs.New()
	fs := &failingFS{
		Filesystem: inner,
		failPath:   "bad.txt",
		errFail:    errors.New("Permission denied"),
	}

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader("payload\n"),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}

	err := Impl{}.Exec(context.Background(), ec, []string{"good1.txt", "bad.txt", "good2.txt"})
	if err == nil {
		t.Fatalf("expected non-nil error (exit status), got nil; stderr=%q", stderr.String())
	}

	if stdout.String() != "payload\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "payload\n")
	}
	if got := readFile(t, inner, "good1.txt"); got != "payload\n" {
		t.Errorf("good1.txt = %q, want %q (failure on bad.txt must not abort prior writes)", got, "payload\n")
	}
	if got := readFile(t, inner, "good2.txt"); got != "payload\n" {
		t.Errorf("good2.txt = %q, want %q (failure on bad.txt must not stop later writes)", got, "payload\n")
	}
	if !strings.Contains(stderr.String(), "tee: bad.txt: Permission denied") {
		t.Errorf("stderr missing per-file failure message: %q", stderr.String())
	}
	// The good files must not show up in stderr.
	if strings.Contains(stderr.String(), "good1.txt") || strings.Contains(stderr.String(), "good2.txt") {
		t.Errorf("stderr should only mention the failing file: %q", stderr.String())
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: tee [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--append") {
		t.Errorf("append flag missing from help: %q", stderr)
	}
}
