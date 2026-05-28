package touch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-billy/v6"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("touch: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("touch: ExecContext has no filesystem")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("touch")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: touch [OPTION]... FILE...\n")
		fmt.Fprint(stderr, "Update the access and modification times of each FILE to the current time.\n\n")
		fmt.Fprint(stderr, "A FILE argument that does not exist is created empty, unless -c is supplied.\n\n")
		fmt.Fprint(stderr, "  -a                 change only the access time\n")
		fmt.Fprint(stderr, "  -c, --no-create    do not create any files\n")
		fmt.Fprint(stderr, "  -d, --date=STRING  parse STRING and use it instead of current time\n")
		fmt.Fprint(stderr, "  -h, --no-dereference  affect each symbolic link rather than its referent\n")
		fmt.Fprint(stderr, "                     (only effective when the backend supports Lstat;\n")
		fmt.Fprint(stderr, "                     time updates still follow the link)\n")
		fmt.Fprint(stderr, "  -m                 change only the modification time\n")
		fmt.Fprint(stderr, "  -r, --reference=FILE  use this file's times instead of current time\n")
		fmt.Fprint(stderr, "  -t STAMP           use [[CC]YY]MMDDhhmm[.ss] instead of current time\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n")
	}
	set.SetUsage(usage)

	noCreate := set.BoolLong("no-create", 'c', "do not create any files")
	dateStr := set.StringLong("date", 'd', "", "parse STRING and use it instead of current time")
	aFlag := set.Bool('a', "change only the access time")
	mFlag := set.Bool('m', "change only the modification time")
	hFlag := set.BoolLong("no-dereference", 'h', "affect each symbolic link rather than any referenced file")
	refFile := set.StringLong("reference", 'r', "", "use this file's times instead of current time")
	tStamp := set.String('t', "", "use [[CC]YY]MMDDhhmm[.ss] instead of current time")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"touch"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "touch: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		fmt.Fprint(stderr, "touch: missing file operand\n")
		return interp.ExitStatus(1)
	}

	sources := 0
	if *refFile != "" {
		sources++
	}
	if *tStamp != "" {
		sources++
	}
	if *dateStr != "" {
		sources++
	}
	if sources > 1 {
		fmt.Fprint(stderr, "touch: cannot specify times from more than one source\n")
		return interp.ExitStatus(1)
	}

	var (
		atime, mtime time.Time
		haveTimes    bool
	)
	switch {
	case *refFile != "":
		ref := resolvePath(ec, *refFile)
		info, err := statMaybeLink(ec, ref, *hFlag)
		if err != nil {
			fmt.Fprintf(stderr, "touch: failed to get attributes of '%s': %s\n", *refFile, err)
			return interp.ExitStatus(1)
		}
		// billy.FileInfo only exposes ModTime; atime is not separately
		// tracked, so we use ModTime for both halves. Real GNU touch -r
		// copies atime and mtime independently.
		mtime = info.ModTime()
		atime = info.ModTime()
		haveTimes = true
	case *tStamp != "":
		parsed, ok := parseTStamp(*tStamp)
		if !ok {
			fmt.Fprintf(stderr, "touch: invalid date format '%s'\n", *tStamp)
			return interp.ExitStatus(1)
		}
		atime = parsed
		mtime = parsed
		haveTimes = true
	case *dateStr != "":
		parsed, ok := parseDateString(*dateStr)
		if !ok {
			fmt.Fprintf(stderr, "touch: invalid date format '%s'\n", *dateStr)
			return interp.ExitStatus(1)
		}
		atime = parsed
		mtime = parsed
		haveTimes = true
	}

	setAtime := *aFlag || (!*aFlag && !*mFlag)
	setMtime := *mFlag || (!*aFlag && !*mFlag)

	// Spec creation mode is 0o666 modified by umask. Billy/Kefka does not
	// expose process umask, so we assume the conventional 0o022 — the same
	// fallback used by the mkdir port. With umask 022 this yields 0o644,
	// matching the historical GNU coreutils default.
	const assumedUmask os.FileMode = 0o022
	createMode := os.FileMode(0o666) &^ assumedUmask

	exitCode := 0
	for _, file := range files {
		full := resolvePath(ec, file)

		info, err := statMaybeLink(ec, full, *hFlag)
		exists := err == nil
		if !exists {
			if *noCreate {
				continue
			}
			f, createErr := ec.FS.OpenFile(full, os.O_CREATE|os.O_WRONLY, createMode)
			if createErr != nil {
				fmt.Fprintf(stderr, "touch: cannot touch '%s': %s\n", file, createErr)
				exitCode = 1
				continue
			}
			f.Close()
			info, err = statMaybeLink(ec, full, *hFlag)
			if err != nil {
				fmt.Fprintf(stderr, "touch: cannot touch '%s': %s\n", file, err)
				exitCode = 1
				continue
			}
		}

		changer, ok := ec.FS.(billy.Change)
		if !ok {
			continue
		}

		now := time.Now()
		newAtime := now
		newMtime := now
		if haveTimes {
			newAtime = atime
			newMtime = mtime
		}

		finalAtime := newAtime
		finalMtime := newMtime
		if !setAtime {
			finalAtime = info.ModTime()
		}
		if !setMtime {
			finalMtime = info.ModTime()
		}

		if err := changer.Chtimes(full, finalAtime, finalMtime); err != nil {
			fmt.Fprintf(stderr, "touch: cannot touch '%s': %s\n", file, err)
			exitCode = 1
			continue
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func parseDateString(s string) (time.Time, bool) {
	normalized := strings.ReplaceAll(s, "/", "-")

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t, true
		}
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, normalized, time.Local); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

// parseTStamp parses a -t argument of the form [[CC]YY]MMDDhhmm[.SS].
// All times are interpreted as UTC since the sandbox runs in UTC.
func parseTStamp(s string) (time.Time, bool) {
	main, secStr, hasSec := strings.Cut(s, ".")
	if hasSec {
		if len(secStr) != 2 {
			return time.Time{}, false
		}
		for _, c := range secStr {
			if c < '0' || c > '9' {
				return time.Time{}, false
			}
		}
	}
	for _, c := range main {
		if c < '0' || c > '9' {
			return time.Time{}, false
		}
	}

	year := time.Now().UTC().Year()
	var datePart string
	switch len(main) {
	case 8:
		datePart = main
	case 10:
		yy, err := strconv.Atoi(main[:2])
		if err != nil {
			return time.Time{}, false
		}
		if yy < 69 {
			year = 2000 + yy
		} else {
			year = 1900 + yy
		}
		datePart = main[2:]
	case 12:
		cc, err := strconv.Atoi(main[:2])
		if err != nil {
			return time.Time{}, false
		}
		yy, err := strconv.Atoi(main[2:4])
		if err != nil {
			return time.Time{}, false
		}
		year = cc*100 + yy
		datePart = main[4:]
	default:
		return time.Time{}, false
	}

	month, err := strconv.Atoi(datePart[0:2])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(datePart[2:4])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	hour, err := strconv.Atoi(datePart[4:6])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, false
	}
	minute, err := strconv.Atoi(datePart[6:8])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	second := 0
	if secStr != "" {
		second, err = strconv.Atoi(secStr)
		if err != nil || second < 0 || second > 60 {
			return time.Time{}, false
		}
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), true
}

// statMaybeLink returns Lstat(name) when noDeref is set and the backend
// implements billy.Symlink, otherwise falls back to Stat. Note that even
// when noDeref is true, the subsequent Chtimes call follows the symlink
// because billy has no Lchtimes equivalent — this matches the limitation
// documented in the help text.
func statMaybeLink(ec *command.ExecContext, name string, noDeref bool) (os.FileInfo, error) {
	if noDeref {
		if sym, ok := ec.FS.(billy.Symlink); ok {
			return sym.Lstat(name)
		}
	}
	return ec.FS.Stat(name)
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
