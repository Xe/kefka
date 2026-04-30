package expand

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("expand: nil ExecContext")
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
	set.SetProgram("expand")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: expand [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Convert TABs in each FILE to spaces, writing to standard output.\n")
		fmt.Fprint(stderr, "If no FILE is specified, standard input is read.\n\n")
		fmt.Fprint(stderr, "  -t N        Use N spaces per tab (default: 8)\n")
		fmt.Fprint(stderr, "  -t LIST     Use comma-separated list of tab stops\n")
		fmt.Fprint(stderr, "  -i          Only convert leading tabs on each line\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	tabsSpec := set.StringLong("tabs", 't', "8", "use N spaces per tab")
	leadingOnly := set.BoolLong("initial", 'i', "only convert leading tabs on each line")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"expand"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "expand: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	tabStops, ok := parseTabStops(*tabsSpec)
	if !ok {
		fmt.Fprintf(stderr, "expand: invalid tab size: '%s'\n", *tabsSpec)
		return interp.ExitStatus(1)
	}

	files := set.Args()

	var output strings.Builder
	var execErr error
	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		output.WriteString(processContent(content, tabStops, *leadingOnly))
	} else {
		for _, file := range files {
			content, err := readFile(ec, file, stderr)
			if err != nil {
				execErr = err
				break
			}
			output.WriteString(processContent(content, tabStops, *leadingOnly))
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

func getTabWidth(column int, tabStops []int) int {
	if len(tabStops) == 1 {
		w := tabStops[0]
		return w - (column % w)
	}
	for _, stop := range tabStops {
		if stop > column {
			return stop - column
		}
	}
	if len(tabStops) >= 2 {
		last := tabStops[len(tabStops)-1]
		prev := tabStops[len(tabStops)-2]
		interval := last - prev
		steps := (column-last)/interval + 1
		next := last + steps*interval
		return next - column
	}
	return 1
}

func expandLine(line string, tabStops []int, leadingOnly bool) string {
	var result strings.Builder
	column := 0
	inLeading := true
	for _, r := range line {
		if r == '\t' {
			if leadingOnly && !inLeading {
				result.WriteByte('\t')
				column++
				continue
			}
			n := getTabWidth(column, tabStops)
			for range n {
				result.WriteByte(' ')
			}
			column += n
			continue
		}
		if r != ' ' {
			inLeading = false
		}
		result.WriteRune(r)
		column++
	}
	return result.String()
}

func processContent(content string, tabStops []int, leadingOnly bool) string {
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
		out.WriteString(expandLine(line, tabStops, leadingOnly))
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
		fmt.Fprintf(stderr, "expand: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "expand: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "expand: %s: %v\n", file, err)
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
