package join

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type parsedLine struct {
	fields  []string
	joinKey string
}

type formatField struct {
	file  int
	field int
}

func (Impl) Exec(_ context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("join: nil ExecContext")
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
	set.SetProgram("join")
	set.SetParameters("FILE1 FILE2")

	usage := func() {
		fmt.Fprint(stderr, "Usage: join [OPTION]... FILE1 FILE2\n")
		fmt.Fprint(stderr, "For each pair of input lines with identical join fields, write a line to\nstandard output. The default join field is the first, delimited by blanks.\n\n")
		fmt.Fprint(stderr, "  -1 FIELD     Join on this FIELD of file 1 (default: 1)\n")
		fmt.Fprint(stderr, "  -2 FIELD     Join on this FIELD of file 2 (default: 1)\n")
		fmt.Fprint(stderr, "  -t CHAR      Use CHAR as input and output field separator\n")
		fmt.Fprint(stderr, "  -a FILENUM   Also print unpairable lines from file FILENUM (1 or 2)\n")
		fmt.Fprint(stderr, "  -v FILENUM   Like -a but only output unpairable lines\n")
		fmt.Fprint(stderr, "  -e STRING    Replace missing fields with STRING\n")
		fmt.Fprint(stderr, "  -o FORMAT    Output format (comma-separated list of FILENUM.FIELD)\n")
		fmt.Fprint(stderr, "  -i           Ignore case when comparing fields\n")
		fmt.Fprint(stderr, "      --help   display this help and exit\n")
	}
	set.SetUsage(usage)

	field1 := set.Int('1', 1, "join on this FIELD of file 1")
	field2 := set.Int('2', 1, "join on this FIELD of file 2")
	separator := set.StringLong("field-separator", 't', "", "use CHAR as input and output field separator")
	aFlag := set.Int('a', 0, "also print unpairable lines from file FILENUM")
	vFlag := set.Int('v', 0, "like -a but only output unpairable lines")
	emptyStr := set.String('e', "", "replace missing fields with STRING")
	oFlag := set.String('o', "", "output format")
	ignoreCase := set.BoolLong("ignore-case", 'i', "ignore case when comparing fields")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"join"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "join: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	if *field1 < 1 {
		fmt.Fprintf(stderr, "join: invalid field number: '%d'\n", *field1)
		return interp.ExitStatus(1)
	}
	if *field2 < 1 {
		fmt.Fprintf(stderr, "join: invalid field number: '%d'\n", *field2)
		return interp.ExitStatus(1)
	}
	if *aFlag != 0 && *aFlag != 1 && *aFlag != 2 {
		fmt.Fprintf(stderr, "join: invalid file number: '%d'\n", *aFlag)
		return interp.ExitStatus(1)
	}
	if *vFlag != 0 && *vFlag != 1 && *vFlag != 2 {
		fmt.Fprintf(stderr, "join: invalid file number: '%d'\n", *vFlag)
		return interp.ExitStatus(1)
	}

	var formatSpec []formatField
	if *oFlag != "" {
		var err error
		formatSpec, err = parseOutputFormat(*oFlag)
		if err != nil {
			fmt.Fprintf(stderr, "join: %s\n", err)
			return interp.ExitStatus(1)
		}
	}

	files := set.Args()
	if len(files) != 2 {
		if len(files) < 2 {
			fmt.Fprint(stderr, "join: missing file operand\n")
		} else {
			fmt.Fprint(stderr, "join: extra operand\n")
		}
		return interp.ExitStatus(1)
	}

	content1, err := readFile(ec, files[0], stderr)
	if err != nil {
		return err
	}
	content2, err := readFile(ec, files[1], stderr)
	if err != nil {
		return err
	}

	lines1 := parseLines(content1, *separator, *field1, *ignoreCase)
	lines2 := parseLines(content2, *separator, *field2, *ignoreCase)

	index2 := make(map[string][]*parsedLine)
	for i := range lines2 {
		key := lines2[i].joinKey
		index2[key] = append(index2[key], &lines2[i])
	}

	sep := *separator
	if sep == "" {
		sep = " "
	}

	var output []string
	matchedKeys2 := make(map[string]bool)

	for i := range lines1 {
		matches := index2[lines1[i].joinKey]
		if len(matches) > 0 {
			matchedKeys2[lines1[i].joinKey] = true
			if *vFlag == 0 {
				for _, m := range matches {
					output = append(output, formatLine(&lines1[i], m, *field1, *field2, sep, *emptyStr, formatSpec))
				}
			}
		} else {
			if *aFlag == 1 || *vFlag == 1 {
				output = append(output, formatLine(&lines1[i], nil, *field1, *field2, sep, *emptyStr, formatSpec))
			}
		}
	}

	if *aFlag == 2 || *vFlag == 2 {
		for i := range lines2 {
			if !matchedKeys2[lines2[i].joinKey] {
				output = append(output, formatLine(nil, &lines2[i], *field1, *field2, sep, *emptyStr, formatSpec))
			}
		}
	}

	if len(output) > 0 {
		fmt.Fprint(stdout, strings.Join(output, "\n")+"\n")
	}

	return nil
}

func splitFields(line, sep string) []string {
	if sep != "" {
		return strings.Split(line, sep)
	}
	return strings.Fields(line)
}

func parseLines(content, sep string, joinField int, ignoreCase bool) []parsedLine {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var result []parsedLine
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := splitFields(line, sep)
		joinKey := ""
		if joinField-1 < len(fields) {
			joinKey = fields[joinField-1]
		}
		if ignoreCase {
			joinKey = strings.ToLower(joinKey)
		}
		result = append(result, parsedLine{fields: fields, joinKey: joinKey})
	}
	return result
}

func formatLine(line1, line2 *parsedLine, field1, field2 int, sep, emptyStr string, formatSpec []formatField) string {
	if len(formatSpec) > 0 {
		var parts []string
		for _, ff := range formatSpec {
			var line *parsedLine
			if ff.file == 1 {
				line = line1
			} else {
				line = line2
			}
			if line != nil && ff.field == 0 {
				parts = append(parts, line.joinKey)
			} else if line != nil && ff.field-1 < len(line.fields) {
				parts = append(parts, line.fields[ff.field-1])
			} else {
				parts = append(parts, emptyStr)
			}
		}
		return strings.Join(parts, sep)
	}

	var parts []string
	joinKey := ""
	if line1 != nil {
		joinKey = line1.joinKey
	} else if line2 != nil {
		joinKey = line2.joinKey
	}
	parts = append(parts, joinKey)

	if line1 != nil {
		for i, f := range line1.fields {
			if i != field1-1 {
				parts = append(parts, f)
			}
		}
	}
	if line2 != nil {
		for i, f := range line2.fields {
			if i != field2-1 {
				parts = append(parts, f)
			}
		}
	}

	return strings.Join(parts, sep)
}

func parseOutputFormat(format string) ([]formatField, error) {
	var result []formatField
	for _, part := range strings.Split(format, ",") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '.')
		if idx < 0 {
			return nil, fmt.Errorf("invalid field spec: '%s'", format)
		}
		file, err := strconv.Atoi(part[:idx])
		if err != nil {
			return nil, fmt.Errorf("invalid field spec: '%s'", format)
		}
		field, err := strconv.Atoi(part[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid field spec: '%s'", format)
		}
		if file != 1 && file != 2 {
			return nil, fmt.Errorf("invalid field spec: '%s'", format)
		}
		result = append(result, formatField{file: file, field: field})
	}
	return result, nil
}

func readFile(ec *command.ExecContext, name string, stderr io.Writer) (string, error) {
	if name == "-" {
		if ec.Stdin == nil {
			return "", nil
		}
		data, err := io.ReadAll(ec.Stdin)
		if err != nil {
			return "", interp.ExitStatus(1)
		}
		return string(data), nil
	}
	if ec.FS == nil {
		fmt.Fprintf(stderr, "join: %s: No such file or directory\n", name)
		return "", interp.ExitStatus(1)
	}
	full := resolvePath(ec, name)
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "join: %s: No such file or directory\n", name)
		return "", interp.ExitStatus(1)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintf(stderr, "join: %s: %v\n", name, err)
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