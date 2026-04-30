package base64

import (
	"context"
	enc "encoding/base64"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("base64: nil ExecContext")
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
	set.SetProgram("base64")
	set.SetParameters("[FILE]")

	usage := func() {
		fmt.Fprint(stderr, "Usage: base64 [OPTION]... [FILE]\n")
		fmt.Fprint(stderr, "Base64 encode or decode FILE, or standard input, to standard output.\n\n")
		fmt.Fprint(stderr, "  -d, --decode      decode data\n")
		fmt.Fprint(stderr, "  -w, --wrap=COLS   wrap encoded lines after COLS character (default 76, 0 to disable)\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	decode := set.BoolLong("decode", 'd', "decode data")
	wrapCols := set.IntLong("wrap", 'w', 76, "wrap encoded lines after COLS character (default 76, 0 to disable)")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"base64"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "base64: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	files := set.Args()

	data, err := readInput(ec, files, stderr)
	if err != nil {
		return err
	}

	if *decode {
		cleaned := stripWhitespace(data)
		decoded, err := enc.StdEncoding.DecodeString(string(cleaned))
		if err != nil {
			fmt.Fprint(stderr, "base64: invalid input\n")
			return interp.ExitStatus(1)
		}
		stdout.Write(decoded)
		return nil
	}

	encoded := enc.StdEncoding.EncodeToString(data)
	if *wrapCols > 0 {
		encoded = wrapLines(encoded, *wrapCols)
	}
	io.WriteString(stdout, encoded)
	return nil
}

func readInput(ec *command.ExecContext, files []string, stderr io.Writer) ([]byte, error) {
	if len(files) == 0 || (len(files) == 1 && files[0] == "-") {
		return readStdin(ec)
	}

	var out []byte
	for _, file := range files {
		if file == "-" {
			data, err := readStdin(ec)
			if err != nil {
				return nil, err
			}
			out = append(out, data...)
			continue
		}
		if ec.FS == nil {
			fmt.Fprintf(stderr, "base64: %s: No such file or directory\n", file)
			return nil, interp.ExitStatus(1)
		}
		full := resolvePath(ec, file)
		f, err := ec.FS.Open(full)
		if err != nil {
			fmt.Fprintf(stderr, "base64: %s: No such file or directory\n", file)
			return nil, interp.ExitStatus(1)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "base64: %s: %v\n", file, err)
			return nil, interp.ExitStatus(1)
		}
		out = append(out, data...)
	}
	return out, nil
}

func readStdin(ec *command.ExecContext) ([]byte, error) {
	if ec.Stdin == nil {
		return nil, nil
	}
	return io.ReadAll(ec.Stdin)
}

func stripWhitespace(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, r := range string(b) {
		if !unicode.IsSpace(r) {
			out = append(out, byte(r))
		}
	}
	return out
}

func wrapLines(s string, cols int) string {
	if cols <= 0 || len(s) == 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i += cols {
		end := min(i+cols, len(s))
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s[i:end])
	}
	b.WriteByte('\n')
	return b.String()
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
