// Package tr implements the tr coreutil for kefka.
//
// kefka aims for GNU-coreutils compatibility, not strict POSIX. Several
// constructs are deliberately implemented as a GNU-subset rather than a
// fully LC_COLLATE/LC_CTYPE-aware implementation:
//
//   - [=c=] equivalence classes match only the character c itself. GNU
//     coreutils itself only treats characters as equivalent to themselves
//     in the C locale (and in practice in any locale, because no system
//     locale defines real equivalence classes). This matches that
//     behaviour without consulting the locale at all.
//   - -C ("complement by character") is treated identically to -c
//     ("complement by byte value"). kefka does not maintain a separate
//     LC_CTYPE-aware character set, so the two are equivalent for our
//     purposes.
//   - [:class:] character classes are populated using Go's unicode
//     package over the ASCII range [0, 0x80). This matches the behaviour
//     of GNU tr in the C locale (which is what kefka effectively is).
//   - Character ranges (a-z) are byte-based. GNU tr is also byte-based
//     by default in the C locale.
package tr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

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
		fmt.Fprint(stderr, "  \\NNN        character with octal value NNN (1 to 3 octal digits)\n")
		fmt.Fprint(stderr, "  \\\\          backslash\n")
		fmt.Fprint(stderr, "  \\a          audible BEL\n")
		fmt.Fprint(stderr, "  \\b          backspace\n")
		fmt.Fprint(stderr, "  \\f          form feed\n")
		fmt.Fprint(stderr, "  \\n          new line\n")
		fmt.Fprint(stderr, "  \\r          return\n")
		fmt.Fprint(stderr, "  \\t          horizontal tab\n")
		fmt.Fprint(stderr, "  \\v          vertical tab\n")
		fmt.Fprint(stderr, "  CHAR1-CHAR2 all characters from CHAR1 to CHAR2 in ascending order\n")
		fmt.Fprint(stderr, "  [CHAR*]     in SET2, copies of CHAR until length of SET1\n")
		fmt.Fprint(stderr, "  [CHAR*REPEAT] REPEAT copies of CHAR, REPEAT octal if starting with 0\n")
		fmt.Fprint(stderr, "  [:alnum:]   all letters and digits\n")
		fmt.Fprint(stderr, "  [:alpha:]   all letters\n")
		fmt.Fprint(stderr, "  [:blank:]   all horizontal whitespace\n")
		fmt.Fprint(stderr, "  [:cntrl:]   all control characters\n")
		fmt.Fprint(stderr, "  [:digit:]   all digits\n")
		fmt.Fprint(stderr, "  [:graph:]   all printable characters, not including space\n")
		fmt.Fprint(stderr, "  [:lower:]   all lower case letters\n")
		fmt.Fprint(stderr, "  [:print:]   all printable characters, including space\n")
		fmt.Fprint(stderr, "  [:punct:]   all punctuation characters\n")
		fmt.Fprint(stderr, "  [:space:]   all horizontal or vertical whitespace\n")
		fmt.Fprint(stderr, "  [:upper:]   all upper case letters\n")
		fmt.Fprint(stderr, "  [:xdigit:]  all hexadecimal digits\n")
		fmt.Fprint(stderr, "  [=CHAR=]    all characters which are equivalent to CHAR\n")
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

	set1, err := expandSet1(sets[0])
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return interp.ExitStatus(1)
	}
	var set2 []rune
	if len(sets) > 1 {
		set2, err = expandSet2(sets[1], len(set1))
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

type classDef struct {
	name string
	in   func(rune) bool
}

var posixClasses = []classDef{
	{"alnum", func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }},
	{"alpha", unicode.IsLetter},
	{"blank", func(r rune) bool { return r == ' ' || r == '\t' }},
	{"cntrl", unicode.IsControl},
	{"digit", unicode.IsDigit},
	{"graph", func(r rune) bool { return unicode.IsPrint(r) && r != ' ' }},
	{"lower", unicode.IsLower},
	{"print", unicode.IsPrint},
	{"punct", unicode.IsPunct},
	{"space", unicode.IsSpace},
	{"upper", unicode.IsUpper},
	{"xdigit", func(r rune) bool {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
	}},
}

func expandClass(name string) ([]rune, bool) {
	for _, c := range posixClasses {
		if c.name == name {
			var out []rune
			for r := rune(0); r < 0x80; r++ {
				if c.in(r) {
					out = append(out, r)
				}
			}
			return out, true
		}
	}
	return nil, false
}

// parseEscape reads one character from rs starting at i, applying backslash
// escapes. It returns the rune, the number of input runes consumed, and any
// error.
func parseEscape(rs []rune, i int) (rune, int, error) {
	if rs[i] != '\\' || i+1 >= len(rs) {
		return rs[i], 1, nil
	}
	c := rs[i+1]
	switch c {
	case '\\':
		return '\\', 2, nil
	case 'a':
		return '\a', 2, nil
	case 'b':
		return '\b', 2, nil
	case 'f':
		return '\f', 2, nil
	case 'n':
		return '\n', 2, nil
	case 'r':
		return '\r', 2, nil
	case 't':
		return '\t', 2, nil
	case 'v':
		return '\v', 2, nil
	}
	if c >= '0' && c <= '7' {
		end := i + 2
		for end < len(rs) && end-(i+1) < 3 && rs[end] >= '0' && rs[end] <= '7' {
			end++
		}
		v, err := strconv.ParseInt(string(rs[i+1:end]), 8, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("tr: invalid octal escape: \\%s", string(rs[i+1:end]))
		}
		return rune(v), end - i, nil
	}
	return c, 2, nil
}

// matchClass returns the class name and consumed length if rs[i:] starts with
// "[:class:]". Returns ok=false if not a class construct.
func matchClass(rs []rune, i int) (string, int, bool) {
	if i+1 >= len(rs) || rs[i] != '[' || rs[i+1] != ':' {
		return "", 0, false
	}
	end := i + 2
	for end < len(rs) && rs[end] != ':' {
		end++
	}
	if end+1 >= len(rs) || rs[end] != ':' || rs[end+1] != ']' {
		return "", 0, false
	}
	return string(rs[i+2 : end]), end + 2 - i, true
}

// matchEquiv returns the equiv rune and consumed length if rs[i:] starts with
// "[=x=]". Returns ok=false if not an equivalence-class construct.
func matchEquiv(rs []rune, i int) (rune, int, bool) {
	if i+1 >= len(rs) || rs[i] != '[' || rs[i+1] != '=' {
		return 0, 0, false
	}
	r, n, err := parseEscape(rs, i+2)
	if err != nil {
		return 0, 0, false
	}
	end := i + 2 + n
	if end+1 >= len(rs) || rs[end] != '=' || rs[end+1] != ']' {
		return 0, 0, false
	}
	return r, end + 2 - i, true
}

// matchRepeat returns the rune, count, count-omitted flag, and consumed length
// if rs[i:] starts with "[x*]" or "[x*N]". Returns ok=false otherwise.
func matchRepeat(rs []rune, i int) (rune, int, bool, int, bool) {
	if rs[i] != '[' || i+1 >= len(rs) {
		return 0, 0, false, 0, false
	}
	r, n, err := parseEscape(rs, i+1)
	if err != nil {
		return 0, 0, false, 0, false
	}
	pos := i + 1 + n
	if pos >= len(rs) || rs[pos] != '*' {
		return 0, 0, false, 0, false
	}
	pos++
	digitsStart := pos
	for pos < len(rs) && rs[pos] != ']' {
		pos++
	}
	if pos >= len(rs) || rs[pos] != ']' {
		return 0, 0, false, 0, false
	}
	digits := string(rs[digitsStart:pos])
	if digits == "" {
		return r, 0, true, pos + 1 - i, true
	}
	base := 10
	if strings.HasPrefix(digits, "0") {
		base = 8
	}
	count, err := strconv.ParseInt(digits, base, 64)
	if err != nil || count < 0 {
		return 0, 0, false, 0, false
	}
	if count == 0 {
		return r, 0, true, pos + 1 - i, true
	}
	return r, int(count), false, pos + 1 - i, true
}

func expandSet1(s string) ([]rune, error) {
	rs := []rune(s)
	var out []rune
	i := 0
	for i < len(rs) {
		if name, n, ok := matchClass(rs, i); ok {
			runes, found := expandClass(name)
			if !found {
				return nil, fmt.Errorf("tr: invalid character class %q", name)
			}
			out = append(out, runes...)
			i += n
			continue
		}
		if r, n, ok := matchEquiv(rs, i); ok {
			out = append(out, r)
			i += n
			continue
		}
		if _, _, _, n, ok := matchRepeat(rs, i); ok {
			_ = n
			return nil, errors.New("tr: the [c*] repeat construct may not appear in string1")
		}

		r, n, err := parseEscape(rs, i)
		if err != nil {
			return nil, err
		}
		nextStart := i + n

		if nextStart < len(rs) && rs[nextStart] == '-' && nextStart+1 < len(rs) {
			end, m, err := parseEscape(rs, nextStart+1)
			if err != nil {
				return nil, err
			}
			if int(end)-int(r) > 65536 {
				return nil, fmt.Errorf("tr: character range too large: '%c-%c'", r, end)
			}
			if end < r {
				return nil, fmt.Errorf("tr: range-endpoints of '%c-%c' are in reverse collating sequence order", r, end)
			}
			for c := r; c <= end; c++ {
				out = append(out, c)
			}
			i = nextStart + 1 + m
			continue
		}

		out = append(out, r)
		i = nextStart
	}
	return out, nil
}

func expandSet2(s string, set1Len int) ([]rune, error) {
	rs := []rune(s)
	var segments []segment
	i := 0
	for i < len(rs) {
		if name, n, ok := matchClass(rs, i); ok {
			runes, found := expandClass(name)
			if !found {
				return nil, fmt.Errorf("tr: invalid character class %q", name)
			}
			segments = append(segments, segment{runes: runes})
			i += n
			continue
		}
		if r, n, ok := matchEquiv(rs, i); ok {
			segments = append(segments, segment{runes: []rune{r}})
			i += n
			continue
		}
		if r, count, omitted, n, ok := matchRepeat(rs, i); ok {
			seg := segment{repeatRune: r, repeatCount: count, repeatPad: omitted}
			segments = append(segments, seg)
			i += n
			continue
		}

		r, n, err := parseEscape(rs, i)
		if err != nil {
			return nil, err
		}
		nextStart := i + n

		if nextStart < len(rs) && rs[nextStart] == '-' && nextStart+1 < len(rs) {
			end, m, err := parseEscape(rs, nextStart+1)
			if err != nil {
				return nil, err
			}
			if int(end)-int(r) > 65536 {
				return nil, fmt.Errorf("tr: character range too large: '%c-%c'", r, end)
			}
			if end < r {
				return nil, fmt.Errorf("tr: range-endpoints of '%c-%c' are in reverse collating sequence order", r, end)
			}
			var rng []rune
			for c := r; c <= end; c++ {
				rng = append(rng, c)
			}
			segments = append(segments, segment{runes: rng})
			i = nextStart + 1 + m
			continue
		}

		segments = append(segments, segment{runes: []rune{r}})
		i = nextStart
	}

	fixed := 0
	pads := 0
	for _, seg := range segments {
		if seg.repeatPad {
			pads++
			continue
		}
		if seg.repeatCount > 0 {
			fixed += seg.repeatCount
			continue
		}
		fixed += len(seg.runes)
	}

	padTotal := 0
	if pads > 0 && fixed < set1Len {
		padTotal = set1Len - fixed
	}
	perPad := 0
	extra := 0
	if pads > 0 {
		perPad = padTotal / pads
		extra = padTotal % pads
	}

	var out []rune
	for _, seg := range segments {
		switch {
		case seg.repeatPad:
			n := perPad
			if extra > 0 {
				n++
				extra--
			}
			for k := 0; k < n; k++ {
				out = append(out, seg.repeatRune)
			}
		case seg.repeatCount > 0:
			for k := 0; k < seg.repeatCount; k++ {
				out = append(out, seg.repeatRune)
			}
		default:
			out = append(out, seg.runes...)
		}
	}
	return out, nil
}

type segment struct {
	runes       []rune
	repeatRune  rune
	repeatCount int
	repeatPad   bool
}
