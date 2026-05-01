package date

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

var (
	days   = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

// validIsoFmts are the precision arguments accepted by `-I[FMT]` /
// `--iso-8601[=FMT]`. Mirrors GNU coreutils.
var validIsoFmts = map[string]string{
	"date":    "%Y-%m-%d",
	"hours":   "%Y-%m-%dT%H%z",
	"minutes": "%Y-%m-%dT%H:%M%z",
	"seconds": "%Y-%m-%dT%H:%M:%S%z",
	"ns":      "%Y-%m-%dT%H:%M:%S.%N%z",
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("date: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	// `-I` takes an OPTIONAL argument (`-I`, `-Idate`, `-Ihours`, etc.) and
	// `--iso-8601` accepts `--iso-8601=FMT`. getopt/v2 does not model
	// optional arguments natively, so we pre-scan args, capture the
	// requested precision, and normalize the flag to a plain boolean before
	// handing the remainder to the parser.
	isoFmt := ""
	args, isoSet, isoFmt, isoErr := extractIsoFlag(args)
	if isoErr != nil {
		fmt.Fprintf(stderr, "date: %s\n", isoErr)
		return interp.ExitStatus(1)
	}

	set := getopt.New()
	set.SetProgram("date")
	set.SetParameters("[+FORMAT]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: date [OPTION]... [+FORMAT]\n")
		fmt.Fprint(stderr, "Display the current time in the given FORMAT.\n\n")
		fmt.Fprint(stderr, "  -d, --date=STRING       display time described by STRING\n")
		fmt.Fprint(stderr, "  -u, --utc               print Coordinated Universal Time (UTC)\n")
		fmt.Fprint(stderr, "  -I[FMT], --iso-8601[=FMT]  output in ISO 8601 format; FMT may be\n")
		fmt.Fprint(stderr, "                          'date' (default), 'hours', 'minutes',\n")
		fmt.Fprint(stderr, "                          'seconds', or 'ns'\n")
		fmt.Fprint(stderr, "  -R, --rfc-email         output RFC 5322 date format\n")
		fmt.Fprint(stderr, "      --help              display this help and exit\n")
	}
	set.SetUsage(usage)

	dateStr := set.StringLong("date", 'd', "", "display time described by STRING")
	utc := set.BoolLong("utc", 'u', "print Coordinated Universal Time (UTC)")
	rfc := set.BoolLong("rfc-email", 'R', "output RFC 5322 date format")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"date"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "date: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	// kefka invariant: the sandbox always renders dates in UTC, regardless
	// of the host's `TZ` environment. POSIX says `-u` selects UTC and that
	// `TZ` controls otherwise, so this is a deliberate deviation. Keeping
	// time output stable also prevents leaking host timezone through `%Z`
	// or `%z`. The flag is accepted for compatibility but is otherwise a
	// no-op. See docs/posix2018/CONFORMANCE.md "### `date`" — please do
	// not "fix" this without updating that document.
	_ = *utc

	var fmtStr string
	hasFmt := false
	for _, p := range set.Args() {
		if strings.HasPrefix(p, "+") {
			fmtStr = p[1:]
			hasFmt = true
			break
		}
	}

	var d time.Time
	if *dateStr != "" {
		parsed, ok := parseDate(*dateStr)
		if !ok {
			fmt.Fprintf(stderr, "date: invalid date '%s'\n", *dateStr)
			return interp.ExitStatus(1)
		}
		d = parsed
	} else {
		d = time.Now()
	}
	d = d.UTC()

	var out string
	switch {
	case hasFmt:
		out = formatDate(d, fmtStr)
	case isoSet:
		out = formatDate(d, isoFmt)
	case *rfc:
		out = formatDate(d, "%a, %d %b %Y %H:%M:%S %z")
	default:
		out = formatDate(d, "%a %b %e %H:%M:%S %Z %Y")
	}

	fmt.Fprintln(stdout, out)
	return nil
}

// extractIsoFlag pre-processes argv to handle `-I[FMT]` and
// `--iso-8601[=FMT]`, which carry optional arguments that getopt/v2 cannot
// represent natively. It returns the remaining args, whether the flag was
// seen, the resolved strftime template, and any error from an invalid FMT.
func extractIsoFlag(args []string) ([]string, bool, string, error) {
	out := make([]string, 0, len(args))
	seen := false
	tmpl := validIsoFmts["date"]
	stopParsing := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if stopParsing {
			out = append(out, a)
			continue
		}
		if a == "--" {
			stopParsing = true
			out = append(out, a)
			continue
		}
		switch {
		case a == "-I" || a == "--iso-8601":
			seen = true
		case strings.HasPrefix(a, "--iso-8601="):
			seen = true
			fmtName := strings.TrimPrefix(a, "--iso-8601=")
			t, ok := validIsoFmts[fmtName]
			if !ok {
				return nil, false, "", fmt.Errorf("invalid argument '%s' for '--iso-8601'", fmtName)
			}
			tmpl = t
		case strings.HasPrefix(a, "-I") && len(a) > 2 && !strings.HasPrefix(a, "--"):
			seen = true
			fmtName := a[2:]
			t, ok := validIsoFmts[fmtName]
			if !ok {
				return nil, false, "", fmt.Errorf("invalid argument '%s' for '-I'", fmtName)
			}
			tmpl = t
		default:
			out = append(out, a)
		}
	}
	return out, seen, tmpl, nil
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func formatDate(d time.Time, f string) string {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		i++
		// POSIX `%E` / `%O` are locale-modifier prefixes. They precede
		// another conversion specifier. Per POSIX, an implementation
		// that does not support the locale alternative MUST fall back
		// to the unmodified specifier. Skip the modifier byte (only
		// when it is followed by an ASCII letter, a real spec char) and
		// let the next iteration handle the spec normally.
		if (f[i] == 'E' || f[i] == 'O') && i+1 < len(f) && isAlpha(f[i+1]) {
			i++
		}
		switch f[i] {
		case '%':
			b.WriteByte('%')
		case 'a':
			b.WriteString(days[int(d.Weekday())])
		case 'b', 'h':
			b.WriteString(months[int(d.Month())-1])
		case 'd':
			fmt.Fprintf(&b, "%02d", d.Day())
		case 'e':
			fmt.Fprintf(&b, "%2d", d.Day())
		case 'F':
			fmt.Fprintf(&b, "%d-%02d-%02d", d.Year(), int(d.Month()), d.Day())
		case 'H':
			fmt.Fprintf(&b, "%02d", d.Hour())
		case 'I':
			h := d.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d", h)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(d.Month()))
		case 'M':
			fmt.Fprintf(&b, "%02d", d.Minute())
		case 'n':
			b.WriteByte('\n')
		case 'N':
			fmt.Fprintf(&b, "%09d", d.Nanosecond())
		case 'p':
			if d.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'P':
			if d.Hour() < 12 {
				b.WriteString("am")
			} else {
				b.WriteString("pm")
			}
		case 'R':
			fmt.Fprintf(&b, "%02d:%02d", d.Hour(), d.Minute())
		case 's':
			fmt.Fprintf(&b, "%d", d.Unix())
		case 'S':
			fmt.Fprintf(&b, "%02d", d.Second())
		case 't':
			b.WriteByte('\t')
		case 'T':
			fmt.Fprintf(&b, "%02d:%02d:%02d", d.Hour(), d.Minute(), d.Second())
		case 'u':
			w := int(d.Weekday())
			if w == 0 {
				w = 7
			}
			fmt.Fprintf(&b, "%d", w)
		case 'w':
			fmt.Fprintf(&b, "%d", int(d.Weekday()))
		case 'y':
			fmt.Fprintf(&b, "%02d", d.Year()%100)
		case 'Y':
			fmt.Fprintf(&b, "%d", d.Year())
		case 'z':
			b.WriteString("+0000")
		case 'Z':
			b.WriteString("UTC")
		default:
			b.WriteByte('%')
			b.WriteByte(f[i])
		}
	}
	return b.String()
}

var (
	digitsOnly = regexp.MustCompile(`^\d+$`)
	// matches `N <unit> ago`, `N <unit>`, `+N <unit>`, `-N <unit>` where
	// unit is one of second(s), minute(s), hour(s), day(s), week(s).
	relativeRe = regexp.MustCompile(`^([+-]?\d+)\s+(second|minute|hour|day|week)s?(\s+ago)?$`)
)

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)

	// `@TIMESTAMP` — explicit GNU epoch syntax.
	if strings.HasPrefix(s, "@") {
		if n, err := strconv.ParseInt(s[1:], 10, 64); err == nil {
			return time.Unix(n, 0).UTC(), true
		}
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if digitsOnly.MatchString(s) {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(n, 0).UTC(), true
		}
	}
	switch strings.ToLower(s) {
	case "now", "today":
		return time.Now().UTC(), true
	case "yesterday":
		return time.Now().UTC().Add(-24 * time.Hour), true
	case "tomorrow":
		return time.Now().UTC().Add(24 * time.Hour), true
	}

	// Basic GNU-style relative parsing: "5 minutes ago", "+1 day", "-2 hours".
	// More elaborate forms ("next Friday", "last week", multi-segment phrases
	// like "1 day 2 hours") are intentionally deferred — they require a full
	// parsedatetime-style engine.
	if m := relativeRe.FindStringSubmatch(strings.ToLower(s)); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		if m[3] != "" { // "ago"
			n = -n
		}
		var dur time.Duration
		switch m[2] {
		case "second":
			dur = time.Duration(n) * time.Second
		case "minute":
			dur = time.Duration(n) * time.Minute
		case "hour":
			dur = time.Duration(n) * time.Hour
		case "day":
			dur = time.Duration(n) * 24 * time.Hour
		case "week":
			dur = time.Duration(n) * 7 * 24 * time.Hour
		}
		return time.Now().UTC().Add(dur), true
	}

	return time.Time{}, false
}
