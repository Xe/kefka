package cut

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
		return errors.New("cut: nil ExecContext")
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
	set.SetProgram("cut")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: cut [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Remove sections from each line of FILE(s).\n\n")
		fmt.Fprint(stderr, "  -b LIST              select only these bytes\n")
		fmt.Fprint(stderr, "  -c LIST              select only these characters\n")
		fmt.Fprint(stderr, "  -d DELIM             use DELIM instead of TAB for field delimiter\n")
		fmt.Fprint(stderr, "  -f LIST              select only these fields\n")
		fmt.Fprint(stderr, "  -n                   with -b: do not split multibyte characters\n")
		fmt.Fprint(stderr, "  -s, --only-delimited  do not print lines without delimiters\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	byteSpec := set.String('b', "", "select only these bytes")
	charSpec := set.String('c', "", "select only these characters")
	delim := set.String('d', "\t", "use DELIM instead of TAB for field delimiter")
	fieldSpec := set.String('f', "", "select only these fields")
	noSplit := set.Bool('n', "with -b: do not split multibyte characters")
	suppressNoDelim := set.BoolLong("only-delimited", 's', "do not print lines without delimiters")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"cut"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "cut: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	modes := 0
	if *byteSpec != "" {
		modes++
	}
	if *charSpec != "" {
		modes++
	}
	if *fieldSpec != "" {
		modes++
	}
	if modes > 1 {
		fmt.Fprint(stderr, "cut: only one type of list may be specified\n")
		return interp.ExitStatus(1)
	}
	if modes == 0 {
		fmt.Fprint(stderr, "cut: you must specify a list of bytes, characters, or fields\n")
		return interp.ExitStatus(1)
	}

	files := set.Args()
	content, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	var spec string
	switch {
	case *byteSpec != "":
		spec = *byteSpec
	case *charSpec != "":
		spec = *charSpec
	default:
		spec = *fieldSpec
	}
	ranges := parseRanges(spec)

	d := *delim
	if d == "" {
		d = "\t"
	}

	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var out strings.Builder
	for _, line := range lines {
		switch {
		case *byteSpec != "":
			lineBytes := []byte(line)
			rs := ranges
			if *noSplit {
				rs = adjustRangesNoSplit(lineBytes, ranges)
			}
			selected := extractBytes(lineBytes, rs)
			out.Write(selected)
			out.WriteString("\n")
		case *charSpec != "":
			chars := []rune(line)
			selected := extractRunes(chars, ranges)
			out.WriteString(string(selected))
			out.WriteString("\n")
		default:
			if *suppressNoDelim && !strings.Contains(line, d) {
				continue
			}
			fields := strings.Split(line, d)
			selected := extractByRanges(fields, ranges)
			out.WriteString(strings.Join(selected, d))
			out.WriteString("\n")
		}
	}

	io.WriteString(stdout, out.String())
	return nil
}

type cutRange struct {
	start, end int
	toEnd      bool
}

func parseRanges(spec string) []cutRange {
	var ranges []cutRange
	for part := range strings.SplitSeq(spec, ",") {
		if strings.Contains(part, "-") {
			pieces := strings.SplitN(part, "-", 2)
			startStr, endStr := pieces[0], pieces[1]
			r := cutRange{start: 1, toEnd: true}
			if startStr != "" {
				if n, err := strconv.Atoi(startStr); err == nil {
					r.start = n
				} else {
					r.start = 0
				}
			}
			if endStr != "" {
				r.toEnd = false
				if n, err := strconv.Atoi(endStr); err == nil {
					r.end = n
				} else {
					r.end = 0
				}
			}
			ranges = append(ranges, r)
		} else {
			if n, err := strconv.Atoi(part); err == nil {
				ranges = append(ranges, cutRange{start: n, end: n})
			} else {
				ranges = append(ranges, cutRange{start: 0, end: 0})
			}
		}
	}
	return ranges
}

func extractByRanges(items []string, ranges []cutRange) []string {
	var result []string
	seen := make(map[int]struct{})
	for _, r := range ranges {
		start := r.start - 1
		end := r.end
		if r.toEnd {
			end = len(items)
		}
		for i := start; i < end && i < len(items); i++ {
			if i >= 0 {
				if _, ok := seen[i]; !ok {
					seen[i] = struct{}{}
					result = append(result, items[i])
				}
			}
		}
	}
	return result
}

func extractRunes(chars []rune, ranges []cutRange) []rune {
	var result []rune
	seen := make(map[int]struct{})
	for _, r := range ranges {
		start := r.start - 1
		end := r.end
		if r.toEnd {
			end = len(chars)
		}
		for i := start; i < end && i < len(chars); i++ {
			if i >= 0 {
				if _, ok := seen[i]; !ok {
					seen[i] = struct{}{}
					result = append(result, chars[i])
				}
			}
		}
	}
	return result
}

func extractBytes(b []byte, ranges []cutRange) []byte {
	var result []byte
	seen := make(map[int]struct{})
	for _, r := range ranges {
		start := r.start - 1
		end := r.end
		if r.toEnd {
			end = len(b)
		}
		for i := start; i < end && i < len(b); i++ {
			if i >= 0 {
				if _, ok := seen[i]; !ok {
					seen[i] = struct{}{}
					result = append(result, b[i])
				}
			}
		}
	}
	return result
}

// adjustRangesNoSplit applies the POSIX -n algorithm to byte ranges. For each
// range low-high: if low is not the first byte of a character, decrement low
// to the character start; if high is not the last byte of a character,
// decrement high to the last byte of the prior character (or zero). Drop the
// range if high becomes zero or low exceeds high.
func adjustRangesNoSplit(b []byte, ranges []cutRange) []cutRange {
	if len(b) == 0 {
		return ranges
	}
	starts, ends := charBoundaries(b)
	var out []cutRange
	for _, r := range ranges {
		low := r.start
		high := r.end
		if r.toEnd {
			high = len(b)
		}
		if low < 1 {
			low = 1
		}
		if high > len(b) {
			high = len(b)
		}
		if low > len(b) {
			out = append(out, cutRange{start: 0, end: 0})
			continue
		}
		if !starts[low-1] {
			for low > 1 && !starts[low-1] {
				low--
			}
		}
		if high >= 1 && high <= len(b) && !ends[high-1] {
			for high > 0 && !ends[high-1] {
				high--
			}
		}
		if high < low || high == 0 {
			out = append(out, cutRange{start: 0, end: 0})
			continue
		}
		out = append(out, cutRange{start: low, end: high})
	}
	return out
}

func charBoundaries(b []byte) (starts, ends []bool) {
	starts = make([]bool, len(b))
	ends = make([]bool, len(b))
	i := 0
	for i < len(b) {
		_, size := utf8.DecodeRune(b[i:])
		if size <= 0 {
			size = 1
		}
		starts[i] = true
		ends[i+size-1] = true
		i += size
	}
	return starts, ends
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
			fmt.Fprintf(stderr, "cut: %s: No such file or directory\n", file)
			return "", interp.ExitStatus(1)
		}
		full := resolvePath(ec, file)
		f, err := ec.FS.Open(full)
		if err != nil {
			fmt.Fprintf(stderr, "cut: %s: No such file or directory\n", file)
			return "", interp.ExitStatus(1)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "cut: %s: %v\n", file, err)
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
