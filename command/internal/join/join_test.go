package join

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
	write("file1.txt", []byte("1 one\n2 two\n3 three\n"))
	write("file2.txt", []byte("1 alpha\n2 beta\n4 delta\n"))
	write("names.txt", []byte("Alice 30\nBob 25\nCharlie 35\n"))
	write("ages.txt", []byte("30 NY\n25 LA\n40 SF\n"))
	write("csv1.txt", []byte("a,1\nb,2\nc,3\n"))
	write("csv2.txt", []byte("a,x\nb,y\nd,z\n"))
	write("case1.txt", []byte("Hello world\nGoodbye moon\n"))
	write("case2.txt", []byte("hello earth\ngoodbye sun\n"))
	write("multi1.txt", []byte("key a\nkey b\nother c\n"))
	write("multi2.txt", []byte("key x\nkey y\n"))
	write("empty.txt", []byte(""))
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

func TestJoin(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "inner join on first field",
			args:       []string{"file1.txt", "file2.txt"},
			wantStdout: "1 one alpha\n2 two beta\n",
		},
		{
			name:       "custom separator",
			args:       []string{"-t", ",", "csv1.txt", "csv2.txt"},
			wantStdout: "a,1,x\nb,2,y\n",
		},
		{
			name:       "long form field-separator",
			args:       []string{"--field-separator", ",", "csv1.txt", "csv2.txt"},
			wantStdout: "a,1,x\nb,2,y\n",
		},
		{
			name:       "different field numbers",
			args:       []string{"-1", "2", "-2", "1", "names.txt", "ages.txt"},
			wantStdout: "30 Alice NY\n25 Bob LA\n",
		},
		{
			name:       "left outer join",
			args:       []string{"-a", "1", "file1.txt", "file2.txt"},
			wantStdout: "1 one alpha\n2 two beta\n3 three\n",
		},
		{
			name:       "right outer join",
			args:       []string{"-a", "2", "file1.txt", "file2.txt"},
			wantStdout: "1 one alpha\n2 two beta\n4 delta\n",
		},
		{
			name:       "unpairable from file 1 only",
			args:       []string{"-v", "1", "file1.txt", "file2.txt"},
			wantStdout: "3 three\n",
		},
		{
			name:       "unpairable from file 2 only",
			args:       []string{"-v", "2", "file1.txt", "file2.txt"},
			wantStdout: "4 delta\n",
		},
		{
			name:       "case-insensitive join",
			args:       []string{"-i", "case1.txt", "case2.txt"},
			wantStdout: "hello world earth\ngoodbye moon sun\n",
		},
		{
			name:       "custom output format",
			args:       []string{"-o", "1.1,1.2,2.2", "file1.txt", "file2.txt"},
			wantStdout: "1 one alpha\n2 two beta\n",
		},
		{
			name:       "empty string replacement with format",
			args:       []string{"-a", "1", "-e", "N/A", "-o", "1.1,1.2,2.2", "file1.txt", "file2.txt"},
			wantStdout: "1 one alpha\n2 two beta\n3 three N/A\n",
		},
		{
			name:       "multiple matches for same key",
			args:       []string{"multi1.txt", "multi2.txt"},
			wantStdout: "key a x\nkey a y\nkey b x\nkey b y\n",
		},
		{
			name:       "empty input file produces no output",
			args:       []string{"empty.txt", "file2.txt"},
			wantStdout: "",
		},
		{
			name:       "no matching keys produces no output",
			args:       []string{"csv1.txt", "ages.txt"},
			wantStdout: "",
		},
		{
			name:       "stdin as first file",
			args:       []string{"-", "file2.txt"},
			stdin:      "1 one\n2 two\n",
			wantStdout: "1 one alpha\n2 two beta\n",
		},
		{
			name:       "missing operand",
			args:       []string{"file1.txt"},
			wantErrSub: "missing file operand",
			wantErr:    true,
		},
		{
			name:       "extra operand",
			args:       []string{"file1.txt", "file2.txt", "file3.txt"},
			wantErrSub: "extra operand",
			wantErr:    true,
		},
		{
			name:       "missing file",
			args:       []string{"nope.txt", "file2.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope", "file1.txt", "file2.txt"},
			wantErr: true,
		},
		{
			name:       "invalid field number for -1",
			args:       []string{"-1", "0", "file1.txt", "file2.txt"},
			wantErrSub: "invalid field number",
			wantErr:    true,
		},
		{
			name:       "invalid file number for -a",
			args:       []string{"-a", "3", "file1.txt", "file2.txt"},
			wantErrSub: "invalid file number",
			wantErr:    true,
		},
		{
			name:       "invalid output format",
			args:       []string{"-o", "bad", "file1.txt", "file2.txt"},
			wantErrSub: "invalid field spec",
			wantErr:    true,
		},
		{
			name:       "output format with wrong file number",
			args:       []string{"-o", "3.1", "file1.txt", "file2.txt"},
			wantErrSub: "invalid field spec",
			wantErr:    true,
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
	if !strings.Contains(stderr, "Usage: join [OPTION]... FILE1 FILE2") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-t CHAR") {
		t.Errorf("separator flag missing from help: %q", stderr)
	}
}
