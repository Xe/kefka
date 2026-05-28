package paste

import (
	"bytes"
	"context"
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
	write("a.txt", []byte("a1\na2\na3\n"))
	write("b.txt", []byte("b1\nb2\n"))
	write("c.txt", []byte("c1\nc2\nc3\nc4\n"))
	write("d.txt", []byte("d1\nd2\n"))
	write("f1.txt", []byte("p\nq\nr\n"))
	write("f2.txt", []byte("s\nt\n"))
	write("f3.txt", []byte("u\nv\nw\n"))
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
		{
			name:       "literal tab via shell escape",
			args:       []string{"-d", "\t", "a.txt", "b.txt"},
			wantStdout: "a1\tb1\na2\tb2\na3\t\n",
		},
		{
			name:       "backslash-t escape parses as tab",
			args:       []string{"-d", `\t`, "a.txt", "b.txt"},
			wantStdout: "a1\tb1\na2\tb2\na3\t\n",
		},
		{
			name:       "backslash-n escape parses as newline",
			args:       []string{"-d", `\n`, "a.txt", "b.txt"},
			wantStdout: "a1\nb1\na2\nb2\na3\n\n",
		},
		{
			name:       "backslash-backslash escape parses as literal backslash",
			args:       []string{"-d", `\\`, "a.txt", "b.txt"},
			wantStdout: "a1\\b1\na2\\b2\na3\\\n",
		},
		{
			name:       "backslash-zero is empty separator slot",
			args:       []string{"-d", `\0X`, "a.txt", "b.txt", "c.txt", "d.txt"},
			wantStdout: "a1b1Xc1d1\na2b2Xc2d2\na3Xc3\nXc4\n",
		},
		{
			name:       "mixed backslash escapes cycle",
			args:       []string{"-d", `a\nb`, "f1.txt", "f2.txt", "f3.txt"},
			wantStdout: "pas\nu\nqat\nv\nra\nw\n",
		},
		{
			// Three files, cycle [a, empty, b]; each line uses two
			// delimiter slots: 'a' between col1/col2, '' (empty) between
			// col2/col3. Cycle resets per output line in non-serial mode.
			name:       "a-null-b cycle skips between cols 2 and 3",
			args:       []string{"-d", `a\0b`, "f1.txt", "f2.txt", "f3.txt"},
			wantStdout: "pasu\nqatv\nraw\n",
		},
		{
			// Cycle [a, empty, b] reaches the 'b' element when there are
			// four files: separators are a, empty, b between cols 1-2, 2-3,
			// 3-4 respectively.
			name:       "a-null-b cycle reaches third element with four files",
			args:       []string{"-d", `a\0b`, "f1.txt", "f2.txt", "f3.txt", "f1.txt"},
			wantStdout: "pasubp\nqatvbq\nrawbr\n",
		},
		{
			// POSIX: empty list is unspecified. GNU and our impl silently
			// accept it as "no separator", matching paste -d '\0'.
			name:       "empty delimiter list matches \\0 behavior",
			args:       []string{"-d", "", "a.txt", "b.txt", "c.txt"},
			wantStdout: "a1b1c1\na2b2c2\na3c3\nc4\n",
		},
		{
			name:       "mixed backslash escapes cycle reaches third element in serial",
			args:       []string{"-s", "-d", `a\nb`, "c.txt"},
			wantStdout: "c1ac2\nc3bc4\n",
		},
		{
			name:       "invalid backslash escape errors",
			args:       []string{"-d", `\q`, "a.txt", "b.txt"},
			wantErrSub: `'\q' is not a valid delimiter`,
			wantErr:    true,
		},
		{
			name:       "trailing backslash errors",
			args:       []string{"-d", `\`, "a.txt", "b.txt"},
			wantErrSub: "delimiter list ends with an unescaped backslash",
			wantErr:    true,
		},
		{
			name:       "serial single delimiter joins lines",
			args:       []string{"-s", "-d", "X", "a.txt"},
			wantStdout: "a1Xa2Xa3\n",
		},
		{
			name:       "serial cyclic delimiters reset between files",
			args:       []string{"-s", "-d", "XY", "f1.txt", "f2.txt"},
			wantStdout: "pXqYr\nsXt\n",
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
