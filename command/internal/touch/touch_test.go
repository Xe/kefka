package touch

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
	f, err := fs.OpenFile("hello.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("hello\n"))
	f.Close()
	return fs
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

func exists(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	_, err := fs.Stat(name)
	return err == nil
}

func TestTouch(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "create new empty file",
			args: []string{"new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
				info, err := fs.Stat("new.txt")
				if err != nil {
					t.Fatal(err)
				}
				if info.Size() != 0 {
					t.Errorf("new.txt size = %d, want 0", info.Size())
				}
			},
		},
		{
			name: "create multiple files",
			args: []string{"a.txt", "b.txt", "c.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
					if !exists(t, fs, f) {
						t.Errorf("%s was not created", f)
					}
				}
			},
		},
		{
			name: "touch existing file does not truncate",
			args: []string{"hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				info, err := fs.Stat("hello.txt")
				if err != nil {
					t.Fatal(err)
				}
				if info.Size() != int64(len("hello\n")) {
					t.Errorf("hello.txt size = %d, want %d", info.Size(), len("hello\n"))
				}
			},
		},
		{
			name: "no-create skips missing file",
			args: []string{"-c", "missing.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "missing.txt") {
					t.Errorf("missing.txt should not have been created with -c")
				}
			},
		},
		{
			name: "long no-create skips missing file",
			args: []string{"--no-create", "missing.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "missing.txt") {
					t.Errorf("missing.txt should not have been created with --no-create")
				}
			},
		},
		{
			name: "no-create still touches existing file",
			args: []string{"-c", "hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should still exist")
				}
			},
		},
		{
			name: "ignored short flags are accepted",
			args: []string{"-am", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "ignored -r consumes its argument",
			args: []string{"-r", "hello.txt", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "ignored -t consumes its argument",
			args: []string{"-t", "202504300000", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "date short flag with valid date creates file",
			args: []string{"-d", "2024-01-15", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "date long flag with equals form",
			args: []string{"--date=2024-01-15 12:30:45", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "date with slash separators",
			args: []string{"-d", "2024/01/15", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name: "date ISO 8601",
			args: []string{"-d", "2024-01-15T12:30:45Z", "new.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "new.txt") {
					t.Errorf("new.txt was not created")
				}
			},
		},
		{
			name:       "invalid date format",
			args:       []string{"-d", "not-a-date", "new.txt"},
			wantErrSub: "invalid date format",
			wantErr:    true,
		},
		{
			name:       "missing file operand",
			args:       []string{},
			wantErrSub: "missing file operand",
			wantErr:    true,
		},
		{
			name:       "unknown flag",
			args:       []string{"--no-such-flag", "foo"},
			wantErr:    true,
			wantErrSub: "touch:",
		},
		{
			name: "double dash separates flags from files",
			args: []string{"--", "-weird-name.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "-weird-name.txt") {
					t.Errorf("-weird-name.txt was not created")
				}
			},
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/abs.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "abs.txt") {
					t.Errorf("abs.txt was not created")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			stdout, stderr, err := run(t, tt.args, fs)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
			if tt.check != nil {
				tt.check(t, fs)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	fs := newFS(t)
	stdout, stderr, err := run(t, []string{"--help"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: touch [OPTION]... FILE...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--no-create") {
		t.Errorf("--no-create flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--date") {
		t.Errorf("--date flag missing from help output: %q", stderr)
	}
}

func TestNoFilesystem(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, []string{"foo"})
	if err == nil {
		t.Fatal("expected error when ExecContext.FS is nil")
	}
}

func TestParseDateString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ok    bool
	}{
		{"YYYY-MM-DD", "2024-01-15", true},
		{"YYYY/MM/DD", "2024/01/15", true},
		{"YYYY-MM-DD HH:MM:SS", "2024-01-15 12:30:45", true},
		{"YYYY/MM/DD HH:MM:SS", "2024/01/15 12:30:45", true},
		{"ISO 8601 UTC", "2024-01-15T12:30:45Z", true},
		{"ISO 8601 with offset", "2024-01-15T12:30:45+05:00", true},
		{"empty", "", false},
		{"garbage", "not-a-date", false},
		{"partial", "2024-01", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseDateString(tt.input)
			if ok != tt.ok {
				t.Errorf("parseDateString(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
		})
	}
}
