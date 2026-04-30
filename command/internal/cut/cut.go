package cut

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
		fmt.Fprint(stderr, "  -c LIST              select only these characters\n")
		fmt.Fprint(stderr, "  -d DELIM             use DELIM instead of TAB for field delimiter\n")
		fmt.Fprint(stderr, "  -f LIST              select only these fields\n")
		fmt.Fprint(stderr, "  -s, --only-delimited  do not print lines without delimiters\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	charSpec := set.String('c', "", "select only these characters")
	delim := set.String('d', "\t", "use DELIM instead of TAB for field delimiter")
	fieldSpec := set.String('f', "", "select only these fields")
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

	if *fieldSpec == "" && *charSpec == "" {
		fmt.Fprint(stderr, "cut: you must specify a list of bytes, characters, or fields\n")
		return interp.ExitStatus(1)
	}

	files := set.Args()
	content, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	spec := *fieldSpec
	if spec == "" {
		spec = *charSpec
	}
	if spec == "" {
		spec = "1"
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
		if *charSpec != "" {
			chars := []rune(line)
			var selected []rune
			for _, r := range ranges {
				start := r.start - 1
				end := r.end
				if r.toEnd {
					end = len(chars)
				}
				for i := start; i < end && i < len(chars); i++ {
					if i >= 0 {
						selected = append(selected, chars[i])
					}
				}
			}
			out.WriteString(string(selected))
			out.WriteString("\n")
		} else {
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
	seen := make(map[string]struct{})
	for _, r := range ranges {
		start := r.start - 1
		end := r.end
		if r.toEnd {
			end = len(items)
		}
		for i := start; i < end && i < len(items); i++ {
			if i >= 0 {
				if _, ok := seen[items[i]]; !ok {
					seen[items[i]] = struct{}{}
					result = append(result, items[i])
				}
			}
		}
	}
	return result
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
