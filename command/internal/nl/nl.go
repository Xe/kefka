package nl

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

type options struct {
	bodyStyle    string
	numberFormat string
	width        int
	separator    string
	startNumber  int
	increment    int
}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("nl: nil ExecContext")
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
	set.SetProgram("nl")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: nl [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Write each FILE to standard output, with line numbers added.\n")
		fmt.Fprint(stderr, "If no FILE is specified, standard input is read.\n\n")
		fmt.Fprint(stderr, "  -b STYLE     Body numbering style: a (all), t (non-empty), n (none)\n")
		fmt.Fprint(stderr, "  -n FORMAT    Number format: ln (left), rn (right), rz (right zeros)\n")
		fmt.Fprint(stderr, "  -w WIDTH     Number width (default: 6)\n")
		fmt.Fprint(stderr, "  -s SEP       Separator after number (default: TAB)\n")
		fmt.Fprint(stderr, "  -v START     Starting line number (default: 1)\n")
		fmt.Fprint(stderr, "  -i INCR      Line number increment (default: 1)\n")
		fmt.Fprint(stderr, "      --help   display this help and exit\n")
	}
	set.SetUsage(usage)

	bodyStyle := set.String('b', "t", "body numbering style")
	numberFormat := set.String('n', "rn", "number format")
	widthSpec := set.String('w', "6", "number width")
	separator := set.String('s', "\t", "separator after number")
	startSpec := set.String('v', "1", "starting line number")
	incrSpec := set.String('i', "1", "line number increment")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"nl"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "nl: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	switch *bodyStyle {
	case "a", "t", "n":
	default:
		fmt.Fprintf(stderr, "nl: invalid body numbering style: '%s'\n", *bodyStyle)
		return interp.ExitStatus(1)
	}

	switch *numberFormat {
	case "ln", "rn", "rz":
	default:
		fmt.Fprintf(stderr, "nl: invalid line numbering format: '%s'\n", *numberFormat)
		return interp.ExitStatus(1)
	}

	width, err := strconv.Atoi(*widthSpec)
	if err != nil || width < 1 {
		fmt.Fprintf(stderr, "nl: invalid line number field width: '%s'\n", *widthSpec)
		return interp.ExitStatus(1)
	}

	startNumber, err := strconv.Atoi(*startSpec)
	if err != nil {
		fmt.Fprintf(stderr, "nl: invalid starting line number: '%s'\n", *startSpec)
		return interp.ExitStatus(1)
	}

	increment, err := strconv.Atoi(*incrSpec)
	if err != nil {
		fmt.Fprintf(stderr, "nl: invalid line number increment: '%s'\n", *incrSpec)
		return interp.ExitStatus(1)
	}

	opts := options{
		bodyStyle:    *bodyStyle,
		numberFormat: *numberFormat,
		width:        width,
		separator:    *separator,
		startNumber:  startNumber,
		increment:    increment,
	}

	files := set.Args()
	lineNumber := opts.startNumber
	var output strings.Builder

	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		out, _ := processContent(content, opts, lineNumber)
		output.WriteString(out)
	} else {
		for _, file := range files {
			content, err := readFile(ec, file, stderr)
			if err != nil {
				io.WriteString(stdout, output.String())
				return err
			}
			out, next := processContent(content, opts, lineNumber)
			output.WriteString(out)
			lineNumber = next
		}
	}

	io.WriteString(stdout, output.String())
	return nil
}

func processContent(content string, opts options, currentNumber int) (string, int) {
	if content == "" {
		return "", currentNumber
	}

	lines := strings.Split(content, "\n")
	hasTrailingNewline := strings.HasSuffix(content, "\n") && lines[len(lines)-1] == ""
	if hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	var out strings.Builder
	lineNumber := currentNumber
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		if shouldNumber(line, opts.bodyStyle) {
			out.WriteString(formatLineNumber(lineNumber, opts.numberFormat, opts.width))
			out.WriteString(opts.separator)
			out.WriteString(line)
			lineNumber += opts.increment
		} else {
			out.WriteString(strings.Repeat(" ", opts.width))
			out.WriteString(opts.separator)
			out.WriteString(line)
		}
	}
	if hasTrailingNewline {
		out.WriteByte('\n')
	}
	return out.String(), lineNumber
}

func shouldNumber(line, style string) bool {
	switch style {
	case "a":
		return true
	case "t":
		return strings.TrimSpace(line) != ""
	case "n":
		return false
	}
	return false
}

func formatLineNumber(num int, format string, width int) string {
	s := strconv.Itoa(num)
	switch format {
	case "ln":
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	case "rn":
		if len(s) >= width {
			return s
		}
		return strings.Repeat(" ", width-len(s)) + s
	case "rz":
		if len(s) >= width {
			return s
		}
		return strings.Repeat("0", width-len(s)) + s
	}
	return s
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
		fmt.Fprintf(stderr, "nl: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "nl: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "nl: %s: %v\n", file, err)
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
