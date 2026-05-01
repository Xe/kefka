package nl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type sectionStyle struct {
	kind string
	re   *regexp.Regexp
}

type options struct {
	bodyStyle    sectionStyle
	headerStyle  sectionStyle
	footerStyle  sectionStyle
	numberFormat string
	width        int
	separator    string
	startNumber  int
	increment    int
	joinBlanks   int
	noRenumber   bool
	headerMark   string
	bodyMark     string
	footerMark   string
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
		fmt.Fprint(stderr, "  -b STYLE     Body numbering style: a (all), t (non-empty), n (none), pBRE\n")
		fmt.Fprint(stderr, "  -d CC        Use CC for logical page delimiters (default \\:)\n")
		fmt.Fprint(stderr, "  -f STYLE     Footer numbering style (default n)\n")
		fmt.Fprint(stderr, "  -h STYLE     Header numbering style (default n)\n")
		fmt.Fprint(stderr, "  -i INCR      Line number increment (default: 1)\n")
		fmt.Fprint(stderr, "  -l NUMBER    Group of NUMBER empty lines counted as one (default 1)\n")
		fmt.Fprint(stderr, "  -n FORMAT    Number format: ln (left), rn (right), rz (right zeros)\n")
		fmt.Fprint(stderr, "  -p           Do not reset line numbers at logical page delimiters\n")
		fmt.Fprint(stderr, "  -s SEP       Separator after number (default: TAB)\n")
		fmt.Fprint(stderr, "  -v START     Starting line number (default: 1)\n")
		fmt.Fprint(stderr, "  -w WIDTH     Number width (default: 6)\n")
		fmt.Fprint(stderr, "      --help   display this help and exit\n")
	}
	set.SetUsage(usage)

	bodyStyle := set.String('b', "t", "body numbering style")
	delimSpec := set.String('d', `\:`, "page delimiter pair")
	footerStyle := set.String('f', "n", "footer numbering style")
	headerStyle := set.String('h', "n", "header numbering style")
	incrSpec := set.String('i', "1", "line number increment")
	joinSpec := set.String('l', "1", "blank line group size")
	numberFormat := set.String('n', "rn", "number format")
	noRenumber := set.Bool('p', "do not reset line numbers at page delimiters")
	separator := set.String('s', "\t", "separator after number")
	startSpec := set.String('v', "1", "starting line number")
	widthSpec := set.String('w', "6", "number width")
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

	body, err := parseStyle(*bodyStyle)
	if err != nil {
		fmt.Fprintf(stderr, "nl: invalid body numbering style: '%s'\n", *bodyStyle)
		return interp.ExitStatus(1)
	}
	header, err := parseStyle(*headerStyle)
	if err != nil {
		fmt.Fprintf(stderr, "nl: invalid header numbering style: '%s'\n", *headerStyle)
		return interp.ExitStatus(1)
	}
	footer, err := parseStyle(*footerStyle)
	if err != nil {
		fmt.Fprintf(stderr, "nl: invalid footer numbering style: '%s'\n", *footerStyle)
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

	join, err := strconv.Atoi(*joinSpec)
	if err != nil || join < 1 {
		fmt.Fprintf(stderr, "nl: invalid line group size: '%s'\n", *joinSpec)
		return interp.ExitStatus(1)
	}

	d1, d2 := parseDelim(*delimSpec)
	headerMark := strings.Repeat(d1+d2, 3)
	bodyMark := strings.Repeat(d1+d2, 2)
	footerMark := d1 + d2

	opts := options{
		bodyStyle:    body,
		headerStyle:  header,
		footerStyle:  footer,
		numberFormat: *numberFormat,
		width:        width,
		separator:    *separator,
		startNumber:  startNumber,
		increment:    increment,
		joinBlanks:   join,
		noRenumber:   *noRenumber,
		headerMark:   headerMark,
		bodyMark:     bodyMark,
		footerMark:   footerMark,
	}

	files := set.Args()
	state := &numberState{lineNumber: opts.startNumber, section: 'b'}
	var output strings.Builder

	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		output.WriteString(processContent(content, opts, state))
	} else {
		for _, file := range files {
			content, err := readFile(ec, file, stderr)
			if err != nil {
				io.WriteString(stdout, output.String())
				return err
			}
			output.WriteString(processContent(content, opts, state))
		}
	}

	io.WriteString(stdout, output.String())
	return nil
}

type numberState struct {
	lineNumber int
	section    byte
	blankRun   int
}

// parseStyle parses a section numbering style. The forms are:
//   - "a" (all), "t" (non-empty), "n" (none)
//   - "pSTRING" — number lines matching STRING. POSIX/GNU specify a
//     basic regular expression here; we accept Go's RE2 syntax instead
//     of implementing a separate BRE engine. Most simple anchors and
//     character classes used in practice are compatible.
func parseStyle(s string) (sectionStyle, error) {
	switch s {
	case "a", "t", "n":
		return sectionStyle{kind: s}, nil
	}
	if strings.HasPrefix(s, "p") && len(s) > 1 {
		re, err := regexp.Compile(s[1:])
		if err != nil {
			return sectionStyle{}, err
		}
		return sectionStyle{kind: "p", re: re}, nil
	}
	return sectionStyle{}, fmt.Errorf("invalid style %q", s)
}

func parseDelim(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	runes := []rune(s)
	if len(runes) == 1 {
		return string(runes[0]), ":"
	}
	return string(runes[0]), string(runes[1])
}

func processContent(content string, opts options, st *numberState) string {
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

		if marker, ok := matchMarker(line, opts); ok {
			st.section = marker
			st.blankRun = 0
			if !opts.noRenumber {
				st.lineNumber = opts.startNumber
			}
			continue
		}

		style := opts.bodyStyle
		switch st.section {
		case 'h':
			style = opts.headerStyle
		case 'f':
			style = opts.footerStyle
		}

		numbered := shouldNumber(line, style, opts.joinBlanks, st)
		if numbered {
			out.WriteString(formatLineNumber(st.lineNumber, opts.numberFormat, opts.width))
			out.WriteString(opts.separator)
			out.WriteString(line)
			st.lineNumber += opts.increment
		} else {
			out.WriteString(strings.Repeat(" ", opts.width+len(opts.separator)))
			out.WriteString(line)
		}
	}
	if hasTrailingNewline {
		out.WriteByte('\n')
	}
	return out.String()
}

func matchMarker(line string, opts options) (byte, bool) {
	if opts.footerMark == "" {
		return 0, false
	}
	if line == opts.headerMark {
		return 'h', true
	}
	if line == opts.bodyMark {
		return 'b', true
	}
	if line == opts.footerMark {
		return 'f', true
	}
	return 0, false
}

func shouldNumber(line string, style sectionStyle, joinBlanks int, st *numberState) bool {
	isBlank := line == ""
	if !isBlank {
		st.blankRun = 0
	}
	switch style.kind {
	case "a":
		if !isBlank {
			return true
		}
		st.blankRun++
		if st.blankRun >= joinBlanks {
			st.blankRun = 0
			return true
		}
		return false
	case "t":
		return strings.TrimSpace(line) != ""
	case "n":
		return false
	case "p":
		if style.re == nil {
			return false
		}
		return style.re.MatchString(line)
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
