package rm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("rm: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("rm: ExecContext has no filesystem")
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
	set.SetProgram("rm")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: rm [OPTION]... FILE...\n")
		fmt.Fprint(stderr, "Remove (unlink) the FILE(s).\n\n")
		fmt.Fprint(stderr, "  -f, --force           ignore nonexistent files and arguments, never prompt\n")
		fmt.Fprint(stderr, "  -i, --interactive     prompt before every removal\n")
		fmt.Fprint(stderr, "  -I                    prompt once before removing more than three files,\n")
		fmt.Fprint(stderr, "                          or when removing recursively; less intrusive than -i\n")
		fmt.Fprint(stderr, "  -d, --dir             remove empty directories\n")
		fmt.Fprint(stderr, "  -r, -R, --recursive   remove directories and their contents recursively\n")
		fmt.Fprint(stderr, "  -v, --verbose         explain what is being done\n")
		fmt.Fprint(stderr, "      --help            display this help and exit\n")
	}
	set.SetUsage(usage)

	recursive := set.BoolLong("recursive", 'r', "remove directories and their contents recursively")
	recursiveUpper := set.Bool('R', "remove directories and their contents recursively")
	force := set.BoolLong("force", 'f', "ignore nonexistent files and arguments, never prompt")
	interactive := set.BoolLong("interactive", 'i', "prompt before every removal")
	lessInteractive := set.Bool('I', "prompt once before removing more than three files, or when removing recursively")
	emptyDir := set.BoolLong("dir", 'd', "remove empty directories")
	verbose := set.BoolLong("verbose", 'v', "explain what is being done")
	help := set.BoolLong("help", 0, "display this help and exit")

	// Track which of -f / -i / -I was specified last for "last one wins"
	// semantics per POSIX (each suppresses previous occurrences of the others).
	lastMode := "" // "force", "interactive", "less", or ""
	cb := func(opt getopt.Option) bool {
		switch {
		case opt.LongName() == "force" || opt.ShortName() == "f":
			lastMode = "force"
		case opt.LongName() == "interactive" || opt.ShortName() == "i":
			lastMode = "interactive"
		case opt.ShortName() == "I":
			lastMode = "less"
		}
		return true
	}

	if err := set.Getopt(append([]string{"rm"}, args...), cb); err != nil {
		fmt.Fprintf(stderr, "rm: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	recurse := *recursive || *recursiveUpper
	effForce := *force
	effInteractive := *interactive
	effLess := *lessInteractive
	switch lastMode {
	case "force":
		effInteractive = false
		effLess = false
	case "interactive":
		effForce = false
		effLess = false
	case "less":
		effForce = false
		effInteractive = false
	}

	paths := set.Args()

	if len(paths) == 0 {
		if effForce {
			return nil
		}
		fmt.Fprint(stderr, "rm: missing operand\n")
		return interp.ExitStatus(1)
	}

	var stdinReader *bufio.Reader
	if ec.Stdin != nil {
		stdinReader = bufio.NewReader(ec.Stdin)
	}

	pr := &prompter{
		stderr:      stderr,
		stdin:       stdinReader,
		interactive: effInteractive,
	}

	// -I: one prompt up front when recursing or when removing 3+ operands.
	// A negative answer suppresses all removals. -f/-i wins over -I (handled
	// above via lastMode).
	if effLess && pr.stdin != nil {
		var msg string
		switch {
		case recurse:
			msg = fmt.Sprintf("rm: remove %d argument%s recursively? ", len(paths), plural(len(paths)))
		case len(paths) > 3:
			msg = fmt.Sprintf("rm: remove %d arguments? ", len(paths))
		}
		if msg != "" && !pr.ask(msg) {
			return nil
		}
	}

	exitCode := 0
	for _, p := range paths {
		if err := removeOne(ec, stdout, stderr, p, recurse, *emptyDir, effForce, pr, *verbose); err != nil {
			exitCode = 1
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// prompter centralizes confirmation reads so that we never read more from
// stdin than necessary and never block when no stdin is attached.
type prompter struct {
	stderr      io.Writer
	stdin       *bufio.Reader
	interactive bool
}

// shouldPrompt reports whether a prompt is required for a non-directory file
// per POSIX rm step 2b/3: prompt when -i is set, or when -f is not set AND
// the file is not writable AND stdin is a terminal. kefka has no terminal
// detection, so we approximate "stdin is a terminal" as "stdin is attached"
// for the write-protected case. -f always wins.
func (p *prompter) shouldPrompt(force, writeProtected bool) bool {
	if force {
		return false
	}
	if p.interactive {
		return true
	}
	if writeProtected && p.stdin != nil {
		return true
	}
	return false
}

// ask writes msg to stderr and reads a response. Returns true only when the
// first non-whitespace character is 'y' or 'Y'. EOF / no stdin / blank line
// all count as a non-affirmative response.
func (p *prompter) ask(msg string) bool {
	fmt.Fprint(p.stderr, msg)
	if p.stdin == nil {
		return false
	}
	line, err := p.stdin.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	line = strings.TrimLeft(line, " \t")
	if line == "" {
		return false
	}
	c := line[0]
	return c == 'y' || c == 'Y'
}

func removeOne(ec *command.ExecContext, stdout, stderr io.Writer, p string, recurse, emptyDir, force bool, pr *prompter, verbose bool) error {
	full := resolvePath(ec, p)
	info, err := lstat(ec.FS, full)
	if err != nil {
		if !force {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stderr, "rm: cannot remove '%s': No such file or directory\n", p)
			} else {
				fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", p, err)
			}
			return err
		}
		return nil
	}

	// Symlinks are removed as-is, never followed. POSIX rm.md step 2c.
	if info.Mode()&fs.ModeSymlink != 0 {
		return removeFile(ec, stdout, stderr, p, full, info, force, pr, verbose)
	}

	if info.IsDir() {
		if !recurse {
			if emptyDir {
				return removeEmptyDir(ec, stdout, stderr, p, full, info, force, pr, verbose)
			}
			fmt.Fprintf(stderr, "rm: cannot remove '%s': Is a directory\n", p)
			return errors.New("is a directory")
		}
		return removeDir(ec, stdout, stderr, p, full, info, force, pr, verbose)
	}

	return removeFile(ec, stdout, stderr, p, full, info, force, pr, verbose)
}

func removeFile(ec *command.ExecContext, stdout, stderr io.Writer, displayPath, full string, info fs.FileInfo, force bool, pr *prompter, verbose bool) error {
	wp := isWriteProtected(info)
	if pr.shouldPrompt(force, wp) {
		msg := fileRemovePrompt(displayPath, info, wp)
		if !pr.ask(msg) {
			return nil
		}
	}
	if err := ec.FS.Remove(full); err != nil {
		if !force {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
			return err
		}
		return nil
	}
	if verbose {
		fmt.Fprintf(stdout, "removed '%s'\n", displayPath)
	}
	return nil
}

// removeEmptyDir handles -d (GNU extension): remove a single empty directory
// without -r.
func removeEmptyDir(ec *command.ExecContext, stdout, stderr io.Writer, displayPath, full string, info fs.FileInfo, force bool, pr *prompter, verbose bool) error {
	entries, err := ec.FS.ReadDir(full)
	if err != nil {
		if !force {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
			return err
		}
		return nil
	}
	if len(entries) > 0 {
		fmt.Fprintf(stderr, "rm: cannot remove '%s': Directory not empty\n", displayPath)
		return errors.New("directory not empty")
	}
	wp := isWriteProtected(info)
	if pr.shouldPrompt(force, wp) {
		if !pr.ask(fmt.Sprintf("rm: remove directory '%s'? ", displayPath)) {
			return nil
		}
	}
	if err := ec.FS.Remove(full); err != nil {
		if !force {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
			return err
		}
		return nil
	}
	if verbose {
		fmt.Fprintf(stdout, "removed directory '%s'\n", displayPath)
	}
	return nil
}

func removeDir(ec *command.ExecContext, stdout, stderr io.Writer, displayPath, full string, info fs.FileInfo, force bool, pr *prompter, verbose bool) error {
	entries, err := ec.FS.ReadDir(full)
	if err != nil {
		if !force {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
			return err
		}
		return nil
	}

	wp := isWriteProtected(info)
	// Empty-directory shortcut: prompt once with "remove directory" wording
	// (POSIX 2b allows skipping straight to 2d for empty dirs).
	if len(entries) == 0 {
		if pr.shouldPrompt(force, wp) {
			if !pr.ask(fmt.Sprintf("rm: remove directory '%s'? ", displayPath)) {
				return nil
			}
		}
		if err := ec.FS.Remove(full); err != nil {
			if !force {
				fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
				return err
			}
			return nil
		}
		if verbose {
			fmt.Fprintf(stdout, "removed directory '%s'\n", displayPath)
		}
		return nil
	}

	// Non-empty directory: prompt to descend (only when interactive).
	if pr.shouldPrompt(force, wp) {
		if !pr.ask(fmt.Sprintf("rm: descend into directory '%s'? ", displayPath)) {
			return nil
		}
	}

	// Walk children manually so we never follow a symlink-to-directory.
	for _, entry := range entries {
		childDisplay := joinDisplay(displayPath, entry.Name())
		childFull := path.Join(full, entry.Name())
		// Re-stat with Lstat to make symlink decisions explicit and to
		// avoid following them into the target directory.
		childInfo, err := lstat(ec.FS, childFull)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Lost a race or already removed; skip silently.
				continue
			}
			if !force {
				fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", childDisplay, formatErr(err))
				return err
			}
			continue
		}
		if childInfo.Mode()&fs.ModeSymlink != 0 {
			if err := removeFile(ec, stdout, stderr, childDisplay, childFull, childInfo, force, pr, verbose); err != nil {
				return err
			}
			continue
		}
		if childInfo.IsDir() {
			if err := removeDir(ec, stdout, stderr, childDisplay, childFull, childInfo, force, pr, verbose); err != nil {
				return err
			}
			continue
		}
		if err := removeFile(ec, stdout, stderr, childDisplay, childFull, childInfo, force, pr, verbose); err != nil {
			return err
		}
	}

	// Final prompt before removing the now-empty directory itself (POSIX 2d).
	if pr.shouldPrompt(force, wp) {
		if !pr.ask(fmt.Sprintf("rm: remove directory '%s'? ", displayPath)) {
			return nil
		}
	}
	if err := ec.FS.Remove(full); err != nil {
		if !force {
			fmt.Fprintf(stderr, "rm: cannot remove '%s': %s\n", displayPath, formatErr(err))
			return err
		}
		return nil
	}
	if verbose {
		fmt.Fprintf(stdout, "removed directory '%s'\n", displayPath)
	}
	return nil
}

// fileRemovePrompt returns the GNU-style prompt string for removing a
// non-directory file, distinguishing regular / symlink / write-protected.
func fileRemovePrompt(displayPath string, info fs.FileInfo, writeProtected bool) string {
	kind := fileKindForPrompt(info)
	if writeProtected {
		return fmt.Sprintf("rm: remove write-protected %s '%s'? ", kind, displayPath)
	}
	return fmt.Sprintf("rm: remove %s '%s'? ", kind, displayPath)
}

func fileKindForPrompt(info fs.FileInfo) string {
	m := info.Mode()
	switch {
	case m&fs.ModeSymlink != 0:
		return "symbolic link"
	case m&fs.ModeNamedPipe != 0:
		return "fifo"
	case m&fs.ModeSocket != 0:
		return "socket"
	case m&fs.ModeDevice != 0:
		return "device"
	case info.Size() == 0 && m.IsRegular():
		return "regular empty file"
	default:
		return "regular file"
	}
}

// isWriteProtected approximates POSIX "permissions do not permit writing"
// using the file's mode owner-write bit. In billy's memfs this isn't
// enforced, but the bit is still recorded and observable.
func isWriteProtected(info fs.FileInfo) bool {
	if info == nil {
		return false
	}
	// Symlinks themselves don't have meaningful permissions; treat as writable.
	if info.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	return info.Mode().Perm()&0o200 == 0
}

// lstat returns FileInfo without following symlinks. On filesystems that
// don't expose billy.Symlink, it falls back to Stat, which means symlinks
// can't be detected — same trade-off as elsewhere in kefka.
func lstat(fsys billy.Filesystem, name string) (fs.FileInfo, error) {
	if sl, ok := fsys.(billy.Symlink); ok {
		return sl.Lstat(name)
	}
	return fsys.Stat(name)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func joinDisplay(displayPath, name string) string {
	if displayPath == "." {
		return name
	}
	return path.Join(displayPath, name)
}

func formatErr(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "No such file or directory"
	case isNotEmpty(err):
		return "Directory not empty"
	default:
		return err.Error()
	}
}

func isNotEmpty(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not empty") || strings.Contains(msg, "ENOTEMPTY")
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
