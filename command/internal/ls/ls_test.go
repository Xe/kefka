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
			wantStdout: "drwxr-xr-x 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name:       "classify suffixes for dir and executable",
			args:       []string{"-F"},
			wantStdout: "alpha.txt\nbeta.txt\ngamma.txt\nhuge.bin\nscript.sh*\nsub/\n",
		},
		{
			name: "long format",
			args: []string{"-l"},
			wantStdout: "total 6\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1500 Jan 15  2020 huge.bin\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 script.sh\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name: "long human readable",
			args: []string{"-lh"},
			wantStdout: "total 6\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1.5K Jan 15  2020 huge.bin\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 script.sh\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 sub/\n",
		},
		{
			name: "long all uses real stat times for dot entries",
			args: []string{"-la"},
			wantStdout: "total 9\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 ./\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 ../\n" +
				"-rw-r--r-- 1 user user     1 Jan 15  2020 .hidden\n" +
				"-rw-r--r-- 1 user user     4 Jan 15  2020 alpha.txt\n" +
				"-rw-r--r-- 1 user user     8 Jan 15  2020 beta.txt\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 gamma.txt\n" +
				"-rw-r--r-- 1 user user  1500 Jan 15  2020 huge.bin\n" +
				"-rw-r--r-- 1 user user     2 Jan 15  2020 script.sh\n" +
				"drwxr-xr-x 1 user user     0 Jan 15  2020 sub/\n",
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
			name:    "unknown flag returns error",
			args:    []string{"--no-such-flag"},
			wantErr: true,
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
