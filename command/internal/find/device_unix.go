//go:build unix

package find

import (
	"io/fs"
	"syscall"
)

// deviceOf returns the device number behind info, when the filesystem
// exposes one. osfs does; memfs and s3fs do not.
func deviceOf(info fs.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}
