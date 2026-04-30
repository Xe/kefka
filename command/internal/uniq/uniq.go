package uniq

import (
	"context"
	"errors"
	"fmt"
	"io"
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
		fmt.Fprint(stderr, "  -c, --count        prefix lines by the number of occurrences\n")
		fmt.Fprint(stderr, "  -d, --repeated     only print duplicate lines\n")
		fmt.Fprint(stderr, "  -i, --ignore-case  ignore case when comparing\n")
		fmt.Fprint(stderr, "  -u, --unique       only print unique lines\n")
		fmt.Fprint(stderr, "      --help         display this help and exit\n")
	}
	set.SetUsage(usage)

	count := set.BoolLong("count", 'c', "prefix lines by the number of occurrences")
	duplicatesOnly := set.BoolLong("repeated", 'd', "only print duplicate lines")
	uniqueOnly := set.BoolLong("unique", 'u', "only print unique lines")
	ignoreCase := set.BoolLong("ignore-case", 'i', "ignore case when comparing")
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

	files := set.Args()
	content, err := readAndConcat(ec, files, stderr)
	if err != nil {
		return err
	}

	io.WriteString(stdout, processUniq(content, *count, *duplicatesOnly, *uniqueOnly, *ignoreCase))
	return nil
}

func processUniq(content string, count, duplicatesOnly, uniqueOnly, ignoreCase bool) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	type entry struct {
		line  string
		count int
	}

	equal := func(a, b string) bool {
		if ignoreCase {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	result := make([]entry, 0, len(lines))
	current := lines[0]
	currentCount := 1
	for i := 1; i < len(lines); i++ {
		if equal(lines[i], current) {
			currentCount++
			continue
		}
		result = append(result, entry{line: current, count: currentCount})
		current = lines[i]
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

func readAndConcat(ec *command.ExecContext, files []string, stderr io.Writer) (string, error) {
	if len(files) == 0 {
		return readStdin(ec)
	}

	var b strings.Builder
	for _, f := range files {
		if f == "-" {
			data, err := readStdin(ec)
			if err != nil {
				return "", err
			}
			b.WriteString(data)
			continue
		}
		data, err := readFile(ec, f, stderr)
		if err != nil {
			return "", err
		}
		b.WriteString(data)
	}
	return b.String(), nil
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
