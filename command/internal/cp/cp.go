package cp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("cp: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("cp: ExecContext has no filesystem")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("cp")
	set.SetParameters("SOURCE... DEST")

	usage := func() {
		fmt.Fprint(stderr, "Usage: cp [OPTION]... SOURCE... DEST\n")
		fmt.Fprint(stderr, "Copy files and directories.\n\n")
		fmt.Fprint(stderr, "  -r, -R, --recursive  copy directories recursively\n")
		fmt.Fprint(stderr, "  -n, --no-clobber     do not overwrite an existing file\n")
		fmt.Fprint(stderr, "  -p, --preserve       preserve file attributes\n")
		fmt.Fprint(stderr, "  -v, --verbose        explain what is being done\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	recursive := set.BoolLong("recursive", 'r', "copy directories recursively")
	recursiveUpper := set.Bool('R', "copy directories recursively")
	noClobber := set.BoolLong("no-clobber", 'n', "do not overwrite an existing file")
	_ = set.BoolLong("preserve", 'p', "preserve file attributes")
	verbose := set.BoolLong("verbose", 'v', "explain what is being done")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"cp"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "cp: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	isRecursive := *recursive || *recursiveUpper
	paths := set.Args()

	if len(paths) < 2 {
		fmt.Fprint(stderr, "cp: missing destination file operand\n")
		return interp.ExitStatus(1)
	}

	dest := paths[len(paths)-1]
	sources := paths[:len(paths)-1]
	destPath := resolvePath(ec, dest)

	destIsDir := false
	if info, err := ec.FS.Stat(destPath); err == nil {
		destIsDir = info.IsDir()
	}

	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(stderr, "cp: target '%s' is not a directory\n", dest)
		return interp.ExitStatus(1)
	}

	exitCode := 0
	for _, src := range sources {
		srcPath := resolvePath(ec, src)
		srcInfo, err := ec.FS.Stat(srcPath)
		if err != nil {
			fmt.Fprintf(stderr, "cp: cannot stat '%s': No such file or directory\n", src)
			exitCode = 1
			continue
		}

		targetPath := destPath
		targetDisplay := dest
		if destIsDir {
			b := path.Base(src)
			targetPath = path.Join(destPath, b)
			if dest == "/" {
				targetDisplay = "/" + b
			} else {
				targetDisplay = strings.TrimSuffix(dest, "/") + "/" + b
			}
		}

		if srcInfo.IsDir() && !isRecursive {
			fmt.Fprintf(stderr, "cp: -r not specified; omitting directory '%s'\n", src)
			exitCode = 1
			continue
		}

		if *noClobber {
			if _, err := ec.FS.Stat(targetPath); err == nil {
				continue
			}
		}

		if err := copyTree(ec.FS, srcPath, targetPath); err != nil {
			fmt.Fprintf(stderr, "cp: cannot copy '%s': %v\n", src, err)
			exitCode = 1
			continue
		}

		if *verbose {
			fmt.Fprintf(stdout, "'%s' -> '%s'\n", src, targetDisplay)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func copyTree(fs billy.Filesystem, src, dst string) error {
	info, err := fs.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := fs.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := fs.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(fs, path.Join(src, e.Name()), path.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(fs, src, dst)
}

func copyFile(fs billy.Filesystem, src, dst string) error {
	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := fs.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return "."
		}
		return path.Clean(p)
	}
	joined := path.Join(dir, p)
	if joined == "" {
		return "."
	}
	return joined
}
