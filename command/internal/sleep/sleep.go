package sleep

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

const maxSleep = time.Hour

var durationRe = regexp.MustCompile(`^(\d+\.?\d*)(s|m|h|d)?$`)

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("sleep: nil ExecContext")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	set := getopt.New()
	set.SetProgram("sleep")
	set.SetParameters("NUMBER[SUFFIX]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: sleep NUMBER[SUFFIX]...\n")
		fmt.Fprint(stderr, "Pause for NUMBER seconds. SUFFIX may be:\n")
		fmt.Fprint(stderr, "  s - seconds (default)\n")
		fmt.Fprint(stderr, "  m - minutes\n")
		fmt.Fprint(stderr, "  h - hours\n")
		fmt.Fprint(stderr, "  d - days\n\n")
		fmt.Fprint(stderr, "NUMBER may be a decimal number.\n\n")
		fmt.Fprint(stderr, "      --help  display this help and exit\n")
	}
	set.SetUsage(usage)

	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"sleep"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "sleep: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	rest := set.Args()
	if len(rest) == 0 {
		fmt.Fprint(stderr, "sleep: missing operand\n")
		return interp.ExitStatus(1)
	}

	var total time.Duration
	for _, arg := range rest {
		d, ok := parseDuration(arg)
		if !ok {
			fmt.Fprintf(stderr, "sleep: invalid time interval '%s'\n", arg)
			return interp.ExitStatus(1)
		}
		total += d
	}

	if total > maxSleep {
		total = maxSleep
	}

	if ctx.Err() != nil {
		return nil
	}

	timer := time.NewTimer(total)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
	return nil
}

func parseDuration(arg string) (time.Duration, bool) {
	m := durationRe.FindStringSubmatch(arg)
	if m == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	suffix := m[2]
	if suffix == "" {
		suffix = "s"
	}
	var unit time.Duration
	switch suffix {
	case "s":
		unit = time.Second
	case "m":
		unit = time.Minute
	case "h":
		unit = time.Hour
	case "d":
		unit = 24 * time.Hour
	default:
		return 0, false
	}
	return time.Duration(value * float64(unit)), true
}
