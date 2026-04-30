package seq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("seq: nil ExecContext")
	}

	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	usage := func() {
		fmt.Fprint(stderr, "Usage: seq [OPTION]... LAST\n")
		fmt.Fprint(stderr, "  or:  seq [OPTION]... FIRST LAST\n")
		fmt.Fprint(stderr, "  or:  seq [OPTION]... FIRST INCREMENT LAST\n")
		fmt.Fprint(stderr, "Print numbers from FIRST to LAST, in steps of INCREMENT.\n\n")
		fmt.Fprint(stderr, "  -s STRING  use STRING to separate numbers (default: newline)\n")
		fmt.Fprint(stderr, "  -w         equalize width by padding with leading zeros\n")
		fmt.Fprint(stderr, "      --help display this help and exit\n")
	}

	separator := "\n"
	equalizeWidth := false
	var nums []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "-s" && i+1 < len(args) {
			separator = args[i+1]
			i += 2
			continue
		}
		if arg == "-w" {
			equalizeWidth = true
			i++
			continue
		}
		if arg == "--" {
			i++
			break
		}
		if arg == "--help" {
			usage()
			return nil
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if strings.HasPrefix(arg, "-s") && len(arg) > 2 {
				separator = arg[2:]
				i++
				continue
			}
			if arg == "-ws" || arg == "-sw" {
				equalizeWidth = true
				if i+1 < len(args) {
					separator = args[i+1]
					i += 2
					continue
				}
			}
		}

		nums = append(nums, arg)
		i++
	}
	for i < len(args) {
		nums = append(nums, args[i])
		i++
	}

	if len(nums) == 0 {
		fmt.Fprint(stderr, "seq: missing operand\n")
		return interp.ExitStatus(1)
	}

	first := 1.0
	increment := 1.0
	var last float64

	parseNum := func(s string) (float64, error) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) {
			return 0, fmt.Errorf("seq: invalid floating point argument: '%s'", s)
		}
		return v, nil
	}

	switch len(nums) {
	case 1:
		v, err := parseNum(nums[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		last = v
	case 2:
		fv, err := parseNum(nums[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		lv, err := parseNum(nums[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		first, last = fv, lv
	default:
		fv, err := parseNum(nums[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		iv, err := parseNum(nums[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		lv, err := parseNum(nums[2])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return interp.ExitStatus(1)
		}
		first, increment, last = fv, iv, lv
	}

	if increment == 0 {
		fmt.Fprint(stderr, "seq: invalid Zero increment value: '0'\n")
		return interp.ExitStatus(1)
	}

	precision := getPrecision(first)
	if p := getPrecision(increment); p > precision {
		precision = p
	}
	if p := getPrecision(last); p > precision {
		precision = p
	}

	const maxIterations = 100000
	const epsilon = 1e-10

	var results []string
	if increment > 0 {
		for n := first; n <= last+epsilon; n += increment {
			if len(results) >= maxIterations {
				break
			}
			results = append(results, formatNum(n, precision))
		}
	} else {
		for n := first; n >= last-epsilon; n += increment {
			if len(results) >= maxIterations {
				break
			}
			results = append(results, formatNum(n, precision))
		}
	}

	if equalizeWidth && len(results) > 0 {
		maxLen := 0
		for _, r := range results {
			l := len(strings.TrimPrefix(r, "-"))
			if l > maxLen {
				maxLen = l
			}
		}
		for j, r := range results {
			negative := strings.HasPrefix(r, "-")
			num := strings.TrimPrefix(r, "-")
			if pad := maxLen - len(num); pad > 0 {
				num = strings.Repeat("0", pad) + num
			}
			if negative {
				results[j] = "-" + num
			} else {
				results[j] = num
			}
		}
	}

	output := strings.Join(results, separator)
	if output != "" {
		output += "\n"
	}
	fmt.Fprint(stdout, output)
	return nil
}

func getPrecision(n float64) int {
	s := strconv.FormatFloat(n, 'g', -1, 64)
	if strings.ContainsAny(s, "eE") {
		return 0
	}
	dotIndex := strings.Index(s, ".")
	if dotIndex == -1 {
		return 0
	}
	return len(s) - dotIndex - 1
}

func formatNum(n float64, precision int) string {
	if precision > 0 {
		return strconv.FormatFloat(n, 'f', precision, 64)
	}
	return strconv.FormatInt(int64(math.Round(n)), 10)
}
