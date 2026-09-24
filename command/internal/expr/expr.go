package expr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("expr: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	for _, a := range args {
		if a == "--help" {
			printUsage(stderr)
			return nil
		}
	}

	operands := args
	if len(operands) > 0 && operands[0] == "--" {
		operands = operands[1:]
	}

	if len(operands) == 0 {
		fmt.Fprint(stderr, "expr: missing operand\n")
		return interp.ExitStatus(2)
	}

	result, err := evaluate(operands)
	if err != nil {
		fmt.Fprintf(stderr, "expr: %s\n", err)
		return interp.ExitStatus(2)
	}

	fmt.Fprintln(stdout, result)
	if result == "0" || result == "" {
		return interp.ExitStatus(1)
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "Usage: expr EXPRESSION\n")
	fmt.Fprint(w, "  or:  expr OPTION\n")
	fmt.Fprint(w, "Print the value of EXPRESSION to standard output.\n\n")
	fmt.Fprint(w, "      --help    display this help and exit\n")
}

type parser struct {
	args []string
	i    int
}

func evaluate(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	p := &parser{args: args}
	return p.parseOr()
}

func (p *parser) parseOr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) && p.args[p.i] == "|" {
		p.i++
		right, err := p.parseAnd()
		if err != nil {
			return "", err
		}
		if left != "0" && left != "" {
			return left, nil
		}
		left = right
	}
	return left, nil
}

func (p *parser) parseAnd() (string, error) {
	left, err := p.parseComparison()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) && p.args[p.i] == "&" {
		p.i++
		right, err := p.parseComparison()
		if err != nil {
			return "", err
		}
		if left == "0" || left == "" || right == "0" || right == "" {
			left = "0"
		}
	}
	return left, nil
}

func isComparisonOp(s string) bool {
	switch s {
	case "=", "!=", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func (p *parser) parseComparison() (string, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) && isComparisonOp(p.args[p.i]) {
		op := p.args[p.i]
		p.i++
		right, err := p.parseAddSub()
		if err != nil {
			return "", err
		}
		ln, lok := jsParseInt(left)
		rn, rok := jsParseInt(right)
		numeric := lok && rok
		var b bool
		switch op {
		case "=":
			if numeric {
				b = ln == rn
			} else {
				b = left == right
			}
		case "!=":
			if numeric {
				b = ln != rn
			} else {
				b = left != right
			}
		case "<":
			if numeric {
				b = ln < rn
			} else {
				b = left < right
			}
		case ">":
			if numeric {
				b = ln > rn
			} else {
				b = left > right
			}
		case "<=":
			if numeric {
				b = ln <= rn
			} else {
				b = left <= right
			}
		case ">=":
			if numeric {
				b = ln >= rn
			} else {
				b = left >= right
			}
		}
		if b {
			left = "1"
		} else {
			left = "0"
		}
	}
	return left, nil
}

func (p *parser) parseAddSub() (string, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) {
		op := p.args[p.i]
		if op != "+" && op != "-" {
			break
		}
		p.i++
		right, err := p.parseMulDiv()
		if err != nil {
			return "", err
		}
		ln, lok := jsParseInt(left)
		rn, rok := jsParseInt(right)
		if !lok || !rok {
			return "", errors.New("non-integer argument")
		}
		if op == "+" {
			left = strconv.FormatInt(ln+rn, 10)
		} else {
			left = strconv.FormatInt(ln-rn, 10)
		}
	}
	return left, nil
}

func (p *parser) parseMulDiv() (string, error) {
	left, err := p.parseMatch()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) {
		op := p.args[p.i]
		if op != "*" && op != "/" && op != "%" {
			break
		}
		p.i++
		right, err := p.parseMatch()
		if err != nil {
			return "", err
		}
		ln, lok := jsParseInt(left)
		rn, rok := jsParseInt(right)
		if !lok || !rok {
			return "", errors.New("non-integer argument")
		}
		if (op == "/" || op == "%") && rn == 0 {
			return "", errors.New("division by zero")
		}
		switch op {
		case "*":
			left = strconv.FormatInt(ln*rn, 10)
		case "/":
			left = strconv.FormatInt(ln/rn, 10)
		case "%":
			left = strconv.FormatInt(ln%rn, 10)
		}
	}
	return left, nil
}

func (p *parser) parseMatch() (string, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	for p.i < len(p.args) && p.args[p.i] == ":" {
		p.i++
		pattern, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		left, err = matchAnchored(left, pattern)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

func (p *parser) parsePrimary() (string, error) {
	if p.i >= len(p.args) {
		return "", errors.New("syntax error")
	}
	token := p.args[p.i]
	switch token {
	case "match":
		p.i++
		str, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		pattern, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return matchUnanchored(str, pattern)
	case "substr":
		p.i++
		str, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		posStr, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		lenStr, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		pos, posOK := jsParseInt(posStr)
		ln, lnOK := jsParseInt(lenStr)
		if !posOK || !lnOK {
			return "", errors.New("non-integer argument")
		}
		return jsSubstring(str, int(pos)-1, int(pos)-1+int(ln)), nil
	case "index":
		p.i++
		str, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		chars, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		for j, r := range []rune(str) {
			if strings.ContainsRune(chars, r) {
				return strconv.Itoa(j + 1), nil
			}
		}
		return "0", nil
	case "length":
		p.i++
		str, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return strconv.Itoa(utf8.RuneCountInString(str)), nil
	case "(":
		p.i++
		result, err := p.parseOr()
		if err != nil {
			return "", err
		}
		if p.i >= len(p.args) || p.args[p.i] != ")" {
			return "", errors.New("syntax error")
		}
		p.i++
		return result, nil
	}
	p.i++
	return token, nil
}

func matchAnchored(s, pattern string) (string, error) {
	goPattern, err := breToGoRegex(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	re, err := regexp.Compile("^(?:" + goPattern + ")")
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	idx := re.FindStringSubmatchIndex(s)
	if idx == nil {
		// When the pattern contains a capture group and no match
		// occurred, GNU expr prints an empty string with exit 1.
		if re.NumSubexp() > 0 {
			return "", nil
		}
		return "0", nil
	}
	if re.NumSubexp() > 0 {
		if len(idx) >= 4 && idx[2] >= 0 {
			return s[idx[2]:idx[3]], nil
		}
		return "", nil
	}
	return strconv.Itoa(idx[1] - idx[0]), nil
}

func matchUnanchored(s, pattern string) (string, error) {
	goPattern, err := breToGoRegex(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	re, err := regexp.Compile(goPattern)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	idx := re.FindStringSubmatchIndex(s)
	if idx == nil {
		if re.NumSubexp() > 0 {
			return "", nil
		}
		return "0", nil
	}
	if re.NumSubexp() > 0 {
		if len(idx) >= 4 && idx[2] >= 0 {
			return s[idx[2]:idx[3]], nil
		}
		return "", nil
	}
	return strconv.Itoa(idx[1] - idx[0]), nil
}

// breToGoRegex translates a POSIX Basic Regular Expression (BRE) into the
// equivalent Go (RE2) regular expression syntax. BRE differs from Go regex in
// the following ways that this translator handles:
//
//   - `\(` and `\)` denote grouping; `(` and `)` are literal.
//   - `\{n,m\}` denotes an interval; `{` and `}` are literal.
//   - A leading `*` (at the start of the pattern or just after `\(`) is
//     literal, not a quantifier.
//   - `\|`, `\+`, `\?` are not standard BRE alternation/quantifiers and are
//     kept as their literal escaped forms in Go regex.
//   - `\1`..`\9` backreferences are not supported by Go's RE2 engine and
//     surface as a translation error.
//   - `.`, `^`, `$`, and bracket expressions `[...]` retain their meaning.
func breToGoRegex(bre string) (string, error) {
	var out strings.Builder
	atStart := true
	// A "group start" is the position immediately after `\(`, where a
	// leading `*` is also literal per POSIX.
	afterGroupStart := false
	for i := 0; i < len(bre); i++ {
		c := bre[i]
		switch c {
		case '\\':
			if i+1 >= len(bre) {
				// Trailing backslash: keep it as a literal backslash escape.
				out.WriteString(`\\`)
				atStart = false
				afterGroupStart = false
				continue
			}
			next := bre[i+1]
			switch next {
			case '(':
				out.WriteByte('(')
				i++
				atStart = false
				afterGroupStart = true
				continue
			case ')':
				out.WriteByte(')')
				i++
				atStart = false
				afterGroupStart = false
				continue
			case '{':
				// Interval: copy `{...\}` as `{...}`.
				j := i + 2
				out.WriteByte('{')
				for j < len(bre) {
					if bre[j] == '\\' && j+1 < len(bre) && bre[j+1] == '}' {
						out.WriteByte('}')
						j += 2
						break
					}
					out.WriteByte(bre[j])
					j++
				}
				i = j - 1
				atStart = false
				afterGroupStart = false
				continue
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return "", fmt.Errorf("backreferences not supported")
			case '.', '*', '[', ']', '^', '$', '\\':
				// Pass through as escaped literal in Go regex too.
				out.WriteByte('\\')
				out.WriteByte(next)
				i++
				atStart = false
				afterGroupStart = false
				continue
			default:
				// Other `\X` sequences: pass through unchanged. This covers
				// character classes like `\b` etc., which Go regex supports.
				out.WriteByte('\\')
				out.WriteByte(next)
				i++
				atStart = false
				afterGroupStart = false
				continue
			}
		case '(', ')':
			// Literal parens in BRE — escape for Go regex.
			out.WriteByte('\\')
			out.WriteByte(c)
			atStart = false
			afterGroupStart = false
		case '{', '}':
			// Literal braces in BRE — escape for Go regex.
			out.WriteByte('\\')
			out.WriteByte(c)
			atStart = false
			afterGroupStart = false
		case '*':
			if atStart || afterGroupStart {
				// Literal `*` at start of expression or just after `\(`.
				out.WriteString(`\*`)
			} else {
				out.WriteByte('*')
			}
			atStart = false
			afterGroupStart = false
		case '[':
			// Copy bracket expression verbatim. Handle a leading `]`
			// (which is a literal in POSIX brackets) and a leading `^`.
			out.WriteByte('[')
			j := i + 1
			if j < len(bre) && bre[j] == '^' {
				out.WriteByte('^')
				j++
			}
			if j < len(bre) && bre[j] == ']' {
				out.WriteByte(']')
				j++
			}
			for j < len(bre) && bre[j] != ']' {
				out.WriteByte(bre[j])
				j++
			}
			if j < len(bre) {
				out.WriteByte(']')
			}
			i = j
			atStart = false
			afterGroupStart = false
		default:
			out.WriteByte(c)
			atStart = false
			afterGroupStart = false
		}
	}
	return out.String(), nil
}

// jsParseInt mimics JavaScript's parseInt(s, 10): skip leading whitespace,
// optional sign, read decimal digits, stop at the first non-digit. Returns
// ok=false when no digits are read.
func jsParseInt(s string) (int64, bool) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == '\f' || s[i] == '\v') {
		i++
	}
	sign := int64(1)
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	start := i
	var n int64
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int64(s[i]-'0')
		i++
	}
	if i == start {
		return 0, false
	}
	return sign * n, true
}

// jsSubstring mimics String.prototype.substring(start, end): negative
// arguments are clamped to 0, indices past the rune length clamp to the
// length, and the bounds are swapped if start > end.
func jsSubstring(s string, start, end int) string {
	runes := []rune(s)
	n := len(runes)
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if start > n {
		start = n
	}
	if end > n {
		end = n
	}
	if start > end {
		start, end = end, start
	}
	return string(runes[start:end])
}
