package find

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/registry"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
	"mvdan.cc/sh/v3/interp"
)

// entry is one fixture file or directory. Paths are shell-absolute.
type entry struct {
	content string
	mode    fs.FileMode // 0 means 0o644 for files, 0o755 for directories
	mtime   time.Time   // zero means "now"
	dir     bool
}

func file(content string) entry { return entry{content: content} }

// testFS overrides mtimes (memfs has no Chtimes) and makes chosen
// directories fail to list, the way a protected macOS folder does.
type testFS struct {
	billy.Filesystem
	mtimes     map[string]time.Time
	unreadable map[string]error
}

type mtimeInfo struct {
	fs.FileInfo
	mtime time.Time
}

func (m mtimeInfo) ModTime() time.Time { return m.mtime }

func (t *testFS) wrap(name string, fi fs.FileInfo, err error) (fs.FileInfo, error) {
	if err != nil {
		return fi, err
	}
	if mt, ok := t.mtimes[path.Clean(name)]; ok {
		return mtimeInfo{FileInfo: fi, mtime: mt}, nil
	}
	return fi, nil
}

func (t *testFS) Stat(name string) (fs.FileInfo, error) {
	fi, err := t.Filesystem.Stat(name)
	return t.wrap(name, fi, err)
}

func (t *testFS) Lstat(name string) (fs.FileInfo, error) {
	fi, err := t.Filesystem.Lstat(name)
	return t.wrap(name, fi, err)
}

func (t *testFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err, ok := t.unreadable[path.Clean(name)]; ok {
		return nil, err
	}
	return t.Filesystem.ReadDir(name)
}

func fsKey(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "."
	}
	return p
}

func newFS(t *testing.T, files map[string]entry) *testFS {
	t.Helper()
	tfs := &testFS{
		Filesystem: memfs.New(),
		mtimes:     map[string]time.Time{},
		unreadable: map[string]error{},
	}
	chmod := tfs.Filesystem.(interface {
		Chmod(string, fs.FileMode) error
	}).Chmod
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		e := files[name]
		key := fsKey(name)
		if e.dir {
			if err := tfs.MkdirAll(key, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", name, err)
			}
		} else {
			f, err := tfs.OpenFile(key, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				t.Fatalf("create %s: %v", name, err)
			}
			if _, err := io.WriteString(f, e.content); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			f.Close()
			if e.mode != 0 {
				if err := chmod(key, e.mode); err != nil {
					t.Fatalf("chmod %s: %v", name, err)
				}
			}
		}
		if !e.mtime.IsZero() {
			tfs.mtimes[key] = e.mtime
		}
	}

	// Creating a file also creates its parents, with the file's mode, so
	// explicit directories get their mode only once everything exists.
	for _, name := range names {
		if e := files[name]; e.dir {
			mode := e.mode
			if mode == 0 {
				mode = 0o755
			}
			if err := chmod(fsKey(name), fs.ModeDir|mode); err != nil {
				t.Fatalf("chmod %s: %v", name, err)
			}
		}
	}
	return tfs
}

// echoImpl and catImpl stand in for the real commands so -exec has
// something to dispatch to.
type echoImpl struct{}

func (echoImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	io.WriteString(ec.Stdout, strings.Join(args, " ")+"\n")
	return nil
}

type catImpl struct{}

func (catImpl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	failed := false
	for _, a := range args {
		data, err := readFile(ec.FS, resolvePath(ec, a))
		if err != nil {
			io.WriteString(ec.Stderr, "cat: "+a+": No such file or directory\n")
			failed = true
			continue
		}
		ec.Stdout.Write(data)
	}
	if failed {
		return interp.ExitStatus(1)
	}
	return nil
}

func readFile(fsys billy.Filesystem, name string) ([]byte, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

type result struct {
	stdout, stderr string
	code           int
	err            error
}

// run executes find with args against fsys, with the shell's working
// directory at dir (shell-absolute; "" means "/").
func run(t *testing.T, fsys billy.Filesystem, dir string, args ...string) result {
	t.Helper()
	reg := registry.New()
	reg.Register("cat", catImpl{})
	reg.Register("echo", echoImpl{})
	reg.Register("find", Impl{})
	if dir != "" {
		if err := reg.Chdir(fsys, dir); err != nil {
			t.Fatalf("chdir %s: %v", dir, err)
		}
	}

	var sh *interp.Runner
	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, sh, args)
		}
	}
	sh, err := interp.New(interp.ExecHandlers(middleware))
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    reg.Pwd(),
		FS:     fsys,
		Runner: sh,
	}
	err = Impl{}.Exec(context.Background(), ec, args)
	code := 0
	var status interp.ExitStatus
	if errors.As(err, &status) {
		code = int(status)
		err = nil
	} else if err != nil {
		code = -1
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code, err: err}
}

type findTest struct {
	name       string
	files      map[string]entry
	unreadable map[string]error
	dir        string
	args       []string
	wantStdout string
	// wantStdoutRE, when set, is matched instead of wantStdout.
	wantStdoutRE string
	wantStderr   string
	// wantStderrContains, when set, is checked instead of wantStderr.
	wantStderrContains string
	wantCode           int
	// after, when set, inspects the filesystem once find returns.
	after func(t *testing.T, fsys billy.Filesystem)
}

func runTable(t *testing.T, tests []findTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := newFS(t, tt.files)
			for p, err := range tt.unreadable {
				fsys.unreadable[fsKey(p)] = err
			}
			got := run(t, fsys, tt.dir, tt.args...)
			if got.err != nil {
				t.Fatalf("unexpected error: %v", got.err)
			}
			if tt.wantStdoutRE != "" {
				if !regexp.MustCompile(tt.wantStdoutRE).MatchString(got.stdout) {
					t.Errorf("stdout = %q, want match for %q", got.stdout, tt.wantStdoutRE)
				}
			} else if got.stdout != tt.wantStdout {
				t.Errorf("stdout:\n got: %q\nwant: %q", got.stdout, tt.wantStdout)
			}
			if tt.wantStderrContains != "" {
				if !strings.Contains(got.stderr, tt.wantStderrContains) {
					t.Errorf("stderr = %q, want it to contain %q", got.stderr, tt.wantStderrContains)
				}
			} else if got.stderr != tt.wantStderr {
				t.Errorf("stderr:\n got: %q\nwant: %q", got.stderr, tt.wantStderr)
			}
			if got.code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got.code, tt.wantCode)
			}
			if tt.after != nil {
				tt.after(t, fsys)
			}
		})
	}
}

func projectFiles() map[string]entry {
	return map[string]entry{
		"/project/README.md":            file("# Project"),
		"/project/src/index.ts":         file("export {}"),
		"/project/src/utils/helpers.ts": file("export function helper() {}"),
		"/project/src/utils/format.ts":  file("export function format() {}"),
		"/project/tests/index.test.ts":  file(`test("works", () => {})`),
		"/project/package.json":         file("{}"),
		"/project/tsconfig.json":        file("{}"),
	}
}

const allTS = "/project/src/index.ts\n/project/src/utils/format.ts\n/project/src/utils/helpers.ts\n/project/tests/index.test.ts\n"

func TestFind_Basic(t *testing.T) {
	runTable(t, []findTest{
		{
			name:  "all files and directories from path",
			files: projectFiles(),
			dir:   "/project",
			args:  []string{"/project"},
			wantStdout: "/project\n/project/README.md\n/project/package.json\n/project/src\n" +
				"/project/src/index.ts\n/project/src/utils\n/project/src/utils/format.ts\n" +
				"/project/src/utils/helpers.ts\n/project/tests\n/project/tests/index.test.ts\n" +
				"/project/tsconfig.json\n",
		},
		{
			name:       "no path defaults to current directory",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/sub/b.txt": file("b")},
			dir:        "/dir",
			args:       nil,
			wantStdout: ".\n./a.txt\n./sub\n./sub/b.txt\n",
		},
		{
			name:       "name pattern",
			files:      projectFiles(),
			args:       []string{"/project", "-name", "*.ts"},
			wantStdout: allTS,
		},
		{
			name:  "type f",
			files: projectFiles(),
			args:  []string{"/project", "-type", "f"},
			wantStdout: "/project/README.md\n/project/package.json\n/project/src/index.ts\n" +
				"/project/src/utils/format.ts\n/project/src/utils/helpers.ts\n" +
				"/project/tests/index.test.ts\n/project/tsconfig.json\n",
		},
		{
			name:       "type d",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "d"},
			wantStdout: "/project\n/project/src\n/project/src/utils\n/project/tests\n",
		},
		{
			name:       "relative dot path",
			files:      projectFiles(),
			dir:        "/project",
			args:       []string{".", "-name", "*.md"},
			wantStdout: "./README.md\n",
		},
		{
			name:       "relative named path",
			files:      projectFiles(),
			dir:        "/project",
			args:       []string{"src", "-path", "src/utils/*"},
			wantStdout: "src/utils/format.ts\nsrc/utils/helpers.ts\n",
		},
		{
			name:       "combine name and type",
			files:      projectFiles(),
			args:       []string{"/project", "-name", "*.ts", "-type", "f"},
			wantStdout: allTS,
		},
		{
			name:       "question mark wildcard",
			files:      projectFiles(),
			args:       []string{"/project", "-name", "???*.json"},
			wantStdout: "/project/package.json\n/project/tsconfig.json\n",
		},
		{
			name:       "non-existent path",
			files:      projectFiles(),
			args:       []string{"/nonexistent"},
			wantStderr: "find: /nonexistent: No such file or directory\n",
			wantCode:   1,
		},
		{
			name:       "non-existent path among others",
			files:      map[string]entry{"/dir1/a.txt": file("a")},
			args:       []string{"/dir1", "/nonexistent", "-type", "f"},
			wantStdout: "/dir1/a.txt\n",
			wantStderr: "find: /nonexistent: No such file or directory\n",
			wantCode:   1,
		},
		{
			name:       "multiple search paths",
			files:      map[string]entry{"/a/file.txt": file("a"), "/b/file.txt": file("b"), "/c/file.txt": file("c")},
			args:       []string{"/a", "/b", "/c", "-name", "*.txt"},
			wantStdout: "/a/file.txt\n/b/file.txt\n/c/file.txt\n",
		},
		{
			name:       "spaces in names",
			files:      map[string]entry{"/dir/file with spaces.txt": file(""), "/dir/another file.txt": file(""), "/dir/normal.txt": file("")},
			args:       []string{"/dir", "-name", "* *"},
			wantStdout: "/dir/another file.txt\n/dir/file with spaces.txt\n",
		},
		{
			name:       "tabs in names",
			files:      map[string]entry{"/dir/file\twith\ttabs.txt": file(""), "/dir/normal.txt": file("")},
			args:       []string{"/dir", "-name", "file\twith\ttabs.txt"},
			wantStdout: "/dir/file\twith\ttabs.txt\n",
		},
		{
			name:       "search from root with dot",
			files:      map[string]entry{"/abc/file.txt": file("")},
			dir:        "/",
			args:       []string{".", "-name", "file.txt"},
			wantStdout: "./abc/file.txt\n",
		},
		{
			name:       "search from slash",
			files:      map[string]entry{"/abc/file.txt": file("")},
			args:       []string{"/", "-name", "*.txt"},
			wantStdout: "/abc/file.txt\n",
		},
		{
			name:       "trailing slash adds no extra slash below it",
			files:      map[string]entry{"/project/src/index.ts": file("")},
			args:       []string{"/project/", "-name", "*.ts"},
			wantStdout: "/project/src/index.ts\n",
		},
		{
			name:       "starting point is printed as given",
			files:      map[string]entry{"/dir/a": file("")},
			args:       []string{"/dir/"},
			wantStdout: "/dir/\n/dir/a\n",
		},
		{
			// POSIX find, EXAMPLES 10.
			name:       "trailing slashes are ignored for name",
			files:      map[string]entry{"/foo": {dir: true}, "/bar": {dir: true}},
			args:       []string{"foo///", "bar///", "-name", "foo", "-o", "-name", "bar?*"},
			wantStdout: "foo///\n",
		},
		{
			name:       "root name is slash",
			files:      map[string]entry{"/a": file("")},
			args:       []string{"/", "-maxdepth", "0", "-name", "/"},
			wantStdout: "/\n",
		},
		{
			name:       "double dash ends options",
			files:      map[string]entry{"/dir/a": file("")},
			args:       []string{"--", "/dir", "-maxdepth", "0"},
			wantStdout: "/dir\n",
		},
		{
			name:       "P option is the default",
			files:      map[string]entry{"/dir/a": file("")},
			args:       []string{"-P", "/dir"},
			wantStdout: "/dir\n/dir/a\n",
		},
		{
			name:       "xdev on a single filesystem changes nothing",
			files:      map[string]entry{"/dir/sub/a": file("")},
			args:       []string{"/dir", "-xdev"},
			wantStdout: "/dir\n/dir/sub\n/dir/sub/a\n",
		},
		{
			name:       "help",
			files:      projectFiles(),
			args:       []string{"--help"},
			wantStderr: helpText(),
		},
		{
			name:       "unknown predicate",
			files:      projectFiles(),
			args:       []string{"/project", "-unknown"},
			wantStderr: "find: unknown predicate '-unknown'\n",
			wantCode:   1,
		},
		{
			name:       "unknown long option",
			files:      projectFiles(),
			args:       []string{"/project", "--badoption"},
			wantStderr: "find: unknown predicate '--badoption'\n",
			wantCode:   1,
		},
		{
			name:       "invalid type",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "x"},
			wantStderr: "find: Unknown argument to -type: x\n",
			wantCode:   1,
		},
		{
			name:       "type takes one letter",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "fd"},
			wantStderr: "find: Unknown argument to -type: fd\n",
			wantCode:   1,
		},
		{
			name:       "path after expression",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "src"},
			wantStderr: "find: paths must precede expression: `src'\n",
			wantCode:   1,
		},
	})
}

func helpText() string {
	var b bytes.Buffer
	usage(&b)
	return b.String()
}

func TestFind_Help(t *testing.T) {
	got := helpText()
	for _, want := range []string{"find - search for files in a directory hierarchy\n", "Usage: find [-H|-P] [path...] [expression]\n", "  -name PATTERN", "  -maxdepth LEVELS"} {
		if !strings.Contains(got, want) {
			t.Errorf("help text missing %q", want)
		}
	}
}

func TestFind_Depth(t *testing.T) {
	runTable(t, []findTest{
		{
			name:       "maxdepth 0",
			files:      projectFiles(),
			args:       []string{"/project", "-maxdepth", "0"},
			wantStdout: "/project\n",
		},
		{
			name:  "maxdepth 1",
			files: projectFiles(),
			args:  []string{"/project", "-maxdepth", "1"},
			wantStdout: "/project\n/project/README.md\n/project/package.json\n/project/src\n" +
				"/project/tests\n/project/tsconfig.json\n",
		},
		{
			name:       "maxdepth 2 with name",
			files:      projectFiles(),
			args:       []string{"/project", "-maxdepth", "2", "-name", "*.ts"},
			wantStdout: "/project/src/index.ts\n/project/tests/index.test.ts\n",
		},
		{
			name:       "mindepth 1",
			files:      projectFiles(),
			args:       []string{"/project", "-mindepth", "1", "-type", "d"},
			wantStdout: "/project/src\n/project/src/utils\n/project/tests\n",
		},
		{
			name:       "mindepth 2",
			files:      projectFiles(),
			args:       []string{"/project", "-mindepth", "2", "-type", "f", "-name", "*.ts"},
			wantStdout: allTS,
		},
		{
			name:       "mindepth and maxdepth",
			files:      projectFiles(),
			args:       []string{"/project", "-mindepth", "1", "-maxdepth", "1", "-type", "f"},
			wantStdout: "/project/README.md\n/project/package.json\n/project/tsconfig.json\n",
		},
		{
			name:       "depth lists contents before directory",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/sub/b.txt": file("b")},
			args:       []string{"/dir", "-depth"},
			wantStdout: "/dir/a.txt\n/dir/sub/b.txt\n/dir/sub\n/dir\n",
		},
		{
			name:       "depth with type f",
			files:      map[string]entry{"/dir/a/1.txt": file("1"), "/dir/b/2.txt": file("2")},
			args:       []string{"/dir", "-depth", "-type", "f"},
			wantStdout: "/dir/a/1.txt\n/dir/b/2.txt\n",
		},
		{
			name:       "prune has no effect under depth",
			files:      map[string]entry{"/dir/skip/hidden.txt": file(""), "/dir/keep/visible.txt": file("")},
			args:       []string{"/dir", "-depth", "-name", "skip", "-prune", "-o", "-type", "f", "-print"},
			wantStdout: "/dir/keep/visible.txt\n/dir/skip/hidden.txt\n",
		},
		{
			name:     "depth with delete removes the tree",
			files:    map[string]entry{"/dir/sub/a.txt": file("a")},
			args:     []string{"/dir/sub", "-depth", "-delete"},
			wantCode: 0,
			after: func(t *testing.T, fsys billy.Filesystem) {
				entries, err := fsys.ReadDir("dir")
				if err != nil {
					t.Fatalf("ReadDir: %v", err)
				}
				if len(entries) != 0 {
					t.Errorf("dir still has %d entries", len(entries))
				}
			},
		},
		{
			name:       "invalid maxdepth",
			files:      projectFiles(),
			args:       []string{"/project", "-maxdepth", "-1"},
			wantStderr: "find: invalid argument `-1' to `-maxdepth'\n",
			wantCode:   1,
		},
		{
			name:       "missing mindepth argument",
			files:      projectFiles(),
			args:       []string{"/project", "-mindepth"},
			wantStderr: "find: missing argument to `-mindepth'\n",
			wantCode:   1,
		},
	})
}

func TestFind_Patterns(t *testing.T) {
	runTable(t, []findTest{
		{
			name: "iname",
			files: map[string]entry{
				"/dir/README.md": file(""), "/dir/readme.txt": file(""),
				"/dir/Readme.rst": file(""), "/dir/other.txt": file(""),
			},
			args:       []string{"/dir", "-iname", "readme*"},
			wantStdout: "/dir/README.md\n/dir/Readme.rst\n/dir/readme.txt\n",
		},
		{
			name:       "iname uppercase pattern",
			files:      map[string]entry{"/dir/config.json": file("")},
			args:       []string{"/dir", "-iname", "CONFIG.JSON"},
			wantStdout: "/dir/config.json\n",
		},
		{
			name:       "path",
			files:      projectFiles(),
			args:       []string{"/project", "-path", "*/utils/*"},
			wantStdout: "/project/src/utils/format.ts\n/project/src/utils/helpers.ts\n",
		},
		{
			name:       "path substring",
			files:      projectFiles(),
			args:       []string{"/project", "-path", "*tests*"},
			wantStdout: "/project/tests\n/project/tests/index.test.ts\n",
		},
		{
			name:       "ipath",
			files:      map[string]entry{"/Project/SRC/file.ts": file(""), "/Project/src/other.ts": file("")},
			args:       []string{"/Project", "-ipath", "*src*"},
			wantStdout: "/Project/SRC\n/Project/SRC/file.ts\n/Project/src\n/Project/src/other.ts\n",
		},
		{
			name: "path with literal segments",
			files: map[string]entry{
				"/repos/project1/pulls/1.json":  file("{}"),
				"/repos/project1/pulls/2.json":  file("{}"),
				"/repos/project1/issues/1.json": file("{}"),
				"/repos/project2/pulls/3.json":  file("{}"),
				"/repos/project2/other/4.json":  file("{}"),
			},
			args:       []string{"/repos", "-path", "*/pulls/*.json", "-type", "f"},
			wantStdout: "/repos/project1/pulls/1.json\n/repos/project1/pulls/2.json\n/repos/project2/pulls/3.json\n",
		},
		{
			name:       "path with dot prefix",
			files:      map[string]entry{"/project/src/index.ts": file(""), "/project/src/utils.ts": file(""), "/project/lib/index.ts": file("")},
			dir:        "/project",
			args:       []string{".", "-path", "./src/*", "-type", "f"},
			wantStdout: "./src/index.ts\n./src/utils.ts\n",
		},
		{
			name: "path star crosses directories",
			files: map[string]entry{
				"/repo/pulls/direct.json":      file(""),
				"/repo/pulls/nested/deep.json": file(""),
				"/repo/pulls/nested/deep.txt":  file(""),
			},
			args:       []string{"/repo", "-type", "f", "-path", "*/pulls/*.json", "-print"},
			wantStdout: "/repo/pulls/direct.json\n/repo/pulls/nested/deep.json\n",
		},
		{
			name:       "bracket negation",
			files:      map[string]entry{"/dir/a1": file(""), "/dir/b1": file(""), "/dir/c1": file("")},
			args:       []string{"/dir", "-name", "[!a]1"},
			wantStdout: "/dir/b1\n/dir/c1\n",
		},
		{
			name:       "named character class",
			files:      map[string]entry{"/dir/7": file(""), "/dir/a": file(""), "/dir/42": file("")},
			args:       []string{"/dir", "-name", "[[:digit:]]"},
			wantStdout: "/dir/7\n",
		},
		{
			name:       "regex",
			files:      map[string]entry{"/dir/file.txt": file(""), "/dir/file.js": file(""), "/dir/sub/other.txt": file("")},
			args:       []string{"/dir", "-regex", `.*\.txt`},
			wantStdout: "/dir/file.txt\n/dir/sub/other.txt\n",
		},
		{
			name:       "regex full path",
			files:      map[string]entry{"/dir/src/file.ts": file(""), "/dir/test/file.ts": file("")},
			args:       []string{"/dir", "-regex", ".*/src/.*"},
			wantStdout: "/dir/src/file.ts\n",
		},
		{
			name:       "regex is anchored to the whole path",
			files:      map[string]entry{"/dir/src/file.ts": file("")},
			args:       []string{"/dir", "-regex", "src"},
			wantStdout: "",
		},
		{
			name:       "iregex",
			files:      map[string]entry{"/dir/FILE.TXT": file(""), "/dir/file.txt": file(""), "/dir/other.js": file("")},
			args:       []string{"/dir", "-iregex", `.*\.txt`},
			wantStdout: "/dir/FILE.TXT\n/dir/file.txt\n",
		},
		{
			name: "regex character class",
			files: map[string]entry{
				"/dir/test1.ts": file(""), "/dir/test2.ts": file(""),
				"/dir/test10.ts": file(""), "/dir/other.ts": file(""),
			},
			args:       []string{"/dir", "-regex", `.*/test[0-9]\.ts`},
			wantStdout: "/dir/test1.ts\n/dir/test2.ts\n",
		},
		{
			name:       "invalid regex",
			files:      map[string]entry{"/dir/a": file("")},
			args:       []string{"/dir", "-regex", "("},
			wantStderr: "find: invalid argument `(' to `-regex'\n",
			wantCode:   1,
		},
		{
			name:       "empty files",
			files:      map[string]entry{"/dir/empty.txt": file(""), "/dir/notempty.txt": file("content")},
			args:       []string{"/dir", "-empty", "-type", "f"},
			wantStdout: "/dir/empty.txt\n",
		},
		{
			name:       "empty directories",
			files:      map[string]entry{"/dir/notempty/file.txt": file("content"), "/dir/emptydir": {dir: true}},
			args:       []string{"/dir", "-empty", "-type", "d"},
			wantStdout: "/dir/emptydir\n",
		},
		{
			name: "prune",
			files: map[string]entry{
				"/dir/skip/hidden.txt":        file(""),
				"/dir/skip/inside/nested.txt": file(""),
				"/dir/include/file.txt":       file(""),
			},
			args:       []string{"/dir", "-name", "skip", "-prune", "-o", "-type", "f", "-print"},
			wantStdout: "/dir/include/file.txt\n",
		},
		{
			name: "prune multiple directories",
			files: map[string]entry{
				"/dir/node_modules/pkg/index.js": file(""),
				"/dir/.git/objects/abc":          file(""),
				"/dir/src/main.ts":               file(""),
			},
			args:       []string{"/dir", "(", "-name", "node_modules", "-o", "-name", ".git", ")", "-prune", "-o", "-type", "f", "-print"},
			wantStdout: "/dir/src/main.ts\n",
		},
		{
			name: "prune with type d",
			files: map[string]entry{
				"/project/dist/bundle.js": file(""),
				"/project/src/index.ts":   file(""),
				"/project/README.md":      file(""),
			},
			args:       []string{"/project", "-type", "d", "-name", "dist", "-prune", "-o", "-type", "f", "-print"},
			wantStdout: "/project/README.md\n/project/src/index.ts\n",
		},
		{
			name:       "pruned directory itself is printed",
			files:      map[string]entry{"/dir/skip/file.txt": file(""), "/dir/keep/file.txt": file("")},
			args:       []string{"/dir", "-name", "skip", "-prune"},
			wantStdout: "/dir/skip\n",
		},
	})
}

func TestFind_Operators(t *testing.T) {
	abc := func() map[string]entry {
		return map[string]entry{"/dir/a.txt": file("a"), "/dir/b.txt": file("b"), "/dir/c.txt": file("c")}
	}
	tsAndJSON := "/project/package.json\n/project/src/index.ts\n/project/src/utils/format.ts\n" +
		"/project/src/utils/helpers.ts\n/project/tests/index.test.ts\n/project/tsconfig.json\n"
	nonTS := "/project/README.md\n/project/package.json\n/project/tsconfig.json\n"

	runTable(t, []findTest{
		{
			name:       "or",
			files:      projectFiles(),
			args:       []string{"/project", "-name", "*.md", "-o", "-name", "*.json"},
			wantStdout: nonTS,
		},
		{
			name:       "-or alias",
			files:      projectFiles(),
			args:       []string{"/project", "-name", "*.md", "-or", "-name", "*.json"},
			wantStdout: nonTS,
		},
		{
			name:  "and binds tighter than or",
			files: projectFiles(),
			args:  []string{"/project", "-type", "f", "-name", "*.ts", "-o", "-type", "d"},
			wantStdout: "/project\n/project/src\n/project/src/index.ts\n/project/src/utils\n" +
				"/project/src/utils/format.ts\n/project/src/utils/helpers.ts\n/project/tests\n" +
				"/project/tests/index.test.ts\n",
		},
		{
			name:       "a or b and c",
			files:      abc(),
			args:       []string{"/dir", "-type", "f", "-name", "a.txt", "-o", "-name", "b.txt", "-name", "c.txt"},
			wantStdout: "/dir/a.txt\n",
		},
		{
			name:       "explicit and",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "-a", "-name", "*.ts"},
			wantStdout: allTS,
		},
		{
			name:       "-and alias",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "-and", "-name", "*.json"},
			wantStdout: "/project/package.json\n/project/tsconfig.json\n",
		},
		{
			name:       "parentheses",
			files:      projectFiles(),
			args:       []string{"/project", "(", "-name", "*.ts", "-o", "-name", "*.json", ")"},
			wantStdout: tsAndJSON,
		},
		{
			name:       "escaped parentheses",
			files:      projectFiles(),
			args:       []string{"/project", `\(`, "-name", "*.ts", "-o", "-name", "*.json", `\)`},
			wantStdout: tsAndJSON,
		},
		{
			name:       "nested parentheses",
			files:      projectFiles(),
			args:       []string{"/project", "(", "-type", "f", "(", "-name", "*.ts", "-o", "-name", "*.json", ")", ")"},
			wantStdout: tsAndJSON,
		},
		{
			name:       "parentheses override precedence",
			files:      abc(),
			args:       []string{"/dir", "(", "-name", "a.txt", "-o", "-name", "b.txt", ")", "-type", "f"},
			wantStdout: "/dir/a.txt\n/dir/b.txt\n",
		},
		{
			name:       "exec inside grouped expression",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "(", "-name", "*.md", "-o", "-name", "*.json", ")", "-exec", "cat", "{}", ";"},
			wantStdout: "# Project{}{}",
		},
		{
			name:       "not",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "-not", "-name", "*.ts"},
			wantStdout: nonTS,
		},
		{
			name:       "bang",
			files:      projectFiles(),
			args:       []string{"/project", "-type", "f", "!", "-name", "*.json", "!", "-name", "*.md"},
			wantStdout: allTS,
		},
		{
			name:       "not type",
			files:      projectFiles(),
			args:       []string{"/project", "-maxdepth", "1", "-not", "-type", "d"},
			wantStdout: nonTS,
		},
	})
}

func TestFind_Predicates(t *testing.T) {
	now := time.Now()
	day := 24 * time.Hour

	runTable(t, []findTest{
		{
			name:       "mtime 0",
			files:      map[string]entry{"/dir/today.txt": {content: "t", mtime: now}, "/dir/old.txt": {content: "o", mtime: now.Add(-3 * day)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "0"},
			wantStdout: "/dir/today.txt\n",
		},
		{
			name:       "mtime more",
			files:      map[string]entry{"/dir/recent.txt": {content: "r", mtime: now}, "/dir/old.txt": {content: "o", mtime: now.Add(-10 * day)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "+7"},
			wantStdout: "/dir/old.txt\n",
		},
		{
			name:       "mtime less",
			files:      map[string]entry{"/dir/recent.txt": {content: "r", mtime: now}, "/dir/old.txt": {content: "o", mtime: now.Add(-10 * day)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "-7"},
			wantStdout: "/dir/recent.txt\n",
		},
		{
			name:       "mtime exact",
			files:      map[string]entry{"/dir/two-days.txt": {content: "2", mtime: now.Add(-2 * day)}, "/dir/today.txt": {content: "t", mtime: now}},
			args:       []string{"/dir", "-type", "f", "-mtime", "2"},
			wantStdout: "/dir/two-days.txt\n",
		},
		{
			name:       "mtime more discards the remainder",
			files:      map[string]entry{"/dir/f": {content: "x", mtime: now.Add(-7*day - 12*time.Hour)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "+7"},
			wantStdout: "",
		},
		{
			name:       "mtime exact discards the remainder",
			files:      map[string]entry{"/dir/f": {content: "x", mtime: now.Add(-7*day - 12*time.Hour)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "7"},
			wantStdout: "/dir/f\n",
		},
		{
			name:       "mtime less discards the remainder",
			files:      map[string]entry{"/dir/f": {content: "x", mtime: now.Add(-7*day - 12*time.Hour)}},
			args:       []string{"/dir", "-type", "f", "-mtime", "-8"},
			wantStdout: "/dir/f\n",
		},
		{
			name: "newer",
			files: map[string]entry{
				"/ref.txt":       {content: "ref", mtime: now.Add(-time.Minute)},
				"/dir/newer.txt": {content: "n", mtime: now},
				"/dir/older.txt": {content: "o", mtime: now.Add(-2 * time.Minute)},
			},
			args:       []string{"/dir", "-type", "f", "-newer", "/ref.txt"},
			wantStdout: "/dir/newer.txt\n",
		},
		{
			name:       "newer with missing reference",
			files:      map[string]entry{"/dir/file.txt": file("content")},
			args:       []string{"/dir", "-type", "f", "-newer", "/nonexistent.txt"},
			wantStdout: "",
		},
		{
			name:       "size more bytes",
			files:      map[string]entry{"/dir/large.txt": file(strings.Repeat("x", 1000)), "/dir/small.txt": file("tiny")},
			args:       []string{"/dir", "-type", "f", "-size", "+100c"},
			wantStdout: "/dir/large.txt\n",
		},
		{
			name:       "size less bytes",
			files:      map[string]entry{"/dir/large.txt": file(strings.Repeat("x", 1000)), "/dir/small.txt": file("tiny")},
			args:       []string{"/dir", "-type", "f", "-size", "-100c"},
			wantStdout: "/dir/small.txt\n",
		},
		{
			name:       "size exact bytes",
			files:      map[string]entry{"/dir/exact.txt": file("12345"), "/dir/other.txt": file("1234")},
			args:       []string{"/dir", "-type", "f", "-size", "5c"},
			wantStdout: "/dir/exact.txt\n",
		},
		{
			name:       "size kilobytes",
			files:      map[string]entry{"/dir/large.txt": file(strings.Repeat("x", 2048)), "/dir/small.txt": file("tiny")},
			args:       []string{"/dir", "-type", "f", "-size", "+1k"},
			wantStdout: "/dir/large.txt\n",
		},
		{
			name:       "size megabytes",
			files:      map[string]entry{"/dir/small.txt": file("tiny")},
			args:       []string{"/dir", "-type", "f", "-size", "-1M"},
			wantStdout: "/dir/small.txt\n",
		},
		{
			name:       "size less counts whole blocks",
			files:      map[string]entry{"/dir/one.txt": file("x"), "/dir/empty.txt": file("")},
			args:       []string{"/dir", "-type", "f", "-size", "-1"},
			wantStdout: "/dir/empty.txt\n",
		},
		{
			name:       "size more counts whole blocks",
			files:      map[string]entry{"/dir/full.txt": file(strings.Repeat("x", 512)), "/dir/over.txt": file(strings.Repeat("x", 513))},
			args:       []string{"/dir", "-type", "f", "-size", "+1"},
			wantStdout: "/dir/over.txt\n",
		},
		{
			name:       "size default 512 byte blocks rounds up",
			files:      map[string]entry{"/dir/one.txt": file("x"), "/dir/two.txt": file(strings.Repeat("x", 513))},
			args:       []string{"/dir", "-type", "f", "-size", "1"},
			wantStdout: "/dir/one.txt\n",
		},
	})
}

func TestFind_Perm(t *testing.T) {
	runTable(t, []findTest{
		{
			name: "exact 644",
			files: map[string]entry{
				"/test/file1.txt": {content: "a", mode: 0o644},
				"/test/file2.txt": {content: "b", mode: 0o755},
				"/test/file3.txt": {content: "c", mode: 0o644},
			},
			args:       []string{"/test", "-type", "f", "-perm", "644"},
			wantStdout: "/test/file1.txt\n/test/file3.txt\n",
		},
		{
			name: "exact mode includes setuid",
			files: map[string]entry{
				"/test/plain":  {content: "a", mode: 0o755},
				"/test/setuid": {content: "b", mode: fs.ModeSetuid | 0o755},
			},
			args:       []string{"/test", "-type", "f", "-perm", "4755"},
			wantStdout: "/test/setuid\n",
		},
		{
			name: "exact mode excludes setuid",
			files: map[string]entry{
				"/test/plain":  {content: "a", mode: 0o755},
				"/test/setuid": {content: "b", mode: fs.ModeSetuid | 0o755},
			},
			args:       []string{"/test", "-type", "f", "-perm", "755"},
			wantStdout: "/test/plain\n",
		},
		{
			name: "all bits with setgid and sticky",
			files: map[string]entry{
				"/test/plain":  {content: "a", mode: 0o755},
				"/test/setgid": {content: "b", mode: fs.ModeSetgid | 0o755},
				"/test/both":   {content: "c", mode: fs.ModeSetgid | fs.ModeSticky | 0o755},
			},
			args:       []string{"/test", "-type", "f", "-perm", "-3000"},
			wantStdout: "/test/both\n",
		},
		{
			name: "all bits",
			files: map[string]entry{
				"/test/exec.sh":  {content: "a", mode: 0o755},
				"/test/data.txt": {content: "b", mode: 0o644},
				"/test/other.sh": {content: "c", mode: 0o700},
			},
			args:       []string{"/test", "-type", "f", "-perm", "-100"},
			wantStdout: "/test/exec.sh\n/test/other.sh\n",
		},
		{
			name: "group read",
			files: map[string]entry{
				"/test/readable.txt": {content: "a", mode: 0o644},
				"/test/private.txt":  {content: "b", mode: 0o600},
			},
			args:       []string{"/test", "-type", "f", "-perm", "-040"},
			wantStdout: "/test/readable.txt\n",
		},
		{
			name: "any bit",
			files: map[string]entry{
				"/test/user_exec.sh":  {content: "a", mode: 0o700},
				"/test/group_exec.sh": {content: "b", mode: 0o070},
				"/test/other_exec.sh": {content: "c", mode: 0o007},
				"/test/no_exec.txt":   {content: "d", mode: 0o644},
			},
			args:       []string{"/test", "-type", "f", "-perm", "/111"},
			wantStdout: "/test/group_exec.sh\n/test/other_exec.sh\n/test/user_exec.sh\n",
		},
	})
}

func TestFind_Actions(t *testing.T) {
	noEntries := func(dir string, want ...string) func(*testing.T, billy.Filesystem) {
		return func(t *testing.T, fsys billy.Filesystem) {
			t.Helper()
			entries, err := fsys.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir(%s): %v", dir, err)
			}
			var got []string
			for _, e := range entries {
				got = append(got, e.Name())
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s contains %v, want %v", dir, got, want)
			}
		}
	}

	runTable(t, []findTest{
		{
			name:       "exec batch",
			files:      map[string]entry{"/dir/a.txt": file("aaa"), "/dir/b.txt": file("bbb"), "/dir/c.txt": file("ccc")},
			args:       []string{"/dir", "-type", "f", "-name", "*.txt", "-exec", "cat", "{}", "+"},
			wantStdout: "aaabbbccc",
		},
		{
			name:       "exec batch passes all paths at once",
			files:      map[string]entry{"/dir/a.txt": file(""), "/dir/b.txt": file(""), "/dir/c.txt": file("")},
			args:       []string{"/dir", "-name", "*.txt", "-exec", "echo", "{}", "+"},
			wantStdout: "/dir/a.txt /dir/b.txt /dir/c.txt\n",
		},
		{
			name:       "exec batch runs after other actions",
			files:      map[string]entry{"/dir/a.txt": file(""), "/dir/b.txt": file("")},
			args:       []string{"/dir", "-type", "f", "-print", "-exec", "echo", "batch", "{}", "+"},
			wantStdout: "/dir/a.txt\n/dir/b.txt\nbatch /dir/a.txt /dir/b.txt\n",
		},
		{
			name:       "exec plus is an argument unless it follows braces",
			files:      map[string]entry{"/dir/a.txt": file("")},
			args:       []string{"/dir", "-type", "f", "-exec", "echo", "+", "{}", ";"},
			wantStdout: "+ /dir/a.txt\n",
		},
		{
			name:       "exec per file",
			files:      map[string]entry{"/dir/a.txt": file("aaa"), "/dir/b.txt": file("bbb")},
			args:       []string{"/dir", "-type", "f", "-name", "*.txt", "-exec", "echo", "FILE:", "{}", ";"},
			wantStdout: "FILE: /dir/a.txt\nFILE: /dir/b.txt\n",
		},
		{
			name:       "exec cat per file",
			files:      map[string]entry{"/dir/a.txt": file("content a"), "/dir/b.txt": file("content b")},
			args:       []string{"/dir", "-name", "*.txt", "-exec", "cat", "{}", ";"},
			wantStdout: "content acontent b",
		},
		{
			name:       "exec multiple braces",
			files:      map[string]entry{"/dir/test.txt": file("")},
			args:       []string{"/dir", "-name", "test.txt", "-exec", "echo", "{}", "is", "{}", ";"},
			wantStdout: "/dir/test.txt is /dir/test.txt\n",
		},
		{
			name:       "exec replaces braces inside an argument",
			files:      map[string]entry{"/dir/test.txt": file("")},
			args:       []string{"/dir", "-name", "test.txt", "-exec", "echo", "{}.bak", ";"},
			wantStdout: "/dir/test.txt.bak\n",
		},
		{
			name:       "exec keeps arguments with spaces whole",
			files:      map[string]entry{"/dir/a b.txt": file("spaced")},
			args:       []string{"/dir", "-type", "f", "-exec", "cat", "{}", ";"},
			wantStdout: "spaced",
		},
		{
			name:               "exec propagates exit codes",
			files:              map[string]entry{"/dir/file.txt": file("")},
			args:               []string{"/dir", "-name", "file.txt", "-exec", "cat", "/nonexistent", ";"},
			wantStderrContains: "No such file",
			wantCode:           1,
		},
		{
			name:       "exec suppresses default print",
			files:      map[string]entry{"/dir/a.txt": file(""), "/dir/b.txt": file("")},
			args:       []string{"/dir", "-name", "*.txt", "-exec", "echo", "found", ";"},
			wantStdout: "found\nfound\n",
		},
		{
			name:       "exec with no matches",
			files:      map[string]entry{"/dir/file.txt": file("")},
			args:       []string{"/dir", "-name", "*.log", "-exec", "echo", "{}", ";"},
			wantStdout: "",
		},
		{
			name:       "exec with maxdepth",
			files:      map[string]entry{"/dir/file.txt": file("top"), "/dir/sub/file.txt": file("nested")},
			args:       []string{"/dir", "-maxdepth", "1", "-type", "f", "-exec", "cat", "{}", ";"},
			wantStdout: "top",
		},
		{
			name:       "exec with relative paths from working directory",
			files:      map[string]entry{"/project/src/a.txt": file("rel")},
			dir:        "/project",
			args:       []string{".", "-type", "f", "-exec", "cat", "{}", ";"},
			wantStdout: "rel",
		},
		{
			name:       "print0",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/b.txt": file("b")},
			args:       []string{"/dir", "-type", "f", "-name", "*.txt", "-print0"},
			wantStdout: "/dir/a.txt\x00/dir/b.txt\x00",
		},
		{
			name:       "explicit print",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/b.md": file("b")},
			args:       []string{"/dir", "-name", "*.txt", "-print"},
			wantStdout: "/dir/a.txt\n",
		},
		{
			name:       "print only in its branch",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/b.md": file("b"), "/dir/c.json": file("c")},
			args:       []string{"/dir", "-name", "*.txt", "-print", "-o", "-name", "*.md"},
			wantStdout: "/dir/a.txt\n",
		},
		{
			name:  "delete files",
			files: map[string]entry{"/dir/a.txt": file("a"), "/dir/b.txt": file("b"), "/dir/keep.md": file("keep")},
			args:  []string{"/dir", "-type", "f", "-name", "*.txt", "-delete"},
			after: noEntries("dir", "keep.md"),
		},
		{
			name:  "delete empty directories",
			files: map[string]entry{"/dir/subdir": {dir: true}},
			args:  []string{"/dir", "-type", "d", "-empty", "-delete"},
			// Actions run after the walk, so /dir still held subdir when
			// it was tested. just-bash leaves /dir behind the same way.
			after: noEntries("dir"),
		},
		{
			name:       "delete refuses non-empty directories",
			files:      map[string]entry{"/dir/subdir/file.txt": file("content")},
			args:       []string{"/dir", "-type", "d", "-name", "subdir", "-delete"},
			wantStderr: "find: cannot delete '/dir/subdir': Directory not empty\n",
			wantCode:   1,
			after:      noEntries("dir/subdir", "file.txt"),
		},
		{
			name:  "delete deepest first",
			files: map[string]entry{"/dir/a.txt": file("a"), "/dir/sub/b.txt": file("b"), "/dir/sub/deep/c.txt": file("c")},
			args:  []string{"/dir", "-name", "*.txt", "-delete"},
			after: func(t *testing.T, fsys billy.Filesystem) {
				got := run(t, fsys, "", "/dir", "-type", "f")
				if got.stdout != "" {
					t.Errorf("files left behind: %q", got.stdout)
				}
			},
		},
	})
}

func TestFind_ExecWithoutRunner(t *testing.T) {
	fsys := newFS(t, map[string]entry{"/dir/a.txt": file("a")})
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{Stdout: &stdout, Stderr: &stderr, Dir: ".", FS: fsys}

	err := Impl{}.Exec(context.Background(), ec, []string{"/dir", "-print", "-exec", "echo", "{}", ";"})
	var status interp.ExitStatus
	if !errors.As(err, &status) || status != 1 {
		t.Errorf("err = %v, want ExitStatus(1)", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if want := "find: -exec not supported in this context\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestFind_Printf(t *testing.T) {
	runTable(t, []findTest{
		{
			name:       "filename",
			files:      map[string]entry{"/dir/a.txt": file("aaa"), "/dir/b.txt": file("bbb")},
			args:       []string{"/dir", "-type", "f", "-name", "*.txt", "-printf", `%f\n`},
			wantStdout: "a.txt\nb.txt\n",
		},
		{
			name:       "full path",
			files:      map[string]entry{"/dir/a.txt": file("aaa")},
			args:       []string{"/dir", "-type", "f", "-printf", `%p\n`},
			wantStdout: "/dir/a.txt\n",
		},
		{
			name:       "dirname",
			files:      map[string]entry{"/dir/sub/file.txt": file("content")},
			args:       []string{"/dir", "-type", "f", "-printf", `%h\n`},
			wantStdout: "/dir/sub\n",
		},
		{
			name:       "size",
			files:      map[string]entry{"/dir/a.txt": file("hello")},
			args:       []string{"/dir", "-type", "f", "-printf", `%f %s\n`},
			wantStdout: "a.txt 5\n",
		},
		{
			name:       "depth",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/sub/b.txt": file("b"), "/dir/sub/deep/c.txt": file("c")},
			args:       []string{"/dir", "-type", "f", "-printf", `%d %f\n`},
			wantStdout: "1 a.txt\n2 b.txt\n3 c.txt\n",
		},
		{
			name:       "octal mode",
			files:      map[string]entry{"/dir/exec.sh": {content: "#!/bin/bash", mode: 0o755}, "/dir/file.txt": {content: "text", mode: 0o644}},
			args:       []string{"/dir", "-type", "f", "-printf", `%f %m\n`},
			wantStdout: "exec.sh 755\nfile.txt 644\n",
		},
		{
			name:       "symbolic mode",
			files:      map[string]entry{"/dir/exec.sh": {content: "#!/bin/bash", mode: 0o755}},
			args:       []string{"/dir", "-type", "f", "-printf", `%M %f\n`},
			wantStdout: "-rwxr-xr-x exec.sh\n",
		},
		{
			name:       "symbolic mode for directories",
			files:      map[string]entry{"/dir": {dir: true}, "/dir/sub": {dir: true}, "/dir/sub/file.txt": file("content")},
			args:       []string{"/dir", "-type", "d", "-printf", `%M %f\n`},
			wantStdout: "drwxr-xr-x dir\ndrwxr-xr-x sub\n",
		},
		{
			name:       "path without starting point",
			files:      map[string]entry{"/dir/sub/file.txt": file("content")},
			args:       []string{"/dir", "-type", "f", "-printf", `%P\n`},
			wantStdout: "sub/file.txt\n",
		},
		{
			name:       "path without starting point that ends in slash",
			files:      map[string]entry{"/dir/sub/file.txt": file("content")},
			args:       []string{"/dir/", "-printf", `[%P]\n`},
			wantStdout: "[]\n[sub]\n[sub/file.txt]\n",
		},
		{
			name:       "literal percent",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `100%% done\n`},
			wantStdout: "100% done\n",
		},
		{
			name:       "multiple directives",
			files:      map[string]entry{"/dir/file.txt": file("hello")},
			args:       []string{"/dir", "-type", "f", "-printf", `%f: %s bytes at %p\n`},
			wantStdout: "file.txt: 5 bytes at /dir/file.txt\n",
		},
		{
			name:       "right justify",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%10f]\n`},
			wantStdout: "[     a.txt]\n",
		},
		{
			name:       "left justify",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%-10f]\n`},
			wantStdout: "[a.txt     ]\n",
		},
		{
			name:       "precision",
			files:      map[string]entry{"/dir/longfilename.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%.3f]\n`},
			wantStdout: "[lon]\n",
		},
		{
			name:       "width and precision",
			files:      map[string]entry{"/dir/longfilename.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%-10.5f]\n`},
			wantStdout: "[longf     ]\n",
		},
		{
			name:       "width on size",
			files:      map[string]entry{"/dir/a.txt": file("hello")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%10s]\n`},
			wantStdout: "[         5]\n",
		},
		{
			name:       "width on depth",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%5d]\n`},
			wantStdout: "[    1]\n",
		},
		{
			name:       "width on mode",
			files:      map[string]entry{"/dir/a.txt": {content: "a", mode: 0o644}},
			args:       []string{"/dir", "-type", "f", "-printf", `[%5m]\n`},
			wantStdout: "[  644]\n",
		},
		{
			name:       "width on symbolic mode",
			files:      map[string]entry{"/dir/a.txt": {content: "a", mode: 0o644}},
			args:       []string{"/dir", "-type", "f", "-printf", `[%15M]\n`},
			wantStdout: "[     -rw-r--r--]\n",
		},
		{
			name:       "width on path",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `[%-20p]\n`},
			wantStdout: "[/dir/a.txt          ]\n",
		},
		{
			name:       "tab escape",
			files:      map[string]entry{"/dir/a.txt": file("hello")},
			args:       []string{"/dir", "-type", "f", "-printf", `%f\t%s\n`},
			wantStdout: "a.txt\t5\n",
		},
		{
			name:       "backslash escape",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `%f\\\n`},
			wantStdout: "a.txt\\\n",
		},
		{
			name:       "nul escape",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/b.txt": file("b")},
			args:       []string{"/dir", "-type", "f", "-name", "*.txt", "-printf", `%f\0`},
			wantStdout: "a.txt\x00b.txt\x00",
		},
		{
			name:       "ansi escape",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `\e[32m%f\e[0m\n`},
			wantStdout: "\x1b[32ma.txt\x1b[0m\n",
		},
		{
			name:       "unicode escape",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `\u2714 %f\n`},
			wantStdout: "\u2714 a.txt\n",
		},
		{
			name:       "long unicode escape",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `\U1F4C4 %f\n`},
			wantStdout: "\U0001F4C4 a.txt\n",
		},
		{
			name:       "unknown directive is kept",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", `%z %5z\n`},
			wantStdout: "%z %5z\n",
		},
		{
			name:         "epoch seconds",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%T@\n`},
			wantStdoutRE: `^\d+(\.\d+)?\n$`,
		},
		{
			name:         "year",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%TY\n`},
			wantStdoutRE: `^\d{4}\n$`,
		},
		{
			name:         "month and day",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%Tm %Td\n`},
			wantStdoutRE: `^(0[1-9]|1[0-2]) (0[1-9]|[12][0-9]|3[01])\n$`,
		},
		{
			name:         "time fields",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%TH:%TM:%TS %TT\n`},
			wantStdoutRE: `^\d{2}:\d{2}:\d{2} \d{2}:\d{2}:\d{2}\n$`,
		},
		{
			name:         "date",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%TF\n`},
			wantStdoutRE: `^\d{4}-\d{2}-\d{2}\n$`,
		},
		{
			name:         "ctime",
			files:        map[string]entry{"/dir/a.txt": file("a")},
			args:         []string{"/dir", "-type", "f", "-printf", `%t\n`},
			wantStdoutRE: `^(Mon|Tue|Wed|Thu|Fri|Sat|Sun) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) [ 0-3]\d \d{2}:\d{2}:\d{2} \d{4}\n$`,
		},
		{
			name:       "with size filter",
			files:      map[string]entry{"/dir/small.txt": file("a"), "/dir/large.txt": file("aaaaaaaaaa")},
			args:       []string{"/dir", "-type", "f", "-size", "+5c", "-printf", `%f: %s bytes\n`},
			wantStdout: "large.txt: 10 bytes\n",
		},
		{
			name:       "with maxdepth",
			files:      map[string]entry{"/dir/a.txt": file("a"), "/dir/sub/b.txt": file("b"), "/dir/sub/deep/c.txt": file("c")},
			args:       []string{"/dir", "-maxdepth", "2", "-type", "f", "-printf", `%f\n`},
			wantStdout: "a.txt\nb.txt\n",
		},
		{
			name:       "width over the limit",
			files:      map[string]entry{"/dir/a.txt": file("a")},
			args:       []string{"/dir", "-type", "f", "-printf", "%99999999999f"},
			wantStderr: "find: format width limit exceeded (10485760 bytes)\n",
			wantCode:   1,
		},
	})
}

func TestFind_FailClosed(t *testing.T) {
	destructive := func() map[string]entry {
		return map[string]entry{"/dir/keep": file("keep"), "/dir/remove": file("remove")}
	}
	untouched := func(t *testing.T, fsys billy.Filesystem) {
		t.Helper()
		entries, err := fsys.ReadDir("dir")
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("dir has %d entries, want 2", len(entries))
		}
	}

	var tests []findTest
	for _, tt := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"/dir", "-size", "invalid", "-delete"}, "find: invalid argument `invalid' to `-size'\n"},
		{[]string{"/dir", "-size", "9007199254740991G", "-delete"}, "find: invalid argument `9007199254740991G' to `-size'\n"},
		{[]string{"/dir", "-mtime", "1x", "-delete"}, "find: invalid argument `1x' to `-mtime'\n"},
		{[]string{"/dir", "-perm", "755x", "-delete"}, "find: invalid argument `755x' to `-perm'\n"},
		{[]string{"/dir", "-maxdepth", "nope", "-delete"}, "find: invalid argument `nope' to `-maxdepth'\n"},
		{[]string{"/dir", "-name"}, "find: missing argument to `-name'\n"},
		{[]string{"/dir", "-name", "keep", "-o"}, "find: expected an expression after `-o'\n"},
		{[]string{"/dir", "-name", "keep", "-a"}, "find: expected an expression after `-a'\n"},
		{[]string{"/dir", "!"}, "find: expected an expression after `!'\n"},
		{[]string{"/dir", "(", "-name", "keep"}, "find: missing closing `)'\n"},
		{[]string{"/dir", "-name", "keep", ")"}, "find: unexpected `)'\n"},
		{[]string{"/dir", "-exec", ";"}, "find: missing argument to `-exec'\n"},
		{[]string{"/dir", "-exec", "echo", "{}"}, "find: missing argument to `-exec'\n"},
		{[]string{"/dir", "-exec", "echo", "+"}, "find: missing argument to `-exec'\n"},
		{[]string{"/dir", "-exec", "echo", "{}", "{}", "+"}, "find: invalid argument `+' to `-exec'\n"},
		{[]string{"-L", "/dir", "-delete"}, "find: -L is not supported\n"},
		{[]string{"/dir", "!", "-delete"}, "find: refusing to evaluate `-delete' under negation\n"},
		{[]string{"/dir", "-name", "remove", "-o", "-regex", "(", "-delete"}, "find: invalid argument `(' to `-regex'\n"},
		{[]string{"/dir", "-regex", ".*)|(?:.*", "-delete"}, "find: invalid argument `.*)|(?:.*' to `-regex'\n"},
		{[]string{"/dir", "-type", "f", "-delete", "-printf", "%99999999999f"}, "find: format width limit exceeded (10485760 bytes)\n"},
		{[]string{"/dir", "-name", "nomatch", "-printf", "%.99999999999f"}, "find: format width limit exceeded (10485760 bytes)\n"},
	} {
		tests = append(tests, findTest{
			name:       "rejects " + strings.Join(tt.args, " "),
			files:      destructive(),
			args:       tt.args,
			wantStderr: tt.stderr,
			wantCode:   1,
			after:      untouched,
		})
	}

	tests = append(tests,
		findTest{
			name:       "print0 and delete only in their branches",
			files:      destructive(),
			args:       []string{"/dir", "-type", "f", "(", "-name", "keep", "-print0", "-o", "-name", "remove", "-delete", ")"},
			wantStdout: "/dir/keep\x00",
			after: func(t *testing.T, fsys billy.Filesystem) {
				if _, err := fsys.Stat("dir/remove"); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("dir/remove still exists: %v", err)
				}
				if _, err := fsys.Stat("dir/keep"); err != nil {
					t.Errorf("dir/keep is gone: %v", err)
				}
			},
		},
		findTest{
			name:  "delete short-circuits on the true side of or",
			files: destructive(),
			args:  []string{"/dir", "-type", "f", "-a", "(", "-name", "keep", "-o", "-delete", ")"},
			after: func(t *testing.T, fsys billy.Filesystem) {
				if _, err := fsys.Stat("dir/keep"); err != nil {
					t.Errorf("dir/keep is gone: %v", err)
				}
			},
		},
		findTest{
			name:       "exec only in its nested branch",
			files:      destructive(),
			args:       []string{"/dir", "-type", "f", "(", "(", "-name", "keep", "-exec", "echo", "EXEC", "{}", ";", ")", "-o", "-name", "remove", "-print", ")"},
			wantStdout: "EXEC /dir/keep\n/dir/remove\n",
		},
		findTest{
			name:       "actions beneath not still run",
			files:      map[string]entry{"/dir/file": file("x")},
			args:       []string{"/dir", "-type", "f", "!", "-print"},
			wantStdout: "/dir/file\n",
		},
		findTest{
			name:       "or, not, and case-insensitive path",
			files:      map[string]entry{"/repo/PULLS/nested/UPPER.JSON": file(""), "/repo/other/nested/value.txt": file("")},
			args:       []string{"/repo", "-type", "f", "(", "-ipath", "*/pulls/*.json", "-o", "!", "-path", "*/PULLS/*", ")"},
			wantStdout: "/repo/PULLS/nested/UPPER.JSON\n/repo/other/nested/value.txt\n",
		},
	)

	runTable(t, tests)
}

func TestFind_Unreadable(t *testing.T) {
	home := func() map[string]entry {
		return map[string]entry{
			"/home/user/.Trash/old.txt":                     file("gone"),
			"/home/user/Documents/notes.md":                 file("# notes"),
			"/home/user/Documents/vault/.obsidian/app.json": file("{}"),
			"/home/user/Downloads/paper.pdf":                file("pdf"),
		}
	}
	denied := func(p string) error {
		return &fs.PathError{Op: "open", Path: p, Err: fs.ErrPermission}
	}
	trash := map[string]error{"/home/user/.Trash": denied("/home/user/.Trash")}
	both := map[string]error{
		"/home/user/.Trash":          denied("/home/user/.Trash"),
		"/home/user/Documents/vault": denied("/home/user/Documents/vault"),
	}
	const trashMsg = "find: /home/user/.Trash: Permission denied\n"
	const bothMsg = trashMsg + "find: /home/user/Documents/vault: Permission denied\n"

	runTable(t, []findTest{
		{
			name:       "names the directory and keeps going",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-name", "*.md"},
			wantStdout: "/home/user/Documents/notes.md\n",
			wantStderr: trashMsg,
			wantCode:   1,
		},
		{
			name:       "still lists the directory itself",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-type", "d", "-name", ".*"},
			wantStdout: "/home/user/.Trash\n/home/user/Documents/vault/.obsidian\n",
			wantStderr: trashMsg,
			wantCode:   1,
		},
		{
			name:       "an unreadable directory is not empty",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-type", "d", "-empty"},
			wantStderr: trashMsg,
			wantCode:   1,
		},
		{
			name:       "not read when not descended into",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-maxdepth", "1", "-type", "d"},
			wantStdout: "/home/user\n/home/user/.Trash\n/home/user/Documents\n/home/user/Downloads\n",
		},
		{
			name:       "not read when pruned",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-name", ".Trash", "-prune", "-o", "-name", "*.md", "-print"},
			wantStdout: "/home/user/Documents/notes.md\n",
		},
		{
			name:       "every unreadable directory in traversal order",
			files:      home(),
			unreadable: both,
			args:       []string{"/home/user", "-type", "d"},
			wantStdout: "/home/user\n/home/user/.Trash\n/home/user/Documents\n/home/user/Documents/vault\n/home/user/Downloads\n",
			wantStderr: bothMsg,
			wantCode:   1,
		},
		{
			name:       "same order under depth",
			files:      home(),
			unreadable: both,
			args:       []string{"/home/user", "-depth", "-type", "d"},
			wantStdout: "/home/user/.Trash\n/home/user/Documents/vault\n/home/user/Documents\n/home/user/Downloads\n/home/user\n",
			wantStderr: bothMsg,
			wantCode:   1,
		},
		{
			name:       "reported below mindepth",
			files:      home(),
			unreadable: trash,
			args:       []string{"/home/user", "-mindepth", "2", "-name", "*.md"},
			wantStdout: "/home/user/Documents/notes.md\n",
			wantStderr: trashMsg,
			wantCode:   1,
		},
		{
			name:               "a failure that is not the directory's own ends the search",
			files:              home(),
			unreadable:         map[string]error{"/home/user/.Trash": errors.New("ABORT_ERR: The operation was aborted")},
			args:               []string{"/home/user", "-name", "*.md"},
			wantStderrContains: "ABORT_ERR",
			wantCode:           1,
		},
	})
}

func TestFind_Symlinks(t *testing.T) {
	fsys := newFS(t, map[string]entry{"/dir/real/a.txt": file("a")})
	if err := fsys.Symlink("real", "dir/link"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	got := run(t, fsys, "", "/dir")
	if want := "/dir\n/dir/link\n/dir/real\n/dir/real/a.txt\n"; got.stdout != want {
		t.Errorf("stdout = %q, want %q (symlinks must not be followed)", got.stdout, want)
	}
	got = run(t, fsys, "", "/dir", "-type", "f")
	if want := "/dir/real/a.txt\n"; got.stdout != want {
		t.Errorf("-type f stdout = %q, want %q", got.stdout, want)
	}
	got = run(t, fsys, "", "/dir", "-type", "l")
	if want := "/dir/link\n"; got.stdout != want {
		t.Errorf("-type l stdout = %q, want %q", got.stdout, want)
	}
}

func TestFind_FollowStart(t *testing.T) {
	newLinked := func(t *testing.T) *testFS {
		t.Helper()
		fsys := newFS(t, map[string]entry{"/real/a.txt": file("a")})
		if err := fsys.Symlink("real", "link"); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		if err := fsys.Symlink("nowhere", "dangling"); err != nil {
			t.Fatalf("Symlink: %v", err)
		}
		return fsys
	}

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "default does not follow", args: []string{"/link"}, want: "/link\n"},
		{name: "H follows a starting point", args: []string{"-H", "/link"}, want: "/link\n/link/a.txt\n"},
		{name: "H reports the target type", args: []string{"-H", "/link", "-maxdepth", "0", "-type", "d"}, want: "/link\n"},
		{name: "H keeps a dangling link", args: []string{"-H", "/dangling", "-type", "l"}, want: "/dangling\n"},
		{name: "last option wins", args: []string{"-H", "-P", "/link"}, want: "/link\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, newLinked(t), "", tt.args...)
			if got.stdout != tt.want || got.stderr != "" || got.code != 0 {
				t.Errorf("stdout = %q, stderr = %q, code = %d; want %q, no stderr, code 0", got.stdout, got.stderr, got.code, tt.want)
			}
		})
	}
}

func TestFileTypeLetter(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode fs.FileMode
		want byte
	}{
		{name: "regular file", mode: 0, want: 'f'},
		{name: "directory", mode: fs.ModeDir, want: 'd'},
		{name: "symbolic link", mode: fs.ModeSymlink, want: 'l'},
		{name: "fifo", mode: fs.ModeNamedPipe, want: 'p'},
		{name: "socket", mode: fs.ModeSocket, want: 's'},
		{name: "character device", mode: fs.ModeDevice | fs.ModeCharDevice, want: 'c'},
		{name: "block device", mode: fs.ModeDevice, want: 'b'},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileTypeLetter(tt.mode); got != tt.want {
				t.Errorf("fileTypeLetter(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestFind_DanglingSymlinkStart(t *testing.T) {
	// memfs Stat does not follow links, so this needs a real directory to
	// show that a dangling starting point is not mistaken for a missing one.
	root := t.TempDir()
	if err := os.Symlink("nowhere", filepath.Join(root, "dangling")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}

	got := run(t, osfs.New(root), "", "/dangling")
	if want := "/dangling\n"; got.stdout != want || got.stderr != "" || got.code != 0 {
		t.Errorf("stdout = %q, stderr = %q, code = %d; want %q, no stderr, code 0", got.stdout, got.stderr, got.code, want)
	}
}

func TestMatchGlob(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pattern    string
		input      string
		ignoreCase bool
		want       bool
	}{
		{name: "literal", pattern: "a.txt", input: "a.txt", want: true},
		{name: "literal mismatch", pattern: "a.txt", input: "b.txt"},
		{name: "star", pattern: "*.txt", input: "a.txt", want: true},
		{name: "star matches empty", pattern: "*a", input: "a", want: true},
		{name: "star crosses slash", pattern: "*/b", input: "x/y/b", want: true},
		{name: "star matches leading dot", pattern: "*", input: ".hidden", want: true},
		{name: "multiple stars backtrack", pattern: "*a*b*c", input: "xaybzc", want: true},
		{name: "multiple stars fail", pattern: "*a*b*c", input: "xaybz"},
		{name: "question", pattern: "a?c", input: "abc", want: true},
		{name: "question needs a rune", pattern: "a?c", input: "ac"},
		{name: "question matches multibyte", pattern: "?", input: "\u00e9", want: true},
		{name: "class", pattern: "[abc]", input: "b", want: true},
		{name: "range", pattern: "[a-c]x", input: "bx", want: true},
		{name: "range miss", pattern: "[a-c]x", input: "dx"},
		{name: "bang negation", pattern: "[!a]", input: "b", want: true},
		{name: "caret negation", pattern: "[^a]", input: "a"},
		{name: "close bracket first is literal", pattern: "[]a]", input: "]", want: true},
		{name: "unterminated bracket is literal", pattern: "[ab", input: "[ab", want: true},
		{name: "escaped star", pattern: `a\*`, input: "a*", want: true},
		{name: "escaped star is not a wildcard", pattern: `a\*`, input: "ab"},
		{name: "regex metacharacters are literal", pattern: "a.(b)+", input: "a.(b)+", want: true},
		{name: "digit class", pattern: "[[:digit:]]", input: "7", want: true},
		{name: "digit class miss", pattern: "[[:digit:]]", input: "a"},
		{name: "negated class", pattern: "[![:digit:]]x", input: "ax", want: true},
		{name: "class mixed with literal", pattern: "[[:alpha:]_]*", input: "_init", want: true},
		{name: "class mixed with range", pattern: "[[:space:]0-9]", input: "5", want: true},
		{name: "punct includes symbols", pattern: "[[:punct:]]", input: "$", want: true},
		{name: "unknown class matches nothing", pattern: "[[:bogus:]]", input: "b"},
		{name: "upper class", pattern: "[[:upper:]]*", input: "README", want: true},
		{name: "upper class is case sensitive", pattern: "[[:upper:]]*", input: "readme"},
		{name: "upper class folds", pattern: "[[:upper:]]*", input: "README", ignoreCase: true, want: true},
		{name: "case sensitive", pattern: "README*", input: "readme.md"},
		{name: "case insensitive", pattern: "README*", input: "readme.md", ignoreCase: true, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchGlob(tt.pattern, tt.input, tt.ignoreCase); got != tt.want {
				t.Errorf("matchGlob(%q, %q, %v) = %v, want %v", tt.pattern, tt.input, tt.ignoreCase, got, tt.want)
			}
		})
	}
}

func TestProcessEscapes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "abc", want: "abc"},
		{name: "newline and tab", input: `a\nb\tc`, want: "a\nb\tc"},
		{name: "octal", input: `\101\0`, want: "A\x00"},
		{name: "hex", input: `\x41\xff`, want: "A\xff"},
		{name: "bad hex keeps backslash", input: `\xzz`, want: `\xzz`},
		{name: "unicode", input: `\u00e9`, want: "\u00e9"},
		{name: "unicode without digits", input: `\u`, want: `\u`},
		{name: "unknown escape", input: `\q`, want: `\q`},
		{name: "trailing backslash", input: `a\`, want: `a\`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := processEscapes(tt.input); got != tt.want {
				t.Errorf("processEscapes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
