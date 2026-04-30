package split

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
	write("five-lines.txt", []byte("a\nb\nc\nd\ne\n"))
	write("no-trailing.txt", []byte("a\nb\nc"))
	write("ten-bytes.bin", []byte("0123456789"))
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

func fileExists(fs billy.Filesystem, name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}

func TestSplit_lines(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantFiles map[string]string
	}{
		{
			name:  "default 1000 lines from stdin",
			args:  nil,
			stdin: "a\nb\nc\n",
			wantFiles: map[string]string{
				"xaa": "a\nb\nc\n",
			},
		},
		{
			name:  "split by 2 lines",
			args:  []string{"-l", "2"},
			stdin: "a\nb\nc\nd\ne\n",
			wantFiles: map[string]string{
				"xaa": "a\nb\n",
				"xab": "c\nd\n",
				"xac": "e\n",
			},
		},
		{
			name: "split by 2 lines from file",
			args: []string{"-l", "2", "five-lines.txt"},
			wantFiles: map[string]string{
				"xaa": "a\nb\n",
				"xab": "c\nd\n",
				"xac": "e\n",
			},
		},
		{
			name: "no trailing newline preserved on last chunk",
			args: []string{"-l", "2", "no-trailing.txt"},
			wantFiles: map[string]string{
				"xaa": "a\nb\n",
				"xab": "c",
			},
		},
		{
			name: "combined short form -l2",
			args: []string{"-l2", "five-lines.txt"},
			wantFiles: map[string]string{
				"xaa": "a\nb\n",
				"xab": "c\nd\n",
				"xac": "e\n",
			},
		},
		{
			name: "dash means stdin",
			args: []string{"-l", "1", "-"},
			stdin: "x\ny\n",
			wantFiles: map[string]string{
				"xaa": "x\n",
				"xab": "y\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			_, stderr, err := run(t, tt.args, tt.stdin, fs)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			for name, want := range tt.wantFiles {
				got := readFile(t, fs, name)
				if got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestSplit_bytes(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantFiles map[string]string
	}{
		{
			name:  "split by 4 bytes from stdin",
			args:  []string{"-b", "4"},
			stdin: "0123456789",
			wantFiles: map[string]string{
				"xaa": "0123",
				"xab": "4567",
				"xac": "89",
			},
		},
		{
			name: "split by bytes from file",
			args: []string{"-b", "3", "ten-bytes.bin"},
			wantFiles: map[string]string{
				"xaa": "012",
				"xab": "345",
				"xac": "678",
				"xad": "9",
			},
		},
		{
			name:  "K suffix",
			args:  []string{"-b", "1K"},
			stdin: strings.Repeat("a", 2048),
			wantFiles: map[string]string{
				"xaa": strings.Repeat("a", 1024),
				"xab": strings.Repeat("a", 1024),
			},
		},
		{
			name:  "combined short form -b4",
			args:  []string{"-b4"},
			stdin: "abcdefgh",
			wantFiles: map[string]string{
				"xaa": "abcd",
				"xab": "efgh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			_, stderr, err := run(t, tt.args, tt.stdin, fs)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			for name, want := range tt.wantFiles {
				got := readFile(t, fs, name)
				if got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestSplit_chunks(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-n", "5", "ten-bytes.bin"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := map[string]string{
		"xaa": "01",
		"xab": "23",
		"xac": "45",
		"xad": "67",
		"xae": "89",
	}
	for name, expected := range want {
		got := readFile(t, fs, name)
		if got != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestSplit_chunksUneven(t *testing.T) {
	// 10 bytes / 3 chunks → ceil = 4 bytes per chunk: "0123", "4567", "89"
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-n", "3", "ten-bytes.bin"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := map[string]string{
		"xaa": "0123",
		"xab": "4567",
		"xac": "89",
	}
	for name, expected := range want {
		got := readFile(t, fs, name)
		if got != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestSplit_numericSuffix(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-d", "-l", "1", "five-lines.txt"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := map[string]string{
		"x00": "a\n",
		"x01": "b\n",
		"x02": "c\n",
		"x03": "d\n",
		"x04": "e\n",
	}
	for name, expected := range want {
		got := readFile(t, fs, name)
		if got != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestSplit_numericSuffixLong(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"--numeric-suffixes", "-l", "1", "five-lines.txt"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if !fileExists(fs, "x00") {
		t.Errorf("expected x00 to exist")
	}
}

func TestSplit_suffixLength(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-a", "3", "-d", "-l", "1", "five-lines.txt"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if !fileExists(fs, "x000") {
		t.Errorf("expected x000 to exist")
	}
	if !fileExists(fs, "x004") {
		t.Errorf("expected x004 to exist")
	}
	if fileExists(fs, "x00") {
		t.Errorf("x00 should not exist with -a 3")
	}
}

func TestSplit_additionalSuffix(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"--additional-suffix=.txt", "-l", "2", "five-lines.txt"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	want := map[string]string{
		"xaa.txt": "a\nb\n",
		"xab.txt": "c\nd\n",
		"xac.txt": "e\n",
	}
	for name, expected := range want {
		got := readFile(t, fs, name)
		if got != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestSplit_customPrefix(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-l", "2", "five-lines.txt", "part_"}, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if !fileExists(fs, "part_aa") {
		t.Errorf("expected part_aa to exist")
	}
	if !fileExists(fs, "part_ac") {
		t.Errorf("expected part_ac to exist")
	}
}

func TestSplit_emptyInput(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, nil, "", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if fileExists(fs, "xaa") {
		t.Errorf("no files should be written for empty input")
	}
}

func TestSplit_errors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantErr    bool
		wantErrSub string
	}{
		{
			name:       "missing file",
			args:       []string{"nope.txt"},
			wantErr:    true,
			wantErrSub: "split: nope.txt: No such file or directory",
		},
		{
			name:       "invalid lines value",
			args:       []string{"-l", "abc"},
			stdin:      "x",
			wantErr:    true,
			wantErrSub: "split: invalid number of lines: 'abc'",
		},
		{
			name:       "zero lines value",
			args:       []string{"-l", "0"},
			stdin:      "x",
			wantErr:    true,
			wantErrSub: "split: invalid number of lines: '0'",
		},
		{
			name:       "invalid bytes value",
			args:       []string{"-b", "1Z"},
			stdin:      "x",
			wantErr:    true,
			wantErrSub: "split: invalid number of bytes: '1Z'",
		},
		{
			name:       "invalid chunks value",
			args:       []string{"-n", "0"},
			stdin:      "x",
			wantErr:    true,
			wantErrSub: "split: invalid number of chunks: '0'",
		},
		{
			name:       "invalid suffix length",
			args:       []string{"-a", "abc"},
			stdin:      "x",
			wantErr:    true,
			wantErrSub: "split: invalid suffix length: 'abc'",
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, err := run(t, tt.args, tt.stdin, newFS(t))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stderr=%q", stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

func TestSplit_modeLastFlagWins(t *testing.T) {
	// -l 2 then -b 4: bytes mode wins
	fs := newFS(t)
	_, stderr, err := run(t, []string{"-l", "2", "-b", "4"}, "abcdefgh", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if got := readFile(t, fs, "xaa"); got != "abcd" {
		t.Errorf("xaa = %q, want %q", got, "abcd")
	}
	if got := readFile(t, fs, "xab"); got != "efgh" {
		t.Errorf("xab = %q, want %q", got, "efgh")
	}
}

func TestSplit_help(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: split [OPTION]... [FILE [PREFIX]]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-l N") {
		t.Errorf("-l flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "--additional-suffix") {
		t.Errorf("--additional-suffix flag missing from help: %q", stderr)
	}
}
