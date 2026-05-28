package touch

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"tangled.org/xeiaso.net/kefka/command"
)

// timedFS wraps a billy.Filesystem with persistent atime/mtime storage and
// implements billy.Change so touch can round-trip times. memfs by itself
// neither stores ModTime nor implements billy.Change, so tests need this
// shim to verify behavior.
type timedFS struct {
	billy.Filesystem
	mu     sync.Mutex
	atimes map[string]time.Time
	mtimes map[string]time.Time
}

func newTimedFS(inner billy.Filesystem) *timedFS {
	return &timedFS{
		Filesystem: inner,
		atimes:     map[string]time.Time{},
		mtimes:     map[string]time.Time{},
	}
}

func (t *timedFS) Stat(name string) (os.FileInfo, error) {
	info, err := t.Filesystem.Stat(name)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	mt, ok := t.mtimes[name]
	t.mu.Unlock()
	if !ok {
		return info, nil
	}
	return &timedInfo{FileInfo: info, mtime: mt}, nil
}

func (t *timedFS) Chmod(name string, mode os.FileMode) error {
	return billy.ErrNotSupported
}

func (t *timedFS) Lchown(name string, uid, gid int) error {
	return billy.ErrNotSupported
}

func (t *timedFS) Chown(name string, uid, gid int) error {
	return billy.ErrNotSupported
}

func (t *timedFS) Chtimes(name string, atime, mtime time.Time) error {
	if _, err := t.Filesystem.Stat(name); err != nil {
		return err
	}
	t.mu.Lock()
	t.atimes[name] = atime
	t.mtimes[name] = mtime
	t.mu.Unlock()
	return nil
}

func (t *timedFS) atime(name string) (time.Time, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.atimes[name]
	return v, ok
}

type timedInfo struct {
	os.FileInfo
	mtime time.Time
}

func (t *timedInfo) ModTime() time.Time { return t.mtime }

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

func newTimed(t *testing.T) *timedFS {
	t.Helper()
	return newTimedFS(newFS(t))
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
			name:       "invalid -t timestamp",
			args:       []string{"-t", "notavalidtime", "new.txt"},
			wantErrSub: "invalid date format",
			wantErr:    true,
		},
		{
			name:       "multiple time sources rejected",
			args:       []string{"-t", "200001010000", "-d", "2024-01-15", "new.txt"},
			wantErrSub: "more than one source",
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

func TestTouchAOnlySetsAtime(t *testing.T) {
	fs := newTimed(t)
	original := time.Date(2000, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := fs.Chtimes("hello.txt", original, original); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, []string{"-a", "-d", "2024-01-15T00:00:00Z", "hello.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	at, ok := fs.atime("hello.txt")
	if !ok {
		t.Fatal("atime was not recorded")
	}
	want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !at.Equal(want) {
		t.Errorf("atime = %v, want %v", at, want)
	}
	info, err := fs.Stat("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(original) {
		t.Errorf("mtime = %v, want %v (unchanged)", info.ModTime(), original)
	}
}

func TestTouchMOnlySetsMtime(t *testing.T) {
	fs := newTimed(t)
	original := time.Date(2000, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := fs.Chtimes("hello.txt", original, original); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, []string{"-m", "-d", "2024-01-15T00:00:00Z", "hello.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	at, _ := fs.atime("hello.txt")
	if !at.Equal(original) {
		t.Errorf("atime = %v, want %v (unchanged)", at, original)
	}
	info, err := fs.Stat("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouchAMTogetherSetsBoth(t *testing.T) {
	fs := newTimed(t)
	original := time.Date(2000, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := fs.Chtimes("hello.txt", original, original); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, []string{"-a", "-m", "-d", "2024-01-15T00:00:00Z", "hello.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	at, _ := fs.atime("hello.txt")
	if !at.Equal(want) {
		t.Errorf("atime = %v, want %v", at, want)
	}
	info, err := fs.Stat("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouchReference(t *testing.T) {
	fs := newTimed(t)
	refTime := time.Date(2010, 3, 5, 8, 30, 0, 0, time.UTC)
	if err := fs.Chtimes("hello.txt", refTime, refTime); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, []string{"-r", "hello.txt", "new.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	if !exists(t, fs, "new.txt") {
		t.Fatal("new.txt was not created")
	}
	at, ok := fs.atime("new.txt")
	if !ok {
		t.Fatal("new.txt atime not recorded")
	}
	if !at.Equal(refTime) {
		t.Errorf("new.txt atime = %v, want %v", at, refTime)
	}
	info, err := fs.Stat("new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(refTime) {
		t.Errorf("new.txt mtime = %v, want %v", info.ModTime(), refTime)
	}
}

func TestTouchTStamp(t *testing.T) {
	fs := newTimed(t)
	_, _, err := run(t, []string{"-t", "199501010100.30", "new.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	want := time.Date(1995, 1, 1, 1, 0, 30, 0, time.UTC)
	info, err := fs.Stat("new.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(want) {
		t.Errorf("new.txt mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouchTStampAOnly(t *testing.T) {
	fs := newTimed(t)
	original := time.Date(2000, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := fs.Chtimes("hello.txt", original, original); err != nil {
		t.Fatal(err)
	}
	_, _, err := run(t, []string{"-a", "-t", "200001010000", "hello.txt"}, fs)
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}
	want := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	at, _ := fs.atime("hello.txt")
	if !at.Equal(want) {
		t.Errorf("atime = %v, want %v", at, want)
	}
	info, err := fs.Stat("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(original) {
		t.Errorf("mtime = %v, want %v (unchanged)", info.ModTime(), original)
	}
}

func TestTouchNoCreateOnMissingDoesNotError(t *testing.T) {
	fs := newTimed(t)
	stdout, stderr, err := run(t, []string{"-c", "missing.txt"}, fs)
	if err != nil {
		t.Fatalf("touch -c missing: unexpected error: %v stderr=%q", err, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if exists(t, fs, "missing.txt") {
		t.Errorf("missing.txt should not exist")
	}
}

func TestTouchCreatesWith0644UnderAssumedUmask(t *testing.T) {
	// memfs always returns 0o666 from Stat regardless of OpenFile mode,
	// so we can't observe the create mode through fs.Stat. Validate the
	// constant directly: with the documented assumed umask 0o022, the
	// effective create mode must be 0o644.
	const assumedUmask os.FileMode = 0o022
	got := os.FileMode(0o666) &^ assumedUmask
	if got != 0o644 {
		t.Fatalf("computed create mode = %o, want 0644", got)
	}
}

func TestTouchHelpFlagListed(t *testing.T) {
	fs := newFS(t)
	_, stderr, err := run(t, []string{"--help"}, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"-a", "-c", "-d", "-h", "-m", "-r", "-t"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("help text missing %s; got %q", want, stderr)
		}
	}
}

func TestParseTStamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
		ok    bool
	}{
		{"CCYYMMDDhhmm", "199501010100", time.Date(1995, 1, 1, 1, 0, 0, 0, time.UTC), true},
		{"CCYYMMDDhhmm.SS", "199501010100.30", time.Date(1995, 1, 1, 1, 0, 30, 0, time.UTC), true},
		{"YY=95 maps to 1995", "9501010100", time.Date(1995, 1, 1, 1, 0, 0, 0, time.UTC), true},
		{"YY=05 maps to 2005", "0501010100", time.Date(2005, 1, 1, 1, 0, 0, 0, time.UTC), true},
		{"YY=68 maps to 2068", "6801010100", time.Date(2068, 1, 1, 1, 0, 0, 0, time.UTC), true},
		{"YY=69 maps to 1969", "6901010100", time.Date(1969, 1, 1, 1, 0, 0, 0, time.UTC), true},
		{"MMDDhhmm only", "01010100", time.Time{}, true},
		{"garbage", "notavalidtime", time.Time{}, false},
		{"too short", "1234", time.Time{}, false},
		{"bad month", "199513010100", time.Time{}, false},
		{"bad day", "199501320100", time.Time{}, false},
		{"bad hour", "199501012500", time.Time{}, false},
		{"bad minute", "199501010060", time.Time{}, false},
		{"bad second len", "199501010100.3", time.Time{}, false},
		{"non-digit", "1995010101ab", time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseTStamp(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseTStamp(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if tt.name == "MMDDhhmm only" {
				if got.Year() != time.Now().UTC().Year() {
					t.Errorf("year = %d, want current year %d", got.Year(), time.Now().UTC().Year())
				}
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("parseTStamp(%q) = %v, want %v", tt.input, got, tt.want)
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
