package grep

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2"
)

type matcher struct {
	re         *regexp2.Regexp
	resetGroup string
}

const matchTimeout = 250 * time.Millisecond

func compilePatterns(o options) (*matcher, error) {
	if o.perl && !o.fixed && !o.extended {
		seen := make(map[string]bool)
		for _, p := range o.patterns {
			seen[p] = true
		}
		if len(seen) > 1 {
			return nil, fmt.Errorf("the -P option only supports a single pattern")
		}
	}
	// Validate independently so an unfinished escape cannot swallow an alternative.
	if len(o.patterns) > 1 && !o.fixed {
		for _, p := range o.patterns {
			if _, err := compilePattern(p, o, false); err != nil {
				return nil, fmt.Errorf("invalid regular expression: %s", p)
			}
		}
	}
	var source string
	switch {
	case len(o.patterns) == 0:
		source = `[^\s\S]`
		o.extended = true
		o.fixed = false
	case len(o.patterns) == 1:
		source = o.patterns[0]
	case o.fixed:
		parts := make([]string, len(o.patterns))
		for i, p := range o.patterns {
			parts[i] = "(?:" + regexp2.Escape(p) + ")"
		}
		source = strings.Join(parts, "|")
		o.fixed = false
		o.extended = true
	case !o.extended && !o.perl:
		source = strings.Join(o.patterns, `\|`)
	default:
		parts := make([]string, len(o.patterns))
		for i, p := range o.patterns {
			parts[i] = "(?:" + p + ")"
		}
		source = strings.Join(parts, "|")
	}
	m, err := compilePattern(source, o, true)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %s", strings.Join(o.patterns, "\n"))
	}
	return m, nil
}

func compilePattern(source string, o options, wrap bool) (*matcher, error) {
	reset := ""
	if o.fixed {
		source = regexp2.Escape(source)
	} else {
		source = posixClasses(source)
		if !o.extended && !o.perl {
			source = basicRegexp(source)
		} else {
			source = strings.ReplaceAll(source, "(?P<", "(?<")
			if o.perl && !o.extended {
				var err error
				source, reset, err = perlRegexp(source)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if wrap && o.word {
		source = `\b(?:` + source + `)\b`
	}
	if wrap && o.line {
		source = "^(?:" + source + ")$"
	}
	flags := regexp2.ECMAScript
	if o.ignoreCase {
		flags |= regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(source, flags, regexp2.OptionMaxBacktrackingStackSize(1<<20))
	if err != nil {
		return nil, err
	}
	if reset != "" {
		// ECMAScript numbers named captures in source order. Exclude our
		// inserted captures when mapping the user's original backreferences.
		numbers := []int{0}
		for _, n := range re.GetGroupNumbers() {
			if n != 0 && re.GroupNameFromNumber(n) != reset {
				numbers = append(numbers, n)
			}
		}
		source = remapBackreferences(source, numbers)
		re, err = regexp2.Compile(source, flags, regexp2.OptionMaxBacktrackingStackSize(1<<20))
		if err != nil {
			return nil, err
		}
	}
	re.MatchTimeout = matchTimeout
	return &matcher{re: re, resetGroup: reset}, nil
}

var posixClass = map[string]string{
	"alpha": "a-zA-Z", "digit": "0-9", "alnum": "a-zA-Z0-9",
	"lower": "a-z", "upper": "A-Z", "xdigit": "0-9A-Fa-f",
	"space": ` \t\n\r\f\v`, "blank": ` \t`, "punct": `!-/:-@\[-` + "`" + `{-~`,
	"graph": "!-~", "print": " -~", "cntrl": `\x00-\x1F\x7F`,
	"ascii": `\x00-\x7F`, "word": "a-zA-Z0-9_",
}

func posixClasses(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "[[:<:]]") || strings.HasPrefix(s[i:], "[[:>:]]") {
			b.WriteString(`\b`)
			i += 7
			continue
		}
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteString(s[i : i+2])
			i += 2
			continue
		}
		if s[i] != '[' {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte('[')
		i++
		if i < len(s) && (s[i] == '^' || s[i] == '!') {
			b.WriteByte('^')
			i++
		}
		if i < len(s) && s[i] == ']' {
			b.WriteString(`\]`)
			i++
		}
		for i < len(s) && s[i] != ']' {
			if strings.HasPrefix(s[i:], "[:") {
				if end := strings.Index(s[i+2:], ":]"); end >= 0 {
					if replacement, ok := posixClass[s[i+2:i+2+end]]; ok {
						b.WriteString(replacement)
						i += end + 4
						continue
					}
				}
			}
			if s[i] == '\\' && i+1 < len(s) {
				b.WriteString(s[i : i+2])
				i += 2
			} else {
				b.WriteByte(s[i])
				i++
			}
		}
		if i < len(s) {
			b.WriteByte(']')
			i++
		}
	}
	return b.String()
}

var basicInterval = regexp.MustCompile(`^\\\{[0-9]+(?:,[0-9]*)?\\\}`)

func basicRegexp(s string) string {
	var b strings.Builder
	start := true
	for i := 0; i < len(s); {
		c := s[i]
		if c == '[' {
			end := i + 1
			for end < len(s) {
				if s[end] == '\\' && end+1 < len(s) {
					end += 2
					continue
				}
				if s[end] == ']' {
					end++
					break
				}
				end++
			}
			b.WriteString(s[i:end])
			i = end
			start = false
			continue
		}
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '|', '(':
				b.WriteByte(next)
				i += 2
				start = true
				continue
			case ')':
				b.WriteByte(next)
				i += 2
				start = false
				continue
			case '{':
				if interval := basicInterval.FindString(s[i:]); interval != "" {
					b.WriteString(strings.ReplaceAll(interval, `\`, ""))
					i += len(interval)
					start = false
					continue
				}
			}
			b.WriteString(s[i : i+2])
			i += 2
			start = false
			continue
		}
		switch c {
		case '*':
			if start {
				b.WriteString(`\*`)
				i++
				continue
			}
		case '^':
			if start {
				b.WriteByte(c)
				i++
				continue
			}
			b.WriteByte('\\')
		case '$':
			if i != len(s)-1 && !strings.HasPrefix(s[i+1:], `\)`) {
				b.WriteByte('\\')
			}
		case '+', '?', '|', '(', ')', '{', '}':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
		i++
		start = false
	}
	return b.String()
}

// Translate the Perl extensions used by just-bash. A zero-width capture records
// the reset position without consuming text, including inside an alternative.
func perlRegexp(s string) (string, string, error) {
	reset := "kefkaMatchReset"
	for strings.Contains(s, reset) {
		reset += "X"
	}
	hasReset := false
	var b strings.Builder
	inClass := false
	var lookarounds []bool
	for i := 0; i < len(s); {
		if !inClass && s[i] == '(' {
			inside := len(lookarounds) > 0 && lookarounds[len(lookarounds)-1]
			inside = inside || strings.HasPrefix(s[i:], "(?=") || strings.HasPrefix(s[i:], "(?!") || strings.HasPrefix(s[i:], "(?<=") || strings.HasPrefix(s[i:], "(?<!")
			lookarounds = append(lookarounds, inside)
		} else if !inClass && s[i] == ')' && len(lookarounds) > 0 {
			lookarounds = lookarounds[:len(lookarounds)-1]
		}
		if s[i] == '[' {
			inClass = true
		} else if s[i] == ']' {
			inClass = false
		}
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch s[i+1] {
		case 'Q':
			i += 2
			end := strings.Index(s[i:], `\E`)
			if end < 0 {
				b.WriteString(regexp2.Escape(s[i:]))
				i = len(s)
			} else {
				b.WriteString(regexp2.Escape(s[i : i+end]))
				i += end + 2
			}
		case 'K':
			if !inClass && len(lookarounds) > 0 && lookarounds[len(lookarounds)-1] {
				return "", "", fmt.Errorf("reset inside lookaround")
			}
			if inClass {
				b.WriteString(`\K`)
			} else {
				b.WriteString("(?<" + reset + ">)")
				hasReset = true
			}
			i += 2
		case 'x':
			if i+2 < len(s) && s[i+2] == '{' {
				end := strings.IndexByte(s[i+3:], '}')
				if end < 0 {
					return "", "", fmt.Errorf("unterminated code point")
				}
				v, err := strconv.ParseUint(s[i+3:i+3+end], 16, 32)
				if err != nil || !utf8.ValidRune(rune(v)) {
					return "", "", fmt.Errorf("invalid code point")
				}
				if !inClass {
					b.WriteString("(?:" + regexp2.Escape(string(rune(v))) + ")")
				} else if v <= 0xffff {
					fmt.Fprintf(&b, `\u%04x`, v)
				} else {
					b.WriteString(regexp2.Escape(string(rune(v))))
				}
				i += end + 4
			} else {
				b.WriteString(s[i : i+2])
				i += 2
			}
		default:
			b.WriteString(s[i : i+2])
			i += 2
		}
	}
	if !hasReset {
		reset = ""
	}
	return b.String(), reset, nil
}

// remapBackreferences keeps numeric references attached to the user's groups.
// Non-reference decimal escapes retain ECMAScript's octal/identity behavior.
func remapBackreferences(s string, numbers []int) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(s); {
		if s[i] == '[' {
			inClass = true
		} else if s[i] == ']' {
			inClass = false
		}
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		if !inClass && strings.HasPrefix(s[i:], `\k<`) {
			if end := strings.IndexByte(s[i+3:], '>'); end >= 0 {
				n, err := strconv.Atoi(s[i+3 : i+3+end])
				if err == nil && n > 0 && n < len(numbers) {
					fmt.Fprintf(&b, `\k<%d>`, numbers[n])
					i += end + 4
					continue
				}
			}
		}
		if inClass || s[i+1] < '1' || s[i+1] > '9' {
			b.WriteString(s[i : i+2])
			i += 2
			continue
		}
		end := i + 1
		for end < len(s) && s[end] >= '0' && s[end] <= '9' {
			end++
		}
		n, err := strconv.Atoi(s[i+1 : end])
		if err == nil && n > 0 && n < len(numbers) {
			fmt.Fprintf(&b, `\k<%d>`, numbers[n])
			i = end
			continue
		}
		// With no original capture, \1 through \7 start an octal escape.
		n, end = 0, i+1
		for end < len(s) && end < i+4 && s[end] >= '0' && s[end] <= '7' && n*8+int(s[end]-'0') <= 255 {
			n = n*8 + int(s[end]-'0')
			end++
		}
		if end > i+1 {
			fmt.Fprintf(&b, `\x%02x`, n)
			i = end
		} else {
			b.WriteByte(s[i+1])
			i += 2
		}
	}
	return b.String()
}
