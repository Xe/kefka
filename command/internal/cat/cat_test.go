package cat

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
	write("hello.txt", []byte("hello\n"))
	write("two.txt", []byte("world\n"))
	write("noeol.txt", []byte("noeol"))
	write("multi.txt", []byte("one\ntwo\nthree\n"))
	write("-file", []byte("dashfile\n"))
	if err := fs.MkdirAll("subdir", 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestCat(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "stdin no files",
			args:       nil,
			stdin:      "hello\n",
			wantStdout: "hello\n",
		},
		{
			name:       "stdin via dash",
			args:       []string{"-"},
			stdin:      "hello\n",
			wantStdout: "hello\n",
		},
		{
			name:       "single file",
			args:       []string{"hello.txt"},
			wantStdout: "hello\n",
		},
		{
			name:       "concatenate multiple files",
			args:       []string{"hello.txt", "two.txt"},
			wantStdout: "hello\nworld\n",
		},
		{
			name:       "file then dash",
			args:       []string{"hello.txt", "-"},
			stdin:      "from stdin\n",
			wantStdout: "hello\nfrom stdin\n",
		},
		{
			name:       "empty stdin",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "preserves missing trailing newline",
			args:       []string{"noeol.txt"},
			wantStdout: "noeol",
		},
		{
			name:       "number short flag single line",
			args:       []string{"-n", "hello.txt"},
			wantStdout: "     1\thello\n",
		},
		{
			name:       "number long flag multi line",
			args:       []string{"--number", "multi.txt"},
			wantStdout: "     1\tone\n     2\ttwo\n     3\tthree\n",
		},
		{
			name:       "number continues across files",
			args:       []string{"-n", "hello.txt", "two.txt"},
			wantStdout: "     1\thello\n     2\tworld\n",
		},
		{
			name:       "number from stdin",
			args:       []string{"-n"},
			stdin:      "a\nb\nc\n",
			wantStdout: "     1\ta\n     2\tb\n     3\tc\n",
		},
		{
			name:       "number with no trailing newline",
			args:       []string{"-n", "noeol.txt"},
			wantStdout: "     1\tnoeol",
		},
		{
			name:       "missing file reports error and continues",
			args:       []string{"nope.txt", "hello.txt"},
			wantStdout: "hello\n",
			wantErrSub: "cat: nope.txt:",
			wantErr:    true,
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "hello.txt"},
			wantStdout: "hello\n",
		},
		{
			name:       "double dash allows dash-prefixed filename",
			args:       []string{"--", "-file"},
			wantStdout: "dashfile\n",
		},
		{
			name:       "u flag accepted as no-op",
			args:       []string{"-u", "hello.txt"},
			wantStdout: "hello\n",
		},
		{
			name:       "two dashes consume stdin once",
			args:       []string{"-", "-"},
			stdin:      "foo",
			wantStdout: "foo",
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args, tt.stdin, newFS(t))
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
		})
	}
}

func TestCatDirectoryDiagnostic(t *testing.T) {
	stdout, stderr, err := run(t, []string{"subdir"}, "", newFS(t))
	if err == nil {
		t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.HasPrefix(stderr, "cat: subdir: ") {
		t.Errorf("stderr should start with %q, got %q", "cat: subdir: ", stderr)
	}
	if strings.Contains(stderr, "No such file or directory") {
		t.Errorf("stderr should not hardcode 'No such file or directory'; got %q", stderr)
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
	if !strings.Contains(stderr, "Usage: cat [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-n, --number") {
		t.Errorf("number flag missing from help: %q", stderr)
	}
}
