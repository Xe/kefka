package tail

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

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("tail: nil ExecContext")
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
	set.SetProgram("tail")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: tail [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Print the last 10 lines of each FILE to standard output.\n")
		fmt.Fprint(stderr, "With more than one FILE, precede each with a header giving the file name.\n")
		fmt.Fprint(stderr, "With no FILE, or when FILE is -, read standard input.\n\n")
		fmt.Fprint(stderr, "  -c, --bytes=NUM    print the last NUM bytes\n")
		fmt.Fprint(stderr, "  -n, --lines=NUM    print the last NUM lines (default 10)\n")
		fmt.Fprint(stderr, "  -n +NUM            print starting from line NUM\n")
		fmt.Fprint(stderr, "  -q, --quiet        never print headers giving file names\n")
		fmt.Fprint(stderr, "  -v, --verbose      always print headers giving file names\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n")
	}
	set.SetUsage(usage)

	bytesSpec := set.StringLong("bytes", 'c', "", "print the last NUM bytes")
	linesSpec := set.StringLong("lines", 'n', "", "print the last NUM lines (default 10)")
	quiet := set.BoolLong("quiet", 'q', "never print headers giving file names")
	silent := set.BoolLong("silent", 0, "alias for --quiet")
	verbose := set.BoolLong("verbose", 'v', "always print headers giving file names")
	help := set.BoolLong("help", 0, "display this help and exit")

	preArgs := preprocessShortNum(args)

	if err := set.Getopt(append([]string{"tail"}, preArgs...), nil); err != nil {
		fmt.Fprintf(stderr, "tail: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	lines := 10
	bytes := 0
	bytesSet := false
	fromLine := false

	if *bytesSpec != "" {
		n, err := strconv.Atoi(*bytesSpec)
		if err != nil || n < 0 {
			fmt.Fprint(stderr, "tail: invalid number of bytes\n")
			return interp.ExitStatus(1)
		}
		bytes = n
		bytesSet = true
	}
	if *linesSpec != "" {
		spec := *linesSpec
		if strings.HasPrefix(spec, "+") {
			fromLine = true
			spec = spec[1:]
		}
		n, err := strconv.Atoi(spec)
		if err != nil || n < 0 {
			fmt.Fprint(stderr, "tail: invalid number of lines\n")
			return interp.ExitStatus(1)
		}
		lines = n
	}

	isQuiet := *quiet || *silent
	files := set.Args()

	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		io.WriteString(stdout, getTail(content, lines, bytes, bytesSet, fromLine))
		return nil
	}

	showHeaders := *verbose || (!isQuiet && len(files) > 1)
	var output strings.Builder
	exitCode := 0
	filesProcessed := 0

	for _, file := range files {
		content, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "tail: %s: No such file or directory\n", file)
			exitCode = 1
			continue
		}
		if showHeaders {
			if filesProcessed > 0 {
				output.WriteByte('\n')
			}
			fmt.Fprintf(&output, "==> %s <==\n", file)
		}
		output.WriteString(getTail(content, lines, bytes, bytesSet, fromLine))
		filesProcessed++
	}

	io.WriteString(stdout, output.String())

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// preprocessShortNum rewrites the GNU coreutils -NUM shorthand (e.g. -5)
// into "-n NUM" so that getopt can parse it. Stops at "--" and skips the
// value of any preceding -c/-n/--bytes/--lines so that "-c -5" is left
// alone for getopt to flag as invalid.
func preprocessShortNum(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false
	seenDoubleDash := false
	for _, a := range args {
		if seenDoubleDash {
			out = append(out, a)
			continue
		}
		if skipNext {
			skipNext = false
			out = append(out, a)
			continue
		}
		if a == "--" {
			seenDoubleDash = true
			out = append(out, a)
			continue
		}
		if a == "-c" || a == "-n" || a == "--bytes" || a == "--lines" {
			skipNext = true
			out = append(out, a)
			continue
		}
		if len(a) >= 2 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			allDigits := true
			for _, c := range a[1:] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				out = append(out, "-n", a[1:])
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func getTail(content string, lines int, bytes int, bytesSet bool, fromLine bool) string {
	if bytesSet {
		if bytes >= len(content) {
			return content
		}
		return content[len(content)-bytes:]
	}
	n := len(content)
	if n == 0 {
		return ""
	}
	if fromLine {
		pos := 0
		lineCount := 1
		for pos < n && lineCount < lines {
			idx := strings.IndexByte(content[pos:], '\n')
			if idx == -1 {
				break
			}
			lineCount++
			pos += idx + 1
		}
		result := content[pos:]
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		return result
	}
	if lines == 0 {
		return ""
	}
	pos := n - 1
	if content[pos] == '\n' {
		pos--
	}
	lineCount := 0
	for pos >= 0 && lineCount < lines {
		if content[pos] == '\n' {
			lineCount++
			if lineCount == lines {
				pos++
				break
			}
		}
		pos--
	}
	if pos < 0 {
		pos = 0
	}
	result := content[pos:]
	if content[n-1] != '\n' {
		result += "\n"
	}
	return result
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

func readFile(ec *command.ExecContext, file string) (string, error) {
	if file == "-" {
		return readStdin(ec)
	}
	if ec.FS == nil {
		return "", errors.New("no filesystem")
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
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
