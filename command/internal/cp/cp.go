package cp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type promptMode int

const (
	promptDefault promptMode = iota
	promptForce
	promptInteractive
	promptNoClobber
)

// symlinkMode controls how cp treats symbolic links in source operands.
// The default differs by recursion: non-recursive cp follows command-line
// symlinks (effectively -L for top-level operands), while recursive cp
// preserves them in-tree (-P). -H/-L/-P override this.
type symlinkMode int

const (
	symlinkDefault symlinkMode = iota
	symlinkFollowCmdLine             // -H
	symlinkFollowAll                 // -L
	symlinkNoFollow                  // -P
)

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
		fmt.Fprint(stderr, "  -f, --force          do not prompt before overwriting\n")
		fmt.Fprint(stderr, "  -H                   follow command-line symbolic links in SOURCE\n")
		fmt.Fprint(stderr, "  -i, --interactive    prompt before overwrite\n")
		fmt.Fprint(stderr, "  -L, --dereference    always follow symbolic links in SOURCE\n")
		fmt.Fprint(stderr, "  -n, --no-clobber     do not overwrite an existing file\n")
		fmt.Fprint(stderr, "  -P, --no-dereference never follow symbolic links in SOURCE\n")
		fmt.Fprint(stderr, "  -p, --preserve       preserve file attributes\n")
		fmt.Fprint(stderr, "  -r, -R, --recursive  copy directories recursively\n")
		fmt.Fprint(stderr, "  -v, --verbose        explain what is being done\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	recursive := set.BoolLong("recursive", 'r', "copy directories recursively")
	recursiveUpper := set.Bool('R', "copy directories recursively")
	_ = set.BoolLong("force", 'f', "do not prompt before overwriting")
	_ = set.BoolLong("interactive", 'i', "prompt before overwrite")
	_ = set.BoolLong("no-clobber", 'n', "do not overwrite an existing file")
	preserve := set.BoolLong("preserve", 'p', "preserve file attributes")
	verbose := set.BoolLong("verbose", 'v', "explain what is being done")
	_ = set.Bool('H', "follow command-line symbolic links in SOURCE")
	_ = set.BoolLong("dereference", 'L', "always follow symbolic links in SOURCE")
	_ = set.BoolLong("no-dereference", 'P', "never follow symbolic links in SOURCE")
	help := set.BoolLong("help", 0, "display this help and exit")

	mode := promptDefault
	symMode := symlinkDefault
	cb := func(opt getopt.Option) bool {
		switch opt.LongName() {
		case "force":
			mode = promptForce
		case "interactive":
			mode = promptInteractive
		case "no-clobber":
			mode = promptNoClobber
		case "dereference":
			symMode = symlinkFollowAll
		case "no-dereference":
			symMode = symlinkNoFollow
		}
		switch opt.ShortName() {
		case "H":
			symMode = symlinkFollowCmdLine
		case "L":
			symMode = symlinkFollowAll
		case "P":
			symMode = symlinkNoFollow
		}
		return true
	}

	if err := set.Getopt(append([]string{"cp"}, args...), cb); err != nil {
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

	stdinReader := bufio.NewReader(stdinOrEmpty(ec.Stdin))

	// Effective symlink mode: GNU's defaults are -P with -R (preserve links
	// in the tree, but follow command-line ones), and -L without -R (follow
	// links anywhere). Explicit -H/-L/-P always wins.
	effSym := symMode
	if effSym == symlinkDefault {
		if isRecursive {
			effSym = symlinkFollowCmdLine
		} else {
			effSym = symlinkFollowAll
		}
	}

	exitCode := 0
	for _, src := range sources {
		srcPath := resolvePath(ec, src)
		// Top-level operands: -L and -H follow the link given on the
		// command line. -P does not. statSrc resolves accordingly.
		followTop := effSym == symlinkFollowAll || effSym == symlinkFollowCmdLine
		srcInfo, err := statSrc(ec.FS, srcPath, followTop)
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

		if srcPath == targetPath {
			fmt.Fprintf(stderr, "cp: '%s' and '%s' are the same file\n", src, targetDisplay)
			exitCode = 1
			continue
		}

		if srcInfo.IsDir() && !isRecursive {
			fmt.Fprintf(stderr, "cp: -r not specified; omitting directory '%s'\n", src)
			exitCode = 1
			continue
		}

		if !srcInfo.IsDir() && srcInfo.Mode()&os.ModeSymlink == 0 {
			if _, err := ec.FS.Stat(targetPath); err == nil {
				switch mode {
				case promptNoClobber:
					continue
				case promptInteractive:
					fmt.Fprintf(stderr, "cp: overwrite '%s'? ", targetDisplay)
					line, _ := stdinReader.ReadString('\n')
					line = strings.TrimRight(line, "\r\n")
					if line == "" || (line[0] != 'y' && line[0] != 'Y') {
						continue
					}
				}
			}
		}

		if err := copyTree(ec.FS, srcPath, targetPath, srcInfo, *preserve, mode == promptForce, effSym, true); err != nil {
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

func copyTree(fs billy.Filesystem, src, dst string, info os.FileInfo, preserve, force bool, sym symlinkMode, isCmdLine bool) error {
	if info == nil {
		// During recursion, decide whether to follow links based on the mode.
		// -L follows everything; -H follows only command-line links; -P never.
		follow := sym == symlinkFollowAll || (sym == symlinkFollowCmdLine && isCmdLine)
		i, err := statSrc(fs, src, follow)
		if err != nil {
			return err
		}
		info = i
	}

	// Symlink that we are not following: recreate it at the destination.
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(fs, src, dst, force)
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
			// Children are never command-line operands.
			if err := copyTree(fs, path.Join(src, e.Name()), path.Join(dst, e.Name()), nil, preserve, force, sym, false); err != nil {
				return err
			}
		}
		if preserve {
			applyPreserve(fs, dst, info)
		}
		return nil
	}
	if err := copyFile(fs, src, dst, info, force); err != nil {
		return err
	}
	if preserve {
		applyPreserve(fs, dst, info)
	}
	return nil
}

// statSrc returns FileInfo for src. When follow is true it uses Stat
// (resolving symlinks). When follow is false it tries Lstat via the
// billy.Symlink capability; if the underlying filesystem doesn't expose
// Lstat, behavior degrades to Stat (links are always followed). That
// limitation is inherent to the billy abstraction.
func statSrc(fsys billy.Filesystem, src string, follow bool) (os.FileInfo, error) {
	if follow {
		return fsys.Stat(src)
	}
	if sl, ok := fsys.(billy.Symlink); ok {
		return sl.Lstat(src)
	}
	return fsys.Stat(src)
}

// copySymlink recreates the symlink at dst pointing to the same target as
// src. If the underlying filesystem does not support symlinks, it returns
// a clear "not supported" diagnostic.
func copySymlink(fs billy.Filesystem, src, dst string, force bool) error {
	sl, ok := fs.(billy.Symlink)
	if !ok {
		return fmt.Errorf("symbolic links not supported by this filesystem")
	}
	target, err := sl.Readlink(src)
	if err != nil {
		return err
	}
	// Best-effort overwrite: remove an existing destination so that
	// Symlink doesn't fail with EEXIST. -f makes this aggressive; without
	// it we still try once because billy has no atomic replace.
	if _, statErr := fs.Stat(dst); statErr == nil {
		if rmErr := fs.Remove(dst); rmErr != nil && force {
			return rmErr
		}
	}
	return sl.Symlink(target, dst)
}

func copyFile(fs billy.Filesystem, src, dst string, srcInfo os.FileInfo, force bool) error {
	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := fs.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		// Spec -f: if a file descriptor for the destination cannot be
		// obtained, attempt to unlink the destination and try again.
		if force {
			if _, statErr := fs.Stat(dst); statErr == nil {
				if rmErr := fs.Remove(dst); rmErr == nil {
					out, err = fs.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
				}
			}
		}
		if err != nil {
			return err
		}
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func applyPreserve(fs billy.Filesystem, dst string, srcInfo os.FileInfo) {
	if chm, ok := fs.(billy.Chmod); ok {
		_ = chm.Chmod(dst, srcInfo.Mode().Perm())
	}
	if ch, ok := fs.(billy.Change); ok {
		mt := srcInfo.ModTime()
		_ = ch.Chtimes(dst, mt, mt)
	}
}

func stdinOrEmpty(r io.Reader) io.Reader {
	if r == nil {
		return strings.NewReader("")
	}
	return r
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
