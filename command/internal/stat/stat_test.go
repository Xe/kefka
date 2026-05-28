package stat

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

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte, perm os.FileMode) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, perm)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("hello.txt", []byte("hello"), 0o644)
	write("script.sh", bytes.Repeat([]byte("x"), 600), 0o755)
	if err := fs.MkdirAll("dir", 0o755); err != nil {
		t.Fatal(err)
	}
	return fs
}

func withFixedTime(t *testing.T) {
	t.Helper()
	prev := formatTime
	formatTime = func(time.Time) string { return "2024-01-15T10:30:45.123Z" }
	t.Cleanup(func() { formatTime = prev })
}

func run(t *testing.T, args []string, fs billy.Filesystem) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     fs,
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestStat(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name: "default format on regular file",
			args: []string{"hello.txt"},
			wantStdout: "  File: hello.txt\n" +
				"  Size: 5\t\tBlocks: 1\n" +
				"Access: (0644/-rw-r--r--)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n",
		},
		{
			name: "default format on directory",
			args: []string{"dir"},
			wantStdout: "  File: dir\n" +
				"  Size: 0\t\tBlocks: 0\n" +
				"Access: (0755/drwxr-xr-x)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n",
		},
		{
			name: "default format computes blocks via ceiling",
			args: []string{"script.sh"},
			wantStdout: "  File: script.sh\n" +
				"  Size: 600\t\tBlocks: 2\n" +
				"Access: (0755/-rwxr-xr-x)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n",
		},
		{
			name:       "custom format %n prints file name",
			args:       []string{"-c", "%n", "hello.txt"},
			wantStdout: "hello.txt\n",
		},
		{
			name:       "custom format %N quotes file name",
			args:       []string{"-c", "%N", "hello.txt"},
			wantStdout: "'hello.txt'\n",
		},
		{
			name:       "custom format %s prints size",
			args:       []string{"-c", "%s", "hello.txt"},
			wantStdout: "5\n",
		},
		{
			name:       "custom format %F regular file",
			args:       []string{"-c", "%F", "hello.txt"},
			wantStdout: "regular file\n",
		},
		{
			name:       "custom format %F directory",
			args:       []string{"-c", "%F", "dir"},
			wantStdout: "directory\n",
		},
		{
			name:       "custom format %a octal mode no padding",
			args:       []string{"-c", "%a", "hello.txt"},
			wantStdout: "644\n",
		},
		{
			name:       "custom format %A symbolic mode",
			args:       []string{"-c", "%A", "hello.txt"},
			wantStdout: "-rw-r--r--\n",
		},
		{
			name:       "custom format owner placeholders",
			args:       []string{"-c", "%u %U %g %G", "hello.txt"},
			wantStdout: "1000 user 1000 group\n",
		},
		{
			name:       "custom format combined",
			args:       []string{"-c", "%n %s %A", "hello.txt"},
			wantStdout: "hello.txt 5 -rw-r--r--\n",
		},
		{
			name:       "long form -c with equals",
			args:       []string{"-c%a", "hello.txt"},
			wantStdout: "644\n",
		},
		{
			name: "multiple files in default format",
			args: []string{"hello.txt", "dir"},
			wantStdout: "  File: hello.txt\n" +
				"  Size: 5\t\tBlocks: 1\n" +
				"Access: (0644/-rw-r--r--)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n" +
				"  File: dir\n" +
				"  Size: 0\t\tBlocks: 0\n" +
				"Access: (0755/drwxr-xr-x)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n",
		},
		{
			name:       "missing operand reports error",
			args:       nil,
			wantStderr: "stat: missing operand\n",
			wantErr:    true,
		},
		{
			name:       "missing file reports error and continues",
			args:       []string{"nope.txt"},
			wantStderr: "stat: cannot stat 'nope.txt': No such file or directory\n",
			wantErr:    true,
		},
		{
			name: "missing file mixed with present file",
			args: []string{"hello.txt", "nope.txt"},
			wantStdout: "  File: hello.txt\n" +
				"  Size: 5\t\tBlocks: 1\n" +
				"Access: (0644/-rw-r--r--)\n" +
				"Modify: 2024-01-15T10:30:45.123Z\n",
			wantStderr: "stat: cannot stat 'nope.txt': No such file or directory\n",
			wantErr:    true,
		},
		{
			name:    "unknown flag returns error",
			args:    []string{"-Z", "hello.txt"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFixedTime(t)
			stdout, stderr, err := run(t, tt.args, newFS(t))
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantStderr)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"}, newFS(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: stat [OPTION]... FILE...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-c FORMAT") {
		t.Errorf("-c flag missing from help: %q", stderr)
	}
}

func TestExec_NilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestExec_NoFS(t *testing.T) {
	ec := &command.ExecContext{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"hello.txt"}); err == nil {
		t.Fatal("expected error for missing filesystem")
	}
}

func TestFormatMode(t *testing.T) {
	tests := []struct {
		name  string
		mode  os.FileMode
		isDir bool
		want  string
	}{
		{"regular 644", 0o644, false, "-rw-r--r--"},
		{"regular 755", 0o755, false, "-rwxr-xr-x"},
		{"directory 755", 0o755, true, "drwxr-xr-x"},
		{"none", 0o000, false, "----------"},
		{"all", 0o777, false, "-rwxrwxrwx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatMode(tt.mode, tt.isDir); got != tt.want {
				t.Errorf("formatMode(%o, %v) = %q, want %q", tt.mode, tt.isDir, got, tt.want)
			}
		})
	}
}
