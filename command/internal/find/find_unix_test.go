//go:build unix

package find

import (
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/go-git/go-billy/v6/osfs"
)

func TestFind_SpecialFileTypes(t *testing.T) {
	// A short directory keeps the socket path under the sun_path limit.
	root, err := os.MkdirTemp("", "find")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })

	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0o644); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}
	ln, err := net.Listen("unix", filepath.Join(root, "sock"))
	if err != nil {
		t.Skipf("cannot create a socket here: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	fsys := osfs.New(root)
	for _, tt := range []struct {
		name     string
		fileType string
		want     string
	}{
		{name: "fifo", fileType: "p", want: "/fifo\n"},
		{name: "socket", fileType: "s", want: "/sock\n"},
		{name: "regular file", fileType: "f", want: "/file\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, fsys, "", "/", "-mindepth", "1", "-type", tt.fileType)
			if got.stdout != tt.want || got.stderr != "" {
				t.Errorf("stdout = %q, stderr = %q; want %q", got.stdout, got.stderr, tt.want)
			}
		})
	}
}

// deviceFS reports every path under mount as being on device 2 and
// everything else as device 1, so -xdev has a mount point to stop at.
type deviceFS struct {
	*testFS
	mount string
}

type deviceInfo struct {
	fs.FileInfo
	stat *syscall.Stat_t
}

func (d deviceInfo) Sys() any { return d.stat }

func (d *deviceFS) Lstat(name string) (fs.FileInfo, error) {
	fi, err := d.testFS.Lstat(name)
	if err != nil {
		return nil, err
	}
	st := &syscall.Stat_t{}
	st.Dev = 1
	if p := path.Clean(name); p == d.mount || strings.HasPrefix(p, d.mount+"/") {
		st.Dev = 2
	}
	return deviceInfo{FileInfo: fi, stat: st}, nil
}

func TestFind_Xdev(t *testing.T) {
	newDevFS := func(t *testing.T) *deviceFS {
		t.Helper()
		return &deviceFS{
			testFS: newFS(t, map[string]entry{"/dir/a": file(""), "/dir/mnt/inner": file("")}),
			mount:  "dir/mnt",
		}
	}

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "without xdev", args: []string{"/dir"}, want: "/dir\n/dir/a\n/dir/mnt\n/dir/mnt/inner\n"},
		{name: "lists the mount point but does not enter it", args: []string{"/dir", "-xdev"}, want: "/dir\n/dir/a\n/dir/mnt\n"},
		{name: "applies even when not evaluated", args: []string{"/dir", "-name", "nothing", "-a", "-xdev", "-o", "-print"}, want: "/dir\n/dir/a\n/dir/mnt\n"},
		{name: "under depth", args: []string{"/dir", "-depth", "-xdev"}, want: "/dir/a\n/dir/mnt\n/dir\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, newDevFS(t), "", tt.args...)
			if got.stdout != tt.want || got.stderr != "" {
				t.Errorf("stdout = %q, stderr = %q; want %q", got.stdout, got.stderr, tt.want)
			}
		})
	}
}
