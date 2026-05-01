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
	write("dup.bin", []byte(strings.Repeat("a", 34)))
	write("twelve.bin", []byte("ABCDEFGHIJKL"))
	write("zeros.bin", make([]byte, 64))
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
			wantStdout: "0000000 064510\n0000002\n",
		},
		{
			name:       "default octal from file",
			args:       []string{"hi.txt"},
			wantStdout: "0000000 064510\n0000002\n",
		},
		{
			name:       "dash means stdin",
			args:       []string{"-"},
			stdin:      "Hi",
			wantStdout: "0000000 064510\n0000002\n",
		},
		{
			name:       "empty input prints terminating address",
			args:       nil,
			stdin:      "",
			wantStdout: "0000000\n",
		},
		{
			name:       "char format printable",
			args:       []string{"-c"},
			stdin:      "ABC",
			wantStdout: "0000000   A   B   C\n0000003\n",
		},
		{
			name:       "char format named escape",
			args:       []string{"-c"},
			stdin:      "A\nB",
			wantStdout: "0000000   A  \\n   B\n0000003\n",
		},
		{
			name:       "char format octal fallback for non-printable",
			args:       []string{"-c", "ab.bin"},
			wantStdout: "0000000 001   A 177 377\n0000004\n",
		},
		{
			name:       "hex via -t x1",
			args:       []string{"-t", "x1"},
			stdin:      "Hi",
			wantStdout: "0000000 48 69\n0000002\n",
		},
		{
			name:       "octal via -t o1",
			args:       []string{"-t", "o1"},
			stdin:      "Hi",
			wantStdout: "0000000 110 151\n0000002\n",
		},
		{
			name:       "octal via -t o (default size 4)",
			args:       []string{"-t", "o"},
			stdin:      "Hi",
			wantStdout: "0000000 00000064510\n0000002\n",
		},
		{
			name:       "char via -t c",
			args:       []string{"-t", "c"},
			stdin:      "AB",
			wantStdout: "0000000   A   B\n0000002\n",
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
			wantStdout: "0000000   A\n         41\n0000001\n",
		},
		{
			name:       "format order is preserved",
			args:       []string{"-t", "x1", "-c"},
			stdin:      "A",
			wantStdout: "0000000  41\n          A\n0000001\n",
		},
		{
			name:       "wraps after 16 bytes per line",
			args:       []string{"-t", "x1"},
			stdin:      "0123456789abcdefXY",
			wantStdout: "0000000 30 31 32 33 34 35 36 37 38 39 61 62 63 64 65 66\n0000020 58 59\n0000022\n",
		},
		{
			name:       "signed decimal -t d2 little-endian",
			args:       []string{"-t", "d2", "-An"},
			stdin:      "AB",
			wantStdout: "  16961\n",
		},
		{
			name:       "unsigned decimal -t u1",
			args:       []string{"-An", "-t", "u1"},
			stdin:      "AB",
			wantStdout: "  65  66\n",
		},
		{
			name:       "named character -t a",
			args:       []string{"-An", "-t", "a"},
			stdin:      "A\nB",
			wantStdout: "   A  nl   B\n",
		},
		{
			name:       "concatenated type chars -t x1c",
			args:       []string{"-An", "-t", "x1c"},
			stdin:      "AB",
			wantStdout: "  41  42\n   A   B\n",
		},
		{
			name:       "duplicate blocks compressed to *",
			args:       []string{"dup.bin"},
			wantStdout: "0000000 060541 060541 060541 060541 060541 060541 060541 060541\n*\n0000040 060541\n0000042\n",
		},
		{
			name:       "verbose -v emits all blocks",
			args:       []string{"-v", "dup.bin"},
			wantStdout: "0000000 060541 060541 060541 060541 060541 060541 060541 060541\n0000020 060541 060541 060541 060541 060541 060541 060541 060541\n0000040 060541\n0000042\n",
		},
		{
			name:       "skip and limit -j 4 -N 4",
			args:       []string{"-j", "4", "-N", "4", "-An", "-t", "x1", "twelve.bin"},
			wantStdout: " 45 46 47 48\n",
		},
		{
			name:       "skip increments address",
			args:       []string{"-j", "4", "-t", "x1", "twelve.bin"},
			wantStdout: "0000004 45 46 47 48 49 4a 4b 4c\n0000014\n",
		},
		{
			name:       "skip past end is an error",
			args:       []string{"-j", "100", "twelve.bin"},
			wantErr:    true,
			wantErrSub: "od: cannot skip past end of combined input",
		},
		{
			name:       "decimal address with -A d",
			args:       []string{"-A", "d", "-t", "x1"},
			stdin:      "AB",
			wantStdout: "0000000 41 42\n0000002\n",
		},
		{
			name:       "hex address with -A x",
			args:       []string{"-A", "x", "-t", "x1"},
			stdin:      "AB",
			wantStdout: "000000 41 42\n000002\n",
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
		{
			name:       "invalid -t type string is an error",
			args:       []string{"-t", "z"},
			stdin:      "Hi",
			wantErr:    true,
			wantErrSub: "od: invalid type string 'z'",
		},
		{
			name:       "invalid -A radix is an error",
			args:       []string{"-A", "q"},
			stdin:      "Hi",
			wantErr:    true,
			wantErrSub: "od: invalid output address radix 'q'",
		},
		// XSI shorthand options.
		{
			name:       "-b is same as -t o1",
			args:       []string{"-An", "-b"},
			stdin:      "AB",
			wantStdout: " 101 102\n",
		},
		{
			name:       "-d is same as -t u2",
			args:       []string{"-An", "-d"},
			stdin:      "AB",
			wantStdout: " 16961\n",
		},
		{
			name:       "-o is same as -t o2",
			args:       []string{"-An", "-o"},
			stdin:      "AB",
			wantStdout: " 041101\n",
		},
		{
			name:       "-s is same as -t d2",
			args:       []string{"-An", "-s"},
			stdin:      "AB",
			wantStdout: "  16961\n",
		},
		{
			name:       "-x is same as -t x2",
			args:       []string{"-An", "-x"},
			stdin:      "AB",
			wantStdout: " 4241\n",
		},
		{
			name:       "XSI shorthands stack in order",
			args:       []string{"-An", "-x", "-c"},
			stdin:      "AB",
			wantStdout: "    4241\n   A   B\n",
		},
		// Extended -t grammar.
		{
			name:       "-t d4 reads 4-byte signed decimal",
			args:       []string{"-An", "-t", "d4"},
			stdin:      "ABCD",
			wantStdout: "  1145258561\n",
		},
		{
			name:       "-t u4 reads 4-byte unsigned decimal",
			args:       []string{"-An", "-t", "u4"},
			stdin:      "ABCD",
			wantStdout: " 1145258561\n",
		},
		{
			name:       "-t x2 reads 2-byte hex little-endian",
			args:       []string{"-An", "-t", "x2"},
			stdin:      "AB",
			wantStdout: " 4241\n",
		},
		{
			name:       "-t o2 reads 2-byte octal little-endian",
			args:       []string{"-An", "-t", "o2"},
			stdin:      "AB",
			wantStdout: " 041101\n",
		},
		{
			name:       "-t dC alias for -t d1",
			args:       []string{"-An", "-t", "dC"},
			stdin:      "AB",
			wantStdout: "   65   66\n",
		},
		{
			name:       "-t dS alias for -t d2",
			args:       []string{"-An", "-t", "dS"},
			stdin:      "AB",
			wantStdout: "  16961\n",
		},
		{
			name:       "-t dI alias for -t d4",
			args:       []string{"-An", "-t", "dI"},
			stdin:      "ABCD",
			wantStdout: "  1145258561\n",
		},
		{
			name:       "-t comma list",
			args:       []string{"-An", "-t", "x1,c"},
			stdin:      "AB",
			wantStdout: "  41  42\n   A   B\n",
		},
		{
			name:       "-t multiple invocations preserved",
			args:       []string{"-An", "-t", "x1", "-t", "c"},
			stdin:      "AB",
			wantStdout: "  41  42\n   A   B\n",
		},
		{
			name:       "-t f4 emits float",
			args:       []string{"-An", "-t", "f4"},
			stdin:      "\x00\x00\x80\x3f", // 1.0 little-endian
			wantStdout: "               1\n",
		},
		{
			name:       "-t fF alias for -t f4",
			args:       []string{"-An", "-t", "fF"},
			stdin:      "\x00\x00\x80\x3f",
			wantStdout: "               1\n",
		},
		{
			name:       "-t fD alias for -t f8",
			args:       []string{"-An", "-t", "fD"},
			stdin:      "\x00\x00\x00\x00\x00\x00\xf0\x3f", // 1.0 double LE
			wantStdout: "                        1\n",
		},
		// -j byte-count parsing variants.
		{
			name:       "-j with octal prefix",
			args:       []string{"-j", "04", "-N", "4", "-An", "-t", "x1", "twelve.bin"},
			wantStdout: " 45 46 47 48\n",
		},
		{
			name:       "-j with hex prefix",
			args:       []string{"-j", "0x4", "-N", "4", "-An", "-t", "x1", "twelve.bin"},
			wantStdout: " 45 46 47 48\n",
		},
		{
			name:       "-N with octal",
			args:       []string{"-N", "04", "-An", "-t", "x1", "twelve.bin"},
			wantStdout: " 41 42 43 44\n",
		},
		// * collapsing edge cases.
		{
			name:       "all zeros collapse",
			args:       []string{"-An", "-t", "x1", "zeros.bin"},
			wantStdout: " 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n*\n 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n",
		},
		{
			name:       "verbose disables collapse for zeros",
			args:       []string{"-v", "-An", "-t", "x1", "zeros.bin"},
			wantStdout: " 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00\n",
		},
		// -w sets width.
		{
			name:       "-w 8 limits to 8 bytes per line",
			args:       []string{"-w", "8", "-An", "-t", "x1"},
			stdin:      "0123456789ab",
			wantStdout: " 30 31 32 33 34 35 36 37\n 38 39 61 62\n",
		},
		// -A modes.
		{
			name:       "-A o is the default address radix",
			args:       []string{"-A", "o", "-t", "x1"},
			stdin:      "AB",
			wantStdout: "0000000 41 42\n0000002\n",
		},
		// Type-string error cases.
		{
			name:       "invalid -t size errors",
			args:       []string{"-t", "d3"},
			stdin:      "ABCD",
			wantErr:    true,
			wantErrSub: "od: invalid type string 'd3'",
		},
		{
			name:       "unknown -t character errors",
			args:       []string{"-t", "z"},
			stdin:      "Hi",
			wantErr:    true,
			wantErrSub: "od: invalid type string 'z'",
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
	if !strings.Contains(stderr, "-v, --output-duplicates") {
		t.Errorf("verbose flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-j, --skip-bytes") {
		t.Errorf("skip-bytes flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-N, --read-bytes") {
		t.Errorf("read-bytes flag missing from help: %q", stderr)
	}
}
