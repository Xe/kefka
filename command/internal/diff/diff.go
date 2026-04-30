package diff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"github.com/pmezard/go-difflib/difflib"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("diff: nil ExecContext")
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
	set.SetProgram("diff")
	set.SetParameters("FILE1 FILE2")

	usage := func() {
		fmt.Fprint(stderr, "Usage: diff [OPTION]... FILE1 FILE2\n")
		fmt.Fprint(stderr, "Compare files line by line.\n\n")
		fmt.Fprint(stderr, "  -u, --unified                 output unified diff format (default)\n")
		fmt.Fprint(stderr, "  -q, --brief                   report only whether files differ\n")
		fmt.Fprint(stderr, "  -s, --report-identical-files  report when files are the same\n")
		fmt.Fprint(stderr, "  -i, --ignore-case             ignore case differences\n")
		fmt.Fprint(stderr, "      --help                    display this help and exit\n")
	}
	set.SetUsage(usage)

	unified := set.BoolLong("unified", 'u', "output unified diff format (default)")
	brief := set.BoolLong("brief", 'q', "report only whether files differ")
	reportSame := set.BoolLong("report-identical-files", 's', "report when files are the same")
	ignoreCase := set.BoolLong("ignore-case", 'i', "ignore case differences")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"diff"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "diff: %s\n", err)
		usage()
		return interp.ExitStatus(2)
	}
	if *help {
		usage()
		return nil
	}
	_ = unified

	files := set.Args()
	if len(files) < 2 {
		fmt.Fprint(stderr, "diff: missing operand\n")
		return interp.ExitStatus(2)
	}

	f1, f2 := files[0], files[1]

	c1, err := readContent(ec, f1)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f1)
		return interp.ExitStatus(2)
	}
	c2, err := readContent(ec, f2)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %s: No such file or directory\n", f2)
		return interp.ExitStatus(2)
	}

	t1, t2 := c1, c2
	if *ignoreCase {
		t1 = strings.ToLower(t1)
		t2 = strings.ToLower(t2)
	}

	if t1 == t2 {
		if *reportSame {
			fmt.Fprintf(stdout, "Files %s and %s are identical\n", f1, f2)
		}
		return nil
	}

	if *brief {
		fmt.Fprintf(stdout, "Files %s and %s differ\n", f1, f2)
		return interp.ExitStatus(1)
	}

	udiff := difflib.UnifiedDiff{
		A:        splitLines(c1),
		B:        splitLines(c2),
		FromFile: f1,
		ToFile:   f2,
		Context:  3,
	}
	out, derr := difflib.GetUnifiedDiffString(udiff)
	if derr != nil {
		fmt.Fprintf(stderr, "diff: %s\n", derr)
		return interp.ExitStatus(2)
	}
	io.WriteString(stdout, out)
	return interp.ExitStatus(1)
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func readContent(ec *command.ExecContext, file string) (string, error) {
	if file == "-" {
		if ec.Stdin == nil {
			return "", nil
		}
		data, err := io.ReadAll(ec.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if ec.FS == nil {
		return "", errors.New("no filesystem")
	}
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
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
