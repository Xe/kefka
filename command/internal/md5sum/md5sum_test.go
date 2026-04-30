package md5sum

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
	write("world.txt", []byte("world"))
	// MD5("hello") = 5d41402abc4b2a76b9719d911017c592
	write("good.sums", []byte("5d41402abc4b2a76b9719d911017c592  hello.txt\n"))
	write("bad.sums", []byte("00000000000000000000000000000000  hello.txt\n"))
	write("mixed.sums", []byte(
		"5d41402abc4b2a76b9719d911017c592  hello.txt\n"+
			"00000000000000000000000000000000  world.txt\n"))
	write("binary.sums", []byte("5d41402abc4b2a76b9719d911017c592 *hello.txt\n"))
	write("missing-target.sums", []byte("5d41402abc4b2a76b9719d911017c592  nope.txt\n"))
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

func TestHash(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "stdin",
			args:       nil,
			stdin:      "hello",
			wantStdout: "5d41402abc4b2a76b9719d911017c592  -\n",
		},
		{
			name:       "stdin via dash",
			args:       []string{"-"},
			stdin:      "hello",
			wantStdout: "5d41402abc4b2a76b9719d911017c592  -\n",
		},
		{
			name:       "empty input",
			args:       nil,
			stdin:      "",
			wantStdout: "d41d8cd98f00b204e9800998ecf8427e  -\n",
		},
		{
			name:       "single file",
			args:       []string{"hello.txt"},
			wantStdout: "5d41402abc4b2a76b9719d911017c592  hello.txt\n",
		},
		{
			name: "multiple files",
			args: []string{"hello.txt", "world.txt"},
			wantStdout: "5d41402abc4b2a76b9719d911017c592  hello.txt\n" +
				"7d793037a0760186574b0282f2f435e7  world.txt\n",
		},
		{
			name:       "binary flag accepted",
			args:       []string{"-b", "hello.txt"},
			wantStdout: "5d41402abc4b2a76b9719d911017c592  hello.txt\n",
		},
		{
			name:       "text flag accepted",
			args:       []string{"--text", "hello.txt"},
			wantStdout: "5d41402abc4b2a76b9719d911017c592  hello.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args, tt.stdin, newFS(t))
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stderr != "" {
				t.Errorf("unexpected stderr: %q", stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}

func TestMissingFile(t *testing.T) {
	stdout, stderr, err := run(t, []string{"nope.txt"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "md5sum: nope.txt: No such file or directory") {
		t.Errorf("stderr = %q, want missing file message", stderr)
	}
}

func TestMissingFileContinuesOthers(t *testing.T) {
	stdout, stderr, err := run(t, []string{"nope.txt", "hello.txt"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error from missing file")
	}
	if !strings.Contains(stdout, "5d41402abc4b2a76b9719d911017c592  hello.txt") {
		t.Errorf("expected hello.txt hash in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "nope.txt: No such file or directory") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "all ok",
			args:       []string{"-c", "good.sums"},
			wantStdout: "hello.txt: OK\n",
		},
		{
			name: "all failed",
			args: []string{"--check", "bad.sums"},
			wantStdout: "hello.txt: FAILED\n" +
				"md5sum: WARNING: 1 computed checksum did NOT match\n",
			wantErr: true,
		},
		{
			name: "mixed",
			args: []string{"-c", "mixed.sums"},
			wantStdout: "hello.txt: OK\n" +
				"world.txt: FAILED\n" +
				"md5sum: WARNING: 1 computed checksum did NOT match\n",
			wantErr: true,
		},
		{
			name:       "binary marker accepted",
			args:       []string{"-c", "binary.sums"},
			wantStdout: "hello.txt: OK\n",
		},
		{
			name: "missing target reported as failed open or read",
			args: []string{"-c", "missing-target.sums"},
			wantStdout: "nope.txt: FAILED open or read\n" +
				"md5sum: WARNING: 1 computed checksum did NOT match\n",
			wantErr: true,
		},
		{
			name:       "check from stdin",
			args:       []string{"-c"},
			stdin:      "5d41402abc4b2a76b9719d911017c592  hello.txt\n",
			wantStdout: "hello.txt: OK\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := run(t, tt.args, tt.stdin, newFS(t))
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}

func TestCheckMultipleFailedPlural(t *testing.T) {
	stdout, _, err := run(t, []string{"-c"},
		"00000000000000000000000000000000  hello.txt\n"+
			"00000000000000000000000000000000  world.txt\n", newFS(t))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stdout, "2 computed checksums did NOT match") {
		t.Errorf("expected plural in warning, got %q", stdout)
	}
}

func TestCheckMissingChecksumFile(t *testing.T) {
	_, stderr, err := run(t, []string{"-c", "nope.sums"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "md5sum: nope.sums: No such file or directory") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestCheckUppercaseHash(t *testing.T) {
	stdout, _, err := run(t, []string{"-c"},
		"5D41402ABC4B2A76B9719D911017C592  hello.txt\n", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "hello.txt: OK\n" {
		t.Errorf("stdout = %q, want OK", stdout)
	}
}

func TestCheckSkipsNonMatchingLines(t *testing.T) {
	stdout, _, err := run(t, []string{"-c"},
		"# this is a comment\n"+
			"\n"+
			"5d41402abc4b2a76b9719d911017c592  hello.txt\n", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "hello.txt: OK\n" {
		t.Errorf("stdout = %q, want OK", stdout)
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("help should not write to stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: md5sum [OPTION]... [FILE]...") {
		t.Errorf("usage missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-c, --check") {
		t.Errorf("check flag missing from help: %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, err := run(t, []string{"--no-such-flag"}, "", newFS(t))
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
