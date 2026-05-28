// Package billyfs adapts a github.com/go-git/go-billy/v6 Filesystem into a
// github.com/tetratelabs/wazero experimental/sys.FS so a WASI guest can read
// and write through the billy filesystem.
package billyfs

import (
	"errors"
	"io"
	stdfs "io/fs"
	"os"

	"github.com/go-git/go-billy/v6"
	expsys "github.com/tetratelabs/wazero/experimental/sys"
	wsys "github.com/tetratelabs/wazero/sys"
)

// New returns an experimental/sys.FS backed by the given billy.Filesystem.
// All paths passed in by the WASM guest are interpreted as relative to the
// billy filesystem's root.
func New(fsys billy.Filesystem) expsys.FS {
	return &billyFS{fsys: fsys}
}

type billyFS struct {
	expsys.UnimplementedFS
	fsys billy.Filesystem
}

func (b *billyFS) OpenFile(path string, flag expsys.Oflag, perm stdfs.FileMode) (expsys.File, expsys.Errno) {
	f, err := b.fsys.OpenFile(path, toOSFlag(flag), perm)
	if err != nil {
		return nil, toErrno(err)
	}
	return &billyFile{file: f, fsys: b.fsys}, 0
}

func (b *billyFS) Stat(path string) (wsys.Stat_t, expsys.Errno) {
	info, err := b.fsys.Stat(path)
	if err != nil {
		return wsys.Stat_t{}, toErrno(err)
	}
	return wsys.NewStat_t(info), 0
}

func (b *billyFS) Lstat(path string) (wsys.Stat_t, expsys.Errno) {
	if sym, ok := b.fsys.(billy.Symlink); ok {
		info, err := sym.Lstat(path)
		if err != nil {
			return wsys.Stat_t{}, toErrno(err)
		}
		return wsys.NewStat_t(info), 0
	}
	return b.Stat(path)
}

func (b *billyFS) Mkdir(path string, perm stdfs.FileMode) expsys.Errno {
	return toErrno(b.fsys.MkdirAll(path, perm))
}

func (b *billyFS) Unlink(path string) expsys.Errno {
	return toErrno(b.fsys.Remove(path))
}

func (b *billyFS) Rmdir(path string) expsys.Errno {
	return toErrno(b.fsys.Remove(path))
}

func (b *billyFS) Rename(from, to string) expsys.Errno {
	return toErrno(b.fsys.Rename(from, to))
}

func (b *billyFS) Readlink(path string) (string, expsys.Errno) {
	sym, ok := b.fsys.(billy.Symlink)
	if !ok {
		return "", expsys.ENOSYS
	}
	target, err := sym.Readlink(path)
	if err != nil {
		return "", toErrno(err)
	}
	return target, 0
}

func (b *billyFS) Symlink(oldPath, linkName string) expsys.Errno {
	sym, ok := b.fsys.(billy.Symlink)
	if !ok {
		return expsys.ENOSYS
	}
	return toErrno(sym.Symlink(oldPath, linkName))
}

type billyFile struct {
	expsys.UnimplementedFile
	file       billy.File
	fsys       billy.Filesystem
	dirEntries []stdfs.DirEntry
	dirRead    bool
	dirOffset  int
}

func (f *billyFile) Stat() (wsys.Stat_t, expsys.Errno) {
	info, err := f.fsys.Stat(f.file.Name())
	if err != nil {
		return wsys.Stat_t{}, toErrno(err)
	}
	return wsys.NewStat_t(info), 0
}

func (f *billyFile) Read(buf []byte) (int, expsys.Errno) {
	n, err := f.file.Read(buf)
	if err == io.EOF {
		return n, 0
	}
	if err != nil {
		return n, toErrno(err)
	}
	return n, 0
}

func (f *billyFile) Write(buf []byte) (int, expsys.Errno) {
	n, err := f.file.Write(buf)
	if err != nil {
		return n, toErrno(err)
	}
	return n, 0
}

// Seek satisfies expsys.File.Seek; the Errno return is required by wazero
// even though it implements error, which trips go vet's stdmethods check.
//
//nolint:stdmethods
func (f *billyFile) Seek(offset int64, whence int) (int64, expsys.Errno) {
	n, err := f.file.Seek(offset, whence)
	if err != nil {
		return n, toErrno(err)
	}
	return n, 0
}

func (f *billyFile) Pread(buf []byte, off int64) (int, expsys.Errno) {
	n, err := f.file.ReadAt(buf, off)
	if err == io.EOF {
		return n, 0
	}
	if err != nil {
		return n, toErrno(err)
	}
	return n, 0
}

func (f *billyFile) Truncate(size int64) expsys.Errno {
	return toErrno(f.file.Truncate(size))
}

func (f *billyFile) Close() expsys.Errno {
	return toErrno(f.file.Close())
}

func (f *billyFile) IsDir() (bool, expsys.Errno) {
	info, err := f.fsys.Stat(f.file.Name())
	if err != nil {
		return false, toErrno(err)
	}
	return info.IsDir(), 0
}

func (f *billyFile) Readdir(n int) ([]expsys.Dirent, expsys.Errno) {
	if !f.dirRead {
		entries, err := f.fsys.ReadDir(f.file.Name())
		if err != nil {
			return nil, toErrno(err)
		}
		f.dirEntries = entries
		f.dirRead = true
	}

	remaining := len(f.dirEntries) - f.dirOffset
	if remaining <= 0 {
		return nil, 0
	}

	count := remaining
	if n > 0 && n < count {
		count = n
	}

	dirents := make([]expsys.Dirent, 0, count)
	for i := 0; i < count; i++ {
		info := f.dirEntries[f.dirOffset+i]
		dirents = append(dirents, expsys.Dirent{
			Name: info.Name(),
			Type: info.Type(),
		})
	}
	f.dirOffset += count
	return dirents, 0
}

func toErrno(err error) expsys.Errno {
	if err == nil {
		return 0
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return expsys.ENOENT
	case errors.Is(err, os.ErrExist):
		return expsys.EEXIST
	case errors.Is(err, os.ErrPermission):
		return expsys.EPERM
	case errors.Is(err, billy.ErrNotSupported):
		return expsys.ENOSYS
	case errors.Is(err, billy.ErrReadOnly):
		return expsys.EROFS
	}
	if e := expsys.UnwrapOSError(err); e != 0 {
		return e
	}
	return expsys.EIO
}

func toOSFlag(flag expsys.Oflag) int {
	var f int
	switch flag & 0b11 {
	case expsys.O_RDONLY:
		f = os.O_RDONLY
	case expsys.O_RDWR:
		f = os.O_RDWR
	case expsys.O_WRONLY:
		f = os.O_WRONLY
	}
	if flag&expsys.O_APPEND != 0 {
		f |= os.O_APPEND
	}
	if flag&expsys.O_CREAT != 0 {
		f |= os.O_CREATE
	}
	if flag&expsys.O_EXCL != 0 {
		f |= os.O_EXCL
	}
	if flag&expsys.O_TRUNC != 0 {
		f |= os.O_TRUNC
	}
	return f
}
