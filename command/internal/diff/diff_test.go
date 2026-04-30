package diff

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
	write("a.txt", []byte("alpha\nbeta\ngamma\n"))
	write("b.txt", []byte("alpha\nBETA\ngamma\n"))
	write("a_copy.txt", []byte("alpha\nbeta\ngamma\n"))
	write("upper.txt", []byte("ALPHA\nBETA\nGAMMA\n"))
	write("lower.txt", []byte("alpha\nbeta\ngamma\n"))
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

func TestDiff(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "identical files no output",
			args:       []string{"a.txt", "a_copy.txt"},
			wantStdout: "",
			wantStderr: "",
		},
		{
			name:       "identical files with -s reports identical",
			args:       []string{"-s", "a.txt", "a_copy.txt"},
			wantStdout: "Files a.txt and a_copy.txt are identical\n",
		},
		{
			name:       "long form report-identical-files",
			args:       []string{"--report-identical-files", "a.txt", "a_copy.txt"},
			wantStdout: "Files a.txt and a_copy.txt are identical\n",
		},
		{
			name:       "different files brief mode",
			args:       []string{"-q", "a.txt", "b.txt"},
			wantStdout: "Files a.txt and b.txt differ\n",
			wantErr:    true,
		},
		{
			name:       "long form brief",
			args:       []string{"--brief", "a.txt", "b.txt"},
			wantStdout: "Files a.txt and b.txt differ\n",
			wantErr:    true,
		},
		{
			name: "different files unified default",
			args: []string{"a.txt", "b.txt"},
			wantStdout: "--- a.txt\n" +
				"+++ b.txt\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name: "explicit -u flag matches default",
			args: []string{"-u", "a.txt", "b.txt"},
			wantStdout: "--- a.txt\n" +
				"+++ b.txt\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name:       "ignore case treats differently-cased files as identical",
			args:       []string{"-i", "lower.txt", "upper.txt"},
			wantStdout: "",
		},
		{
			name:       "ignore case with -s reports identical",
			args:       []string{"-i", "-s", "lower.txt", "upper.txt"},
			wantStdout: "Files lower.txt and upper.txt are identical\n",
		},
		{
			name:       "long form ignore-case",
			args:       []string{"--ignore-case", "lower.txt", "upper.txt"},
			wantStdout: "",
		},
		{
			name: "stdin via dash for first file",
			args: []string{"-", "b.txt"},
			stdin: "alpha\nbeta\ngamma\n",
			wantStdout: "--- -\n" +
				"+++ b.txt\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name:       "missing operand exits 2",
			args:       []string{"a.txt"},
			wantStderr: "diff: missing operand\n",
			wantErr:    true,
		},
		{
			name:       "missing first file errors",
			args:       []string{"nope.txt", "a.txt"},
			wantStderr: "diff: nope.txt: No such file or directory\n",
			wantErr:    true,
		},
		{
			name:       "missing second file errors",
			args:       []string{"a.txt", "nope.txt"},
			wantStderr: "diff: nope.txt: No such file or directory\n",
			wantErr:    true,
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--no-such-flag", "a.txt", "b.txt"},
			wantErr: true,
		},
		{
			name:       "double dash terminator",
			args:       []string{"--", "a.txt", "a_copy.txt"},
			wantStdout: "",
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
				t.Errorf("stdout mismatch\nwant: %q\ngot:  %q", tt.wantStdout, stdout)
			}
			if tt.wantStderr != "" && stderr != tt.wantStderr {
				t.Errorf("stderr mismatch\nwant: %q\ngot:  %q", tt.wantStderr, stderr)
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
	if !strings.Contains(stderr, "Usage: diff [OPTION]... FILE1 FILE2") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--brief") {
		t.Errorf("brief flag missing from help: %q", stderr)
	}
}

func TestNilExecContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}
