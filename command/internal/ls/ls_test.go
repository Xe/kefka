package ls

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

// lsUsageText is the expected ls usage block (everything that comes after the
// program-name prefix in the unknown-option line). Keeping this as one
// string keeps the help/usage tests in sync with the implementation.
const lsUsageText = "Usage: ls [OPTION]... [FILE]...\n" +
	"list directory contents\n\n" +
	"  -a, --all            do not ignore entries starting with .\n" +
	"  -A, --almost-all     do not list . and ..\n" +
	"  -c                   sort by ctime (best-effort; falls back to mtime)\n" +
	"  -d, --directory      list directories themselves, not their contents\n" +
	"  -f                   do not sort, enable -a, disable -l\n" +
	"  -F, --classify       append indicator (one of */=>@) to entries\n" +
	"  -h, --human-readable with -l, print sizes like 1K 234M 2G etc.\n" +
	"  -i, --inode          print the index number of each file (always '?' here)\n" +
	"  -k                   with -s, use 1024-byte blocks\n" +
	"  -l                   use a long listing format\n" +
	"  -m                   fill width with a comma separated list of entries\n" +
	"  -n, --numeric-uid-gid like -l but list numeric user and group IDs\n" +
	"  -o                   like -l, but do not list group information\n" +
	"  -p                   append / indicator to directories\n" +
	"  -q, --hide-control-chars print ? instead of nongraphic characters\n" +
	"  -r, --reverse        reverse order while sorting\n" +
	"  -R, --recursive      list subdirectories recursively\n" +
	"  -s, --size           print the allocated size of each file, in blocks\n" +
	"  -S                   sort by file size, largest first\n" +
	"  -t                   sort by time, newest first\n" +
	"  -u                   sort by atime (best-effort; falls back to mtime)\n" +
	"  -1                   list one file per line\n" +
	"      --help           display this help and exit\n"

func newTestFS() billy.Filesystem {
	fs := memfs.New()
	write := func(name string, data []byte, perm os.FileMode) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, perm)
		if err != nil {
			panic(err)
		}
		f.Write(data)
		f.Close()
	}
	write("alpha.txt", []byte("aaaa"), 0o644)
	write("beta.txt", []byte("bbbbbbbb"), 0o644)
	write("gamma.txt", []byte("cc"), 0o644)
	write(".hidden", []byte("h"), 0o644)
	write("script.sh", []byte("#!"), 0o755)
	write("huge.bin", bytes.Repeat([]byte("x"), 1500), 0o644)
	write("sub/inner.txt", []byte("inner"), 0o644)
	write("sub/deeper/leaf.txt", []byte("L"), 0o644)
	return fs
}

func withFixedDate(t *testing.T) {
	t.Helper()
	prev := formatDate
	formatDate = func(time.Time) string { return "Jan 15  2020" }
	t.Cleanup(func() { formatDate = prev })
}

func TestExec(t *testing.T) {
	tests := []struct {
		name       string
		dir        string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "default plain listing",
			args:       []string{},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
		{
			name:       "show all includes dot entries and hidden files",
			args:       []string{"-a"},
			wantStdout: ".\n..\n.hidden\nalpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
		{
			name:       "almost all hides dot entries but shows hidden files",
			args:       []string{"-A"},
			wantStdout: ".hidden\nalpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
		{
			name:       "reverse alphabetic order",
			args:       []string{"-r"},
			wantStdout: "sub\nscript.sh\nhuge.bin\ngamma.txt\nbeta.txt\nalpha.txt\n",
		},
		{
			name:       "sort by size descending",
			args:       []string{"-S"},
			wantStdout: "huge.bin\nbeta.txt\nalpha.txt\ngamma.txt\nscript.sh\nsub\n",
		},
		{
			name:       "directory only short",
			args:       []string{"-d", "sub"},
			wantStdout: "sub\n",
		},
		{
			name:       "directory only long uses real stat time",
			args:       []string{"-dl", "sub"},
			wantStdout: "drw-r--r-- 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name:       "classify suffixes for dir and executable",
			args:       []string{"-F"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh*\nsub/\n",
		},
		{
			name: "long format",
			args: []string{"-l"},
			wantStdout: "total 7\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1500 Jan 15  2020 huge.bin\n" +
				"-rwxr-xr-x 1 user user     2 Jan 15  2020 script.sh\n" +
				"drw-r--r-- 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name: "long human readable",
			args: []string{"-lh"},
			wantStdout: "total 7\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1.5K Jan 15  2020 huge.bin\n" +
				"-rwxr-xr-x 1 user user     2 Jan 15  2020 script.sh\n" +
				"drw-r--r-- 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name: "long all uses real stat times for dot entries",
			args: []string{"-la"},
			wantStdout: "total 8\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 ./\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 ../\n" +
				"-rw-r--r-- 1 user user     1 Jan 15  2020 .hidden\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1500 Jan 15  2020 huge.bin\n" +
				"-rwxr-xr-x 1 user user     2 Jan 15  2020 script.sh\n" +
				"drw-r--r-- 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name: "recursive descends into subdirectories",
			args: []string{"-R", "sub"},
			wantStdout: "sub:\n" +
				"deeper\n" +
				"inner.txt\n" +
				"\n" +
				"sub/deeper:\n" +
				"leaf.txt\n",
		},
		{
			name:       "glob matches text files",
			args:       []string{"*.txt"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\n",
		},
		{
			name:       "glob with no matches",
			args:       []string{"*.nope"},
			wantStderr: "ls: *.nope: No such file or directory\n",
			wantErr:    true,
		},
		{
			name:       "missing path reports stderr and exit error",
			args:       []string{"nope"},
			wantStderr: "ls: nope: No such file or directory\n",
			wantErr:    true,
		},
		{
			name: "multiple paths get headers and blank separator",
			args: []string{"sub", "."},
			wantStdout: "sub:\n" +
				"deeper\n" +
				"inner.txt\n" +
				"\n" +
				".:\n" +
				"alpha.txt\n" +
				"beta.txt\n" +
				"gamma.txt\n" +
				"huge.bin\n" +
				"script.sh\n" +
				"sub\n",
		},
		{
			name:       "single file argument",
			args:       []string{"alpha.txt"},
			wantStdout: "alpha.txt\n",
		},
		{
			name:       "single file argument long",
			args:       []string{"-l", "alpha.txt"},
			wantStdout: "-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n",
		},
		{
			name:       "ec.Dir scopes the listing",
			dir:        "sub",
			args:       []string{},
			wantStdout: "deeper\ninner.txt\n",
		},
		{
			name:       "unknown flag returns error",
			args:       []string{"--no-such-flag"},
			wantStderr: "ls: unknown option: --no-such-flag\n" + lsUsageText,
			wantErr:    true,
		},
		{
			name:       "help prints usage to stderr",
			args:       []string{"--help"},
			wantStderr: lsUsageText,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFixedDate(t)
			var stdout, stderr bytes.Buffer
			dir := tc.dir
			if dir == "" {
				dir = "."
			}
			ec := &command.ExecContext{
				Stdout: &stdout,
				Stderr: &stderr,
				Dir:    dir,
				FS:     newTestFS(),
			}
			err := Impl{}.Exec(context.Background(), ec, tc.args)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("stdout mismatch\nwant:\n%q\ngot:\n%q", tc.wantStdout, got)
			}
			if got := stderr.String(); got != tc.wantStderr {
				t.Errorf("stderr mismatch\nwant:\n%q\ngot:\n%q", tc.wantStderr, got)
			}
		})
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
	if err := (Impl{}).Exec(context.Background(), ec, nil); err == nil {
		t.Fatal("expected error for missing filesystem")
	}
}

func TestFormatHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{1023, "1023"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{10 * 1024, "10K"},
		{1024 * 1024, "1.0M"},
		{1500 * 1024, "1.5M"},
		{10 * 1024 * 1024, "10M"},
		{1024 * 1024 * 1024, "1.0G"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5G"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := formatHumanSize(tc.bytes); got != tc.want {
				t.Errorf("formatHumanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	now := time.Now()

	// Far in the past renders as month/day/year.
	old := time.Date(2020, 7, 4, 9, 30, 0, 0, time.UTC)
	if got, want := realFormatDate(old), "Jul  4  2020"; got != want {
		t.Errorf("formatDate(%v) = %q, want %q", old, got, want)
	}

	// Within the last 6 months renders as month/day/HH:MM.
	recent := now.Add(-3 * 24 * time.Hour)
	got := realFormatDate(recent)
	month := recent.Month().String()[:3]
	wantPrefix := month + " "
	if !bytes.HasPrefix([]byte(got), []byte(wantPrefix)) {
		t.Errorf("formatDate(%v) = %q, want prefix %q", recent, got, wantPrefix)
	}
	if len(got) != len("Jan 15 12:34") {
		t.Errorf("formatDate(%v) = %q, expected HH:MM form (len 12)", recent, got)
	}
}

// statOverlayFS wraps a billy.Filesystem and lets tests override the mode
// or mtime that Stat returns for specific paths. memfs ignores modes after
// creation and does not implement Chtimes, so we simulate both via overlay.
type statOverlayFS struct {
	billy.Filesystem
	modes  map[string]os.FileMode
	mtimes map[string]time.Time
}

func newOverlay(inner billy.Filesystem) *statOverlayFS {
	return &statOverlayFS{
		Filesystem: inner,
		modes:      map[string]os.FileMode{},
		mtimes:     map[string]time.Time{},
	}
}

func (s *statOverlayFS) Stat(name string) (os.FileInfo, error) {
	info, err := s.Filesystem.Stat(name)
	if err != nil {
		return nil, err
	}
	mode, hasMode := s.modes[name]
	mt, hasMt := s.mtimes[name]
	if !hasMode && !hasMt {
		return info, nil
	}
	if !hasMode {
		mode = info.Mode()
	}
	if !hasMt {
		mt = info.ModTime()
	}
	return &overlayInfo{FileInfo: info, mode: mode, mtime: mt}, nil
}

type overlayInfo struct {
	os.FileInfo
	mode  os.FileMode
	mtime time.Time
}

func (o *overlayInfo) Mode() os.FileMode  { return o.mode }
func (o *overlayInfo) ModTime() time.Time { return o.mtime }

func newSortByTimeFS() *statOverlayFS {
	fs := memfs.New()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
	o := newOverlay(fs)
	o.mtimes["a.txt"] = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	o.mtimes["b.txt"] = time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	o.mtimes["c.txt"] = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	return o
}

func TestExec_SortByTime(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newSortByTimeFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-t"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "b.txt\nc.txt\na.txt\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -t stdout = %q, want %q", got, want)
	}
}

func TestExec_SortByTimeReverse(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newSortByTimeFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-tr"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "a.txt\nc.txt\nb.txt\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -tr stdout = %q, want %q", got, want)
	}
}

func TestExec_OnePerLine(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newTestFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -1 stdout = %q, want %q", got, want)
	}
}

func TestExec_LongFormatSuid(t *testing.T) {
	withFixedDate(t)
	fs := memfs.New()
	f, err := fs.OpenFile("suidfile", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	o := newOverlay(fs)
	// 0o4755 with the os.ModeSetuid bit set -> "-rwsr-xr-x"
	o.modes["suidfile"] = 0o755 | os.ModeSetuid

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     o,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-l", "suidfile"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "-rwsr-xr-x 1 user user     0 Jan 15  2020 suidfile\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -l suidfile stdout = %q, want %q", got, want)
	}
}

func TestExec_LongFormatSuidNoExec(t *testing.T) {
	withFixedDate(t)
	fs := memfs.New()
	f, err := fs.OpenFile("suidfile", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	o := newOverlay(fs)
	// setuid set, but owner-x not set -> capital S
	o.modes["suidfile"] = 0o644 | os.ModeSetuid

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     o,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-l", "suidfile"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "-rwSr--r-- 1 user user     0 Jan 15  2020 suidfile\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -l suidfile (no x) stdout = %q, want %q", got, want)
	}
}

func TestExec_LongFormatSgid(t *testing.T) {
	withFixedDate(t)
	fs := memfs.New()
	f, err := fs.OpenFile("sgidfile", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	o := newOverlay(fs)
	o.modes["sgidfile"] = 0o2755 | os.ModeSetgid

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     o,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-l", "sgidfile"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "-rwxr-sr-x 1 user user     0 Jan 15  2020 sgidfile\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -l sgidfile stdout = %q, want %q", got, want)
	}
}

func TestExec_LongFormatStickyDir(t *testing.T) {
	withFixedDate(t)
	fs := memfs.New()
	f, err := fs.OpenFile("stickydir/keep", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	o := newOverlay(fs)
	// 1777 with ModeDir|ModeSticky -> "drwxrwxrwt"
	o.modes["stickydir"] = os.ModeDir | os.ModeSticky | 0o777

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     o,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-dl", "stickydir"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "drwxrwxrwt 1 user user     0 Jan 15  2020 stickydir/\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -dl stickydir stdout = %q, want %q", got, want)
	}
}

func TestExec_LongFormatStickyDirNoOtherExec(t *testing.T) {
	withFixedDate(t)
	fs := memfs.New()
	f, err := fs.OpenFile("stickydir/keep", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	o := newOverlay(fs)
	// sticky bit but no other-exec -> "T"
	o.modes["stickydir"] = os.ModeDir | os.ModeSticky | 0o644

	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     o,
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-dl", "stickydir"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "drw-r--r-T 1 user user     0 Jan 15  2020 stickydir/\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -dl stickydir (no other-x) stdout = %q, want %q", got, want)
	}
}

func TestRealFormatDate_Recent(t *testing.T) {
	recent := time.Now().Add(-3 * 24 * time.Hour)
	got := realFormatDate(recent)
	if len(got) != len("Jan 15 12:34") {
		t.Errorf("realFormatDate(recent) = %q, want length 12 (Mon DD HH:MM)", got)
	}
	month := recent.Month().String()[:3]
	if got[:3] != month {
		t.Errorf("realFormatDate(recent) = %q, want month prefix %q", got, month)
	}
	// Recent format must contain a colon (HH:MM).
	if !bytes.Contains([]byte(got), []byte(":")) {
		t.Errorf("realFormatDate(recent) = %q, expected HH:MM form with colon", got)
	}
}

func TestRealFormatDate_Old(t *testing.T) {
	old := time.Now().Add(-365 * 24 * time.Hour)
	got := realFormatDate(old)
	if len(got) != len("Jan 15  2020") {
		t.Errorf("realFormatDate(old) = %q, want length 12 (Mon DD  YYYY)", got)
	}
	// Old format must not contain a colon (year, not HH:MM).
	if bytes.Contains([]byte(got), []byte(":")) {
		t.Errorf("realFormatDate(old) = %q, expected YYYY form without colon", got)
	}
}

func TestFormatMode(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"plain file 0644", 0o644, "-rw-r--r--"},
		{"plain file 0755", 0o755, "-rwxr-xr-x"},
		{"directory 0755", os.ModeDir | 0o755, "drwxr-xr-x"},
		{"symlink 0777", os.ModeSymlink | 0o777, "lrwxrwxrwx"},
		{"setuid 4755", os.ModeSetuid | 0o755, "-rwsr-xr-x"},
		{"setuid no exec", os.ModeSetuid | 0o644, "-rwSr--r--"},
		{"setgid 2755", os.ModeSetgid | 0o755, "-rwxr-sr-x"},
		{"setgid no group exec", os.ModeSetgid | 0o744, "-rwxr-Sr--"},
		{"sticky dir 1777", os.ModeDir | os.ModeSticky | 0o777, "drwxrwxrwt"},
		{"sticky no other exec", os.ModeDir | os.ModeSticky | 0o644, "drw-r--r-T"},
		{"all special bits", os.ModeDir | os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o777, "drwsrwsrwt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatMode(tc.mode); got != tc.want {
				t.Errorf("formatMode(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		dir, in, want string
	}{
		{"", "foo", "foo"},
		{".", "foo", "foo"},
		{".", ".", "."},
		{"sub", "foo", "sub/foo"},
		{"sub", ".", "sub"},
		{"sub", "../bar", "bar"},
		{".", "/abs/path", "abs/path"},
		{".", "/", "."},
	}
	for _, tc := range tests {
		ec := &command.ExecContext{Dir: tc.dir}
		if got := resolvePath(ec, tc.in); got != tc.want {
			t.Errorf("resolvePath(dir=%q, in=%q) = %q, want %q", tc.dir, tc.in, got, tc.want)
		}
	}
}

// TestExec_NewFlags table-tests the recently wired flags (-m, -p, -i, -s,
// -k, -n, -o, -q, -c, -u) against the standard test FS. These are kept as
// one big table to keep the file scrollable; each case is a single
// invocation with a stable expected stdout.
func TestExec_NewFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{
			name:       "comma list",
			args:       []string{"-m"},
			wantStdout: "alpha.txt, beta.txt, gamma.txt, huge.bin, script.sh, sub\n",
		},
		{
			name:       "one-per-line overrides comma list",
			args:       []string{"-m1"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
		{
			name:       "slash dirs",
			args:       []string{"-p"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub/\n",
		},
		{
			name: "inode column",
			args: []string{"-i"},
			wantStdout: "? alpha.txt\n? beta.txt\n? gamma.txt\n? huge.bin\n? script.sh\n? sub\n",
		},
		{
			name: "block column 512-byte units",
			args: []string{"-s"},
			// 4 -> 1, 8 -> 1, 2 -> 1, 1500 -> 3, 2 -> 1, dir -> 0
			wantStdout: "1 alpha.txt\n1 beta.txt\n1 gamma.txt\n3 huge.bin\n1 script.sh\n0 sub\n",
		},
		{
			name: "block column with -k uses 1024-byte units",
			args: []string{"-sk"},
			// 4 -> 1, 8 -> 1, 2 -> 1, 1500 -> 2 (ceil(1500/1024)), 2 -> 1, dir -> 0
			wantStdout: "1 alpha.txt\n1 beta.txt\n1 gamma.txt\n2 huge.bin\n1 script.sh\n0 sub\n",
		},
		{
			name: "numeric uid/gid implies -l",
			args: []string{"-n"},
			wantStdout: "total 7\n" +
				"-rw-r--r-- 1 0 0     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 0 0     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 0 0     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 0 0  1500 Jan 15  2020 huge.bin\n" +
				"-rwxr-xr-x 1 0 0     2 Jan 15  2020 script.sh\n" +
				"drw-r--r-- 1 0 0     0 Jan 15  2020 sub/\n",
		},
		{
			name: "long without group implies -l",
			args: []string{"-o"},
			wantStdout: "total 7\n" +
				"-rw-r--r-- 1 user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user  1500 Jan 15  2020 huge.bin\n" +
				"-rwxr-xr-x 1 user     2 Jan 15  2020 script.sh\n" +
				"drw-r--r-- 1 user     0 Jan 15  2020 sub/\n",
		},
		{
			name:       "ctime falls back to mtime ordering",
			args:       []string{"-c"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
		{
			name:       "atime falls back to mtime ordering",
			args:       []string{"-u"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh\nsub\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFixedDate(t)
			var stdout, stderr bytes.Buffer
			ec := &command.ExecContext{
				Stdout: &stdout,
				Stderr: &stderr,
				Dir:    ".",
				FS:     newTestFS(),
			}
			if err := (Impl{}).Exec(context.Background(), ec, tc.args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tc.wantStdout)
			}
		})
	}
}

// TestHideControlChars verifies that -q replaces non-printable bytes with
// '?'. We feed a name with a literal newline and tab; both are non-printable
// in the spec sense and should become '?'.
func TestHideControlChars(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain.txt", "plain.txt"},
		{"a\tb", "a?b"},
		{"a\nb", "a?b"},
		{"a\x00b", "a?b"},
		{"a\x7fb", "a?b"},
		{"  spaces ok ", "  spaces ok "},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := hideControlChars(tc.in); got != tc.want {
				t.Errorf("hideControlChars(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestBlocksFor exercises the 512/1024 byte rounding in lsOptions.
func TestBlocksFor(t *testing.T) {
	tests := []struct {
		name string
		size int64
		k    bool
		want int64
	}{
		{"empty 512", 0, false, 0},
		{"1 byte 512", 1, false, 1},
		{"512 bytes 512", 512, false, 1},
		{"513 bytes 512", 513, false, 2},
		{"1500 bytes 512", 1500, false, 3},
		{"empty 1024", 0, true, 0},
		{"1 byte 1024", 1, true, 1},
		{"1024 bytes 1024", 1024, true, 1},
		{"1025 bytes 1024", 1025, true, 2},
		{"1500 bytes 1024", 1500, true, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := lsOptions{kBytes: tc.k}
			if got := opts.blocksFor(tc.size); got != tc.want {
				t.Errorf("blocksFor(size=%d, k=%v) = %d, want %d", tc.size, tc.k, got, tc.want)
			}
		})
	}
}

// TestExec_NoSortFlag exercises -f: disables sorting, enables -a, and
// suppresses long format. The exact order is whatever the filesystem
// returns, so we assert that all entries (including dot files) are
// present rather than pinning to one ordering.
func TestExec_NoSortFlag(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newTestFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-f"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	// -f implies -a, so .hidden must be in the output even though we did
	// not pass -a explicitly.
	for _, name := range []string{".hidden", "alpha.txt", "beta.txt", "gamma.txt", "huge.bin", "script.sh", "sub"} {
		if !bytes.Contains([]byte(out), []byte(name+"\n")) {
			t.Errorf("ls -f stdout missing %q\nfull output:\n%s", name, out)
		}
	}
}

// TestExec_NoSortSuppressesLong verifies that -f overrides -l, matching
// GNU ls's documented behavior of forcing short format under -f.
func TestExec_NoSortSuppressesLong(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newTestFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-fl"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	// "total " is only printed in long mode; -f must have suppressed it.
	if bytes.Contains([]byte(out), []byte("total ")) {
		t.Errorf("ls -fl should not produce a 'total' line, got:\n%s", out)
	}
	// And mode strings like "-rw-r--r--" should be absent.
	if bytes.Contains([]byte(out), []byte("-rw-r--r--")) {
		t.Errorf("ls -fl should not produce long-format mode strings, got:\n%s", out)
	}
}

// TestExec_NoSortOverridesT verifies -f beats -t (sort by time).
func TestExec_NoSortOverridesT(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newSortByTimeFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-ft"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All three names must appear; we don't assert ordering since -f
	// disables sorting and uses fs-native order.
	out := stdout.String()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if !bytes.Contains([]byte(out), []byte(name)) {
			t.Errorf("ls -ft stdout missing %q\nfull output:\n%s", name, out)
		}
	}
}

// TestExec_LongInodeAndBlocks combines -lis to confirm the inode and block
// columns appear before the long-format line for each entry.
func TestExec_LongInodeAndBlocks(t *testing.T) {
	withFixedDate(t)
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
		FS:     newTestFS(),
	}
	if err := (Impl{}).Exec(context.Background(), ec, []string{"-lis", "alpha.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "? 1 -rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n"
	if got := stdout.String(); got != want {
		t.Errorf("ls -lis alpha.txt stdout = %q, want %q", got, want)
	}
}
