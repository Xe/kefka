package split

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

const maxOutputFiles = 100_000

type splitMode int

const (
	modeLines splitMode = iota
	modeBytes
	modeChunks
)

type chunk struct {
	content    []byte
	hasContent bool
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("split: nil ExecContext")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("split")
	set.SetParameters("[FILE [PREFIX]]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: split [OPTION]... [FILE [PREFIX]]\n")
		fmt.Fprint(stderr, "Output pieces of FILE to PREFIXaa, PREFIXab, ...;\n")
		fmt.Fprint(stderr, "default size is 1000 lines, and default PREFIX is 'x'.\n\n")
		fmt.Fprint(stderr, "  -l N                        put N lines per output file\n")
		fmt.Fprint(stderr, "  -b SIZE                     put SIZE bytes per output file (K, M, G suffixes)\n")
		fmt.Fprint(stderr, "  -n CHUNKS                   split into CHUNKS equal-sized files\n")
		fmt.Fprint(stderr, "  -d, --numeric-suffixes      use numeric suffixes (00, 01, ...) instead of alphabetic\n")
		fmt.Fprint(stderr, "  -a LENGTH                   use suffixes of length LENGTH (default: 2)\n")
		fmt.Fprint(stderr, "      --additional-suffix=SUFFIX  append SUFFIX to file names\n")
		fmt.Fprint(stderr, "      --help                  display this help and exit\n")
	}
	set.SetUsage(usage)

	linesStr := set.String('l', "", "put N lines per output file")
	bytesStr := set.String('b', "", "put SIZE bytes per output file")
	chunksStr := set.String('n', "", "split into CHUNKS equal-sized files")
	suffixLenStr := set.String('a', "", "use suffixes of length LENGTH")
	useNumeric := set.BoolLong("numeric-suffixes", 'd', "use numeric suffixes")
	additionalSuffix := set.StringLong("additional-suffix", 0, "", "append SUFFIX to file names")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	var modeOrder []byte
	callback := func(opt getopt.Option) bool {
		switch opt.ShortName() {
		case "l", "b", "n":
			modeOrder = append(modeOrder, opt.ShortName()[0])
		}
		return true
	}

	if err := set.Getopt(append([]string{"split"}, args...), callback); err != nil {
		fmt.Fprintf(stderr, "split: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *helpFlag {
		usage()
		return nil
	}

	mode := modeLines
	linesPerFile := 1000
	bytesPerFile := 0
	numChunks := 0
	suffixLength := 2

	if len(modeOrder) > 0 {
		switch modeOrder[len(modeOrder)-1] {
		case 'l':
			mode = modeLines
		case 'b':
			mode = modeBytes
		case 'n':
			mode = modeChunks
		}
	}

	if *linesStr != "" {
		n, err := strconv.Atoi(*linesStr)
		if err != nil || n < 1 {
			fmt.Fprintf(stderr, "split: invalid number of lines: '%s'\n", *linesStr)
			return interp.ExitStatus(1)
		}
		linesPerFile = n
	}
	if *bytesStr != "" {
		n, ok := parseSize(*bytesStr)
		if !ok {
			fmt.Fprintf(stderr, "split: invalid number of bytes: '%s'\n", *bytesStr)
			return interp.ExitStatus(1)
		}
		bytesPerFile = n
	}
	if *chunksStr != "" {
		n, err := strconv.Atoi(*chunksStr)
		if err != nil || n < 1 {
			fmt.Fprintf(stderr, "split: invalid number of chunks: '%s'\n", *chunksStr)
			return interp.ExitStatus(1)
		}
		numChunks = n
	}
	if *suffixLenStr != "" {
		n, err := strconv.Atoi(*suffixLenStr)
		if err != nil || n < 1 {
			fmt.Fprintf(stderr, "split: invalid suffix length: '%s'\n", *suffixLenStr)
			return interp.ExitStatus(1)
		}
		suffixLength = n
	}

	positional := set.Args()
	inputFile := "-"
	prefix := "x"
	if len(positional) >= 1 {
		inputFile = positional[0]
	}
	if len(positional) >= 2 {
		prefix = positional[1]
	}

	content, err := readInput(ec, inputFile, stderr)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}

	var chunks []chunk
	switch mode {
	case modeLines:
		chunks = splitByLines(content, linesPerFile)
	case modeBytes:
		chunks = splitByBytes(content, bytesPerFile)
	case modeChunks:
		chunks = splitIntoChunks(content, numChunks)
	}

	if len(chunks) > maxOutputFiles {
		fmt.Fprintf(stderr, "split: too many output files (%d), limit is %d\n", len(chunks), maxOutputFiles)
		return interp.ExitStatus(1)
	}

	if ec.FS == nil {
		return errors.New("split: ExecContext has no filesystem")
	}

	for i, ch := range chunks {
		if !ch.hasContent {
			continue
		}
		filename := prefix + generateSuffix(i, *useNumeric, suffixLength) + *additionalSuffix
		full := resolvePath(ec, filename)
		f, err := ec.FS.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "split: %s: %v\n", filename, err)
			return interp.ExitStatus(1)
		}
		if _, err := f.Write(ch.content); err != nil {
			f.Close()
			fmt.Fprintf(stderr, "split: %s: %v\n", filename, err)
			return interp.ExitStatus(1)
		}
		f.Close()
	}

	return nil
}

func readInput(ec *command.ExecContext, inputFile string, stderr io.Writer) ([]byte, error) {
	if inputFile == "-" {
		if ec.Stdin == nil {
			return nil, nil
		}
		return io.ReadAll(ec.Stdin)
	}
	if ec.FS == nil {
		fmt.Fprintf(stderr, "split: %s: No such file or directory\n", inputFile)
		return nil, interp.ExitStatus(1)
	}
	f, err := ec.FS.Open(resolvePath(ec, inputFile))
	if err != nil {
		fmt.Fprintf(stderr, "split: %s: No such file or directory\n", inputFile)
		return nil, interp.ExitStatus(1)
	}
	defer f.Close()
	return io.ReadAll(f)
}

var sizeRe = regexp.MustCompile(`(?i)^(\d+)([KMGTPEZY]?)([B]?)$`)

func parseSize(s string) (int, bool) {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, false
	}
	multipliers := map[string]int{
		"":  1,
		"K": 1024,
		"M": 1024 * 1024,
		"G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024,
		"P": 1024 * 1024 * 1024 * 1024 * 1024,
	}
	mult, ok := multipliers[strings.ToUpper(m[2])]
	if !ok {
		return 0, false
	}
	return n * mult, true
}

func generateSuffix(index int, useNumeric bool, length int) string {
	if useNumeric {
		s := strconv.Itoa(index)
		if len(s) < length {
			s = strings.Repeat("0", length-len(s)) + s
		}
		return s
	}
	const chars = "abcdefghijklmnopqrstuvwxyz"
	suffix := make([]byte, length)
	remaining := index
	for i := length - 1; i >= 0; i-- {
		suffix[i] = chars[remaining%26]
		remaining /= 26
	}
	return string(suffix)
}

func splitByLines(content []byte, linesPerFile int) []chunk {
	s := string(content)
	lines := strings.Split(s, "\n")
	hasTrailingNewline := strings.HasSuffix(s, "\n") && lines[len(lines)-1] == ""
	if hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	var chunks []chunk
	for i := 0; i < len(lines); i += linesPerFile {
		end := i + linesPerFile
		if end > len(lines) {
			end = len(lines)
		}
		chunkLines := lines[i:end]
		isLastChunk := i+linesPerFile >= len(lines)
		var c string
		if isLastChunk && !hasTrailingNewline {
			c = strings.Join(chunkLines, "\n")
		} else {
			c = strings.Join(chunkLines, "\n") + "\n"
		}
		chunks = append(chunks, chunk{content: []byte(c), hasContent: true})
	}
	return chunks
}

func splitByBytes(content []byte, bytesPerFile int) []chunk {
	var chunks []chunk
	for i := 0; i < len(content); i += bytesPerFile {
		end := i + bytesPerFile
		if end > len(content) {
			end = len(content)
		}
		b := content[i:end]
		chunks = append(chunks, chunk{content: b, hasContent: len(b) > 0})
	}
	return chunks
}

func splitIntoChunks(content []byte, numChunks int) []chunk {
	if numChunks < 1 {
		return nil
	}
	bytesPerChunk := (len(content) + numChunks - 1) / numChunks
	if bytesPerChunk < 1 {
		bytesPerChunk = 1
	}
	chunks := make([]chunk, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		start := i * bytesPerChunk
		if start > len(content) {
			start = len(content)
		}
		end := start + bytesPerChunk
		if end > len(content) {
			end = len(content)
		}
		b := content[start:end]
		chunks = append(chunks, chunk{content: b, hasContent: len(b) > 0})
	}
	return chunks
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
