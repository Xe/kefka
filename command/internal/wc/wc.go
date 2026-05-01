package wc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pborman/getopt/v2"
	"golang.org/x/text/width"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type stats struct {
	lines   int
	words   int
	chars   int
	bytes   int
	maxLine int
}

type fileResult struct {
	filename string
	stats    stats
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("wc: nil ExecContext")
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
	set.SetProgram("wc")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: wc [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Print newline, word, and byte counts for each FILE.\n\n")
		fmt.Fprint(stderr, "  -c, --bytes              print the byte counts\n")
		fmt.Fprint(stderr, "  -m, --chars              print the character counts\n")
		fmt.Fprint(stderr, "  -l, --lines              print the newline counts\n")
		fmt.Fprint(stderr, "  -w, --words              print the word counts\n")
		fmt.Fprint(stderr, "  -L, --max-line-length    print the maximum display width\n")
		fmt.Fprint(stderr, "      --help               display this help and exit\n")
	}
	set.SetUsage(usage)

	linesFlag := set.BoolLong("lines", 'l', "print the newline counts")
	wordsFlag := set.BoolLong("words", 'w', "print the word counts")
	bytesFlag := set.BoolLong("bytes", 'c', "print the byte counts")
	charsFlag := set.BoolLong("chars", 'm', "print the character counts")
	maxLineFlag := set.BoolLong("max-line-length", 'L', "print the maximum display width")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"wc"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "wc: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	showLines := *linesFlag
	showWords := *wordsFlag
	showBytes := *bytesFlag
	showChars := *charsFlag
	showMaxLine := *maxLineFlag

	if !showLines && !showWords && !showBytes && !showChars && !showMaxLine {
		showLines = true
		showWords = true
		showBytes = true
	}

	files := set.Args()

	if len(files) == 0 {
		data, err := readStdin(ec)
		if err != nil {
			fmt.Fprintf(stderr, "wc: %s\n", err)
			return interp.ExitStatus(1)
		}
		s := countStats(data)
		io.WriteString(stdout, formatStats(s, showLines, showWords, showChars, showBytes, showMaxLine, "", 0)+"\n")
		return nil
	}

	results := make([]fileResult, 0, len(files))
	var total stats
	exitCode := 0

	for _, file := range files {
		data, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "wc: %s: %s\n", file, err)
			exitCode = 1
			continue
		}
		s := countStats(data)
		total.lines += s.lines
		total.words += s.words
		total.chars += s.chars
		total.bytes += s.bytes
		if s.maxLine > total.maxLine {
			total.maxLine = s.maxLine
		}
		results = append(results, fileResult{filename: file, stats: s})
	}

	maxLines := 0
	maxWords := 0
	maxChars := 0
	maxBytes := 0
	maxL := 0
	if len(files) > 1 {
		maxLines = total.lines
		maxWords = total.words
		maxChars = total.chars
		maxBytes = total.bytes
		maxL = total.maxLine
	} else {
		for _, r := range results {
			if r.stats.lines > maxLines {
				maxLines = r.stats.lines
			}
			if r.stats.words > maxWords {
				maxWords = r.stats.words
			}
			if r.stats.chars > maxChars {
				maxChars = r.stats.chars
			}
			if r.stats.bytes > maxBytes {
				maxBytes = r.stats.bytes
			}
			if r.stats.maxLine > maxL {
				maxL = r.stats.maxLine
			}
		}
	}

	maxWidth := 0
	if len(files) > 1 {
		maxWidth = 3
	}
	if showLines {
		if w := len(strconv.Itoa(maxLines)); w > maxWidth {
			maxWidth = w
		}
	}
	if showWords {
		if w := len(strconv.Itoa(maxWords)); w > maxWidth {
			maxWidth = w
		}
	}
	if showChars {
		if w := len(strconv.Itoa(maxChars)); w > maxWidth {
			maxWidth = w
		}
	}
	if showBytes {
		if w := len(strconv.Itoa(maxBytes)); w > maxWidth {
			maxWidth = w
		}
	}
	if showMaxLine {
		if w := len(strconv.Itoa(maxL)); w > maxWidth {
			maxWidth = w
		}
	}

	var out strings.Builder
	for _, r := range results {
		out.WriteString(formatStats(r.stats, showLines, showWords, showChars, showBytes, showMaxLine, r.filename, maxWidth))
		out.WriteByte('\n')
	}

	if len(files) > 1 {
		out.WriteString(formatStats(total, showLines, showWords, showChars, showBytes, showMaxLine, "total", maxWidth))
		out.WriteByte('\n')
	}

	io.WriteString(stdout, out.String())

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func readStdin(ec *command.ExecContext) ([]byte, error) {
	if ec.Stdin == nil {
		return nil, nil
	}
	return io.ReadAll(ec.Stdin)
}

func readFile(ec *command.ExecContext, name string) ([]byte, error) {
	if name == "-" {
		return readStdin(ec)
	}
	if ec.FS == nil {
		return nil, errors.New("no filesystem")
	}
	f, err := ec.FS.Open(resolvePath(ec, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func countStats(content []byte) stats {
	var s stats
	s.bytes = len(content)
	s.chars = utf8.RuneCount(content)

	inWord := false
	col := 0
	lineMax := 0
	for _, r := range string(content) {
		switch r {
		case '\n':
			s.lines++
			if col > lineMax {
				lineMax = col
			}
			if lineMax > s.maxLine {
				s.maxLine = lineMax
			}
			lineMax = 0
			col = 0
		case '\r':
			if col > lineMax {
				lineMax = col
			}
			col = 0
		case '\t':
			col = (col/8 + 1) * 8
		case '\b':
			if col > 0 {
				col--
			}
		default:
			col += runeWidth(r)
		}

		if unicode.IsSpace(r) {
			if inWord {
				s.words++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		s.words++
	}
	if col > lineMax {
		lineMax = col
	}
	if lineMax > s.maxLine {
		s.maxLine = lineMax
	}
	return s
}

// runeWidth returns the display width of r in fixed-width cells, mirroring the
// behaviour wc uses when computing -L: control characters contribute zero,
// East Asian Wide / Fullwidth runes contribute two, and everything else
// contributes one.
func runeWidth(r rune) int {
	if r < 0x20 || r == 0x7f {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	}
	return 1
}

func formatStats(s stats, showLines, showWords, showChars, showBytes, showMaxLine bool, filename string, minWidth int) string {
	var values []string
	if showLines {
		values = append(values, padLeft(strconv.Itoa(s.lines), minWidth))
	}
	if showWords {
		values = append(values, padLeft(strconv.Itoa(s.words), minWidth))
	}
	if showChars {
		values = append(values, padLeft(strconv.Itoa(s.chars), minWidth))
	}
	if showBytes {
		values = append(values, padLeft(strconv.Itoa(s.bytes), minWidth))
	}
	if showMaxLine {
		values = append(values, padLeft(strconv.Itoa(s.maxLine), minWidth))
	}
	result := strings.Join(values, " ")
	if filename != "" {
		result += " " + filename
	}
	return result
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
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
