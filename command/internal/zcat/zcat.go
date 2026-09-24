package zcat

import (
	"bytes"
	gzlib "compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("zcat: nil ExecContext")
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
	set.SetProgram("zcat")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: zcat [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Decompress FILEs to standard output.\n\n")
		fmt.Fprint(stderr, "When no FILE is given, or when FILE is -, read from standard input.\n\n")
		fmt.Fprint(stderr, "  -f, --force       force; read compressed data even from a terminal\n")
		fmt.Fprint(stderr, "  -l, --list        list compressed file contents\n")
		fmt.Fprint(stderr, "  -q, --quiet       suppress all warnings\n")
		fmt.Fprint(stderr, "  -S, --suffix=SUF  use suffix SUF on compressed files (default: .gz)\n")
		fmt.Fprint(stderr, "  -t, --test        test compressed file integrity\n")
		fmt.Fprint(stderr, "  -v, --verbose     verbose mode\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	_ = set.BoolLong("force", 'f', "force; read compressed data even from a terminal")
	listFlag := set.BoolLong("list", 'l', "list compressed file contents")
	quietFlag := set.BoolLong("quiet", 'q', "suppress all warnings")
	_ = set.StringLong("suffix", 'S', ".gz", "use suffix SUF on compressed files (default: .gz)")
	testFlag := set.BoolLong("test", 't', "test compressed file integrity")
	verboseFlag := set.BoolLong("verbose", 'v', "verbose mode")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"zcat"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "zcat: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	files := set.Args()

	if len(files) == 0 {
		files = []string{"-"}
	}

	if *listFlag {
		return listFiles(ec, files, *quietFlag, *verboseFlag, stderr)
	}

	if *testFlag {
		return testFiles(ec, files, *quietFlag, *verboseFlag, stderr)
	}

	for _, file := range files {
		data, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "zcat: %s: %v\n", file, err)
			return interp.ExitStatus(1)
		}

		if !isGzip(data) {
			if !*quietFlag {
				fmt.Fprintf(stderr, "zcat: %s: not in gzip format\n", file)
			}
			return interp.ExitStatus(1)
		}

		decompressed, err := decompressData(data)
		if err != nil {
			if !*quietFlag {
				fmt.Fprintf(stderr, "zcat: %s: %v\n", file, err)
			}
			return interp.ExitStatus(1)
		}

		stdout.Write(decompressed)
		if *verboseFlag {
			fmt.Fprintf(stderr, "%s:\tOK\n", file)
		}
	}

	return nil
}

func readFile(ec *command.ExecContext, file string) ([]byte, error) {
	if ec.Stdin != nil && file == "-" {
		return io.ReadAll(ec.Stdin)
	}

	if ec.FS == nil {
		return nil, fmt.Errorf("no filesystem")
	}

	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("No such file or directory")
		}
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func listFiles(ec *command.ExecContext, files []string, quiet, verbose bool, stderr io.Writer) error {
	fmt.Fprintf(stderr, "  compressed uncompressed  ratio uncompressed_name\n")

	for _, file := range files {
		data, err := readFile(ec, file)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "zcat: %s: %v\n", file, err)
			}
			continue
		}

		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "zcat: %s: not in gzip format\n", file)
			}
			continue
		}

		compressed := len(data)
		uncompressed := uncompressedSize(data)

		ratio := "0.0"
		if uncompressed > 0 {
			ratio = fmt.Sprintf("%.1f", (1.0-float64(compressed)/float64(uncompressed))*100.0)
		}

		fmt.Fprintf(stderr, "%10d %10d %5s%% %s\n", compressed, uncompressed, ratio, file)

		if verbose {
			fmt.Fprintf(stderr, "%s:\tOK\n", file)
		}
	}

	return nil
}

func testFiles(ec *command.ExecContext, files []string, quiet, verbose bool, stderr io.Writer) error {
	for _, file := range files {
		data, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "zcat: %s: %v\n", file, err)
			return interp.ExitStatus(1)
		}

		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "zcat: %s: not in gzip format\n", file)
			}
			return interp.ExitStatus(1)
		}

		_, err = decompressData(data)
		if err != nil {
			fmt.Fprintf(stderr, "zcat: %s: %v\n", file, err)
			return interp.ExitStatus(1)
		}

		if verbose {
			fmt.Fprintf(stderr, "%s:\tOK\n", file)
		}
	}

	return nil
}

func decompressData(data []byte) ([]byte, error) {
	r, err := gzlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func isGzip(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func uncompressedSize(data []byte) int {
	if len(data) < 4 {
		return 0
	}
	n := len(data)
	return int(data[n-4]) | int(data[n-3])<<8 | int(data[n-2])<<16 | int(data[n-1])<<24
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
