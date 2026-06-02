package uutils_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v6/osfs"
	"tangled.org/xeiaso.net/kefka/command"
	"tangled.org/xeiaso.net/kefka/command/uutils"
)

// TestExec_WorkingDirectory verifies that ec.Dir flows through to the WASI guest
// as PWD, so the guest (patched to chdir to $PWD at startup; see pwd-hack.patch)
// resolves paths the same way the shell does: relative against the current
// directory, absolute against the filesystem root. Before the fix the guest had
// no cwd and resolved everything against the mount root.
func TestExec_WorkingDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "hello.txt"), []byte("in-sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rootfile.txt"), []byte("at-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := osfs.New(root)

	run := func(t *testing.T, dir, name string, args ...string) (string, string, error) {
		t.Helper()
		var out, errb bytes.Buffer
		ec := &command.ExecContext{Dir: dir, FS: fsys, Stdout: &out, Stderr: &errb}
		err := uutils.For(name).Exec(context.Background(), ec, args)
		return out.String(), errb.String(), err
	}

	tests := []struct {
		name    string
		dir     string
		cmd     string
		args    []string
		wantOut string
		wantErr bool // non-nil error (non-zero exit) expected
	}{
		{"pwd reflects Dir", "sub", "pwd", nil, "/sub\n", false},
		{"relative resolves against cwd", "sub", "cat", []string{"hello.txt"}, "in-sub\n", false},
		{"absolute reaches fsys root", "sub", "cat", []string{"/rootfile.txt"}, "at-root\n", false},
		{"absolute with cwd prefix, no doubling", "sub", "cat", []string{"/sub/hello.txt"}, "in-sub\n", false},
		{"relative at root", ".", "cat", []string{"rootfile.txt"}, "at-root\n", false},
		{"relative missing in cwd fails", "sub", "cat", []string{"rootfile.txt"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errb, err := run(t, tt.dir, tt.cmd, tt.args...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (out=%q)", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v (stderr=%q)", err, errb)
			}
			if out != tt.wantOut {
				t.Errorf("out = %q, want %q", out, tt.wantOut)
			}
		})
	}
}
