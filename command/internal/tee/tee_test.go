package tee

import (
	"bytes"
	"context"
	"io"
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
