package basename

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("basename: nil ExecContext")
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
	set.SetProgram("basename")
	set.SetParameters("NAME [SUFFIX]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: basename NAME [SUFFIX]\n")
		fmt.Fprint(stderr, "  or:  basename OPTION... NAME...\n")
		fmt.Fprint(stderr, "Strip directory and suffix from filenames.\n\n")
		fmt.Fprint(stderr, "  -a, --multiple       support multiple arguments\n")
		fmt.Fprint(stderr, "  -s, --suffix=SUFFIX  remove a trailing SUFFIX\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	multiple := set.BoolLong("multiple", 'a', "support multiple arguments")
	suffix := set.StringLong("suffix", 's', "", "remove a trailing SUFFIX")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"basename"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "basename: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	if set.Lookup("suffix").Seen() {
		*multiple = true
	}

	names := set.Args()
	if len(names) == 0 {
		fmt.Fprint(stderr, "basename: missing operand\n")
		return interp.ExitStatus(1)
	}

	suf := *suffix
	if !*multiple && len(names) >= 2 {
		suf = names[len(names)-1]
		names = names[:len(names)-1]
	}

	results := make([]string, 0, len(names))
	for _, name := range names {
		results = append(results, basenameOf(name, suf))
	}

	io.WriteString(stdout, strings.Join(results, "\n"))
	io.WriteString(stdout, "\n")
	return nil
}

func basenameOf(name, suffix string) string {
	clean := strings.TrimRight(name, "/")
	idx := strings.LastIndex(clean, "/")
	base := clean[idx+1:]
	if suffix != "" && strings.HasSuffix(base, suffix) {
		base = base[:len(base)-len(suffix)]
	}
	return base
}
