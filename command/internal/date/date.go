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

	set := getopt.New()
	set.SetProgram("date")
	set.SetParameters("[+FORMAT]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: date [OPTION]... [+FORMAT]\n")
		fmt.Fprint(stderr, "Display the current time in the given FORMAT.\n\n")
		fmt.Fprint(stderr, "  -d, --date=STRING   display time described by STRING\n")
		fmt.Fprint(stderr, "  -u, --utc           print Coordinated Universal Time (UTC)\n")
		fmt.Fprint(stderr, "  -I, --iso-8601      output date/time in ISO 8601 format\n")
		fmt.Fprint(stderr, "  -R, --rfc-email     output RFC 5322 date format\n")
		fmt.Fprint(stderr, "      --help          display this help and exit\n")
	}
	set.SetUsage(usage)

	dateStr := set.StringLong("date", 'd', "", "display time described by STRING")
	utc := set.BoolLong("utc", 'u', "print Coordinated Universal Time (UTC)")
	iso := set.BoolLong("iso-8601", 'I', "output date/time in ISO 8601 format")
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

	// The sandbox always renders in UTC to avoid leaking the host
	// timezone through %Z, %z, or wall-clock fields. The flag is
	// accepted for compatibility but is otherwise a no-op.
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
	case *iso:
		out = formatDate(d, "%Y-%m-%dT%H:%M:%S%z")
	case *rfc:
		out = formatDate(d, "%a, %d %b %Y %H:%M:%S %z")
	default:
		out = formatDate(d, "%a %b %e %H:%M:%S %Z %Y")
	}

	fmt.Fprintln(stdout, out)
	return nil
}

func formatDate(d time.Time, f string) string {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		i++
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

var digitsOnly = regexp.MustCompile(`^\d+$`)

func parseDate(s string) (time.Time, bool) {
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
	return time.Time{}, false
}
