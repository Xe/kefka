package head

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
		return errors.New("head: nil ExecContext")
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
	set.SetProgram("head")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: head [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Print the first 10 lines of each FILE to standard output.\n")
		fmt.Fprint(stderr, "With more than one FILE, precede each with a header giving the file name.\n")
		fmt.Fprint(stderr, "With no FILE, or when FILE is -, read standard input.\n\n")
		fmt.Fprint(stderr, "  -c, --bytes=[-]NUM    print the first NUM bytes of each file;\n")
		fmt.Fprint(stderr, "                          with the leading '-', print all but the last\n")
		fmt.Fprint(stderr, "                          NUM bytes of each file\n")
		fmt.Fprint(stderr, "  -n, --lines=[-]NUM    print the first NUM lines instead of the first 10;\n")
		fmt.Fprint(stderr, "                          with the leading '-', print all but the last\n")
		fmt.Fprint(stderr, "                          NUM lines of each file\n")
		fmt.Fprint(stderr, "  -q, --quiet, --silent never print headers giving file names\n")
		fmt.Fprint(stderr, "  -v, --verbose         always print headers giving file names\n")
		fmt.Fprint(stderr, "      --help            display this help and exit\n\n")
		fmt.Fprint(stderr, "NUM may have a multiplier suffix:\n")
		fmt.Fprint(stderr, "b 512, kB 1000, K 1024, MB 1000*1000, M 1024*1024,\n")
		fmt.Fprint(stderr, "GB 1000*1000*1000, G 1024*1024*1024, and so on for T, P, E, Z, Y.\n")
	}
	set.SetUsage(usage)

	bytesSpec := set.StringLong("bytes", 'c', "", "print the first NUM bytes")
	linesSpec := set.StringLong("lines", 'n', "", "print the first NUM lines (default 10)")
	quiet := set.BoolLong("quiet", 'q', "never print headers giving file names")
	silent := set.BoolLong("silent", 0, "alias for --quiet")
	verbose := set.BoolLong("verbose", 'v', "always print headers giving file names")
	help := set.BoolLong("help", 0, "display this help and exit")

	preArgs := preprocessShortNum(args)

	if err := set.Getopt(append([]string{"head"}, preArgs...), nil); err != nil {
		fmt.Fprintf(stderr, "head: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	lines := 10
	linesNeg := false
	bytes := 0
	bytesNeg := false
	bytesSet := false

	if *bytesSpec != "" {
		n, neg, err := parseHeadCount(*bytesSpec)
		if err != nil {
			fmt.Fprintf(stderr, "head: invalid number of bytes: '%s'\n", *bytesSpec)
			return interp.ExitStatus(1)
		}
		bytes = n
		bytesNeg = neg
		bytesSet = true
	}
	if *linesSpec != "" {
		n, neg, err := parseHeadCount(*linesSpec)
		if err != nil {
			fmt.Fprintf(stderr, "head: invalid number of lines: '%s'\n", *linesSpec)
			return interp.ExitStatus(1)
		}
		lines = n
		linesNeg = neg
	}

	isQuiet := *quiet || *silent
	files := set.Args()

	if len(files) == 0 {
		content, err := readStdin(ec)
		if err != nil {
			return err
		}
		io.WriteString(stdout, getHead(content, lines, linesNeg, bytes, bytesNeg, bytesSet))
		return nil
	}

	showHeaders := *verbose || (!isQuiet && len(files) > 1)
	var output strings.Builder
	exitCode := 0
	filesProcessed := 0

	for _, file := range files {
		content, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "head: %s: No such file or directory\n", file)
			exitCode = 1
			continue
		}
		if showHeaders {
			if filesProcessed > 0 {
				output.WriteByte('\n')
			}
			fmt.Fprintf(&output, "==> %s <==\n", file)
		}
		output.WriteString(getHead(content, lines, linesNeg, bytes, bytesNeg, bytesSet))
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
		// GNU shorthand: -NUM (digits only) means "-n NUM".
		// Note that -NUM with a non-digit suffix (e.g. -nK) is not handled here.
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

// parseHeadCount parses a GNU-style count for -n/-c. It accepts an optional
// leading '-' for "all but last K" semantics, optional size suffix
// (b, kB, K, MB, M, GB, G, T, P, E, Z, Y; with optional trailing 'B' for
// the binary forms), and returns (value, negative, error).
func parseHeadCount(s string) (int, bool, error) {
	if s == "" {
		return 0, false, errors.New("empty")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}
	if s == "" {
		return 0, false, errors.New("missing number")
	}

	// Split numeric prefix from suffix.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false, errors.New("not a number")
	}
	numPart := s[:i]
	suf := s[i:]

	n, err := strconv.Atoi(numPart)
	if err != nil || n < 0 {
		return 0, false, errors.New("not a number")
	}

	mult, ok := sizeMultiplier(suf)
	if !ok {
		return 0, false, fmt.Errorf("invalid suffix: %q", suf)
	}
	return n * mult, neg, nil
}

// sizeMultiplier returns the GNU coreutils size suffix multiplier.
// Recognizes (case-sensitive for kB vs K):
//
//	"" = 1
//	b  = 512
//	kB = 1000        K  = 1024
//	MB = 1000^2      M  = 1024^2
//	GB = 1000^3      G  = 1024^3
//	... up through Y. A trailing 'B' on the binary form (e.g. KB) is
//	accepted as an alias for the binary multiplier.
func sizeMultiplier(suf string) (int, bool) {
	if suf == "" {
		return 1, true
	}
	// Special-case "b" = 512.
	if suf == "b" {
		return 512, true
	}
	// "kB" = 1000.
	if suf == "kB" {
		return 1000, true
	}

	// SI (decimal) suffixes: "MB", "GB", "TB", "PB", "EB", "ZB", "YB".
	siLetters := []byte{'M', 'G', 'T', 'P', 'E', 'Z', 'Y'}
	for idx, c := range siLetters {
		if len(suf) == 2 && suf[0] == c && suf[1] == 'B' {
			mult := 1
			for k := 0; k <= idx+1; k++ {
				mult *= 1000
			}
			return mult, true
		}
	}

	// IEC (binary) suffixes: "K", "M", "G", "T", "P", "E", "Z", "Y";
	// also accept e.g. "KB" as alias for "K".
	binLetters := []byte{'K', 'M', 'G', 'T', 'P', 'E', 'Z', 'Y'}
	for idx, c := range binLetters {
		if suf == string(c) || suf == string(c)+"B" {
			mult := 1
			for k := 0; k <= idx; k++ {
				mult *= 1024
			}
			return mult, true
		}
	}
	return 0, false
}

func getHead(content string, lines int, linesNeg bool, bytes int, bytesNeg bool, bytesSet bool) string {
	if bytesSet {
		if bytesNeg {
			// Print all but the last `bytes` bytes.
			if bytes >= len(content) {
				return ""
			}
			return content[:len(content)-bytes]
		}
		if bytes >= len(content) {
			return content
		}
		return content[:bytes]
	}
	if linesNeg {
		// Print all but the last `lines` lines.
		// Count total complete lines (lines terminated by '\n'); a final
		// chunk without '\n' counts as a line for the purpose of dropping.
		if lines == 0 {
			return content
		}
		// Find positions of newlines so we can drop the last `lines` lines.
		// We treat the file as a sequence of lines separated by '\n'. A
		// trailing '\n' does NOT introduce an empty extra line; instead,
		// the absence of a final '\n' on the last chunk still counts as a
		// line.
		newlineIdx := []int{}
		for i := 0; i < len(content); i++ {
			if content[i] == '\n' {
				newlineIdx = append(newlineIdx, i)
			}
		}
		totalLines := len(newlineIdx)
		hasTrailingPartial := len(content) > 0 && content[len(content)-1] != '\n'
		if hasTrailingPartial {
			totalLines++
		}
		keep := totalLines - lines
		if keep <= 0 {
			return ""
		}
		// Truncate content after the keep-th newline.
		if keep <= len(newlineIdx) {
			return content[:newlineIdx[keep-1]+1]
		}
		// keep > number of newlines means keep includes the partial last line.
		return content
	}
	if lines == 0 {
		return ""
	}
	pos := 0
	lineCount := 0
	for pos < len(content) && lineCount < lines {
		idx := strings.IndexByte(content[pos:], '\n')
		if idx == -1 {
			// Final line without a trailing newline: copy as-is, do not synthesize one.
			return content
		}
		lineCount++
		pos += idx + 1
	}
	if pos > 0 {
		return content[:pos]
	}
	return ""
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
