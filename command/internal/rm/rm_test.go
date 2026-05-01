package rm

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
	if err := fs.MkdirAll("emptydir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("populated/sub", 0o755); err != nil {
		t.Fatal(err)
	}
	write("hello.txt", []byte("hello\n"))
	write("other.txt", []byte("other\n"))
	write("populated/inside.txt", []byte("inside\n"))
	write("populated/sub/deep.txt", []byte("deep\n"))
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

func runWithStdin(t *testing.T, args []string, fs billy.Filesystem, stdin string) (string, string, error) {
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

func exists(t *testing.T, fs billy.Filesystem, name string) bool {
	t.Helper()
	_, err := fs.Stat(name)
	return err == nil
}

func TestRm(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name: "remove single file",
			args: []string{"hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
			},
		},
		{
			name: "remove multiple files",
			args: []string{"hello.txt", "other.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed")
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
			name: "force suppresses missing operand",
			args: []string{"-f"},
		},
		{
			name:       "missing file errors without force",
			args:       []string{"nope.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
		},
		{
			name: "missing file silent with force",
			args: []string{"-f", "nope.txt"},
		},
		{
			name:       "directory without recursive errors",
			args:       []string{"emptydir"},
			wantErrSub: "Is a directory",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "emptydir") {
					t.Errorf("emptydir was removed despite missing -r")
				}
			},
		},
		{
			name: "recursive removes empty directory",
			args: []string{"-r", "emptydir"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir was not removed")
				}
			},
		},
		{
			name: "recursive removes populated directory",
			args: []string{"-r", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				for _, p := range []string{"populated", "populated/inside.txt", "populated/sub", "populated/sub/deep.txt"} {
					if exists(t, fs, p) {
						t.Errorf("%s was not removed", p)
					}
				}
			},
		},
		{
			name: "uppercase R is recursive",
			args: []string{"-R", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "populated") {
					t.Errorf("populated was not removed via -R")
				}
			},
		},
		{
			name: "long recursive flag",
			args: []string{"--recursive", "populated"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "populated") {
					t.Errorf("populated was not removed via --recursive")
				}
			},
		},
		{
			name:       "verbose prints removed line",
			args:       []string{"-v", "hello.txt"},
			wantStdout: "removed 'hello.txt'\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
			},
		},
		{
			name:       "long verbose flag",
			args:       []string{"--verbose", "hello.txt"},
			wantStdout: "removed 'hello.txt'\n",
		},
		{
			name:       "verbose with multiple files",
			args:       []string{"-v", "hello.txt", "other.txt"},
			wantStdout: "removed 'hello.txt'\nremoved 'other.txt'\n",
		},
		{
			name:       "verbose recursive on directory",
			args:       []string{"-rv", "populated"},
			wantStdout: "removed 'populated/inside.txt'\nremoved 'populated/sub/deep.txt'\nremoved directory 'populated/sub'\nremoved directory 'populated'\n",
		},
		{
			name:       "partial success continues but reports error",
			args:       []string{"hello.txt", "nope.txt", "other.txt"},
			wantErrSub: "No such file or directory",
			wantErr:    true,
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed despite later failure")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed despite earlier failure")
				}
			},
		},
		{
			name: "force ignores missing in middle of list",
			args: []string{"-f", "hello.txt", "nope.txt", "other.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed")
				}
				if exists(t, fs, "other.txt") {
					t.Errorf("other.txt was not removed")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--no-such-flag", "hello.txt"},
			wantErr: true,
		},
		{
			name: "absolute path resolves under fs root",
			args: []string{"/hello.txt"},
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt was not removed via absolute path")
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
	if !strings.Contains(stderr, "Usage: rm [OPTION]... FILE...") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--recursive") {
		t.Errorf("--recursive flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("--force flag missing from help output: %q", stderr)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("--verbose flag missing from help output: %q", stderr)
	}
	if exists(t, fs, "hello.txt") == false {
		t.Errorf("--help should not have removed any files")
	}
}

func TestInteractive(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStderr string
		check      func(t *testing.T, fs billy.Filesystem)
	}{
		{
			name:       "yes removes file",
			args:       []string{"-i", "hello.txt"},
			stdin:      "y\n",
			wantStderr: "rm: remove regular file 'hello.txt'? ",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been removed")
				}
			},
		},
		{
			name:       "no keeps file",
			args:       []string{"-i", "hello.txt"},
			stdin:      "n\n",
			wantStderr: "rm: remove regular file 'hello.txt'? ",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been kept")
				}
			},
		},
		{
			name:       "empty input keeps file",
			args:       []string{"-i", "hello.txt"},
			stdin:      "",
			wantStderr: "rm: remove regular file 'hello.txt'? ",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been kept")
				}
			},
		},
		{
			name:  "if force wins last so no prompt",
			args:  []string{"-if", "hello.txt"},
			stdin: "",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been removed (force wins)")
				}
			},
		},
		{
			name:       "fi interactive wins last so prompts",
			args:       []string{"-fi", "hello.txt"},
			stdin:      "n\n",
			wantStderr: "rm: remove regular file 'hello.txt'? ",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been kept (interactive wins)")
				}
			},
		},
		{
			name:  "multiple files mixed answers",
			args:  []string{"-i", "hello.txt", "other.txt", "populated/inside.txt"},
			stdin: "y\nn\ny\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been removed")
				}
				if !exists(t, fs, "other.txt") {
					t.Errorf("other.txt should have been kept")
				}
				if exists(t, fs, "populated/inside.txt") {
					t.Errorf("populated/inside.txt should have been removed")
				}
			},
		},
		{
			name:  "uppercase Y also removes",
			args:  []string{"-i", "hello.txt"},
			stdin: "Yes\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been removed")
				}
			},
		},
		{
			name:       "long form interactive flag",
			args:       []string{"--interactive", "hello.txt"},
			stdin:      "n\n",
			wantStderr: "rm: remove regular file 'hello.txt'? ",
			check: func(t *testing.T, fs billy.Filesystem) {
				if !exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been kept")
				}
			},
		},
		{
			name:       "long force after long interactive force wins",
			args:       []string{"--interactive", "--force", "hello.txt"},
			stdin:      "",
			wantStderr: "",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "hello.txt") {
					t.Errorf("hello.txt should have been removed (force wins)")
				}
			},
		},
		{
			name:  "empty directory prompt with -ri",
			args:  []string{"-ri", "emptydir"},
			stdin: "y\n",
			check: func(t *testing.T, fs billy.Filesystem) {
				if exists(t, fs, "emptydir") {
					t.Errorf("emptydir should have been removed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFS(t)
			_, stderr, err := runWithStdin(t, tt.args, fs, tt.stdin)
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantStderr)
			}
			if tt.check != nil {
				tt.check(t, fs)
			}
		})
	}
}

func TestInteractiveEmptyDirPrompt(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-ri", "emptydir"}, fs, "y\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "rm: remove directory 'emptydir'? ") {
		t.Errorf("expected 'remove directory' prompt, got: %q", stderr)
	}
}

func TestInteractiveDescendPrompt(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := runWithStdin(t, []string{"-ri", "populated"}, fs, "n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "rm: descend into directory 'populated'? ") {
		t.Errorf("expected 'descend into directory' prompt, got: %q", stderr)
	}
	if !exists(t, fs, "populated") {
		t.Errorf("populated should have been kept after refusing descent")
	}
}

func TestWriteProtected(t *testing.T) {
	// A write-protected (mode 0o400) file with stdin attached should prompt
	// with the GNU "write-protected" wording even without -i. -f always
	// suppresses the prompt.
	makeFS := func(t *testing.T) billy.Filesystem {
		t.Helper()
		fs := memfs.New()
		f, err := fs.OpenFile("ro.txt", os.O_CREATE|os.O_WRONLY, 0o400)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte("ro\n"))
		f.Close()
		return fs
	}

	t.Run("yes removes write-protected file", func(t *testing.T) {
		fs := makeFS(t)
		_, stderr, err := runWithStdin(t, []string{"ro.txt"}, fs, "y\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove write-protected regular file 'ro.txt'? ") {
			t.Errorf("stderr missing write-protected prompt: %q", stderr)
		}
		if exists(t, fs, "ro.txt") {
			t.Errorf("ro.txt should have been removed after y answer")
		}
	})

	t.Run("no keeps write-protected file", func(t *testing.T) {
		fs := makeFS(t)
		_, stderr, err := runWithStdin(t, []string{"ro.txt"}, fs, "n\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove write-protected regular file 'ro.txt'? ") {
			t.Errorf("stderr missing write-protected prompt: %q", stderr)
		}
		if !exists(t, fs, "ro.txt") {
			t.Errorf("ro.txt should have been kept after n answer")
		}
	})

	t.Run("force suppresses write-protected prompt", func(t *testing.T) {
		fs := makeFS(t)
		_, stderr, err := runWithStdin(t, []string{"-f", "ro.txt"}, fs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if strings.Contains(stderr, "write-protected") {
			t.Errorf("stderr should not include prompt under -f: %q", stderr)
		}
		if exists(t, fs, "ro.txt") {
			t.Errorf("ro.txt should have been removed under -f")
		}
	})

	t.Run("interactive on write-protected uses write-protected wording", func(t *testing.T) {
		fs := makeFS(t)
		_, stderr, err := runWithStdin(t, []string{"-i", "ro.txt"}, fs, "n\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove write-protected regular file 'ro.txt'? ") {
			t.Errorf("expected write-protected prompt under -i: %q", stderr)
		}
	})

	t.Run("no stdin: no prompt no removal failure when writable", func(t *testing.T) {
		fs := memfs.New()
		f, err := fs.OpenFile("rw.txt", os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		// run() uses no stdin. A writable file should be removed without
		// any prompt or failure.
		_, stderr, err := run(t, []string{"rw.txt"}, fs)
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if stderr != "" {
			t.Errorf("expected empty stderr, got %q", stderr)
		}
		if exists(t, fs, "rw.txt") {
			t.Errorf("rw.txt should have been removed")
		}
	})

	t.Run("no stdin write-protected without -f silently skips prompt path", func(t *testing.T) {
		// With no stdin attached we can't prompt; the file should still be
		// removed (matches GNU rm behavior when stdin is not a terminal).
		fs := makeFS(t)
		_, _, err := run(t, []string{"ro.txt"}, fs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists(t, fs, "ro.txt") {
			t.Errorf("ro.txt should have been removed when no stdin attached")
		}
	})
}

func TestRecursiveDoesNotFollowSymlinks(t *testing.T) {
	// Build a layout where dir/ contains a symlink pointing to outside/.
	// rm -r dir must remove the symlink itself but must NOT touch outside/
	// or its contents.
	fs := memfs.New()
	if err := fs.MkdirAll("dir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("outside", 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("dir/inside.txt", []byte("inside"))
	write("outside/keep.txt", []byte("keep"))

	sym, ok := fs.(billy.Symlink)
	if !ok {
		t.Skip("memfs does not implement billy.Symlink")
	}
	if err := sym.Symlink("/outside", "dir/escape"); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, stderr, err := run(t, []string{"-r", "dir"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}

	if exists(t, fs, "dir") {
		t.Errorf("dir was not removed")
	}
	// The crucial assertion: rm -r must NOT have followed the symlink.
	if !exists(t, fs, "outside") {
		t.Errorf("outside/ was incorrectly removed via symlink traversal")
	}
	if !exists(t, fs, "outside/keep.txt") {
		t.Errorf("outside/keep.txt was incorrectly removed via symlink traversal")
	}
}

func TestSymlinkRemoval(t *testing.T) {
	// rm of a symlink should remove the link, not the target.
	fs := memfs.New()
	f, err := fs.OpenFile("target.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("target"))
	f.Close()

	sym, ok := fs.(billy.Symlink)
	if !ok {
		t.Skip("memfs does not implement billy.Symlink")
	}
	if err := sym.Symlink("target.txt", "link"); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if _, _, err := run(t, []string{"link"}, fs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists(t, fs, "link") {
		t.Errorf("link should have been removed")
	}
	if !exists(t, fs, "target.txt") {
		t.Errorf("target.txt must not be removed when only the symlink was named")
	}
}

func TestDFlag(t *testing.T) {
	t.Run("removes empty dir without -r", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := run(t, []string{"-d", "emptydir"}, fs)
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if exists(t, fs, "emptydir") {
			t.Errorf("emptydir should have been removed via -d")
		}
	})

	t.Run("refuses to remove non-empty dir", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := run(t, []string{"-d", "populated"}, fs)
		if err == nil {
			t.Fatalf("expected error when -d on non-empty dir")
		}
		if !strings.Contains(stderr, "Directory not empty") {
			t.Errorf("expected 'Directory not empty' in stderr, got %q", stderr)
		}
		if !exists(t, fs, "populated") {
			t.Errorf("populated should still exist after -d on non-empty dir")
		}
	})
}

func TestInteractivePromptKinds(t *testing.T) {
	// Directly exercise fileKindForPrompt via integration: an empty regular
	// file should produce "regular empty file" wording in the prompt.
	fs := memfs.New()
	f, err := fs.OpenFile("empty.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, stderr, err := runWithStdin(t, []string{"-i", "empty.txt"}, fs, "n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "rm: remove regular empty file 'empty.txt'? ") {
		t.Errorf("expected 'regular empty file' prompt, got %q", stderr)
	}
}

func TestInteractiveSymlinkPrompt(t *testing.T) {
	fs := memfs.New()
	sym, ok := fs.(billy.Symlink)
	if !ok {
		t.Skip("memfs does not implement billy.Symlink")
	}
	if err := sym.Symlink("nowhere", "dangling"); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, stderr, err := runWithStdin(t, []string{"-i", "dangling"}, fs, "y\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "rm: remove symbolic link 'dangling'? ") {
		t.Errorf("expected 'symbolic link' prompt, got %q", stderr)
	}
	if exists(t, fs, "dangling") {
		t.Errorf("dangling should have been removed")
	}
}

func TestLessInteractive(t *testing.T) {
	t.Run("prompts once before removing 4 files yes", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-I", "hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"}, fs, "y\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove 4 arguments? ") {
			t.Errorf("expected 'remove 4 arguments?' prompt, got %q", stderr)
		}
		// Count prompts: should be exactly one.
		if c := strings.Count(stderr, "? "); c != 1 {
			t.Errorf("expected exactly one prompt, got %d in %q", c, stderr)
		}
		for _, p := range []string{"hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"} {
			if exists(t, fs, p) {
				t.Errorf("%s should have been removed", p)
			}
		}
	})

	t.Run("prompts once before removing 4 files no", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-I", "hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"}, fs, "n\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove 4 arguments? ") {
			t.Errorf("expected prompt, got %q", stderr)
		}
		// Refusal must keep all of them.
		for _, p := range []string{"hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"} {
			if !exists(t, fs, p) {
				t.Errorf("%s should still exist after 'n' answer", p)
			}
		}
	})

	t.Run("does not prompt for 3 files", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-I", "hello.txt", "other.txt", "populated/inside.txt"}, fs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if strings.Contains(stderr, "? ") {
			t.Errorf("should not have prompted for 3 files, got %q", stderr)
		}
		for _, p := range []string{"hello.txt", "other.txt", "populated/inside.txt"} {
			if exists(t, fs, p) {
				t.Errorf("%s should have been removed", p)
			}
		}
	})

	t.Run("prompts once when recursing yes", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-Ir", "populated"}, fs, "y\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "rm: remove 1 argument recursively? ") {
			t.Errorf("expected recursive prompt, got %q", stderr)
		}
		if c := strings.Count(stderr, "? "); c != 1 {
			t.Errorf("expected exactly one prompt under -I, got %d in %q", c, stderr)
		}
		if exists(t, fs, "populated") {
			t.Errorf("populated should have been removed")
		}
	})

	t.Run("prompts once when recursing no", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-Ir", "populated"}, fs, "n\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if !strings.Contains(stderr, "recursively? ") {
			t.Errorf("expected recursive prompt, got %q", stderr)
		}
		if !exists(t, fs, "populated") {
			t.Errorf("populated should have been kept after refusal")
		}
	})

	t.Run("force overrides -I", func(t *testing.T) {
		fs := newFS(t)
		_, stderr, err := runWithStdin(t, []string{"-If", "hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"}, fs, "")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if strings.Contains(stderr, "? ") {
			t.Errorf("should not have prompted under -f, got %q", stderr)
		}
		if exists(t, fs, "hello.txt") {
			t.Errorf("hello.txt should have been removed under -f")
		}
	})

	t.Run("interactive overrides -I", func(t *testing.T) {
		fs := newFS(t)
		// -Ii means -i wins (last one). Expect per-file prompts, not the
		// single -I prompt.
		_, stderr, err := runWithStdin(t, []string{"-Ii", "hello.txt", "other.txt", "populated/inside.txt", "populated/sub/deep.txt"}, fs, "n\nn\nn\nn\n")
		if err != nil {
			t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
		}
		if strings.Contains(stderr, "remove 4 arguments?") {
			t.Errorf("should not show -I prompt when -i wins, got %q", stderr)
		}
		if !strings.Contains(stderr, "rm: remove regular file 'hello.txt'? ") {
			t.Errorf("expected per-file prompt, got %q", stderr)
		}
	})
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
