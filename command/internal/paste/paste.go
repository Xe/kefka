package paste

import (
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

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("paste: nil ExecContext")
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
	set.SetProgram("paste")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: paste [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Write lines consisting of the sequentially corresponding lines from\n")
		fmt.Fprint(stderr, "each FILE, separated by TABs, to standard output.\n")
		fmt.Fprint(stderr, "With no FILE, or when FILE is -, read standard input.\n\n")
		fmt.Fprint(stderr, "  -d, --delimiters=LIST   reuse characters from LIST instead of TABs\n")
		fmt.Fprint(stderr, "  -s, --serial            paste one file at a time instead of in parallel\n")
		fmt.Fprint(stderr, "      --help              display this help and exit\n")
	}
	set.SetUsage(usage)

	delimiters := set.StringLong("delimiters", 'd', "\t", "reuse characters from LIST instead of TABs")
	serial := set.BoolLong("serial", 's', "paste one file at a time instead of in parallel")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"paste"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "paste: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		fmt.Fprint(stderr, "usage: paste [-s] [-d delimiters] file ...\n")
		return interp.ExitStatus(1)
	}

	stdinCount := 0
	for _, f := range files {
		if f == "-" {
			stdinCount++
		}
	}

	var stdinLines []string
	if stdinCount > 0 {
		var err error
		stdinLines, err = readStdinLines(ec)
		if err != nil {
			return err
		}
	}

	fileContents := make([][]string, 0, len(files))
	stdinIndex := 0
	for _, file := range files {
		if file == "-" {
			var slice []string
			for i := stdinIndex; i < len(stdinLines); i += stdinCount {
				slice = append(slice, stdinLines[i])
			}
			fileContents = append(fileContents, slice)
			stdinIndex++
			continue
		}
		lines, err := readFileLines(ec, file, stderr)
		if err != nil {
			return err
		}
		fileContents = append(fileContents, lines)
	}

	delim := *delimiters
	var output strings.Builder

	if *serial {
		for _, lines := range fileContents {
			output.WriteString(joinWithDelimiters(lines, delim))
			output.WriteByte('\n')
		}
	} else {
		maxLines := 0
		for _, lines := range fileContents {
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			parts := make([]string, len(fileContents))
			for i, lines := range fileContents {
				if lineIdx < len(lines) {
					parts[i] = lines[lineIdx]
				}
			}
			output.WriteString(joinWithDelimiters(parts, delim))
			output.WriteByte('\n')
		}
	}

	io.WriteString(stdout, output.String())
	return nil
}

func joinWithDelimiters(parts []string, delimiters string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	delimRunes := []rune(delimiters)
	if len(delimRunes) == 0 {
		return strings.Join(parts, "")
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		idx := (i - 1) % len(delimRunes)
		b.WriteRune(delimRunes[idx])
		b.WriteString(parts[i])
	}
	return b.String()
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func readStdinLines(ec *command.ExecContext) ([]string, error) {
	if ec.Stdin == nil {
		return nil, nil
	}
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		return nil, interp.ExitStatus(1)
	}
	return splitLines(string(data)), nil
}

func readFileLines(ec *command.ExecContext, file string, stderr io.Writer) ([]string, error) {
	if ec.FS == nil {
		fmt.Fprintf(stderr, "paste: %s: No such file or directory\n", file)
		return nil, interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "paste: %s: No such file or directory\n", file)
		return nil, interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "paste: %s: %v\n", file, err)
		return nil, interp.ExitStatus(1)
	}
	return splitLines(string(data)), nil
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
