package gzip

import (
	"bytes"
	"compress/flate"
	gzlib "compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct{}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("gzip: nil ExecContext")
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
	set.SetProgram("gzip")
	set.SetParameters("[FILE]...")

	stdoutFlag := set.BoolLong("stdout", 'c', "write to standard output, keep original files")
	toStdoutFlag := set.BoolLong("to-stdout", 0, "alias for -c")
	decompressFlag := set.BoolLong("decompress", 'd', "decompress")
	uncompressFlag := set.BoolLong("uncompress", 0, "alias for -d")
	forceFlag := set.BoolLong("force", 'f', "force overwrite of output file")
	keepFlag := set.BoolLong("keep", 'k', "keep (don't delete) input files")
	listFlag := set.BoolLong("list", 'l', "list compressed file contents")
	set.BoolLong("no-name", 'n', "do not save or restore the original name and timestamp")
	set.BoolLong("name", 'N', "save or restore the original file name and timestamp")
	quietFlag := set.BoolLong("quiet", 'q', "suppress all warnings")
	set.BoolLong("recursive", 'r', "operate recursively on directories")
	suffixFlag := set.StringLong("suffix", 'S', ".gz", "use suffix SUF on compressed files (default: .gz)")
	testFlag := set.BoolLong("test", 't', "test compressed file integrity")
	verboseFlag := set.BoolLong("verbose", 'v', "verbose mode")
	fastFlag := set.BoolLong("fast", '1', "compress faster")
	bestFlag := set.BoolLong("best", '9', "compress better")
	helpFlag := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"gzip"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "gzip: %s\n", err)
		printUsage(stderr)
		return interp.ExitStatus(1)
	}

	if *helpFlag {
		printUsage(stderr)
		return nil
	}

	decompress := *decompressFlag || *uncompressFlag
	toStdout := *stdoutFlag || *toStdoutFlag
	suffix := *suffixFlag
	quiet := *quietFlag
	force := *forceFlag
	keep := *keepFlag

	files := set.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	if *listFlag {
		return listMode(ec, files, suffix, quiet, stderr)
	}

	if *testFlag {
		return testMode(ec, files, quiet, *verboseFlag, stderr)
	}

	if decompress {
		return decompressMode(ec, files, toStdout, force, keep, suffix, quiet, *verboseFlag, stderr)
	}

	return compressMode(ec, files, toStdout, force, keep, suffix, quiet, *verboseFlag, *fastFlag, *bestFlag, stderr)
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "Usage: gzip [OPTION]... [FILE]...\n")
	fmt.Fprint(w, "Compress FILEs (by default, in-place).\n\n")
	fmt.Fprint(w, "When no FILE is given, or when FILE is -, read from standard input.\n\n")
	fmt.Fprint(w, "With -d, decompress instead.\n\n")
	fmt.Fprint(w, "  -c, --stdout      write to standard output, keep original files\n")
	fmt.Fprint(w, "  -d, --decompress  decompress\n")
	fmt.Fprint(w, "  -f, --force       force overwrite of output file\n")
	fmt.Fprint(w, "  -k, --keep        keep (don't delete) input files\n")
	fmt.Fprint(w, "  -l, --list        list compressed file contents\n")
	fmt.Fprint(w, "  -n, --no-name     do not save or restore the original name and timestamp\n")
	fmt.Fprint(w, "  -N, --name        save or restore the original file name and timestamp\n")
	fmt.Fprint(w, "  -q, --quiet       suppress all warnings\n")
	fmt.Fprint(w, "  -r, --recursive   operate recursively on directories\n")
	fmt.Fprint(w, "  -S, --suffix=SUF  use suffix SUF on compressed files (default: .gz)\n")
	fmt.Fprint(w, "  -t, --test        test compressed file integrity\n")
	fmt.Fprint(w, "  -v, --verbose     verbose mode\n")
	fmt.Fprint(w, "  -1, --fast        compress faster\n")
	fmt.Fprint(w, "  -9, --best        compress better\n")
	fmt.Fprint(w, "      --help        display this help and exit\n")
}

func listMode(ec *command.ExecContext, files []string, suffix string, quiet bool, stderr io.Writer) error {
	fmt.Fprint(stderr, "  compressed uncompressed  ratio uncompressed_name\n")
	exitStatus := 0
	for _, file := range files {
		if file == "-" {
			data, err := io.ReadAll(ec.Stdin)
			if err != nil {
				return err
			}
			printListEntry(data, "stdin", stderr)
			continue
		}
		if ec.FS == nil {
			continue
		}
		full := resolvePath(ec, file)
		f, err := ec.FS.Open(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}
		printListEntry(data, strings.TrimSuffix(file, suffix), stderr)
	}
	if exitStatus != 0 {
		return interp.ExitStatus(exitStatus)
	}
	return nil
}

func printListEntry(data []byte, name string, w io.Writer) {
	compressed := len(data)
	uncompressed := uncompressedSize(data)
	ratio := "0.0"
	if uncompressed > 0 {
		ratio = fmt.Sprintf("%.1f", (1.0-float64(compressed)/float64(uncompressed))*100.0)
	}
	fmt.Fprintf(w, "%10d %10d %5s%% %s\n", compressed, uncompressed, ratio, name)
}

func testMode(ec *command.ExecContext, files []string, quiet, verbose bool, stderr io.Writer) error {
	exitStatus := 0
	for _, file := range files {
		if file == "-" {
			data, err := io.ReadAll(ec.Stdin)
			if err != nil {
				return err
			}
			if !isGzip(data) {
				if !quiet {
					fmt.Fprintf(stderr, "gzip: -: not in gzip format\n")
				}
				exitStatus = 1
				continue
			}
			_, err = decompressData(data)
			if err != nil {
				if !quiet {
					fmt.Fprintf(stderr, "gzip: -: %v\n", err)
				}
				exitStatus = 1
				continue
			}
			if verbose {
				fmt.Fprintf(stderr, "-:\tOK\n")
			}
			continue
		}
		if ec.FS == nil {
			continue
		}
		full := resolvePath(ec, file)
		f, err := ec.FS.Open(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}
		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: not in gzip format\n", file)
			}
			exitStatus = 1
			continue
		}
		_, err = decompressData(data)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}
		if verbose {
			fmt.Fprintf(stderr, "%s:\tOK\n", file)
		}
	}
	if exitStatus != 0 {
		return interp.ExitStatus(exitStatus)
	}
	return nil
}

func compressMode(ec *command.ExecContext, files []string, toStdout, force, keep bool, suffix string, quiet, verbose, fast, best bool, stderr io.Writer) error {
	level := level(fast, best)
	exitStatus := 0
	for _, file := range files {
		if file == "-" {
			data, err := io.ReadAll(ec.Stdin)
			if err != nil {
				return err
			}
			compressed, err := compressData(data, level)
			if err != nil {
				return err
			}
			ec.Stdout.Write(compressed)
			continue
		}

		if ec.FS == nil {
			continue
		}

		full := resolvePath(ec, file)
		info, err := ec.FS.Stat(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}

		if info.IsDir() {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: is a directory -- ignored\n", file)
			}
			continue
		}

		if strings.HasSuffix(file, suffix) {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s already has %s suffix -- unchanged\n", file, suffix)
			}
			continue
		}

		f, err := ec.FS.Open(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}

		compressed, err := compressData(data, level)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}

		if toStdout {
			ec.Stdout.Write(compressed)
			if verbose {
				ratio := 0.0
				if len(data) > 0 {
					ratio = (1.0 - float64(len(compressed))/float64(len(data))) * 100.0
				}
				fmt.Fprintf(stderr, "%s:\t%.1f%% -- written to stdout\n", file, ratio)
			}
			continue
		}

		outputPath := file + suffix
		outputFull := resolvePath(ec, outputPath)
		if _, err := ec.FS.Stat(outputFull); err == nil && !force {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s already exists; not overwritten\n", outputPath)
			}
			continue
		}

		if err := writeOutput(ec, outputFull, compressed); err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", outputPath, err)
			}
			exitStatus = 1
			continue
		}

		if verbose {
			ratio := 0.0
			if len(data) > 0 {
				ratio = (1.0 - float64(len(compressed))/float64(len(data))) * 100.0
			}
			fmt.Fprintf(stderr, "%s:\t%.1f%% -- replaced with %s\n", file, ratio, outputPath)
		}

		if !keep {
			ec.FS.Remove(full)
		}
	}
	if exitStatus != 0 {
		return interp.ExitStatus(exitStatus)
	}
	return nil
}

func decompressMode(ec *command.ExecContext, files []string, toStdout, force, keep bool, suffix string, quiet, verbose bool, stderr io.Writer) error {
	exitStatus := 0
	for _, file := range files {
		if file == "-" {
			data, err := io.ReadAll(ec.Stdin)
			if err != nil {
				return err
			}
			if !isGzip(data) {
				if !quiet {
					fmt.Fprintf(stderr, "gzip: stdin: not in gzip format\n")
				}
				continue
			}
			decompressed, err := decompressData(data)
			if err != nil {
				if !quiet {
					fmt.Fprintf(stderr, "gzip: stdin: %v\n", err)
				}
				continue
			}
			ec.Stdout.Write(decompressed)
			continue
		}

		if ec.FS == nil {
			continue
		}

		full := resolvePath(ec, file)
		info, err := ec.FS.Stat(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}

		if info.IsDir() {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: is a directory -- ignored\n", file)
			}
			continue
		}

		if !strings.HasSuffix(file, suffix) {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: unknown suffix -- ignored\n", file)
			}
			continue
		}

		f, err := ec.FS.Open(full)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: No such file or directory\n", file)
			}
			exitStatus = 1
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}

		if !isGzip(data) {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: not in gzip format\n", file)
			}
			exitStatus = 1
			continue
		}

		decompressed, err := decompressData(data)
		if err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", file, err)
			}
			exitStatus = 1
			continue
		}

		if toStdout {
			ec.Stdout.Write(decompressed)
			if verbose {
				ratio := 0.0
				if len(decompressed) > 0 {
					ratio = (1.0 - float64(len(data))/float64(len(decompressed))) * 100.0
				}
				fmt.Fprintf(stderr, "%s:\t%.1f%% -- written to stdout\n", file, ratio)
			}
			continue
		}

		outputPath := strings.TrimSuffix(file, suffix)
		outputFull := resolvePath(ec, outputPath)
		if _, err := ec.FS.Stat(outputFull); err == nil && !force {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s already exists; not overwritten\n", outputPath)
			}
			continue
		}

		if err := writeOutput(ec, outputFull, decompressed); err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "gzip: %s: %v\n", outputPath, err)
			}
			exitStatus = 1
			continue
		}

		if verbose {
			ratio := 0.0
			if len(decompressed) > 0 {
				ratio = (1.0 - float64(len(data))/float64(len(decompressed))) * 100.0
			}
			fmt.Fprintf(stderr, "%s:\t%.1f%% -- replaced with %s\n", file, ratio, outputPath)
		}

		if !keep {
			ec.FS.Remove(full)
		}
	}
	if exitStatus != 0 {
		return interp.ExitStatus(exitStatus)
	}
	return nil
}

// writeOutput writes payload to outputFull. If the underlying filesystem
// short-writes or fails to close cleanly, it deletes the (presumed corrupt)
// output and returns an error so the caller can refuse to delete the source.
// This guards against backends like s3fs where a misbehaving Write can claim
// success while persisting nothing.
func writeOutput(ec *command.ExecContext, outputFull string, payload []byte) error {
	outF, err := ec.FS.Create(outputFull)
	if err != nil {
		return err
	}
	n, writeErr := outF.Write(payload)
	closeErr := outF.Close()
	if writeErr == nil && n != len(payload) {
		writeErr = fmt.Errorf("short write: wrote %d of %d bytes", n, len(payload))
	}
	if writeErr != nil || closeErr != nil {
		ec.FS.Remove(outputFull)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func compressData(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

func level(fast, best bool) int {
	if best {
		return flate.BestCompression
	}
	if fast {
		return flate.BestSpeed
	}
	return flate.DefaultCompression
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
