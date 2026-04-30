package stat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

var formatTime = func(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("stat: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("stat: ExecContext has no filesystem")
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
	set.SetProgram("stat")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: stat [OPTION]... FILE...\n")
		fmt.Fprint(stderr, "Display file or file system status.\n\n")
		fmt.Fprint(stderr, "  -c FORMAT   use the specified FORMAT instead of the default\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	format := set.String('c', "", "use the specified FORMAT instead of the default")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"stat"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "stat: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		fmt.Fprint(stderr, "stat: missing operand\n")
		return interp.ExitStatus(1)
	}

	hasError := false
	for _, file := range files {
		full := resolvePath(ec, file)
		info, err := ec.FS.Stat(full)
		if err != nil {
			fmt.Fprintf(stderr, "stat: cannot stat '%s': No such file or directory\n", file)
			hasError = true
			continue
		}

		if *format != "" {
			io.WriteString(stdout, applyFormat(*format, file, info)+"\n")
			continue
		}

		mode := info.Mode().Perm()
		modeOctal := fmt.Sprintf("%04o", uint32(mode))
		modeStr := formatMode(mode, info.IsDir())
		size := info.Size()
		blocks := (size + 511) / 512
		fmt.Fprintf(stdout, "  File: %s\n", file)
		fmt.Fprintf(stdout, "  Size: %d\t\tBlocks: %d\n", size, blocks)
		fmt.Fprintf(stdout, "Access: (%s/%s)\n", modeOctal, modeStr)
		fmt.Fprintf(stdout, "Modify: %s\n", formatTime(info.ModTime()))
	}

	if hasError {
		return interp.ExitStatus(1)
	}
	return nil
}

func applyFormat(format, file string, info os.FileInfo) string {
	mode := info.Mode().Perm()
	modeOctal := fmt.Sprintf("%o", uint32(mode))
	modeStr := formatMode(mode, info.IsDir())
	fileType := "regular file"
	if info.IsDir() {
		fileType = "directory"
	}
	out := format
	out = strings.ReplaceAll(out, "%n", file)
	out = strings.ReplaceAll(out, "%N", "'"+file+"'")
	out = strings.ReplaceAll(out, "%s", fmt.Sprintf("%d", info.Size()))
	out = strings.ReplaceAll(out, "%F", fileType)
	out = strings.ReplaceAll(out, "%a", modeOctal)
	out = strings.ReplaceAll(out, "%A", modeStr)
	out = strings.ReplaceAll(out, "%u", "1000")
	out = strings.ReplaceAll(out, "%U", "user")
	out = strings.ReplaceAll(out, "%g", "1000")
	out = strings.ReplaceAll(out, "%G", "group")
	return out
}

func formatMode(mode os.FileMode, isDir bool) string {
	var b strings.Builder
	if isDir {
		b.WriteByte('d')
	} else {
		b.WriteByte('-')
	}
	bits := []struct {
		bit  os.FileMode
		char byte
	}{
		{0o400, 'r'}, {0o200, 'w'}, {0o100, 'x'},
		{0o040, 'r'}, {0o020, 'w'}, {0o010, 'x'},
		{0o004, 'r'}, {0o002, 'w'}, {0o001, 'x'},
	}
	for _, bp := range bits {
		if mode&bp.bit != 0 {
			b.WriteByte(bp.char)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
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
