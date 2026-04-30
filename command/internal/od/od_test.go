package od

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
	write("hi.txt", []byte("Hi"))
	write("ab.bin", []byte{0x01, 0x41, 0x7f, 0xff})
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

func TestOd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default octal from stdin",
			args:       nil,
			stdin:      "Hi",
			wantStdout: "0000000  110 151\n0000002\n",
		},
		{
			name:       "default octal from file",
			args:       []string{"hi.txt"},
			wantStdout: "0000000  110 151\n0000002\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "Hi",
			wantStdout: "0000000  110 151\n0000002\n",
		},
		{
			name:       "empty input produces no output",
			args:       nil,
			stdin:      "",
			wantStdout: "",
		},
		{
			name:       "char format printable",
			args:       []string{"-c"},
			stdin:      "ABC",
			wantStdout: "0000000    A   B   C\n0000003\n",
		},
		{
			name:       "char format named escape",
			args:       []string{"-c"},
			stdin:      "A\nB",
			wantStdout: "0000000    A  \\n   B\n0000003\n",
		},
		{
			name:       "char format octal fallback for non-printable",
			args:       []string{"-c", "ab.bin"},
			wantStdout: "0000000  001   A 177 377\n0000004\n",
		},
		{
			name:       "hex via -t x1",
			args:       []string{"-t", "x1"},
			stdin:      "Hi",
			wantStdout: "0000000  48 69\n0000002\n",
		},
		{
			name:       "octal via -t o",
			args:       []string{"-t", "o"},
			stdin:      "Hi",
			wantStdout: "0000000  110 151\n0000002\n",
		},
		{
			name:       "char via -t c",
			args:       []string{"-t", "c"},
			stdin:      "AB",
			wantStdout: "0000000    A   B\n0000002\n",
		},
		{
			name:       "no address with -An",
			args:       []string{"-An", "-c"},
			stdin:      "AB",
			wantStdout: "   A   B\n",
		},
		{
			name:       "no address with separated -A n",
			args:       []string{"-A", "n", "-c"},
			stdin:      "AB",
			wantStdout: "   A   B\n",
		},
		{
			name:       "char and hex together widens hex field",
			args:       []string{"-c", "-t", "x1"},
			stdin:      "A",
			wantStdout: "0000000    A\n          41\n0000001\n",
		},
		{
			name:       "format order is preserved",
			args:       []string{"-t", "x1", "-c"},
			stdin:      "A",
			wantStdout: "0000000   41\n           A\n0000001\n",
		},
		{
			name:       "wraps after 16 bytes per line",
			args:       []string{"-t", "x1"},
			stdin:      "0123456789abcdefXY",
			wantStdout: "0000000  30 31 32 33 34 35 36 37 38 39 61 62 63 64 65 66\n0000020  58 59\n0000022\n",
		},
		{
			name:       "unrecognized -t format is silently ignored",
			args:       []string{"-t", "d"},
			stdin:      "Hi",
			wantStdout: "0000000  110 151\n0000002\n",
		},
		{
			name:       "missing file reports error",
			args:       []string{"nope.bin"},
			wantStdout: "",
			wantErrSub: "od: nope.bin: No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag returns error",
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
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, tt.wantStdout)
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
	if !strings.Contains(stderr, "Usage: od [OPTION]... [FILE]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-A, --address-radix") {
		t.Errorf("address-radix flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-t, --format") {
		t.Errorf("format flag missing from help: %q", stderr)
	}
}
