package tr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("tr: nil ExecContext")
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
	set.SetProgram("tr")
	set.SetParameters("SET1 [SET2]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: tr [OPTION]... SET1 [SET2]\n")
		fmt.Fprint(stderr, "Translate, squeeze, and/or delete characters from standard input,\n")
		fmt.Fprint(stderr, "writing to standard output.\n\n")
		fmt.Fprint(stderr, "  -c, -C, --complement   use the complement of SET1\n")
		fmt.Fprint(stderr, "  -d, --delete           delete characters in SET1\n")
		fmt.Fprint(stderr, "  -s, --squeeze-repeats  squeeze repeated characters\n")
		fmt.Fprint(stderr, "      --help             display this help and exit\n\n")
		fmt.Fprint(stderr, "SET syntax:\n")
		fmt.Fprint(stderr, "  a-z         character range\n")
		fmt.Fprint(stderr, "  [:alnum:]   all letters and digits\n")
		fmt.Fprint(stderr, "  [:alpha:]   all letters\n")
		fmt.Fprint(stderr, "  [:digit:]   all digits\n")
		fmt.Fprint(stderr, "  [:lower:]   all lowercase letters\n")
		fmt.Fprint(stderr, "  [:upper:]   all uppercase letters\n")
		fmt.Fprint(stderr, "  [:space:]   all whitespace\n")
		fmt.Fprint(stderr, "  [:blank:]   horizontal whitespace\n")
		fmt.Fprint(stderr, "  [:punct:]   all punctuation\n")
		fmt.Fprint(stderr, "  [:print:]   all printable characters\n")
		fmt.Fprint(stderr, "  [:graph:]   all printable characters except space\n")
		fmt.Fprint(stderr, "  [:cntrl:]   all control characters\n")
		fmt.Fprint(stderr, "  [:xdigit:]  all hexadecimal digits\n")
		fmt.Fprint(stderr, "  \\n, \\t, \\r  escape sequences\n")
	}
	set.SetUsage(usage)

	complementLong := set.BoolLong("complement", 'c', "use the complement of SET1")
	complementUpper := set.Bool('C', "use the complement of SET1")
	deleteFlag := set.BoolLong("delete", 'd', "delete characters in SET1")
	squeezeFlag := set.BoolLong("squeeze-repeats", 's', "squeeze repeated characters")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"tr"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "tr: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	complement := *complementLong || *complementUpper
	deleteMode := *deleteFlag
	squeezeMode := *squeezeFlag
	sets := set.Args()

	if len(sets) < 1 {
		fmt.Fprint(stderr, "tr: missing operand\n")
		return interp.ExitStatus(1)
	}
	if !deleteMode && !squeezeMode && len(sets) < 2 {
		fmt.Fprint(stderr, "tr: missing operand after SET1\n")
		return interp.ExitStatus(1)
	}

	set1, err := expandSet(sets[0])
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return interp.ExitStatus(1)
	}
	var set2 []rune
	if len(sets) > 1 {
		set2, err = expandSet(sets[1])
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", err)
			return interp.ExitStatus(1)
		}
	}

	var input []byte
	if ec.Stdin != nil {
		input, err = io.ReadAll(ec.Stdin)
		if err != nil {
			return interp.ExitStatus(1)
		}
	}
	content := []rune(string(input))

	set1Has := func(r rune) bool {
		return slices.Contains(set1, r)
	}
	inSet1 := func(r rune) bool {
		has := set1Has(r)
		if complement {
			return !has
		}
		return has
	}

	var output []rune

	switch {
	case deleteMode:
		for _, r := range content {
			if !inSet1(r) {
				output = append(output, r)
			}
		}
	case squeezeMode && len(sets) == 1:
		var prev rune
		havePrev := false
		for _, r := range content {
			if havePrev && inSet1(r) && r == prev {
				continue
			}
			output = append(output, r)
			prev = r
			havePrev = true
		}
	default:
		if complement {
			var target rune
			haveTarget := len(set2) > 0
			if haveTarget {
				target = set2[len(set2)-1]
			}
			for _, r := range content {
				if !set1Has(r) {
					if haveTarget {
						output = append(output, target)
					}
				} else {
					output = append(output, r)
				}
			}
		} else {
			tmap := make(map[rune]rune, len(set1))
			for i, r := range set1 {
				switch {
				case i < len(set2):
					tmap[r] = set2[i]
				case len(set2) > 0:
					tmap[r] = set2[len(set2)-1]
				default:
					tmap[r] = r
				}
			}
			for _, r := range content {
				if t, ok := tmap[r]; ok {
					output = append(output, t)
				} else {
					output = append(output, r)
				}
			}
		}

		if squeezeMode {
			set2Has := func(r rune) bool {
				return slices.Contains(set2, r)
			}
			squeezed := make([]rune, 0, len(output))
			var prev rune
			havePrev := false
			for _, r := range output {
				if havePrev && set2Has(r) && r == prev {
					continue
				}
				squeezed = append(squeezed, r)
				prev = r
				havePrev = true
			}
			output = squeezed
		}
	}

	io.WriteString(stdout, string(output))
	return nil
}

var posixClasses = []struct {
	name  string
	chars string
}{
	{"[:alnum:]", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"},
	{"[:alpha:]", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"},
	{"[:blank:]", " \t"},
	{"[:cntrl:]", buildCntrl()},
	{"[:digit:]", "0123456789"},
	{"[:graph:]", buildRange(33, 126)},
	{"[:lower:]", "abcdefghijklmnopqrstuvwxyz"},
	{"[:print:]", buildRange(32, 126)},
	{"[:punct:]", "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"},
	{"[:space:]", " \t\n\r\f\v"},
	{"[:upper:]", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
	{"[:xdigit:]", "0123456789ABCDEFabcdef"},
}

func buildRange(lo, hi int) string {
	var b strings.Builder
	for c := lo; c <= hi; c++ {
		b.WriteByte(byte(c))
	}
	return b.String()
}

func buildCntrl() string {
	var b strings.Builder
	for c := range 32 {
		b.WriteByte(byte(c))
	}
	b.WriteByte(127)
	return b.String()
}

func expandSet(s string) ([]rune, error) {
	rs := []rune(s)
	var out []rune
	i := 0
	for i < len(rs) {
		if rs[i] == '[' && i+1 < len(rs) && rs[i+1] == ':' {
			matched := false
			for _, cls := range posixClasses {
				if strings.HasPrefix(string(rs[i:]), cls.name) {
					out = append(out, []rune(cls.chars)...)
					i += len([]rune(cls.name))
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}

		if rs[i] == '\\' && i+1 < len(rs) {
			switch rs[i+1] {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			default:
				out = append(out, rs[i+1])
			}
			i += 2
			continue
		}

		if i+2 < len(rs) && rs[i+1] == '-' {
			start := rs[i]
			end := rs[i+2]
			if int(end)-int(start) > 65536 {
				return nil, fmt.Errorf("tr: character range too large: '%c-%c'", start, end)
			}
			for c := start; c <= end; c++ {
				out = append(out, c)
			}
			i += 3
			continue
		}

		out = append(out, rs[i])
		i++
	}
	return out, nil
}
