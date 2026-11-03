package find

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// maxFormatBytes caps -printf width and precision so that a format such
// as "%999999999999f" fails instead of trying to allocate that much.
const maxFormatBytes = 10 * 1024 * 1024

var errFormatLimit = fmt.Errorf("format width limit exceeded (%d bytes)", maxFormatBytes)

// printfData is what -printf directives can show about one entry.
type printfData struct {
	path          string
	name          string
	size          int64
	mtime         time.Time
	mode          fs.FileMode
	isDir         bool
	depth         int
	startingPoint string
}

// formatPrintf expands a -printf format. Escapes are expanded first, then
// directives:
//
//	%f basename   %h dirname   %p path   %P path without starting point
//	%s size       %d depth     %m octal mode   %M symbolic mode
//	%t ctime-style mtime       %Tk mtime field k (@ Y m d H M S T F)
//	%% literal %
//
// Every directive takes an optional width and precision, as in %-20.10f.
func formatPrintf(format string, d printfData) (string, error) {
	s := processEscapes(format)
	var out strings.Builder

	i := 0
	for i < len(s) {
		if s[i] != '%' || i+1 >= len(s) {
			out.WriteByte(s[i])
			i++
			continue
		}
		start := i
		i++
		if s[i] == '%' {
			out.WriteByte('%')
			i++
			continue
		}

		width, precision, consumed, err := parseWidthPrecision(s[i:])
		if err != nil {
			return "", err
		}
		i += consumed
		if i >= len(s) {
			out.WriteByte('%')
			break
		}

		directive, size := utf8.DecodeRuneInString(s[i:])
		i += size

		var value string
		switch directive {
		case 'f':
			value = d.name
		case 'h':
			if idx := strings.LastIndexByte(d.path, '/'); idx > 0 {
				value = d.path[:idx]
			} else {
				value = "."
			}
		case 'p':
			value = d.path
		case 'P':
			sp := d.startingPoint
			switch {
			case d.path == sp:
				value = ""
			case strings.HasPrefix(d.path, sp+"/"):
				value = d.path[len(sp)+1:]
			case sp == "." && strings.HasPrefix(d.path, "./"):
				value = d.path[2:]
			default:
				value = d.path
			}
		case 's':
			value = strconv.FormatInt(d.size, 10)
		case 'd':
			value = strconv.Itoa(d.depth)
		case 'm':
			value = strconv.FormatUint(uint64(d.mode.Perm()), 8)
		case 'M':
			value = formatMode(d.mode, d.isDir)
		case 't':
			value = d.mtime.Local().Format("Mon Jan _2 15:04:05 2006")
		case 'T':
			if i >= len(s) {
				value = "%T"
				break
			}
			field, n := utf8.DecodeRuneInString(s[i:])
			i += n
			value = formatTimeField(d.mtime.Local(), field)
		default:
			// Unknown directive: keep the whole spec as written.
			out.WriteString(s[start:i])
			continue
		}

		padded, err := applyWidth(value, width, precision)
		if err != nil {
			return "", err
		}
		out.WriteString(padded)
	}

	return out.String(), nil
}

// parseWidthPrecision reads an optional "-", width, and ".precision".
// Width is negative for left-justify; precision is -1 when absent.
func parseWidthPrecision(s string) (width, precision, consumed int, err error) {
	i := 0
	precision = -1
	left := false
	if i < len(s) && s[i] == '-' {
		left = true
		i++
	}

	digits := func() (int, error) {
		n := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int(s[i]-'0')
			if n > maxFormatBytes {
				return 0, errFormatLimit
			}
			i++
		}
		return n, nil
	}

	if width, err = digits(); err != nil {
		return 0, 0, 0, err
	}
	if i < len(s) && s[i] == '.' {
		i++
		if precision, err = digits(); err != nil {
			return 0, 0, 0, err
		}
	}
	if left {
		width = -width
	}
	return width, precision, i, nil
}

// applyWidth truncates value to precision runes, then pads it with spaces
// to |width| runes: on the left for positive width, on the right for
// negative width.
func applyWidth(value string, width, precision int) (string, error) {
	if precision >= 0 && utf8.RuneCountInString(value) > precision {
		runes := []rune(value)
		value = string(runes[:precision])
	}
	abs := width
	if abs < 0 {
		abs = -abs
	}
	n := utf8.RuneCountInString(value)
	if abs <= n {
		return value, nil
	}
	if len(value)+abs-n > maxFormatBytes {
		return "", errFormatLimit
	}
	pad := strings.Repeat(" ", abs-n)
	if width < 0 {
		return value + pad, nil
	}
	return pad + value, nil
}

func formatMode(mode fs.FileMode, isDir bool) string {
	const rwx = "rwxrwxrwx"
	var b [10]byte
	b[0] = '-'
	if isDir {
		b[0] = 'd'
	}
	perm := mode.Perm()
	for i := range 9 {
		if perm&(1<<uint(8-i)) != 0 {
			b[i+1] = rwx[i]
		} else {
			b[i+1] = '-'
		}
	}
	return string(b[:])
}

func formatTimeField(t time.Time, field rune) string {
	switch field {
	case '@':
		return strconv.FormatFloat(float64(t.UnixMilli())/1000, 'f', -1, 64)
	case 'Y':
		return strconv.Itoa(t.Year())
	case 'm':
		return fmt.Sprintf("%02d", int(t.Month()))
	case 'd':
		return fmt.Sprintf("%02d", t.Day())
	case 'H':
		return fmt.Sprintf("%02d", t.Hour())
	case 'M':
		return fmt.Sprintf("%02d", t.Minute())
	case 'S':
		return fmt.Sprintf("%02d", t.Second())
	case 'T':
		return t.Format("15:04:05")
	case 'F':
		return t.Format("2006-01-02")
	}
	return "%T" + string(field)
}

// processEscapes expands backslash escapes: \n \t \r \\ \a \b \f \v \e \E,
// \NNN octal (up to three digits), \xHH (one byte each), \uHHHH, and
// \UHHHHHHHH. Unknown escapes are kept as written.
func processEscapes(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			out.WriteByte(s[i])
			i++
			continue
		}
		switch next := s[i+1]; next {
		case 'n':
			out.WriteByte('\n')
			i += 2
		case 't':
			out.WriteByte('\t')
			i += 2
		case 'r':
			out.WriteByte('\r')
			i += 2
		case '\\':
			out.WriteByte('\\')
			i += 2
		case 'a':
			out.WriteByte('\a')
			i += 2
		case 'b':
			out.WriteByte('\b')
			i += 2
		case 'f':
			out.WriteByte('\f')
			i += 2
		case 'v':
			out.WriteByte('\v')
			i += 2
		case 'e', 'E':
			out.WriteByte(0x1b)
			i += 2
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i + 1
			n := 0
			for j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7' {
				n = n*8 + int(s[j]-'0')
				j++
			}
			out.WriteByte(byte(n))
			i = j
		case 'x':
			if i+3 < len(s) && isHex(s[i+2]) && isHex(s[i+3]) {
				n, _ := strconv.ParseUint(s[i+2:i+4], 16, 8)
				out.WriteByte(byte(n))
				i += 4
			} else {
				out.WriteByte('\\')
				i++
			}
		case 'u', 'U':
			limit := 4
			if next == 'U' {
				limit = 8
			}
			j := i + 2
			for j < len(s) && j < i+2+limit && isHex(s[j]) {
				j++
			}
			if j == i+2 {
				out.WriteString(s[i : i+2])
				i += 2
				break
			}
			n, _ := strconv.ParseUint(s[i+2:j], 16, 32)
			out.WriteRune(rune(n))
			i = j
		default:
			out.WriteByte('\\')
			i++
		}
	}
	return out.String()
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
