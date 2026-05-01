package mkdir

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

const assumedUmask os.FileMode = 0o022

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("mkdir: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("mkdir: ExecContext has no filesystem")
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
	set.SetProgram("mkdir")
	set.SetParameters("DIRECTORY...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: mkdir [OPTION]... DIRECTORY...\n")
		fmt.Fprint(stderr, "Create the DIRECTORY(ies), if they do not already exist.\n\n")
		fmt.Fprint(stderr, "  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask\n")
		fmt.Fprint(stderr, "  -p, --parents     no error if existing, make parent directories as needed\n")
		fmt.Fprint(stderr, "  -v, --verbose     print a message for each created directory\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	modeSpec := set.StringLong("mode", 'm', "", "set file mode (as in chmod), not a=rwx - umask")
	parents := set.BoolLong("parents", 'p', "no error if existing, make parent directories as needed")
	verbose := set.BoolLong("verbose", 'v', "print a message for each created directory")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"mkdir"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "mkdir: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	dirs := set.Args()
	if len(dirs) == 0 {
		fmt.Fprint(stderr, "mkdir: missing operand\n")
		return interp.ExitStatus(1)
	}

	// Default mode for the final directory: 0o777 with umask applied.
	// Intermediate directories created with -p get u+wx forced on top of
	// the default so the user can always traverse them, regardless of the
	// umask, per POSIX (S_IWUSR|S_IXUSR|~filemask) & 0777.
	leafMode := os.FileMode(0o777) &^ assumedUmask
	intermediateMode := (os.FileMode(0o300) | (os.FileMode(0o777) &^ assumedUmask)) & 0o777
	modeSet := false
	if *modeSpec != "" {
		m, ok := parseMode(*modeSpec, assumedUmask)
		if !ok {
			fmt.Fprintf(stderr, "mkdir: invalid mode '%s'\n", *modeSpec)
			return interp.ExitStatus(1)
		}
		leafMode = m
		modeSet = true
	}

	exitCode := 0
	for _, dir := range dirs {
		full := resolvePath(ec, dir)

		if !*parents {
			if _, err := ec.FS.Stat(full); err == nil {
				fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': File exists\n", dir)
				exitCode = 1
				continue
			}
			parent := path.Dir(full)
			if parent != "." && parent != "/" && parent != "" {
				if _, err := ec.FS.Stat(parent); err != nil {
					fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': No such file or directory\n", dir)
					exitCode = 1
					continue
				}
			}

			// Without -p, just create the leaf directly.
			if err := ec.FS.MkdirAll(full, leafMode); err != nil {
				fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': %s\n", dir, err)
				exitCode = 1
				continue
			}
		} else {
			// With -p, create each missing intermediate with the
			// intermediate mode; create (or leave alone) the leaf with the
			// leaf mode.
			if err := mkdirAllWithModes(ec.FS, full, leafMode, intermediateMode); err != nil {
				fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': %s\n", dir, err)
				exitCode = 1
				continue
			}
		}

		// If -m was given, ensure the leaf has exactly the requested mode
		// (the chmod is required by POSIX since the mkdir mode argument's
		// effective value is unspecified when -m is in use).
		if modeSet {
			if ch, ok := ec.FS.(billy.Chmod); ok {
				if err := ch.Chmod(full, leafMode|os.ModeDir); err != nil {
					fmt.Fprintf(stderr, "mkdir: cannot set permissions of '%s': %s\n", dir, err)
					exitCode = 1
					continue
				}
			}
		}

		if *verbose {
			fmt.Fprintf(stdout, "mkdir: created directory '%s'\n", dir)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// mkdirAllWithModes is the -p path: it creates each missing intermediate
// directory of full with intermediateMode, and the leaf with leafMode.
// Existing directories along the path are left alone (silent, no error).
func mkdirAllWithModes(fs billy.Filesystem, full string, leafMode, intermediateMode os.FileMode) error {
	if full == "" || full == "." || full == "/" {
		return nil
	}
	cleaned := path.Clean(full)
	parts := strings.Split(cleaned, "/")
	cur := ""
	for i, p := range parts {
		if p == "" {
			// Leading "/" produces an empty first segment; skip it.
			continue
		}
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		isLeaf := i == len(parts)-1
		mode := intermediateMode
		if isLeaf {
			mode = leafMode
		}
		if info, err := fs.Stat(cur); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", cur)
			}
			// Already exists: leave permissions as-is. The leaf will be
			// chmodded by the caller iff -m was supplied.
			continue
		}
		if err := fs.MkdirAll(cur, mode); err != nil {
			return err
		}
		if ch, ok := fs.(billy.Chmod); ok {
			if err := ch.Chmod(cur, mode|os.ModeDir); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseMode(spec string, umask os.FileMode) (os.FileMode, bool) {
	if spec == "" {
		return 0, false
	}
	if isOctalSpec(spec) {
		return parseOctalMode(spec)
	}
	return parseSymbolicMode(spec, umask)
}

func isOctalSpec(spec string) bool {
	s := spec
	if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		s = s[2:]
	}
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '7' {
			return false
		}
	}
	return true
}

func parseOctalMode(spec string) (os.FileMode, bool) {
	s := spec
	if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		s = s[2:]
	}
	if len(s) < 1 || len(s) > 4 {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, false
	}
	if n > 0o7777 {
		return 0, false
	}
	return os.FileMode(n), true
}

func parseSymbolicMode(spec string, umask os.FileMode) (os.FileMode, bool) {
	mode := os.FileMode(0o777) &^ umask
	for clause := range strings.SplitSeq(spec, ",") {
		m, ok := applySymbolicClause(mode, clause)
		if !ok {
			return 0, false
		}
		mode = m
	}
	return mode & 0o7777, true
}

func applySymbolicClause(mode os.FileMode, clause string) (os.FileMode, bool) {
	i := 0
	whoMask := os.FileMode(0)
	whoSet := false
	for i < len(clause) {
		c := clause[i]
		switch c {
		case 'u':
			whoMask |= 0o700 | os.ModeSetuid
		case 'g':
			whoMask |= 0o070 | os.ModeSetgid
		case 'o':
			whoMask |= 0o007
		case 'a':
			whoMask |= 0o777 | os.ModeSetuid | os.ModeSetgid
		default:
			goto doneWho
		}
		whoSet = true
		i++
	}
doneWho:
	if i >= len(clause) {
		return 0, false
	}
	op := clause[i]
	if op != '+' && op != '-' && op != '=' {
		return 0, false
	}
	i++

	perm := os.FileMode(0)
	permSet := false
	for i < len(clause) {
		c := clause[i]
		switch c {
		case 'r':
			perm |= 0o444
		case 'w':
			perm |= 0o222
		case 'x', 'X':
			perm |= 0o111
		case 's':
			perm |= os.ModeSetuid | os.ModeSetgid
		case 't':
			perm |= os.ModeSticky
		default:
			return 0, false
		}
		permSet = true
		i++
	}

	if op != '=' && !whoSet && !permSet {
		return 0, false
	}

	var applyMask os.FileMode
	if whoSet {
		applyMask = whoMask
	} else {
		applyMask = 0o777 | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	}

	switch op {
	case '+':
		if whoSet {
			mode |= perm & applyMask
		} else {
			mode |= perm &^ os.FileMode(assumedUmask)
		}
	case '-':
		if whoSet {
			mode &^= perm & applyMask
		} else {
			mode &^= perm
		}
	case '=':
		mode &^= applyMask
		if permSet {
			if whoSet {
				mode |= perm & applyMask
			} else {
				mode |= perm &^ os.FileMode(assumedUmask)
			}
		}
	}

	return mode, true
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
