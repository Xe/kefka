package mv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

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

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("mv: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("mv: ExecContext has no filesystem")
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
	set.SetProgram("mv")
	set.SetParameters("SOURCE... DEST")

	usage := func() {
		fmt.Fprint(stderr, "Usage: mv [OPTION]... SOURCE... DEST\n")
		fmt.Fprint(stderr, "Move (rename) files.\n\n")
		fmt.Fprint(stderr, "  -f, --force        do not prompt before overwriting\n")
		fmt.Fprint(stderr, "  -i, --interactive  prompt before overwrite\n")
		fmt.Fprint(stderr, "  -n, --no-clobber   do not overwrite an existing file\n")
		fmt.Fprint(stderr, "  -v, --verbose      explain what is being done\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n")
	}
	set.SetUsage(usage)

	_ = set.BoolLong("force", 'f', "do not prompt before overwriting")
	_ = set.BoolLong("interactive", 'i', "prompt before overwrite")
	_ = set.BoolLong("no-clobber", 'n', "do not overwrite an existing file")
	verbose := set.BoolLong("verbose", 'v', "explain what is being done")
	help := set.BoolLong("help", 0, "display this help and exit")

	mode := promptDefault
	cb := func(opt getopt.Option) bool {
		switch opt.LongName() {
		case "force":
			mode = promptForce
		case "interactive":
			mode = promptInteractive
		case "no-clobber":
			mode = promptNoClobber
		}
		return true
	}

	if err := set.Getopt(append([]string{"mv"}, args...), cb); err != nil {
		fmt.Fprintf(stderr, "mv: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	paths := set.Args()
	if len(paths) < 2 {
		fmt.Fprint(stderr, "mv: missing destination file operand\n")
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
		fmt.Fprintf(stderr, "mv: target '%s' is not a directory\n", dest)
		return interp.ExitStatus(1)
	}

	stdinReader := bufio.NewReader(stdinOrEmpty(ec.Stdin))

	// POSIX (mv.md DESCRIPTION): if source is a non-directory and the target
	// ends with a trailing slash, this is an error and no sources are
	// processed. This only applies to the single-source rename form, since
	// the multi-source form already requires the dest to be an existing
	// directory.
	destHasTrailingSlash := strings.HasSuffix(dest, "/") && dest != "/"
	if destHasTrailingSlash && !destIsDir && len(sources) == 1 {
		srcPath := resolvePath(ec, sources[0])
		if info, err := ec.FS.Stat(srcPath); err == nil && !info.IsDir() {
			fmt.Fprintf(stderr, "mv: cannot move '%s' to '%s': Not a directory\n", sources[0], dest)
			return interp.ExitStatus(1)
		}
	}

	exitCode := 0
	for _, src := range sources {
		srcPath := resolvePath(ec, src)
		if _, err := ec.FS.Stat(srcPath); err != nil {
			fmt.Fprintf(stderr, "mv: cannot stat '%s': No such file or directory\n", src)
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
			fmt.Fprintf(stderr, "mv: '%s' and '%s' are the same file\n", src, targetDisplay)
			exitCode = 1
			continue
		}

		if _, err := ec.FS.Stat(targetPath); err == nil {
			switch mode {
			case promptNoClobber:
				continue
			case promptInteractive:
				fmt.Fprintf(stderr, "mv: overwrite '%s'? ", targetDisplay)
				line, _ := stdinReader.ReadString('\n')
				line = strings.TrimRight(line, "\r\n")
				if line == "" || (line[0] != 'y' && line[0] != 'Y') {
					continue
				}
			}
		}

		// Cross-filesystem fallback (POSIX mv.md steps 4-7: copy hierarchy,
		// preserve metadata, unlink source) is intentionally unimplemented.
		// kefka's command FS is a single billy.Filesystem at any moment,
		// so EXDEV is not reachable from within one Exec call. If billy
		// returns a "cross device" error from its Rename for any other
		// reason, the move will fail rather than silently copy. uid/gid
		// preservation similarly is not exposed by billy.
		if err := ec.FS.Rename(srcPath, targetPath); err != nil {
			fmt.Fprintf(stderr, "mv: cannot move '%s': %v\n", src, err)
			exitCode = 1
			continue
		}

		if *verbose {
			fmt.Fprintf(stdout, "renamed '%s' -> '%s'\n", src, targetDisplay)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
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
