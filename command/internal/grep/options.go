package grep

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
)

type options struct {
	ignoreCase, invert, word, line, extended, perl, fixed                 bool
	count, list, listWithout, numbers, noFilename, only, quiet, recursive bool
	before, after, maxCount                                               int
	patterns, patternFiles, include, exclude, excludeDir                  []string
}

func parseOptions(args []string, stderr io.Writer) (options, []string, bool, error) {
	var o options
	set := getopt.New()
	set.SetProgram("grep")
	set.SetParameters("PATTERN [FILE]...")
	set.SetUsage(func() { io.WriteString(stderr, helpText) })
	for _, f := range []struct {
		value *bool
		long  string
		short rune
	}{
		{&o.ignoreCase, "ignore-case", 'i'}, {&o.invert, "invert-match", 'v'},
		{&o.word, "word-regexp", 'w'}, {&o.line, "line-regexp", 'x'},
		{&o.extended, "extended-regexp", 'E'}, {&o.perl, "perl-regexp", 'P'},
		{&o.fixed, "fixed-strings", 'F'}, {&o.count, "count", 'c'},
		{&o.list, "files-with-matches", 'l'}, {&o.listWithout, "files-without-match", 'L'},
		{&o.numbers, "line-number", 'n'}, {&o.noFilename, "no-filename", 'h'},
		{&o.only, "only-matching", 'o'}, {&o.quiet, "quiet", 'q'},
		{&o.recursive, "recursive", 'r'},
	} {
		set.FlagLong(f.value, f.long, f.short)
	}
	set.Flag(&o.recursive, 'R')
	set.FlagLong(&o.quiet, "silent", 0)
	set.Flag(&o.patterns, 'e')
	set.FlagLong(&o.patternFiles, "file", 'f')
	set.FlagLong(&o.include, "include", 0)
	set.FlagLong(&o.exclude, "exclude", 0)
	set.FlagLong(&o.excludeDir, "exclude-dir", 0)
	set.StringLong("max-count", 'm', "")
	set.String('A', "")
	set.String('B', "")
	set.String('C', "")
	help := set.BoolLong("help", 0)
	var operands []string
	// getopt stops at each operand. Resume parsing to accept options after PATTERN,
	// but preserve -- and option-like values consumed by -e and -f.
	argv := append([]string{"grep"}, args...)
	for len(argv) > 1 {
		err := set.Getopt(argv, func(opt getopt.Option) bool {
			switch opt.ShortName() {
			case "A":
				o.after = parseNumber(opt.String())
			case "B":
				o.before = parseNumber(opt.String())
			case "C":
				o.before = parseNumber(opt.String())
				o.after = o.before
			case "m":
				o.maxCount = parseNumber(opt.String())
			}
			return !*help
		})
		if err != nil {
			return o, nil, false, err
		}
		if *help {
			io.WriteString(stderr, helpText)
			return o, nil, true, nil
		}
		rest := set.Args()
		if set.State() == getopt.DashDash {
			operands = append(operands, rest...)
			break
		}
		if len(rest) == 0 {
			break
		}
		operands = append(operands, rest[0])
		// The consumed operand occupies argv[0] on the next call. The program
		// name is already set, so there is no need to copy the remaining args.
		argv = rest
	}
	if len(o.patterns) == 0 && len(o.patternFiles) == 0 {
		if len(operands) == 0 {
			return o, nil, false, fmt.Errorf("missing pattern")
		}
		o.patterns = append(o.patterns, operands[0])
		operands = operands[1:]
	}
	var patterns []string
	for _, p := range o.patterns {
		patterns = append(patterns, strings.Split(p, "\n")...)
	}
	o.patterns = patterns
	return o, operands, false, nil
}

// Upstream uses parseInt: a decimal prefix is accepted, and nonpositive values
// disable limits. Clamp invalid and overflowing values to the same default.
func parseNumber(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

const helpText = `grep - print lines that match patterns

Usage: grep [OPTION]... PATTERN [FILE]...

Description:
  Search for PATTERN in each FILE.
  With no FILE, or when FILE is -, read standard input.

Options:
  -E, --extended-regexp    PATTERN is an extended regular expression
  -P, --perl-regexp        PATTERN is a Perl regular expression
  -F, --fixed-strings      PATTERN is a set of newline-separated strings
  -i, --ignore-case        ignore case distinctions
  -v, --invert-match       select non-matching lines
  -w, --word-regexp        match only whole words
  -x, --line-regexp        match only whole lines
  -c, --count              print only a count of matching lines
  -l, --files-with-matches print only names of files with matches
  -L, --files-without-match print names of files with no matches
  -m NUM, --max-count=NUM  stop after NUM matches
  -n, --line-number        print line number with output lines
  -h, --no-filename        suppress the file name prefix on output
  -o, --only-matching      show only nonempty parts of lines that match
  -q, --quiet, --silent    suppress all normal output
  -r, -R, --recursive      search directories recursively
  -A NUM                   print NUM lines of trailing context
  -B NUM                   print NUM lines of leading context
  -C NUM                   print NUM lines of context
  -e PATTERN               use PATTERN for matching
  -f FILE, --file=FILE     obtain patterns from FILE, one per line
      --include=GLOB       search only files matching GLOB
      --exclude=GLOB       skip files matching GLOB
      --exclude-dir=DIR    skip directories matching DIR
      --help               display this help and exit
`
