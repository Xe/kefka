package column

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("column: nil ExecContext")
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
	set.SetProgram("column")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: column [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Format input into multiple columns. By default, fills rows first. Use -t to create a table based on whitespace-delimited input.\n\n")
		fmt.Fprint(stderr, "  -t, --table       create a table (determine columns from input)\n")
		fmt.Fprint(stderr, "  -s SEP            input field delimiter (default: whitespace)\n")
		fmt.Fprint(stderr, "  -o SEP            output field delimiter (default: two spaces)\n")
		fmt.Fprint(stderr, "  -c WIDTH          output width for fill mode (default: 80)\n")
		fmt.Fprint(stderr, "  -n                don't merge multiple adjacent delimiters\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	table := set.BoolLong("table", 't', "create a table (determine columns from input)")
	sep := set.String('s', "", "input field delimiter (default: whitespace)")
	outSep := set.String('o', "  ", "output field delimiter (default: two spaces)")
	width := set.Int('c', 80, "output width for fill mode (default: 80)")
	noMerge := set.Bool('n', "don't merge multiple adjacent delimiters")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"column"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "column: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	files := set.Args()

	content, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	output := formatColumns(content, *table, *sep, *outSep, *width, *noMerge)
	io.WriteString(stdout, output)
	return nil
}

func formatColumns(content string, table bool, sep, outSep string, width int, noMerge bool) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	var output string
	if table {
		rows := make([][]string, 0, len(nonEmpty))
		for _, line := range nonEmpty {
			rows = append(rows, splitFields(line, sep, noMerge))
		}
		output = formatTable(rows, outSep)
	} else {
		var items []string
		for _, line := range nonEmpty {
			items = append(items, splitFields(line, sep, noMerge)...)
		}
		output = formatFill(items, width, outSep)
	}

	if len(output) > 0 {
		output += "\n"
	}
	return output
}

var (
	spaceTabSingle = regexp.MustCompile(`[ \t]`)
	spaceTabRun    = regexp.MustCompile(`[ \t]+`)
)

func splitFields(line, sep string, noMerge bool) []string {
	if sep != "" {
		parts := strings.Split(line, sep)
		if noMerge {
			return parts
		}
		return removeEmpty(parts)
	}
	if noMerge {
		return spaceTabSingle.Split(line, -1)
	}
	return removeEmpty(spaceTabRun.Split(line, -1))
}

func removeEmpty(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func formatTable(rows [][]string, outSep string) string {
	if len(rows) == 0 {
		return ""
	}
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			if i == len(row)-1 {
				cells[i] = cell
			} else {
				cells[i] = padEnd(cell, widths[i])
			}
		}
		lines = append(lines, strings.Join(cells, outSep))
	}
	return strings.Join(lines, "\n")
}

func formatFill(items []string, width int, outSep string) string {
	if len(items) == 0 {
		return ""
	}
	maxItemWidth := 0
	for _, item := range items {
		maxItemWidth = max(maxItemWidth, len(item))
	}
	sepWidth := len(outSep)
	colWidth := maxItemWidth + sepWidth
	numColumns := 1
	if colWidth > 0 {
		numColumns = max(1, (width+sepWidth)/colWidth)
	}
	numRows := (len(items) + numColumns - 1) / numColumns

	lines := make([]string, 0, numRows)
	for row := range numRows {
		var cells []string
		for col := range numColumns {
			index := col*numRows + row
			if index >= len(items) {
				continue
			}
			isLastInRow := col == numColumns-1 || (col+1)*numRows+row >= len(items)
			if isLastInRow {
				cells = append(cells, items[index])
			} else {
				cells = append(cells, padEnd(items[index], maxItemWidth))
			}
		}
		lines = append(lines, strings.Join(cells, outSep))
	}
	return strings.Join(lines, "\n")
}

func padEnd(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func readInput(ec *command.ExecContext, files []string, stderr io.Writer) (string, error) {
	if len(files) == 0 {
		if ec.Stdin == nil {
			return "", nil
		}
		data, err := io.ReadAll(ec.Stdin)
		if err != nil {
			return "", interp.ExitStatus(1)
		}
		return string(data), nil
	}
	var buf strings.Builder
	for _, file := range files {
		if file == "-" {
			if ec.Stdin == nil {
				continue
			}
			data, err := io.ReadAll(ec.Stdin)
			if err != nil {
				return "", interp.ExitStatus(1)
			}
			buf.Write(data)
			continue
		}
		if ec.FS == nil {
			fmt.Fprintf(stderr, "column: %s: No such file or directory\n", file)
			return "", interp.ExitStatus(1)
		}
		full := resolvePath(ec, file)
		f, err := ec.FS.Open(full)
		if err != nil {
			fmt.Fprintf(stderr, "column: %s: No such file or directory\n", file)
			return "", interp.ExitStatus(1)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "column: %s: %v\n", file, err)
			return "", interp.ExitStatus(1)
		}
		buf.Write(data)
	}
	return buf.String(), nil
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
