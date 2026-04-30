package paste

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
	write("a.txt", []byte("a1\na2\na3\n"))
	write("b.txt", []byte("b1\nb2\n"))
	write("c.txt", []byte("c1\nc2\nc3\nc4\n"))
	write("empty.txt", []byte(""))
	write("nofinalnl.txt", []byte("x1\nx2"))
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

func TestPaste(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "parallel two files pads shorter",
			args:       []string{"a.txt", "b.txt"},
			wantStdout: "a1\tb1\na2\tb2\na3\t\n",
		},
		{
			name:       "parallel three files",
			args:       []string{"a.txt", "b.txt", "c.txt"},
			wantStdout: "a1\tb1\tc1\na2\tb2\tc2\na3\t\tc3\n\t\tc4\n",
		},
		{
			name:       "comma delimiter",
			args:       []string{"-d", ",", "a.txt", "b.txt"},
			wantStdout: "a1,b1\na2,b2\na3,\n",
		},
		{
			name:       "long delimiter flag",
			args:       []string{"--delimiters=,", "a.txt", "b.txt"},
			wantStdout: "a1,b1\na2,b2\na3,\n",
		},
		{
			name:       "cyclic delimiters with three files",
			args:       []string{"-d", ",;", "a.txt", "b.txt", "c.txt"},
			wantStdout: "a1,b1;c1\na2,b2;c2\na3,;c3\n,;c4\n",
		},
		{
			name:       "serial mode joins file lines",
			args:       []string{"-s", "a.txt", "b.txt"},
			wantStdout: "a1\ta2\ta3\nb1\tb2\n",
		},
		{
			name:       "serial with comma delimiter",
			args:       []string{"-s", "-d", ",", "a.txt"},
			wantStdout: "a1,a2,a3\n",
		},
		{
			name:       "single file parallel echoes lines",
			args:       []string{"a.txt"},
			wantStdout: "a1\na2\na3\n",
		},
		{
			name:       "stdin via dash",
			args:       []string{"-"},
			stdin:      "x1\nx2\n",
			wantStdout: "x1\nx2\n",
		},
		{
			name:       "two dashes distribute stdin lines",
			args:       []string{"-", "-"},
			stdin:      "a\nb\nc\nd\n",
			wantStdout: "a\tb\nc\td\n",
		},
		{
			name:       "file mixed with dash",
			args:       []string{"a.txt", "-"},
			stdin:      "y1\ny2\n",
			wantStdout: "a1\ty1\na2\ty2\na3\t\n",
		},
		{
			name:       "no trailing newline still gets one",
			args:       []string{"nofinalnl.txt"},
			wantStdout: "x1\nx2\n",
		},
		{
			name:       "empty file parallel produces nothing",
			args:       []string{"empty.txt"},
			wantStdout: "",
		},
		{
			name:       "empty file serial produces single newline",
			args:       []string{"-s", "empty.txt"},
			wantStdout: "\n",
		},
		{
			name:       "empty delimiter concatenates",
			args:       []string{"-d", "", "a.txt", "b.txt"},
			wantStdout: "a1b1\na2b2\na3\n",
		},
		{
			name:       "no args is usage error",
			args:       nil,
			wantStdout: "",
			wantErrSub: "usage: paste",
			wantErr:    true,
		},
		{
			name:       "missing file errors",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag errors",
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
	if !strings.Contains(stderr, "Usage: paste [OPTION]... [FILE]...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--delimiters=LIST") {
		t.Errorf("delimiters flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "--serial") {
		t.Errorf("serial flag missing from help: %q", stderr)
	}
}
