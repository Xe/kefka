package billysh

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/go-git/go-billy/v5"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command/registry"
)

func FsysStatHandler(reg *registry.Impl, fsys billy.Filesystem) interp.StatHandlerFunc {
	return func(ctx context.Context, name string, followSymlinks bool) (fs.FileInfo, error) {
		resolved := reg.Resolve(name)
		if !followSymlinks {
			if r, ok := fsys.(billy.Symlink); ok {
				return r.Lstat(resolved)
			}
		}
		return fsys.Stat(resolved)
	}
}

func FsysOpenHandler(reg *registry.Impl, fsys billy.Filesystem) interp.OpenHandlerFunc {
	return func(ctx context.Context, name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC) != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
		}
		f, err := fsys.Open(reg.Resolve(name))
		if err != nil {
			return nil, err
		}
		return readOnlyFile{f}, nil
	}
}

func FsysReadDirHandler(reg *registry.Impl, fsys billy.Filesystem) interp.ReadDirHandlerFunc2 {
	return func(ctx context.Context, name string) ([]fs.DirEntry, error) {
		entries, err := fsys.ReadDir(reg.Resolve(name))
		if err != nil {
			return nil, err
		}
		out := make([]fs.DirEntry, len(entries))
		for i, e := range entries {
			out[i] = fs.FileInfoToDirEntry(e)
		}
		return out, nil
	}
}

type readOnlyFile struct{ billy.File }

func (readOnlyFile) Write([]byte) (int, error) { return 0, fs.ErrPermission }

// CallHandler intercepts cd and pwd before interp's builtins handle them,
// so we can route directory state through the registry's fsys-relative pwd
// instead of interp's host-rooted Dir. Intercepted calls are replaced with
// `:` (no-op) so interp's builtin doesn't run.
func CallHandler(reg *registry.Impl, fsys billy.Filesystem, stdout, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) == 0 {
			return args, nil
		}
		switch args[0] {
		case "cd":
			target := ""
			if len(args) > 1 {
				target = args[1]
			}
			if err := reg.Chdir(fsys, target); err != nil {
				fmt.Fprintln(stderr, err)
				return []string{"false"}, nil
			}
			return []string{":"}, nil
		case "pwd":
			pwd := reg.Pwd()
			if pwd == "." {
				fmt.Fprintln(stdout, "/")
			} else {
				fmt.Fprintln(stdout, "/"+pwd)
			}
			return []string{":"}, nil
		}
		return args, nil
	}
}
