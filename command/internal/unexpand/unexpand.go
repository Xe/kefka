package unexpand

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"golang.org/x/text/width"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("unexpand: nil ExecContext")
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
	set.SetProgram("unexpand")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: unexpand [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Convert blanks in each FILE to TABs, writing to standard output.\n")
		fmt.Fprint(stderr, "If no FILE is specified, standard input is read.\n\n")
		fmt.Fprint(stderr, "  -t N        Use N spaces per tab (default: 8)\n")
		fmt.Fprint(stderr, "  -t LIST     Use comma-separated list of tab stops\n")
		fmt.Fprint(stderr, "  -a          Convert all sequences of blanks (not just leading)\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	tabsSpec := set.StringLong("tabs", 't', "8", "use N spaces per tab")
	allBlanks := set.BoolLong("all", 'a', "convert all blanks, not just leading ones")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"unexpand"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "unexpand: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	tabStops, ok := parseTabStops(*tabsSpec)
	if !ok {
		fmt.Fprintf(stderr, "unexpand: invalid tab size: '%s'\n", *tabsSpec)
		return interp.ExitStatus(1)
	}

	convertAll := *allBlanks || set.IsSet("tabs")

	files := set.Args()

	var output strings.Builder
	var execErr error
	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		output.WriteString(processContent(content, tabStops, convertAll))
	} else {
		for _, file := range files {
			content, err := readFile(ec, file, stderr)
			if err != nil {
				execErr = err
				break
			}
			output.WriteString(processContent(content, tabStops, convertAll))
		}
	}

	io.WriteString(stdout, output.String())
	return execErr
}

func parseTabStops(spec string) ([]int, bool) {
	parts := strings.Split(spec, ",")
	stops := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 {
			return nil, false
		}
		stops = append(stops, n)
	}
	for i := 1; i < len(stops); i++ {
		if stops[i] <= stops[i-1] {
			return nil, false
		}
	}
	return stops, true
}

func getNextTabStop(column int, tabStops []int) int {
	if len(tabStops) == 1 {
		w := tabStops[0]
		return column + (w - (column % w))
	}
	for _, stop := range tabStops {
		if stop > column {
			return stop
		}
	}
	return -1
}

func runeWidth(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	}
	return 1
}

func tabAdvance(column int, tabStops []int) int {
	next := getNextTabStop(column, tabStops)
	if next > column {
		return next
	}
	if len(tabStops) >= 2 {
		last := tabStops[len(tabStops)-1]
		prev := tabStops[len(tabStops)-2]
		interval := last - prev
		steps := (column-last)/interval + 1
		return last + steps*interval
	}
	return column + 1
}

func unexpandLine(line string, tabStops []int, allBlanks bool) string {
	var result strings.Builder
	column := 0
	spaceRun := 0
	spaceRunStart := 0
	inLeading := true

	flushSpaces := func() {
		if spaceRun == 0 {
			return
		}
		endColumn := spaceRunStart + spaceRun
		if !allBlanks && !inLeading {
			for range spaceRun {
				result.WriteByte(' ')
			}
			spaceRun = 0
			return
		}
		// Per GNU and POSIX -a: only sequences of two or more blanks
		// immediately preceding a tab stop convert in non-leading runs.
		// Leading runs (any size) may still convert.
		if !inLeading && spaceRun < 2 {
			for range spaceRun {
				result.WriteByte(' ')
			}
			spaceRun = 0
			return
		}
		currentPos := spaceRunStart
		for currentPos < endColumn {
			nextStop := getNextTabStop(currentPos, tabStops)
			if nextStop > 0 && nextStop <= endColumn && nextStop > currentPos {
				result.WriteByte('\t')
				currentPos = nextStop
				continue
			}
			break
		}
		remaining := endColumn - currentPos
		for range remaining {
			result.WriteByte(' ')
		}
		spaceRun = 0
	}

	for _, r := range line {
		switch r {
		case ' ':
			if spaceRun == 0 {
				spaceRunStart = column
			}
			spaceRun++
			column++
		case '\t':
			flushSpaces()
			result.WriteByte('\t')
			column = tabAdvance(column, tabStops)
		case '\b':
			flushSpaces()
			result.WriteByte('\b')
			if column > 0 {
				column--
			}
			inLeading = false
		default:
			flushSpaces()
			result.WriteRune(r)
			column += runeWidth(r)
			inLeading = false
		}
	}
	flushSpaces()
	return result.String()
}

func processContent(content string, tabStops []int, allBlanks bool) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	hasTrailingNewline := strings.HasSuffix(content, "\n") && lines[len(lines)-1] == ""
	if hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(unexpandLine(line, tabStops, allBlanks))
	}
	if hasTrailingNewline {
		out.WriteByte('\n')
	}
	return out.String()
}

func readStdin(ec *command.ExecContext) (string, error) {
	if ec.Stdin == nil {
		return "", nil
	}
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
}

func readFile(ec *command.ExecContext, file string, stderr io.Writer) (string, error) {
	if file == "-" {
		return readStdin(ec)
	}
	if ec.FS == nil {
		fmt.Fprintf(stderr, "unexpand: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "unexpand: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "unexpand: %s: %v\n", file, err)
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
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
