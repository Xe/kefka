package fold

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("fold: nil ExecContext")
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
	set.SetProgram("fold")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: fold [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Wrap input lines in each FILE, writing to standard output.\n")
		fmt.Fprint(stderr, "If no FILE is specified, standard input is read.\n\n")
		fmt.Fprint(stderr, "  -w WIDTH    Use WIDTH columns instead of 80\n")
		fmt.Fprint(stderr, "  -s          Break at spaces\n")
		fmt.Fprint(stderr, "  -b          Count bytes rather than columns\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	widthSpec := set.StringLong("width", 'w', "80", "use WIDTH columns instead of 80")
	breakAtSpaces := set.BoolLong("spaces", 's', "break at spaces")
	countBytes := set.BoolLong("bytes", 'b', "count bytes rather than columns")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"fold"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "fold: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	width, err := strconv.Atoi(*widthSpec)
	if err != nil || width < 1 {
		fmt.Fprintf(stderr, "fold: invalid number of columns: '%s'\n", *widthSpec)
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
		output.WriteString(processContent(content, width, *breakAtSpaces, *countBytes))
	} else {
		for _, file := range files {
			content, err := readFile(ec, file, stderr)
			if err != nil {
				io.WriteString(stdout, output.String())
				return err
			}
			output.WriteString(processContent(content, width, *breakAtSpaces, *countBytes))
		}
	}

	io.WriteString(stdout, output.String())
	return execErr
}

func processContent(content string, width int, breakAtSpaces, countBytes bool) string {
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
		out.WriteString(foldLine(line, width, breakAtSpaces, countBytes))
	}
	if hasTrailingNewline {
		out.WriteByte('\n')
	}
	return out.String()
}

func foldLine(line string, width int, breakAtSpaces, countBytes bool) string {
	if line == "" {
		return line
	}

	var (
		result        []string
		currentLine   []byte
		currentColumn int
		lastSpace     = -1
		lastSpaceCol  int
	)

	emit := func(charWidth int, isSpace bool, ch []byte) {
		if currentColumn+charWidth > width && len(currentLine) > 0 {
			if breakAtSpaces && lastSpace >= 0 {
				result = append(result, string(currentLine[:lastSpace+1]))
				rest := append([]byte(nil), currentLine[lastSpace+1:]...)
				currentLine = append(rest, ch...)
				currentColumn = currentColumn - lastSpaceCol - 1 + charWidth
				lastSpace = -1
				lastSpaceCol = 0
				return
			}
			result = append(result, string(currentLine))
			currentLine = append(currentLine[:0:0], ch...)
			currentColumn = charWidth
			lastSpace = -1
			lastSpaceCol = 0
			return
		}
		preLen := len(currentLine)
		currentLine = append(currentLine, ch...)
		currentColumn += charWidth
		if isSpace {
			lastSpace = preLen + len(ch) - 1
			lastSpaceCol = currentColumn - charWidth
		}
	}

	if countBytes {
		for i := 0; i < len(line); i++ {
			b := line[i]
			emit(1, b == ' ' || b == '\t', []byte{b})
		}
	} else {
		var buf [utf8.UTFMax]byte
		for _, r := range line {
			charWidth := 1
			switch r {
			case '\t':
				charWidth = 8 - (currentColumn % 8)
			case '\b':
				charWidth = -1
			}
			n := utf8.EncodeRune(buf[:], r)
			emit(charWidth, r == ' ' || r == '\t', buf[:n])
		}
	}

	if len(currentLine) > 0 {
		result = append(result, string(currentLine))
	}
	return strings.Join(result, "\n")
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
		fmt.Fprintf(stderr, "fold: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "fold: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "fold: %s: %v\n", file, err)
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
