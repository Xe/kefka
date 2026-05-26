package s3fs

import (
	"io/fs"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"tangled.org/xeiaso.net/kefka/s3fs/unixmeta"
)

// FileStat is the value returned by s3FileInfo.Sys() when the Unix-metadata
// feature is enabled. It carries the raw numeric owner and group so consumers
// can resolve them to names however they like.
type FileStat struct {
	UID, GID uint32
}

// s3FileInfo implements os.FileInfo
type s3FileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	sys     *FileStat
}

func newFileInfo(name string, size int64, modTime time.Time) os.FileInfo {
	return s3FileInfo{
		name:    name,
		size:    size,
		mode:    0666,
		modTime: modTime,
	}
}

func newDirInfo(name string) os.FileInfo {
	return s3FileInfo{
		name:    name,
		mode:    fs.ModeDir,
		modTime: time.Time{},
	}
}

// newFileInfoFromHead builds a FileInfo from a HeadObject response. When cfg is
// nil the Unix-metadata feature is off and the result matches newFileInfo;
// otherwise the x-amz-meta-* attributes are decoded, falling back to the
// session defaults for any header that is missing.
func newFileInfoFromHead(name string, head *s3.HeadObjectOutput, cfg *unixMetaConfig) os.FileInfo {
	size := aws.ToInt64(head.ContentLength)
	modTime := aws.ToTime(head.LastModified)
	if cfg == nil {
		return newFileInfo(name, size, modTime)
	}

	attrs := unixmeta.Decode(head.Metadata, unixmeta.Attrs{
		UID:   cfg.uid,
		GID:   cfg.gid,
		Mode:  0o666 &^ cfg.umask,
		Mtime: modTime,
	})

	return s3FileInfo{
		name:    name,
		size:    size,
		mode:    attrs.Mode,
		modTime: attrs.Mtime,
		sys:     &FileStat{UID: attrs.UID, GID: attrs.GID},
	}
}

func (fi s3FileInfo) Name() string {
	return fi.name
}

func (fi s3FileInfo) Size() int64 {
	return fi.size
}

func (fi s3FileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi s3FileInfo) IsDir() bool {
	return fi.mode.IsDir()
}

func (fi s3FileInfo) Sys() any {
	if fi.sys == nil {
		return nil
	}
	return fi.sys
}

func (fi s3FileInfo) ModTime() time.Time {
	return fi.modTime
}
