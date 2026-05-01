package tail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

// followPollInterval is the duration tail -f sleeps between polls for new
// data. It is a package-level variable so tests can shorten it.
var followPollInterval = time.Second

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
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
		fmt.Fprint(stderr, "  -c, --bytes=[+]NUM print the last NUM bytes; or use -c +NUM to\n")
		fmt.Fprint(stderr, "                       output starting with byte NUM of each file\n")
		fmt.Fprint(stderr, "  -f, --follow       output appended data as the file grows\n")
		fmt.Fprint(stderr, "  -n, --lines=[+]NUM print the last NUM lines (default 10); or use\n")
		fmt.Fprint(stderr, "                       -n +NUM to output starting with line NUM\n")
		fmt.Fprint(stderr, "  -q, --quiet        never print headers giving file names\n")
		fmt.Fprint(stderr, "  -v, --verbose      always print headers giving file names\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n\n")
		fmt.Fprint(stderr, "NUM may have a multiplier suffix:\n")
		fmt.Fprint(stderr, "b 512, kB 1000, K 1024, MB 1000*1000, M 1024*1024,\n")
		fmt.Fprint(stderr, "GB 1000*1000*1000, G 1024*1024*1024, and so on for T, P, E, Z, Y.\n\n")
		fmt.Fprint(stderr, "Note: -F (follow by name through renames) is not supported; use -f.\n")
	}
	set.SetUsage(usage)

	bytesSpec := set.StringLong("bytes", 'c', "", "print the last NUM bytes")
	linesSpec := set.StringLong("lines", 'n', "", "print the last NUM lines (default 10)")
	follow := set.BoolLong("follow", 'f', "output appended data as the file grows")
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
	fromByte := false

	if *bytesSpec != "" {
		spec := *bytesSpec
		if strings.HasPrefix(spec, "+") {
			fromByte = true
			spec = spec[1:]
		}
		n, err := parseTailCount(spec)
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
		n, err := parseTailCount(spec)
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
		io.WriteString(stdout, getTail(content, lines, bytes, bytesSet, fromLine, fromByte))
		// POSIX: -f is ignored when reading standard input that is a pipe
		// or FIFO. We have no reliable way to tell from a generic Reader,
		// so treat stdin as non-followable: -f is a no-op here.
		return nil
	}

	showHeaders := *verbose || (!isQuiet && len(files) > 1)

	// Track the post-read size for each file operand so that follow mode
	// (if requested) knows where to start reading new bytes from.
	states := make([]fileState, 0, len(files))

	var output strings.Builder
	exitCode := 0
	filesProcessed := 0

	for _, file := range files {
		content, err := readFile(ec, file)
		st := fileState{name: file}
		if err != nil {
			fmt.Fprintf(stderr, "tail: %s: No such file or directory\n", file)
			exitCode = 1
			states = append(states, st)
			continue
		}
		if showHeaders {
			if filesProcessed > 0 {
				output.WriteByte('\n')
			}
			fmt.Fprintf(&output, "==> %s <==\n", file)
		}
		output.WriteString(getTail(content, lines, bytes, bytesSet, fromLine, fromByte))
		filesProcessed++

		if file != "-" && ec.FS != nil {
			st.size = int64(len(content))
			st.isRegFS = true
		}
		states = append(states, st)
	}

	io.WriteString(stdout, output.String())

	if *follow && filesProcessed > 0 {
		// Flush any deferred header logic before entering the follow loop.
		if err := followLoop(ctx, ec, stdout, states, showHeaders); err != nil {
			// followLoop only returns errors from context cancellation,
			// which is the normal shutdown path; suppress.
			_ = err
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

// followLoop polls each followable file for appended bytes and writes them
// to stdout. With multiple files, a "==> name <==" header is emitted when
// switching between files, matching GNU coreutils behaviour.
//
// Returns when ctx is done. Per POSIX/GNU, follow mode runs until the
// process is killed; here, we use the supplied context as the kill
// signal.
func followLoop(ctx context.Context, ec *command.ExecContext, stdout io.Writer, states []fileState, showHeaders bool) error {
	// Determine if we have anything followable at all.
	any := false
	for _, s := range states {
		if s.isRegFS {
			any = true
			break
		}
	}
	if !any {
		return nil
	}

	lastEmittedFile := ""
	if showHeaders {
		// The header for the last successfully read file was already
		// emitted in the synchronous phase; remember it so we don't
		// duplicate it on the first new-bytes write.
		for i := len(states) - 1; i >= 0; i-- {
			if states[i].isRegFS {
				lastEmittedFile = states[i].name
				break
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(followPollInterval):
		}

		for i := range states {
			s := &states[i]
			if !s.isRegFS {
				continue
			}
			info, err := ec.FS.Stat(resolvePath(ec, s.name))
			if err != nil {
				continue
			}
			cur := info.Size()
			if cur <= s.size {
				// Truncation: GNU prints "file truncated" and resets.
				// For our minimal impl, just reset position so we
				// don't emit stale tail.
				if cur < s.size {
					s.size = cur
				}
				continue
			}
			// New bytes available.
			data, err := readBytesFrom(ec, s.name, s.size, cur-s.size)
			if err != nil {
				continue
			}
			if showHeaders && lastEmittedFile != s.name {
				if lastEmittedFile != "" {
					io.WriteString(stdout, "\n")
				}
				fmt.Fprintf(stdout, "==> %s <==\n", s.name)
				lastEmittedFile = s.name
			}
			stdout.Write(data)
			s.size = cur
		}
	}
}

// readBytesFrom opens the file, seeks to off, and reads up to n bytes.
func readBytesFrom(ec *command.ExecContext, name string, off, n int64) ([]byte, error) {
	f, err := ec.FS.Open(resolvePath(ec, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:read], nil
}

// fileState tracks per-file follow-mode state.
type fileState struct {
	name    string
	size    int64
	isRegFS bool
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

// parseTailCount accepts a non-negative decimal integer, optionally with a
// GNU coreutils size-suffix multiplier (b, kB, K, MB, M, ...). A leading
// '+' is rejected here; sign handling is the caller's responsibility.
func parseTailCount(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	// Reject signs at this layer; the line/byte parsers strip the
	// '+' for fromLine mode before calling us.
	if s[0] == '+' || s[0] == '-' {
		return 0, errors.New("signed")
	}
	// Split numeric prefix from suffix.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, errors.New("not a number")
	}
	numPart := s[:i]
	suf := s[i:]

	n, err := strconv.Atoi(numPart)
	if err != nil || n < 0 {
		return 0, errors.New("not a number")
	}
	mult, ok := sizeMultiplier(suf)
	if !ok {
		return 0, fmt.Errorf("invalid suffix: %q", suf)
	}
	return n * mult, nil
}

// sizeMultiplier returns the GNU coreutils size suffix multiplier.
func sizeMultiplier(suf string) (int, bool) {
	if suf == "" {
		return 1, true
	}
	if suf == "b" {
		return 512, true
	}
	if suf == "kB" {
		return 1000, true
	}
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

func getTail(content string, lines int, bytes int, bytesSet bool, fromLine bool, fromByte bool) string {
	if bytesSet {
		if fromByte {
			// GNU semantics: -c +N starts output at byte N (1-based).
			// "+0" and "+1" both mean "from the very beginning".
			start := bytes - 1
			if start < 0 {
				start = 0
			}
			if start >= len(content) {
				return ""
			}
			return content[start:]
		}
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
		// Preserve the input's trailing-newline state: do not synthesize one.
		return content[pos:]
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
	// Preserve the input's trailing-newline state: do not synthesize one.
	return content[pos:]
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
