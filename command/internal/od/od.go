package od

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

type outputFormat int

const (
	fmtOctal outputFormat = iota
	fmtHex
	fmtChar
)

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("od: nil ExecContext")
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
	set.SetProgram("od")
	set.SetParameters("[FILE]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: od [OPTION]... [FILE]\n")
		fmt.Fprint(stderr, "Write an unambiguous representation, octal bytes by default,\n")
		fmt.Fprint(stderr, "of FILE to standard output.\n\n")
		fmt.Fprint(stderr, "  -A, --address-radix=RADIX  output format for addresses; n means no address\n")
		fmt.Fprint(stderr, "  -c                         same as -t c\n")
		fmt.Fprint(stderr, "  -t, --format=TYPE          select output format; one of x1, c, o*\n")
		fmt.Fprint(stderr, "      --help                 display this help and exit\n")
	}
	set.SetUsage(usage)

	addrOpt := set.StringLong("address-radix", 'A', "o", "output format for addresses; n means no address")
	_ = set.Bool('c', "same as -t c")
	tList := set.ListLong("format", 't', "select output format; one of x1, c, o*")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	var formats []outputFormat
	tIdx := 0
	callback := func(opt getopt.Option) bool {
		switch opt.ShortName() {
		case "c":
			formats = append(formats, fmtChar)
		case "t":
			if tIdx < len(*tList) {
				v := (*tList)[tIdx]
				tIdx++
				switch {
				case v == "x1":
					formats = append(formats, fmtHex)
				case v == "c":
					formats = append(formats, fmtChar)
				case strings.HasPrefix(v, "o"):
					formats = append(formats, fmtOctal)
				}
			}
		}
		return true
	}

	if err := set.Getopt(append([]string{"od"}, args...), callback); err != nil {
		fmt.Fprintf(stderr, "od: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	addressNone := *addrOpt == "n"

	if len(formats) == 0 {
		formats = []outputFormat{fmtOctal}
	}

	files := set.Args()
	data, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	io.WriteString(stdout, buildOutput(data, formats, addressNone))
	return nil
}

func readInput(ec *command.ExecContext, files []string, stderr io.Writer) ([]byte, error) {
	if len(files) == 0 || files[0] == "-" {
		if ec.Stdin == nil {
			return nil, nil
		}
		return io.ReadAll(ec.Stdin)
	}
	if ec.FS == nil {
		fmt.Fprintf(stderr, "od: %s: No such file or directory\n", files[0])
		return nil, interp.ExitStatus(1)
	}
	full := resolvePath(ec, files[0])
	f, err := ec.FS.Open(full)
	if err != nil {
		fmt.Fprintf(stderr, "od: %s: No such file or directory\n", files[0])
		return nil, interp.ExitStatus(1)
	}
	defer f.Close()
	return io.ReadAll(f)
}

func buildOutput(data []byte, formats []outputFormat, addressNone bool) string {
	if len(data) == 0 {
		return ""
	}

	hasCharFormat := slices.Contains(formats, fmtChar)

	const bytesPerLine = 16
	var b strings.Builder

	for offset := 0; offset < len(data); offset += bytesPerLine {
		chunk := data[offset:min(offset+bytesPerLine, len(data))]

		for fIdx, f := range formats {
			switch {
			case fIdx == 0 && !addressNone:
				fmt.Fprintf(&b, "%07o ", offset)
			case fIdx > 0 && !addressNone:
				b.WriteString("        ")
			}

			for _, code := range chunk {
				switch f {
				case fmtChar:
					b.WriteString(formatCharByte(code))
				case fmtHex:
					b.WriteString(formatHexByte(code, hasCharFormat))
				case fmtOctal:
					fmt.Fprintf(&b, " %03o", code)
				}
			}
			b.WriteByte('\n')
		}
	}

	if !addressNone {
		fmt.Fprintf(&b, "%07o\n", len(data))
	}

	return b.String()
}

func formatCharByte(code byte) string {
	switch code {
	case 0:
		return `  \0`
	case 7:
		return `  \a`
	case 8:
		return `  \b`
	case 9:
		return `  \t`
	case 10:
		return `  \n`
	case 11:
		return `  \v`
	case 12:
		return `  \f`
	case 13:
		return `  \r`
	}
	if code >= 32 && code < 127 {
		return fmt.Sprintf("   %c", code)
	}
	return fmt.Sprintf(" %03o", code)
}

func formatHexByte(code byte, padForChar bool) string {
	if padForChar {
		return fmt.Sprintf("  %02x", code)
	}
	return fmt.Sprintf(" %02x", code)
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
