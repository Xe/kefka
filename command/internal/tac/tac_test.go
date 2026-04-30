package tac

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
	write("hello.txt", []byte("alpha\nbravo\ncharlie\n"))
	write("nofinalnl.txt", []byte("foo\nbar"))
	write("blanks.txt", []byte("a\n\nb\n"))
	write("single.txt", []byte("only\n"))
	write("empty.txt", []byte(""))
	write("justnewline.txt", []byte("\n"))
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

func TestTac(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "reverses lines from stdin",
			args:       nil,
			stdin:      "alpha\nbravo\ncharlie\n",
			wantStdout: "charlie\nbravo\nalpha\n",
		},
		{
			name:       "reverses lines from file",
			args:       []string{"hello.txt"},
			wantStdout: "charlie\nbravo\nalpha\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "one\ntwo\n",
			wantStdout: "two\none\n",
		},
		{
			name:       "no trailing newline still reverses",
			args:       []string{"nofinalnl.txt"},
			wantStdout: "bar\nfoo\n",
		},
		{
			name:       "preserves blank lines",
			args:       []string{"blanks.txt"},
			wantStdout: "b\n\na\n",
		},
		{
			name:       "single line file",
			args:       []string{"single.txt"},
			wantStdout: "only\n",
		},
		{
			name:       "empty file produces no output",
			args:       []string{"empty.txt"},
			wantStdout: "",
		},
		{
			name:       "empty stdin produces no output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "lone newline",
			args:       []string{"justnewline.txt"},
			wantStdout: "\n",
		},
		{
			name:       "stdin without trailing newline",
			args:       nil,
			stdin:      "x\ny",
			wantStdout: "y\nx\n",
		},
		{
			name:       "round trip via stdin",
			args:       nil,
			stdin:      "charlie\nbravo\nalpha\n",
			wantStdout: "alpha\nbravo\ncharlie\n",
		},
		{
			name:       "missing file",
			args:       []string{"nope.txt"},
			wantErrSub: "tac: nope.txt: No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
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

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, "", newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: tac [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
}
