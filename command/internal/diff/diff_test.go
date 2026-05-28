package diff

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

const fixedStamp = "2024-01-02 03:04:05.000000000 +0000"

func init() {
	t, _ := time.Parse("2006-01-02 15:04:05 -0700", "2024-01-02 03:04:05 +0000")
	nowFunc = func() time.Time { return t }
}

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
	write("ws1.txt", []byte("a b\n"))
	write("ws2.txt", []byte("a  b\n"))
	write("multi1.txt", []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\nn\n"))
	write("multi2.txt", []byte("a\nB\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nM\nn\n"))
	write("empty.txt", []byte(""))
	write("one.txt", []byte("alpha\n"))
	// Directories for -r tests
	write("dir1/same.txt", []byte("hello\n"))
	write("dir1/different.txt", []byte("a\nb\nc\n"))
	write("dir1/only1.txt", []byte("only-1\n"))
	write("dir1/sub/inner.txt", []byte("nested\n"))
	write("dir2/same.txt", []byte("hello\n"))
	write("dir2/different.txt", []byte("a\nB\nc\n"))
	write("dir2/only2.txt", []byte("only-2\n"))
	write("dir2/sub/inner.txt", []byte("nested\n"))
	// Identical directory pair for the "same" exit-code path
	write("samedir1/x.txt", []byte("x\n"))
	write("samedir2/x.txt", []byte("x\n"))
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
			name: "different files default normal format",
			args: []string{"a.txt", "b.txt"},
			wantStdout: "2c2\n" +
				"< beta\n" +
				"---\n" +
				"> BETA\n",
			wantErr: true,
		},
		{
			name: "explicit -u flag emits unified format",
			args: []string{"-u", "a.txt", "b.txt"},
			wantStdout: "--- a.txt\t" + fixedStamp + "\n" +
				"+++ b.txt\t" + fixedStamp + "\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name: "-U with explicit context size",
			args: []string{"-U", "1", "a.txt", "b.txt"},
			wantStdout: "--- a.txt\t" + fixedStamp + "\n" +
				"+++ b.txt\t" + fixedStamp + "\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name: "-U 0 emits no context",
			args: []string{"-U", "0", "a.txt", "b.txt"},
			wantStdout: "--- a.txt\t" + fixedStamp + "\n" +
				"+++ b.txt\t" + fixedStamp + "\n" +
				"@@ -2 +2 @@\n" +
				"-beta\n" +
				"+BETA\n",
			wantErr: true,
		},
		{
			name: "explicit -c flag emits context format",
			args: []string{"-c", "a.txt", "b.txt"},
			wantStdout: "*** a.txt\t" + fixedStamp + "\n" +
				"--- b.txt\t" + fixedStamp + "\n" +
				"***************\n" +
				"*** 1,3 ****\n" +
				"  alpha\n" +
				"! beta\n" +
				"  gamma\n" +
				"--- 1,3 ----\n" +
				"  alpha\n" +
				"! BETA\n" +
				"  gamma\n",
			wantErr: true,
		},
		{
			name: "-C with explicit context size",
			args: []string{"-C", "1", "a.txt", "b.txt"},
			wantStdout: "*** a.txt\t" + fixedStamp + "\n" +
				"--- b.txt\t" + fixedStamp + "\n" +
				"***************\n" +
				"*** 1,3 ****\n" +
				"  alpha\n" +
				"! beta\n" +
				"  gamma\n" +
				"--- 1,3 ----\n" +
				"  alpha\n" +
				"! BETA\n" +
				"  gamma\n",
			wantErr: true,
		},
		{
			name: "-e ed script for change",
			args: []string{"-e", "a.txt", "b.txt"},
			wantStdout: "2c\n" +
				"BETA\n" +
				".\n",
			wantErr: true,
		},
		{
			name: "-e ed script in reverse order",
			args: []string{"-e", "multi1.txt", "multi2.txt"},
			wantStdout: "13c\n" +
				"M\n" +
				".\n" +
				"2c\n" +
				"B\n" +
				".\n",
			wantErr: true,
		},
		{
			name: "-e ed script for add to empty",
			args: []string{"-e", "empty.txt", "one.txt"},
			wantStdout: "0a\n" +
				"alpha\n" +
				".\n",
			wantErr: true,
		},
		{
			name: "-e ed script for delete to empty",
			args: []string{"-e", "one.txt", "empty.txt"},
			wantStdout: "1d\n",
			wantErr:    true,
		},
		{
			name:       "-b ignores whitespace amount differences",
			args:       []string{"-b", "ws1.txt", "ws2.txt"},
			wantStdout: "",
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
			name:  "stdin via dash for first file with -u",
			args:  []string{"-u", "-", "b.txt"},
			stdin: "alpha\nbeta\ngamma\n",
			wantStdout: "--- -\t" + fixedStamp + "\n" +
				"+++ b.txt\t" + fixedStamp + "\n" +
				"@@ -1,3 +1,3 @@\n" +
				" alpha\n" +
				"-beta\n" +
				"+BETA\n" +
				" gamma\n",
			wantErr: true,
		},
		{
			name: "default normal format for add",
			args: []string{"empty.txt", "one.txt"},
			wantStdout: "0a1\n" +
				"> alpha\n",
			wantErr: true,
		},
		{
			name: "default normal format for delete",
			args: []string{"one.txt", "empty.txt"},
			wantStdout: "1d0\n" +
				"< alpha\n",
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
		{
			name:       "invalid context length errors",
			args:       []string{"-U", "abc", "a.txt", "b.txt"},
			wantStderr: "diff: invalid context length 'abc'\n",
			wantErr:    true,
		},
		{
			name: "-f forward ed script for change",
			args: []string{"-f", "a.txt", "b.txt"},
			wantStdout: "c2\n" +
				"BETA\n" +
				".\n",
			wantErr: true,
		},
		{
			name: "-f forward ed script preserves forward order",
			args: []string{"-f", "multi1.txt", "multi2.txt"},
			wantStdout: "c2\n" +
				"B\n" +
				".\n" +
				"c13\n" +
				"M\n" +
				".\n",
			wantErr: true,
		},
		{
			name: "-f forward ed for add",
			args: []string{"-f", "empty.txt", "one.txt"},
			wantStdout: "a0\n" +
				"alpha\n" +
				".\n",
			wantErr: true,
		},
		{
			name:       "-f forward ed for delete",
			args:       []string{"-f", "one.txt", "empty.txt"},
			wantStdout: "d1\n",
			wantErr:    true,
		},
		{
			name: "directory comparison reports differences",
			args: []string{"dir1", "dir2"},
			wantStdout: "diff dir1/different.txt dir2/different.txt\n" +
				"2c2\n" +
				"< b\n" +
				"---\n" +
				"> B\n" +
				"Only in dir1: only1.txt\n" +
				"Only in dir2: only2.txt\n" +
				"Common subdirectories: dir1/sub and dir2/sub\n",
			wantErr: true,
		},
		{
			name: "recursive -r descends into common subdirectories",
			args: []string{"-r", "dir1", "dir2"},
			wantStdout: "diff dir1/different.txt dir2/different.txt\n" +
				"2c2\n" +
				"< b\n" +
				"---\n" +
				"> B\n" +
				"Only in dir1: only1.txt\n" +
				"Only in dir2: only2.txt\n",
			wantErr: true,
		},
		{
			name:       "identical directory tree -r exits 0",
			args:       []string{"-r", "samedir1", "samedir2"},
			wantStdout: "",
		},
		{
			name:       "identical directories with -s reports identical files",
			args:       []string{"-s", "samedir1", "samedir2"},
			wantStdout: "Files samedir1/x.txt and samedir2/x.txt are identical\n",
		},
		{
			name: "directory vs file appends basename to dir",
			args: []string{"dir1", "dir2/different.txt"},
			wantStdout: "2c2\n" +
				"< b\n" +
				"---\n" +
				"> B\n",
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
				t.Errorf("stdout mismatch\nwant: %q\ngot:  %q", tt.wantStdout, stdout)
			}
			if tt.wantStderr != "" && stderr != tt.wantStderr {
				t.Errorf("stderr mismatch\nwant: %q\ngot:  %q", tt.wantStderr, stderr)
			}
		})
	}
}

func TestExitStatus(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{name: "identical files exit 0", args: []string{"a.txt", "a_copy.txt"}, wantCode: 0},
		{name: "different files exit 1", args: []string{"a.txt", "b.txt"}, wantCode: 1},
		{name: "missing file exit 2", args: []string{"nope.txt", "a.txt"}, wantCode: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := run(t, tt.args, "", newFS(t))
			got := exitCode(err)
			if got != tt.wantCode {
				t.Errorf("exit code: want %d, got %d (err=%v)", tt.wantCode, got, err)
			}
		})
	}
}

// exitCode extracts the integer exit status from an error returned by Exec.
// 0 = no error; matches mvdan.cc/sh/v3/interp.ExitStatus encoding.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	type exitStatus interface{ Error() string }
	_ = exitStatus(nil)
	// interp.ExitStatus is uint8 implementing error.
	if es, ok := err.(interface{ Error() string }); ok {
		s := es.Error()
		// "exit status N"
		const prefix = "exit status "
		if strings.HasPrefix(s, prefix) {
			n := 0
			for _, c := range s[len(prefix):] {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			return n
		}
	}
	return -1
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
	if !strings.Contains(stderr, "--ignore-space-change") {
		t.Errorf("-b help missing: %q", stderr)
	}
	if !strings.Contains(stderr, "--ed") {
		t.Errorf("-e help missing: %q", stderr)
	}
}

func TestNilExecContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}
