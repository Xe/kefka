package gunzip

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
		return errors.New("gunzip: nil ExecContext")
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
	set.SetProgram("gunzip")
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: gunzip [OPTION]... [FILE]...\n")
		fmt.Fprint(stderr, "Decompress FILEs (by default, in-place).\n\n")
		fmt.Fprint(stderr, "When no FILE is given, or when FILE is -, read from standard input.\n\n")
		fmt.Fprint(stderr, "  -c, --stdout      write to standard output, keep original files\n")
		fmt.Fprint(stderr, "  -f, --force       force overwrite of output file\n")
		fmt.Fprint(stderr, "  -k, --keep        keep (don't delete) input files\n")
		fmt.Fprint(stderr, "  -l, --list        list compressed file contents\n")
		fmt.Fprint(stderr, "  -n, --no-name     do not restore the original name and timestamp\n")
		fmt.Fprint(stderr, "  -N, --name        restore the original file name and timestamp\n")
		fmt.Fprint(stderr, "  -q, --quiet       suppress all warnings\n")
		fmt.Fprint(stderr, "  -r, --recursive   operate recursively on directories\n")
		fmt.Fprint(stderr, "  -S, --suffix=SUF  use suffix SUF on compressed files (default: .gz)\n")
		fmt.Fprint(stderr, "  -t, --test        test compressed file integrity\n")
		fmt.Fprint(stderr, "  -v, --verbose     verbose mode\n")
		fmt.Fprint(stderr, "      --help        display this help and exit\n")
	}
	set.SetUsage(usage)

	stdoutFlag := set.BoolLong("stdout", 'c', "write to standard output, keep original files")
	forceFlag := set.BoolLong("force", 'f', "force overwrite of output file")
	keepFlag := set.BoolLong("keep", 'k', "keep (don't delete) input files")
	listFlag := set.BoolLong("list", 'l', "list compressed file contents")
	noNameFlag := set.BoolLong("no-name", 'n', "do not restore the original name and timestamp")
	nameFlag := set.BoolLong("name", 'N', "restore the original file name and timestamp")
	quietFlag := set.BoolLong("quiet", 'q', "suppress all warnings")
	recursiveFlag := set.BoolLong("recursive", 'r', "operate recursively on directories")
	suffixFlag := set.StringLong("suffix", 'S', ".gz", "use suffix SUF on compressed files (default: .gz)")
	testFlag := set.BoolLong("test", 't', "test compressed file integrity")
	verboseFlag := set.BoolLong("verbose", 'v', "verbose mode")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"gunzip"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "gunzip: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		usage()
		return nil
	}

	files := set.Args()
	suffix := *suffixFlag
	toStdout := *stdoutFlag

	if *listFlag {
		return listFiles(ec, files, suffix, *quietFlag, stderr)
	}

	if *testFlag {
		return testFiles(ec, files, *verboseFlag, stderr)
	}

	return decompressFiles(ec, files, suffix, toStdout, *forceFlag, *keepFlag, *recursiveFlag, *quietFlag, *verboseFlag, *noNameFlag, *nameFlag, stderr)
}

func listFiles(ec *command.ExecContext, files []string, suffix string, quiet bool, stderr io.Writer) error {
	fmt.Fprintf(stderr, "  compressed uncompressed  ratio uncompressed_name\n")

	toProcess, err := collectFiles(ec, files, suffix, false, quiet, stderr)
	if err != nil {
		return err
	}

	for _, file := range toProcess {
		data, err := readFile(ec, file)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s\n", err)
			}
			return err
		}

		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: not in gzip format\n", file)
			}
			return interp.ExitStatus(1)
		}

		compressed := len(data)
		uncompressed := uncompressedSize(data)

		ratio := "0.0"
		if uncompressed > 0 {
			ratio = fmt.Sprintf("%.1f", (1.0-float64(compressed)/float64(uncompressed))*100.0)
		}

		name := strings.TrimSuffix(file, suffix)
		fmt.Fprintf(stderr, "%10d %10d %5s%% %s\n", compressed, uncompressed, ratio, name)
	}

	return nil
}

func testFiles(ec *command.ExecContext, files []string, verbose bool, stderr io.Writer) error {
	toProcess, err := collectFiles(ec, files, ".gz", false, false, stderr)
	if err != nil {
		return err
	}

	for _, file := range toProcess {
		data, err := readFile(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "gunzip: %s: %v\n", file, err)
			return err
		}

		if !isGzip(data) {
			fmt.Fprintf(stderr, "gunzip: %s: not in gzip format\n", file)
			return interp.ExitStatus(1)
		}

		_, err = decompressData(data)
		if err != nil {
			fmt.Fprintf(stderr, "gunzip: %s: %v\n", file, err)
			return interp.ExitStatus(1)
		}

		if verbose {
			fmt.Fprintf(stderr, "%s:\tOK\n", file)
		}
	}

	return nil
}

func decompressFiles(ec *command.ExecContext, files []string, suffix string, toStdout, force, keep, recursive, quiet, verbose, _ /* noName */, _ /* name */ bool, stderr io.Writer) error {
	if len(files) == 0 || (len(files) == 1 && files[0] == "-") {
		return decompressStdin(ec, stderr)
	}

	toProcess, err := collectFiles(ec, files, suffix, recursive, quiet, stderr)
	if err != nil {
		return err
	}

	for _, file := range toProcess {
		data, err := readFile(ec, file)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: %v\n", file, err)
			}
			return err
		}

		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: not in gzip format\n", file)
			}
			return interp.ExitStatus(1)
		}

		decompressed, err := decompressData(data)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: %v\n", file, err)
			}
			return err
		}

		if toStdout {
			ec.Stdout.Write(decompressed)
			if verbose {
				ratio := "0.0"
				if len(decompressed) > 0 {
					ratio = fmt.Sprintf("%.1f", (1.0-float64(len(data))/float64(len(decompressed)))*100.0)
				}
				fmt.Fprintf(stderr, "%s:\t%s%% -- written to stdout\n", file, ratio)
			}
			continue
		}

		outPath := strings.TrimSuffix(file, suffix)

		if outPath == file {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: unknown suffix -- ignored\n", file)
			}
			continue
		}

		_, err = ec.FS.Stat(outPath)
		if err == nil && !force {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s already exists; not overwritten\n", outPath)
			}
			continue
		}

		if err := writeOutput(ec, outPath, decompressed); err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gunzip: %s: %v\n", outPath, err)
			}
			return err
		}

		if verbose {
			ratio := "0.0"
			if len(decompressed) > 0 {
				ratio = fmt.Sprintf("%.1f", (1.0-float64(len(data))/float64(len(decompressed)))*100.0)
			}
			fmt.Fprintf(stderr, "%s:\t%s%% -- replaced with %s\n", file, ratio, outPath)
		}

		if !keep {
			ec.FS.Remove(resolvePath(ec, file))
		}
	}

	return nil
}

func decompressStdin(ec *command.ExecContext, stderr io.Writer) error {
	data, err := io.ReadAll(ec.Stdin)
	if err != nil {
		fmt.Fprintf(stderr, "gunzip: %v\n", err)
		return err
	}

	if !isGzip(data) {
		fmt.Fprintf(stderr, "gunzip: stdin: not in gzip format\n")
		return interp.ExitStatus(1)
	}

	decompressed, err := decompressData(data)
	if err != nil {
		fmt.Fprintf(stderr, "gunzip: %v\n", err)
		return err
	}

	ec.Stdout.Write(decompressed)
	return nil
}

func collectFiles(ec *command.ExecContext, files []string, suffix string, recursive, quiet bool, stderr io.Writer) ([]string, error) {
	var result []string

	for _, file := range files {
		full := resolvePath(ec, file)

		info, err := ec.FS.Stat(full)
		if err != nil {
			fmt.Fprintf(stderr, "gunzip: %s: No such file or directory\n", file)
			return nil, interp.ExitStatus(1)
		}

		if info.IsDir() {
			if recursive {
				dirFiles, err := ec.FS.ReadDir(full)
				if err != nil {
					if !quiet {
						fmt.Fprintf(stderr, "gunzip: %s: %v\n", file, err)
					}
					return nil, err
				}
				for _, df := range dirFiles {
					if !df.IsDir() && strings.HasSuffix(df.Name(), suffix) {
						result = append(result, path.Join(full, df.Name()))
					}
				}
			} else {
				if !quiet {
					fmt.Fprintf(stderr, "gunzip: %s: is a directory -- ignored\n", file)
				}
			}
			continue
		}

		result = append(result, file)
	}

	return result, nil
}

func readFile(ec *command.ExecContext, file string) ([]byte, error) {
	full := resolvePath(ec, file)
	f, err := ec.FS.Open(full)
	if err != nil {
		return nil, fmt.Errorf("%s: No such file or directory", file)
	}
	defer f.Close()

	return io.ReadAll(f)
}

// writeOutput writes payload to outPath. If the underlying filesystem
// short-writes or fails to close cleanly, it deletes the (presumed corrupt)
// output and returns an error so the caller can refuse to delete the source.
// This guards against backends like s3fs where a misbehaving Write can claim
// success while persisting nothing.
func writeOutput(ec *command.ExecContext, outPath string, payload []byte) error {
	f, err := ec.FS.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(payload)
	closeErr := f.Close()
	if writeErr == nil && n != len(payload) {
		writeErr = fmt.Errorf("short write: wrote %d of %d bytes", n, len(payload))
	}
	if writeErr != nil || closeErr != nil {
		ec.FS.Remove(outPath)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
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
