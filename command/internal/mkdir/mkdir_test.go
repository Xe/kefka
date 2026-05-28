package mkdir

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
	if err := fs.MkdirAll("existing", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("parent", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := fs.OpenFile("file.txt", os.O_CREATE|os.O_WRONLY, 0o644)
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

func isDir(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	info, err := fs.Stat(name)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func TestMkdir(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "create single directory",
			args: []string{"newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created")
				}
			},
		},
		{
			name: "create multiple directories",
			args: []string{"a", "b", "c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"a", "b", "c"} {
					if !isDir(t, fs, d) {
						t.Errorf("%s was not created", d)
					}
				}
			},
		},
		{
			name: "create inside existing parent",
			args: []string{"parent/child"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "parent/child") {
					t.Errorf("parent/child was not created")
				}
			},
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantErrSub: "missing operand",
			wantErr:    true,
		},
		{
			name:       "fails when directory exists",
			args:       []string{"existing"},
			wantErrSub: "File exists",
			wantErr:    true,
		},
		{
			name:       "fails when parent does not exist",
			args:       []string{"nope/child"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name: "parents flag silent on existing",
			args: []string{"-p", "existing"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "existing") {
					t.Errorf("existing was removed")
				}
			},
		},
		{
			name: "parents flag creates intermediate directories",
			args: []string{"-p", "a/b/c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, d := range []string{"a", "a/b", "a/b/c"} {
					if !isDir(t, fs, d) {
						t.Errorf("%s was not created", d)
					}
				}
			},
		},
		{
			name: "long parents flag",
			args: []string{"--parents", "deep/path/here"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "deep/path/here") {
					t.Errorf("deep/path/here was not created")
				}
			},
		},
		{
			name:       "verbose prints to stdout",
			args:       []string{"-v", "newdir"},
			wantStdout: "mkdir: created directory 'newdir'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created")
				}
			},
		},
		{
			name:       "long verbose flag",
			args:       []string{"--verbose", "newdir"},
			wantStdout: "mkdir: created directory 'newdir'\n",
		},
		{
			name:       "verbose with multiple",
			args:       []string{"-pv", "a/b", "c"},
			wantStdout: "mkdir: created directory 'a/b'\nmkdir: created directory 'c'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "a/b") {
					t.Errorf("a/b was not created")
				}
				if !isDir(t, fs, "c") {
					t.Errorf("c was not created")
				}
			},
		},
		{
			name:       "partial success returns error and continues",
			args:       []string{"existing", "newdir"},
			wantErrSub: "File exists",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "newdir") {
					t.Errorf("newdir was not created after earlier failure")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "foo"},
			wantErr: true,
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/abs"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "abs") {
					t.Errorf("abs was not created")
				}
			},
		},
		{
			name: "mode numeric leading zero",
			args: []string{"-m", "0700", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o700)
			},
		},
		{
			name: "mode numeric no leading zero",
			args: []string{"-m", "755", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o755)
			},
		},
		{
			name: "mode numeric 0o prefix",
			args: []string{"-m", "0o750", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o750)
			},
		},
		{
			name: "mode symbolic clauses",
			args: []string{"-m", "u=rwx,g=rx,o=", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o750)
			},
		},
		{
			name: "mode symbolic plus",
			args: []string{"-m", "u+w", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o755)
			},
		},
		{
			name: "mode symbolic minus",
			args: []string{"-m", "g-x", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o745)
			},
		},
		{
			name:       "mode invalid spec",
			args:       []string{"-m", "garbage", "newdir"},
			wantErr:    true,
			wantErrSub: "invalid mode 'garbage'",
			check: func(t *testing.T, fs billy.Filesystem) {
				if isDir(t, fs, "newdir") {
					t.Errorf("newdir should not have been created on parse failure")
				}
			},
		},
		{
			name:       "mode invalid octal too long",
			args:       []string{"-m", "12345", "newdir"},
			wantErr:    true,
			wantErrSub: "invalid mode '12345'",
		},
		{
			name:       "mode invalid octal digit",
			args:       []string{"-m", "799", "newdir"},
			wantErr:    true,
			wantErrSub: "invalid mode '799'",
		},
		{
			name: "mode long flag",
			args: []string{"--mode=0700", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o700)
			},
		},
		{
			name: "mode with parents leaf only",
			args: []string{"-p", "-m", "0700", "a/b/c"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "a/b/c", 0o700)
				if !isDir(t, fs, "a") {
					t.Errorf("intermediate a was not created")
				}
				if !isDir(t, fs, "a/b") {
					t.Errorf("intermediate a/b was not created")
				}
			},
		},
		{
			name: "parents intermediates traversable with restrictive leaf mode",
			args: []string{"-p", "-m", "0700", "i1/i2/leaf"},
			check: func(t *testing.T, fs billy.Filesystem) {
				// Per POSIX, intermediates get
				// (S_IWUSR|S_IXUSR|~filemask) & 0777 so the user can
				// always traverse them. With our assumed umask of 022
				// that resolves to 0755.
				assertDirMode(t, fs, "i1", 0o755)
				assertDirMode(t, fs, "i1/i2", 0o755)
				assertDirMode(t, fs, "i1/i2/leaf", 0o700)
			},
		},
		{
			name: "default leaf mode applies umask",
			args: []string{"newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				// Default = 0777 &^ umask (0022) = 0755.
				assertDirMode(t, fs, "newdir", 0o755)
			},
		},
		{
			name: "parents leaves existing directory mode untouched",
			args: []string{"-p", "-m", "0700", "parent/child"},
			check: func(t *testing.T, fs billy.Filesystem) {
				// parent already existed at 0o755 from newFS;
				// -p must not change its mode.
				assertDirMode(t, fs, "parent", 0o755)
				assertDirMode(t, fs, "parent/child", 0o700)
			},
		},
		{
			name: "parents existing with -p is silent and exit zero",
			args: []string{"-p", "existing"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if !isDir(t, fs, "existing") {
					t.Errorf("existing was removed")
				}
			},
		},
		{
			name:       "mode on existing dir errors",
			args:       []string{"-m", "0700", "existing"},
			wantErr:    true,
			wantErrSub: "File exists",
		},
		{
			name: "symbolic mode setuid",
			args: []string{"-m", "u=rwxs,g=rx,o=", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				// We accept the setuid bit being set on the dir, but
				// memfs does not necessarily round-trip the mode-flag
				// portion. Just check the perm bits.
				assertDirMode(t, fs, "newdir", 0o750)
			},
		},
		{
			name: "symbolic mode all clauses",
			args: []string{"-m", "a=rwx", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o777)
			},
		},
		{
			name: "verbose with mode",
			args: []string{"-v", "-m", "0700", "newdir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				assertDirMode(t, fs, "newdir", 0o700)
			},
			wantStdout: "mkdir: created directory 'newdir'\n",
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

func assertDirMode(t *testing.T, fs billy.Filesystem, name string, want os.FileMode) {
	t.Helper()
	info, err := fs.Stat(name)
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory after mkdir", name)
	}
	got := info.Mode().Perm()
	if got != want {
		t.Errorf("%s mode = %#o, want %#o", name, got, want)
	}
}

func TestParseMode(t *testing.T) {
	const umask os.FileMode = 0o022
	tests := []struct {
		spec string
		want os.FileMode
		ok   bool
	}{
		{"0700", 0o700, true},
		{"700", 0o700, true},
		{"755", 0o755, true},
		{"0o755", 0o755, true},
		{"7777", 0o7777, true},
		{"u=rwx,g=rx,o=", 0o750, true},
		{"u+w", 0o755, true},
		{"g-x", 0o745, true},
		{"a=", 0, true},
		{"o=r", 0o754, true},
		{"", 0, false},
		{"garbage", 0, false},
		{"12345", 0, false},
		{"799", 0, false},
		{"u=q", 0, false},
		{"+", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, ok := parseMode(tt.spec, umask)
			if ok != tt.ok {
				t.Fatalf("parseMode(%q) ok = %v, want %v", tt.spec, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("parseMode(%q) = %#o, want %#o", tt.spec, got, tt.want)
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
	if !strings.Contains(stderr, "Usage: mkdir [OPTION]... DIRECTORY...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--parents") {
		t.Errorf("--parents flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("--verbose flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--mode") {
		t.Errorf("--mode flag missing from help output: %q", stderr)
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
