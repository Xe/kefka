package printf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("printf: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	if slices.Contains(args, "--help") {
		printHelp(stderr)
		return nil
	}

	if len(args) == 0 {
		fmt.Fprint(stderr, "printf: usage: printf format [arguments]\n")
		return interp.ExitStatus(2)
	}

	var targetVar string
	hasTargetVar := false
	argIdx := 0

	for argIdx < len(args) {
		arg := args[argIdx]
		if arg == "--" {
			argIdx++
			break
		}
		if arg == "-v" {
			if argIdx+1 >= len(args) {
				fmt.Fprint(stderr, "printf: -v: option requires an argument\n")
				return interp.ExitStatus(1)
			}
			targetVar = args[argIdx+1]
			hasTargetVar = true
			if !validIdentifier(targetVar) {
				fmt.Fprintf(stderr, "printf: `%s': not a valid identifier\n", targetVar)
				return interp.ExitStatus(2)
			}
			argIdx += 2
			continue
		}
		break
	}

	if argIdx >= len(args) {
		fmt.Fprint(stderr, "printf: usage: printf format [arguments]\n")
		return interp.ExitStatus(1)
	}

	format := args[argIdx]
	formatArgs := args[argIdx+1:]
	processed := processEscapes(format)

	tz := ""
	if ec.Environ != nil {
		if v := ec.Environ.Get("TZ"); v.IsSet() {
			tz = v.String()
		}
	}

	var output strings.Builder
	var errMsg string
	hadError := false
	argPos := 0

	for {
		out, consumed, gotErr, gotMsg, stopped := formatOnce(processed, formatArgs, argPos, tz)
		output.WriteString(out)
		argPos += consumed
		if gotErr {
			hadError = true
			if gotMsg != "" {
				errMsg = gotMsg
			}
		}
		if stopped {
			break
		}
		if consumed == 0 || argPos >= len(formatArgs) {
			break
		}
	}

	if errMsg != "" {
		fmt.Fprint(stderr, errMsg)
	}

	if hasTargetVar {
		if err := assignVar(ec.Environ, targetVar, output.String()); err != nil {
			fmt.Fprintf(stderr, "printf: %s\n", err)
			return interp.ExitStatus(1)
		}
	} else {
		io.WriteString(stdout, output.String())
	}

	if hadError {
		return interp.ExitStatus(1)
	}
	return nil
}

func printHelp(w io.Writer) {
	io.WriteString(w, "Usage: printf [-v var] FORMAT [ARGUMENT...]\n")
	io.WriteString(w, "Format and print data.\n\n")
	io.WriteString(w, "    -v var     assign the output to shell variable VAR rather than display it\n")
	io.WriteString(w, "    --help     display this help and exit\n\n")
	io.WriteString(w, "FORMAT controls the output like in C printf.\n")
	io.WriteString(w, "Escape sequences: \\n (newline), \\t (tab), \\\\ (backslash)\n")
	io.WriteString(w, "Format specifiers: %s (string), %d (integer), %f (float), %x (hex), %o (octal), %% (literal %)\n")
	io.WriteString(w, "Width and precision: %10s (width 10), %.2f (2 decimal places), %010d (zero-padded)\n")
	io.WriteString(w, "Flags: %- (left-justify), %+ (show sign), %0 (zero-pad)\n")
}

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\[[a-zA-Z0-9_@*"'$]+\])?$`)

func validIdentifier(s string) bool {
	return identifierRe.MatchString(s)
}

var dollarVarRe = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// parseArraySubscript matches name[key], name['key'], or name["key"]. Go's RE2
// does not support the backreference the original TS regex used, so this is a
// manual parse.
func parseArraySubscript(s string) (name, key string, ok bool) {
	open := strings.IndexByte(s, '[')
	if open < 1 || !strings.HasSuffix(s, "]") {
		return "", "", false
	}
	name = s[:open]
	inner := s[open+1 : len(s)-1]
	if len(inner) >= 2 {
		first, last := inner[0], inner[len(inner)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			inner = inner[1 : len(inner)-1]
		}
	}
	return name, inner, true
}

func assignVar(env expand.Environ, name, value string) error {
	we, ok := env.(expand.WriteEnviron)
	if !ok {
		return errors.New("cannot assign: environment is read-only")
	}
	if arrayName, key, ok := parseArraySubscript(name); ok {
		key = dollarVarRe.ReplaceAllStringFunc(key, func(s string) string {
			varName := s[1:]
			if v := env.Get(varName); v.IsSet() {
				return v.String()
			}
			return ""
		})
		return we.Set(arrayName+"_"+key, expand.Variable{Set: true, Kind: expand.String, Str: value})
	}
	return we.Set(name, expand.Variable{Set: true, Kind: expand.String, Str: value})
}

var strftimeSpecRe = regexp.MustCompile(`^%(-?\d*)(?:\.(\d+))?\(([^)]*)\)T`)

func formatOnce(format string, args []string, argPos int, tz string) (string, int, bool, string, bool) {
	var result strings.Builder
	consumed := 0
	hadError := false
	errMsg := ""

	i := 0
	for i < len(format) {
		if format[i] != '%' || i+1 >= len(format) {
			result.WriteByte(format[i])
			i++
			continue
		}

		specStart := i
		i++ // skip %

		if format[i] == '%' {
			result.WriteByte('%')
			i++
			continue
		}

		if m := strftimeSpecRe.FindStringSubmatch(format[specStart:]); m != nil {
			width := 0
			if m[1] != "" {
				width, _ = strconv.Atoi(m[1])
			}
			precision := -1
			if m[2] != "" {
				precision, _ = strconv.Atoi(m[2])
			}
			strftimeFmt := m[3]
			fullMatch := m[0]

			arg := ""
			if argPos+consumed < len(args) {
				arg = args[argPos+consumed]
			}
			consumed++

			var ts time.Time
			if arg == "" || arg == "-1" || arg == "-2" {
				ts = time.Now()
			} else if n, err := strconv.ParseInt(arg, 10, 64); err == nil {
				ts = time.Unix(n, 0)
			} else {
				ts = time.Unix(0, 0)
			}

			formatted := formatStrftime(strftimeFmt, ts, tz)
			if precision >= 0 && len(formatted) > precision {
				formatted = formatted[:precision]
			}
			if width != 0 {
				abs := width
				if abs < 0 {
					abs = -abs
				}
				if len(formatted) < abs {
					if width < 0 {
						formatted = padRight(formatted, abs, ' ')
					} else {
						formatted = padLeft(formatted, abs, ' ')
					}
				}
			}
			result.WriteString(formatted)
			i = specStart + len(fullMatch)
			continue
		}

		// Flags
		for i < len(format) && strings.ContainsRune("+-0 #'", rune(format[i])) {
			i++
		}

		// Width (* or digits)
		widthFromArg := false
		if i < len(format) && format[i] == '*' {
			widthFromArg = true
			i++
		} else {
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}

		// Precision (. then * or digits)
		precisionFromArg := false
		if i < len(format) && format[i] == '.' {
			i++
			if i < len(format) && format[i] == '*' {
				precisionFromArg = true
				i++
			} else {
				for i < len(format) && format[i] >= '0' && format[i] <= '9' {
					i++
				}
			}
		}

		// Length modifier
		if i < len(format) && strings.ContainsRune("hlL", rune(format[i])) {
			i++
		}

		if i >= len(format) {
			result.WriteString(format[specStart:])
			break
		}

		specifier := format[i]
		i++

		fullSpec := format[specStart:i]
		adjustedSpec := fullSpec

		if widthFromArg {
			w := 0
			if argPos+consumed < len(args) {
				w, _ = strconv.Atoi(args[argPos+consumed])
			}
			consumed++
			adjustedSpec = strings.Replace(adjustedSpec, "*", strconv.Itoa(w), 1)
		}
		if precisionFromArg {
			p := 0
			if argPos+consumed < len(args) {
				p, _ = strconv.Atoi(args[argPos+consumed])
			}
			consumed++
			adjustedSpec = strings.Replace(adjustedSpec, ".*", "."+strconv.Itoa(p), 1)
		}

		arg := ""
		if argPos+consumed < len(args) {
			arg = args[argPos+consumed]
		}
		consumed++

		val, gotErr, gotMsg, stopped := formatValue(adjustedSpec, specifier, arg)
		result.WriteString(val)
		if gotErr {
			hadError = true
			if gotMsg != "" {
				errMsg = gotMsg
			}
		}
		if stopped {
			return result.String(), consumed, hadError, errMsg, true
		}
	}

	return result.String(), consumed, hadError, errMsg, false
}

var (
	intSpecRe   = regexp.MustCompile(`^%([- +#0']*)(\d*)(\.(\d*))?[diu]$`)
	octalSpecRe = regexp.MustCompile(`^%([- +#0']*)(\d*)(\.(\d*))?o$`)
	hexSpecRe   = regexp.MustCompile(`^%([- +#0']*)(\d*)(\.(\d*))?[xX]$`)
	floatSpecRe = regexp.MustCompile(`^%([- +#0']*)(\d*)(\.(\d*))?[eEfFgG]$`)
	stringSpecRe = regexp.MustCompile(`^%(-?)(\d*)(\.(\d*))?s$`)
	quotedSpecRe = regexp.MustCompile(`^%(-?)(\d*)q$`)
)

func formatValue(spec string, specifier byte, arg string) (string, bool, string, bool) {
	switch specifier {
	case 'd', 'i':
		num, parseErr := parseIntArg(arg)
		var msg string
		if parseErr {
			msg = fmt.Sprintf("printf: %s: invalid number\n", arg)
		}
		return formatInteger(spec, num), parseErr, msg, false
	case 'o':
		num, parseErr := parseIntArg(arg)
		var msg string
		if parseErr {
			msg = fmt.Sprintf("printf: %s: invalid number\n", arg)
		}
		return formatOctal(spec, num), parseErr, msg, false
	case 'u':
		num, parseErr := parseIntArg(arg)
		var msg string
		if parseErr {
			msg = fmt.Sprintf("printf: %s: invalid number\n", arg)
		}
		unsigned := num
		if num < 0 {
			unsigned = int64(uint32(num))
		}
		return formatInteger(strings.Replace(spec, "u", "d", 1), unsigned), parseErr, msg, false
	case 'x', 'X':
		num, parseErr := parseIntArg(arg)
		var msg string
		if parseErr {
			msg = fmt.Sprintf("printf: %s: invalid number\n", arg)
		}
		return formatHex(spec, num), parseErr, msg, false
	case 'e', 'E', 'f', 'F', 'g', 'G':
		num, _ := strconv.ParseFloat(arg, 64)
		return formatFloat(spec, specifier, num), false, "", false
	case 'c':
		if arg == "" {
			return "", false, "", false
		}
		return string(rune(arg[0])), false, "", false
	case 's':
		return formatString(spec, arg), false, "", false
	case 'q':
		return formatQuoted(spec, arg), false, "", false
	case 'b':
		val, stopped := processBEscapes(arg)
		return val, false, "", stopped
	default:
		return "", true, fmt.Sprintf("printf: %%%c: invalid directive\n", specifier), false
	}
}

func parseIntArg(arg string) (int64, bool) {
	trimmed := strings.TrimLeft(arg, " \t\n\r\v\f")
	hasTrailing := trimmed != strings.TrimRight(trimmed, " \t\n\r\v\f")
	arg = strings.TrimRight(trimmed, " \t\n\r\v\f")

	// Character notation: 'x' or "x" or \'x or \"x
	if strings.HasPrefix(arg, "\\'") && len(arg) >= 3 {
		r, _ := utf8.DecodeRuneInString(arg[2:])
		return int64(r), false
	}
	if strings.HasPrefix(arg, "\\\"") && len(arg) >= 3 {
		r, _ := utf8.DecodeRuneInString(arg[2:])
		return int64(r), false
	}
	if strings.HasPrefix(arg, "'") && len(arg) >= 2 {
		r, _ := utf8.DecodeRuneInString(arg[1:])
		return int64(r), false
	}
	if strings.HasPrefix(arg, "\"") && len(arg) >= 2 {
		r, _ := utf8.DecodeRuneInString(arg[1:])
		return int64(r), false
	}

	if arg == "" {
		return 0, false
	}

	arg = strings.TrimPrefix(arg, "+")

	if strings.HasPrefix(arg, "0x") || strings.HasPrefix(arg, "0X") {
		n, err := strconv.ParseInt(arg[2:], 16, 64)
		if err != nil {
			return 0, true
		}
		return n, hasTrailing
	}
	if strings.HasPrefix(arg, "-0x") || strings.HasPrefix(arg, "-0X") {
		n, err := strconv.ParseInt(arg[3:], 16, 64)
		if err != nil {
			return 0, true
		}
		return -n, hasTrailing
	}

	// Octal: leading 0 followed by octal digits
	if octalNumRe.MatchString(arg) {
		n, _ := strconv.ParseInt(arg, 8, 64)
		return n, hasTrailing
	}

	// Reject base notation like 64#a
	if m := baseNotationRe.FindStringSubmatch(arg); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		return n, true
	}

	if !decimalRe.MatchString(arg) {
		// Try to parse what we can (bash behavior: 3abc -> 3)
		n, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			n = bestEffortInt(arg)
		}
		return n, true
	}

	n, _ := strconv.ParseInt(arg, 10, 64)
	return n, hasTrailing
}

var (
	octalNumRe     = regexp.MustCompile(`^-?0[0-7]+$`)
	baseNotationRe = regexp.MustCompile(`^(\d+)#`)
	decimalRe      = regexp.MustCompile(`^-?\d+$`)
	leadingIntRe   = regexp.MustCompile(`^-?\d+`)
)

func bestEffortInt(s string) int64 {
	m := leadingIntRe.FindString(s)
	if m == "" {
		return 0
	}
	n, _ := strconv.ParseInt(m, 10, 64)
	return n
}

func formatInteger(spec string, num int64) string {
	m := intSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return fmt.Sprintf("%d", num)
	}
	flags := m[1]
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	precision := -1
	if m[3] != "" {
		if m[4] == "" {
			precision = 0
		} else {
			precision, _ = strconv.Atoi(m[4])
		}
	}

	negative := num < 0
	abs := num
	if negative {
		abs = -num
	}
	numStr := strconv.FormatInt(abs, 10)

	if precision >= 0 {
		numStr = padLeft(numStr, precision, '0')
	}

	sign := ""
	switch {
	case negative:
		sign = "-"
	case strings.Contains(flags, "+"):
		sign = "+"
	case strings.Contains(flags, " "):
		sign = " "
	}

	result := sign + numStr
	if width > len(result) {
		switch {
		case strings.Contains(flags, "-"):
			result = padRight(result, width, ' ')
		case strings.Contains(flags, "0") && precision < 0:
			result = sign + padLeft(numStr, width-len(sign), '0')
		default:
			result = padLeft(result, width, ' ')
		}
	}
	return result
}

func formatOctal(spec string, num int64) string {
	m := octalSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return strconv.FormatInt(num, 8)
	}
	flags := m[1]
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	precision := -1
	if m[3] != "" {
		if m[4] == "" {
			precision = 0
		} else {
			precision, _ = strconv.Atoi(m[4])
		}
	}

	abs := num
	if abs < 0 {
		abs = -abs
	}
	numStr := strconv.FormatInt(abs, 8)

	if precision >= 0 {
		numStr = padLeft(numStr, precision, '0')
	}
	if strings.Contains(flags, "#") && !strings.HasPrefix(numStr, "0") {
		numStr = "0" + numStr
	}

	result := numStr
	if width > len(result) {
		switch {
		case strings.Contains(flags, "-"):
			result = padRight(result, width, ' ')
		case strings.Contains(flags, "0") && precision < 0:
			result = padLeft(result, width, '0')
		default:
			result = padLeft(result, width, ' ')
		}
	}
	return result
}

func formatHex(spec string, num int64) string {
	m := hexSpecRe.FindStringSubmatch(spec)
	upper := strings.Contains(spec, "X")
	if m == nil {
		if upper {
			return strings.ToUpper(strconv.FormatInt(num, 16))
		}
		return strconv.FormatInt(num, 16)
	}
	flags := m[1]
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	precision := -1
	if m[3] != "" {
		if m[4] == "" {
			precision = 0
		} else {
			precision, _ = strconv.Atoi(m[4])
		}
	}

	abs := num
	if abs < 0 {
		abs = -abs
	}
	numStr := strconv.FormatInt(abs, 16)
	if upper {
		numStr = strings.ToUpper(numStr)
	}
	if precision >= 0 {
		numStr = padLeft(numStr, precision, '0')
	}

	prefix := ""
	if strings.Contains(flags, "#") && num != 0 {
		if upper {
			prefix = "0X"
		} else {
			prefix = "0x"
		}
	}

	result := prefix + numStr
	if width > len(result) {
		switch {
		case strings.Contains(flags, "-"):
			result = padRight(result, width, ' ')
		case strings.Contains(flags, "0") && precision < 0:
			result = prefix + padLeft(numStr, width-len(prefix), '0')
		default:
			result = padLeft(result, width, ' ')
		}
	}
	return result
}

func formatFloat(spec string, specifier byte, num float64) string {
	m := floatSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return strconv.FormatFloat(num, byte(specifier), 6, 64)
	}
	flags := m[1]
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	precision := 6
	if m[3] != "" {
		if m[4] == "" {
			precision = 0
		} else {
			precision, _ = strconv.Atoi(m[4])
		}
	}

	var result string
	lower := specifier
	if lower >= 'A' && lower <= 'Z' {
		lower += 32
	}

	switch lower {
	case 'e':
		result = strconv.FormatFloat(num, 'e', precision, 64)
		result = ensureExponentTwoDigits(result)
		if specifier == 'E' {
			result = strings.ToUpper(result)
		}
	case 'f':
		result = strconv.FormatFloat(num, 'f', precision, 64)
		if strings.Contains(flags, "#") && precision == 0 && !strings.Contains(result, ".") {
			result += "."
		}
	case 'g':
		p := precision
		if p == 0 {
			p = 1
		}
		result = strconv.FormatFloat(num, 'g', p, 64)
		if !strings.Contains(flags, "#") {
			// Go's %g already trims trailing zeros, no-op
		}
		result = ensureExponentTwoDigits(result)
		if specifier == 'G' {
			result = strings.ToUpper(result)
		}
	default:
		result = strconv.FormatFloat(num, 'g', -1, 64)
	}

	if num >= 0 && !math.IsNaN(num) {
		switch {
		case strings.Contains(flags, "+"):
			result = "+" + result
		case strings.Contains(flags, " "):
			result = " " + result
		}
	}

	if width > len(result) {
		switch {
		case strings.Contains(flags, "-"):
			result = padRight(result, width, ' ')
		case strings.Contains(flags, "0"):
			signPrefix := ""
			rest := result
			if len(result) > 0 && (result[0] == '+' || result[0] == '-' || result[0] == ' ') {
				signPrefix = result[:1]
				rest = result[1:]
			}
			result = signPrefix + padLeft(rest, width-len(signPrefix), '0')
		default:
			result = padLeft(result, width, ' ')
		}
	}
	return result
}

var expDigitRe = regexp.MustCompile(`e([+-])(\d)$`)

func ensureExponentTwoDigits(s string) string {
	return expDigitRe.ReplaceAllString(s, "e${1}0${2}")
}

func formatString(spec string, str string) string {
	m := stringSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return str
	}
	leftJustify := m[1] == "-"
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	precision := -1
	if m[3] != "" {
		if m[4] == "" {
			precision = 0
		} else {
			precision, _ = strconv.Atoi(m[4])
		}
	}

	if precision >= 0 && len(str) > precision {
		str = str[:precision]
	}
	if width > len(str) {
		if leftJustify {
			str = padRight(str, width, ' ')
		} else {
			str = padLeft(str, width, ' ')
		}
	}
	return str
}

func formatQuoted(spec string, str string) string {
	quoted := shellQuote(str)
	m := quotedSpecRe.FindStringSubmatch(spec)
	if m == nil {
		return quoted
	}
	leftJustify := m[1] == "-"
	width := 0
	if m[2] != "" {
		width, _ = strconv.Atoi(m[2])
	}
	if width > len(quoted) {
		if leftJustify {
			return padRight(quoted, width, ' ')
		}
		return padLeft(quoted, width, ' ')
	}
	return quoted
}

var safeQuoteRe = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if safeQuoteRe.MatchString(s) {
		return s
	}

	needsDollar := false
	for _, c := range s {
		if c < 0x20 || (c >= 0x7f && c <= 0xff) {
			needsDollar = true
			break
		}
	}

	var b strings.Builder
	if needsDollar {
		b.WriteString("$'")
		for _, c := range s {
			switch {
			case c == '\'':
				b.WriteString("\\'")
			case c == '\\':
				b.WriteString("\\\\")
			case c == '\n':
				b.WriteString("\\n")
			case c == '\t':
				b.WriteString("\\t")
			case c == '\r':
				b.WriteString("\\r")
			case c == 0x07:
				b.WriteString("\\a")
			case c == '\b':
				b.WriteString("\\b")
			case c == '\f':
				b.WriteString("\\f")
			case c == '\v':
				b.WriteString("\\v")
			case c == 0x1b:
				b.WriteString("\\E")
			case c < 0x20 || (c >= 0x7f && c <= 0xff):
				fmt.Fprintf(&b, "\\%03o", c)
			case c == '"':
				b.WriteString("\\\"")
			default:
				b.WriteRune(c)
			}
		}
		b.WriteByte('\'')
		return b.String()
	}

	specials := " \t|&;<>()$`\\\"'*?[#~=%!{}"
	for _, c := range s {
		if strings.ContainsRune(specials, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// processEscapes interprets backslash escapes in a printf format string.
// Supports \n \t \r \\ \a \b \f \v \e \E \NNN \xHH \uHHHH \UHHHHHHHH.
func processEscapes(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		next := s[i+1]
		switch next {
		case 'n':
			b.WriteByte('\n')
			i += 2
		case 't':
			b.WriteByte('\t')
			i += 2
		case 'r':
			b.WriteByte('\r')
			i += 2
		case '\\':
			b.WriteByte('\\')
			i += 2
		case 'a':
			b.WriteByte(0x07)
			i += 2
		case 'b':
			b.WriteByte('\b')
			i += 2
		case 'f':
			b.WriteByte('\f')
			i += 2
		case 'v':
			b.WriteByte('\v')
			i += 2
		case 'e', 'E':
			b.WriteByte(0x1b)
			i += 2
		case '0', '1', '2', '3', '4', '5', '6', '7':
			oct, end := readOctalEscape(s, i+1, 3)
			b.WriteByte(byte(oct))
			i = end
		case 'x':
			bytes := []byte{}
			j := i
			for j+1 < len(s) && s[j] == '\\' && s[j+1] == 'x' {
				hex := ""
				k := j + 2
				for k < len(s) && k < j+4 && isHex(s[k]) {
					hex += string(s[k])
					k++
				}
				if hex == "" {
					break
				}
				v, _ := strconv.ParseInt(hex, 16, 32)
				bytes = append(bytes, byte(v))
				j = k
			}
			if len(bytes) > 0 {
				if utf8.Valid(bytes) {
					b.Write(bytes)
				} else {
					for _, c := range bytes {
						b.WriteRune(rune(c))
					}
				}
				i = j
			} else {
				b.WriteByte(s[i])
				i++
			}
		case 'u':
			hex, end := readHexEscape(s, i+2, 4)
			if hex != "" {
				v, _ := strconv.ParseInt(hex, 16, 32)
				b.WriteRune(rune(v))
				i = end
			} else {
				b.WriteString("\\u")
				i += 2
			}
		case 'U':
			hex, end := readHexEscape(s, i+2, 8)
			if hex != "" {
				v, _ := strconv.ParseInt(hex, 16, 32)
				b.WriteRune(rune(v))
				i = end
			} else {
				b.WriteString("\\U")
				i += 2
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func readOctalEscape(s string, start, max int) (int, int) {
	var b strings.Builder
	j := start
	for j < len(s) && j < start+max && s[j] >= '0' && s[j] <= '7' {
		b.WriteByte(s[j])
		j++
	}
	if b.Len() == 0 {
		return 0, start
	}
	v, _ := strconv.ParseInt(b.String(), 8, 32)
	return int(v), j
}

func readHexEscape(s string, start, max int) (string, int) {
	var b strings.Builder
	j := start
	for j < len(s) && j < start+max && isHex(s[j]) {
		b.WriteByte(s[j])
		j++
	}
	return b.String(), j
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// processBEscapes interprets escapes in a %b argument. Returns (value, stopped)
// where stopped=true means \c was encountered and output should halt.
func processBEscapes(s string) (string, bool) {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		next := s[i+1]
		switch next {
		case 'n':
			b.WriteByte('\n')
			i += 2
		case 't':
			b.WriteByte('\t')
			i += 2
		case 'r':
			b.WriteByte('\r')
			i += 2
		case '\\':
			b.WriteByte('\\')
			i += 2
		case 'a':
			b.WriteByte(0x07)
			i += 2
		case 'b':
			b.WriteByte('\b')
			i += 2
		case 'f':
			b.WriteByte('\f')
			i += 2
		case 'v':
			b.WriteByte('\v')
			i += 2
		case 'c':
			return b.String(), true
		case 'x':
			bytes := []byte{}
			j := i
			for j+1 < len(s) && s[j] == '\\' && s[j+1] == 'x' {
				hex, k := readHexEscape(s, j+2, 2)
				if hex == "" {
					break
				}
				v, _ := strconv.ParseInt(hex, 16, 32)
				bytes = append(bytes, byte(v))
				j = k
			}
			if len(bytes) > 0 {
				if utf8.Valid(bytes) {
					b.Write(bytes)
				} else {
					for _, c := range bytes {
						b.WriteRune(rune(c))
					}
				}
				i = j
			} else {
				b.WriteString("\\x")
				i += 2
			}
		case 'u':
			hex, end := readHexEscape(s, i+2, 4)
			if hex != "" {
				v, _ := strconv.ParseInt(hex, 16, 32)
				b.WriteRune(rune(v))
				i = end
			} else {
				b.WriteString("\\u")
				i += 2
			}
		case '0':
			oct, end := readOctalEscape(s, i+2, 3)
			if end > i+2 {
				b.WriteByte(byte(oct))
				i = end
			} else {
				b.WriteByte(0)
				i += 2
			}
		case '1', '2', '3', '4', '5', '6', '7':
			oct, end := readOctalEscape(s, i+1, 3)
			b.WriteByte(byte(oct))
			i = end
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String(), false
}

func padLeft(s string, width int, pad byte) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(string(pad), width-len(s)) + s
}

func padRight(s string, width int, pad byte) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(string(pad), width-len(s))
}

// formatStrftime formats a Unix timestamp using a strftime-style format string.
// Mirrors the date command's UTC-only semantics; tz is accepted but ignored.
func formatStrftime(format string, t time.Time, tz string) string {
	t = t.UTC()
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			t = t.In(loc)
		}
	}

	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case 'a':
			b.WriteString(t.Weekday().String()[:3])
		case 'A':
			b.WriteString(t.Weekday().String())
		case 'b', 'h':
			b.WriteString(t.Month().String()[:3])
		case 'B':
			b.WriteString(t.Month().String())
		case 'c':
			fmt.Fprintf(&b, "%s %s %2d %02d:%02d:%02d %d",
				t.Weekday().String()[:3], t.Month().String()[:3],
				t.Day(), t.Hour(), t.Minute(), t.Second(), t.Year())
		case 'C':
			fmt.Fprintf(&b, "%02d", t.Year()/100)
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'D':
			fmt.Fprintf(&b, "%02d/%02d/%02d", int(t.Month()), t.Day(), t.Year()%100)
		case 'e':
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'F':
			fmt.Fprintf(&b, "%d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d", h)
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'k':
			fmt.Fprintf(&b, "%2d", t.Hour())
		case 'l':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%2d", h)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'n':
			b.WriteByte('\n')
		case 'N':
			b.WriteString("000000000")
		case 'p':
			if t.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'P':
			if t.Hour() < 12 {
				b.WriteString("am")
			} else {
				b.WriteString("pm")
			}
		case 'r':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			ap := "AM"
			if t.Hour() >= 12 {
				ap = "PM"
			}
			fmt.Fprintf(&b, "%02d:%02d:%02d %s", h, t.Minute(), t.Second(), ap)
		case 'R':
			fmt.Fprintf(&b, "%02d:%02d", t.Hour(), t.Minute())
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 't':
			b.WriteByte('\t')
		case 'T', 'X':
			fmt.Fprintf(&b, "%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		case 'u':
			w := int(t.Weekday())
			if w == 0 {
				w = 7
			}
			fmt.Fprintf(&b, "%d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'x':
			fmt.Fprintf(&b, "%02d/%02d/%02d", int(t.Month()), t.Day(), t.Year()%100)
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'Y':
			fmt.Fprintf(&b, "%d", t.Year())
		case 'z':
			_, off := t.Zone()
			sign := "+"
			if off < 0 {
				sign = "-"
				off = -off
			}
			fmt.Fprintf(&b, "%s%02d%02d", sign, off/3600, (off%3600)/60)
		case 'Z':
			name, _ := t.Zone()
			b.WriteString(name)
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}