//go:build !unix

package find

import "io/fs"

// deviceOf reports no device number. Without one, -xdev treats the whole
// tree as one device.
func deviceOf(fs.FileInfo) (uint64, bool) { return 0, false }
