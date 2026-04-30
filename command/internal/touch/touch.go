package touch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
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
		fmt.Fprint(stderr, "  -a                 (ignored) change only the access time\n")
		fmt.Fprint(stderr, "  -c, --no-create    do not create any files\n")
		fmt.Fprint(stderr, "  -d, --date=STRING  parse STRING and use it instead of current time\n")
		fmt.Fprint(stderr, "  -m                 (ignored) change only the modification time\n")
		fmt.Fprint(stderr, "  -r, --reference=FILE  (ignored) use this file's times instead of current time\n")
		fmt.Fprint(stderr, "  -t STAMP           (ignored) use [[CC]YY]MMDDhhmm[.ss] instead of current time\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n")
	}
	set.SetUsage(usage)

	noCreate := set.BoolLong("no-create", 'c', "do not create any files")
	dateStr := set.StringLong("date", 'd', "", "parse STRING and use it instead of current time")
	_ = set.Bool('a', "(ignored) change only the access time")
	_ = set.Bool('m', "(ignored) change only the modification time")
	_ = set.StringLong("reference", 'r', "", "(ignored) use this file's times instead of current time")
	_ = set.String('t', "", "(ignored) use [[CC]YY]MMDDhhmm[.ss] instead of current time")
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

	var targetTime *time.Time
	if *dateStr != "" {
		parsed, ok := parseDateString(*dateStr)
		if !ok {
			fmt.Fprintf(stderr, "touch: invalid date format '%s'\n", *dateStr)
			return interp.ExitStatus(1)
		}
		targetTime = &parsed
	}

	exitCode := 0
	for _, file := range files {
		full := resolvePath(ec, file)

		_, err := ec.FS.Stat(full)
		exists := err == nil
		if !exists {
			if *noCreate {
				continue
			}
			f, createErr := ec.FS.OpenFile(full, os.O_CREATE|os.O_WRONLY, 0o644)
			if createErr != nil {
				fmt.Fprintf(stderr, "touch: cannot touch '%s': %s\n", file, createErr)
				exitCode = 1
				continue
			}
			f.Close()
		}

		if changer, ok := ec.FS.(billy.Change); ok {
			mtime := time.Now()
			if targetTime != nil {
				mtime = *targetTime
			}
			if err := changer.Chtimes(full, mtime, mtime); err != nil {
				fmt.Fprintf(stderr, "touch: cannot touch '%s': %s\n", file, err)
				exitCode = 1
				continue
			}
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
