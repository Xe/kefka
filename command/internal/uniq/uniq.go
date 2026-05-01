package uniq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("uniq: nil ExecContext")
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
	set.SetProgram("uniq")
	set.SetParameters("[INPUT [OUTPUT]]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: uniq [OPTION]... [INPUT [OUTPUT]]\n")
		fmt.Fprint(stderr, "report or omit repeated lines\n\n")
		fmt.Fprint(stderr, "  -c, --count            prefix lines by the number of occurrences\n")
		fmt.Fprint(stderr, "  -d, --repeated         only print duplicate lines\n")
		fmt.Fprint(stderr, "  -f, --skip-fields=N    avoid comparing the first N fields\n")
		fmt.Fprint(stderr, "  -i, --ignore-case      ignore case when comparing\n")
		fmt.Fprint(stderr, "  -s, --skip-chars=N     avoid comparing the first N characters\n")
		fmt.Fprint(stderr, "  -u, --unique           only print unique lines\n")
		fmt.Fprint(stderr, "  -w, --check-chars=N    compare no more than N characters in lines\n")
		fmt.Fprint(stderr, "      --help             display this help and exit\n")
	}
	set.SetUsage(usage)

	count := set.BoolLong("count", 'c', "prefix lines by the number of occurrences")
	duplicatesOnly := set.BoolLong("repeated", 'd', "only print duplicate lines")
	uniqueOnly := set.BoolLong("unique", 'u', "only print unique lines")
	ignoreCase := set.BoolLong("ignore-case", 'i', "ignore case when comparing")
	skipFields := set.IntLong("skip-fields", 'f', 0, "avoid comparing the first N fields")
	skipChars := set.IntLong("skip-chars", 's', 0, "avoid comparing the first N characters")
	checkChars := set.IntLong("check-chars", 'w', -1, "compare no more than N characters in lines")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"uniq"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "uniq: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	if *skipFields < 0 {
		fmt.Fprintf(stderr, "uniq: invalid number of fields to skip: %d\n", *skipFields)
		return interp.ExitStatus(1)
	}
	if *skipChars < 0 {
		fmt.Fprintf(stderr, "uniq: invalid number of characters to skip: %d\n", *skipChars)
		return interp.ExitStatus(1)
	}
	if set.IsSet("check-chars") && *checkChars < 0 {
		fmt.Fprintf(stderr, "uniq: invalid number of characters to compare: %d\n", *checkChars)
		return interp.ExitStatus(1)
	}

	positional := set.Args()
	if len(positional) > 2 {
		fmt.Fprintf(stderr, "uniq: extra operand %q\n", positional[2])
		usage()
		return interp.ExitStatus(1)
	}

	var inputName string
	if len(positional) >= 1 {
		inputName = positional[0]
	}
	content, err := readInput(ec, inputName, stderr)
	if err != nil {
		return err
	}

	checkLimit := -1
	if set.IsSet("check-chars") {
		checkLimit = *checkChars
	}
	out := processUniq(content, *count, *duplicatesOnly, *uniqueOnly, *ignoreCase, *skipFields, *skipChars, checkLimit)

	if len(positional) == 2 && positional[1] != "-" {
		if ec.FS == nil {
			fmt.Fprintf(stderr, "uniq: %s: No such file or directory\n", positional[1])
			return interp.ExitStatus(1)
		}
		full := resolvePath(ec, positional[1])
		f, err := ec.FS.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "uniq: %s: %v\n", positional[1], err)
			return interp.ExitStatus(1)
		}
		if _, err := io.WriteString(f, out); err != nil {
			f.Close()
			fmt.Fprintf(stderr, "uniq: %s: %v\n", positional[1], err)
			return interp.ExitStatus(1)
		}
		if err := f.Close(); err != nil {
			fmt.Fprintf(stderr, "uniq: %s: %v\n", positional[1], err)
			return interp.ExitStatus(1)
		}
		return nil
	}

	io.WriteString(stdout, out)
	return nil
}

func processUniq(content string, count, duplicatesOnly, uniqueOnly, ignoreCase bool, skipFields, skipChars, checkChars int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	keyOf := func(line string) string {
		k := skipFieldsAndChars(line, skipFields, skipChars)
		if checkChars >= 0 {
			kr := []rune(k)
			if checkChars < len(kr) {
				k = string(kr[:checkChars])
			}
		}
		if ignoreCase {
			return strings.ToLower(k)
		}
		return k
	}

	type entry struct {
		line  string
		count int
	}

	result := make([]entry, 0, len(lines))
	current := lines[0]
	currentKey := keyOf(current)
	currentCount := 1
	for i := 1; i < len(lines); i++ {
		k := keyOf(lines[i])
		if k == currentKey {
			currentCount++
			continue
		}
		result = append(result, entry{line: current, count: currentCount})
		current = lines[i]
		currentKey = k
		currentCount = 1
	}
	result = append(result, entry{line: current, count: currentCount})

	var out strings.Builder
	for _, e := range result {
		switch {
		case duplicatesOnly && e.count <= 1:
			continue
		case uniqueOnly && e.count != 1:
			continue
		}
		if count {
			fmt.Fprintf(&out, "%4d %s\n", e.count, e.line)
		} else {
			out.WriteString(e.line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func skipFieldsAndChars(line string, fields, chars int) string {
	runes := []rune(line)
	i := 0
	for f := 0; f < fields && i < len(runes); f++ {
		for i < len(runes) && isBlank(runes[i]) {
			i++
		}
		for i < len(runes) && !isBlank(runes[i]) {
			i++
		}
	}
	for c := 0; c < chars && i < len(runes); c++ {
		i++
	}
	return string(runes[i:])
}

func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}

func readInput(ec *command.ExecContext, name string, stderr io.Writer) (string, error) {
	if name == "" || name == "-" {
		return readStdin(ec)
	}
	return readFile(ec, name, stderr)
}

func readStdin(ec *command.ExecContext) (string, error) {
	if ec.Stdin == nil {
		return "", nil
	}
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
}

func readFile(ec *command.ExecContext, file string, stderr io.Writer) (string, error) {
	if ec.FS == nil {
		fmt.Fprintf(stderr, "uniq: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %s: No such file or directory\n", file)
		return "", interp.ExitStatus(1)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %s: %v\n", file, err)
		return "", interp.ExitStatus(1)
	}
	return string(data), nil
}

func resolvePath(ec *command.ExecContext, p string) string {
	dir := ec.Dir
	if dir == "" {
		dir = "."
	}
	if path.IsAbs(p) {
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return "."
		}
		return path.Clean(p)
	}
	joined := path.Join(dir, p)
	if joined == "" {
		return "."
	}
	return joined
}
