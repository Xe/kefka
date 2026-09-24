package time

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strings"
	stdtime "time"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
	"github.com/Xe/kefka/command"
)

// Impl times the execution of another command.
//
// The inner command runs inside a subshell of ec.Runner, so it goes back
// through the shell's exec-handler chain (registered builtins, functions,
// path lookup) instead of being dispatched directly. ec.Runner must be set
// for the command-execution path; without it, `time CMD ...` cannot
// dispatch to the inner command. The no-command path (silent success)
// still works without a runner.
type Impl struct{}

func (impl Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("time: nil ExecContext")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	usage := func() {
		io.WriteString(stderr, "Usage: time [OPTION]... COMMAND [ARGUMENT]...\n")
		io.WriteString(stderr, "Time the execution of COMMAND.\n\n")
		io.WriteString(stderr, "  -f, --format=FORMAT     use FORMAT for output (default \"%e %M\")\n")
		io.WriteString(stderr, "  -o, --output=FILE       write timing output to FILE\n")
		io.WriteString(stderr, "  -a, --append            append to output file (with -o)\n")
		io.WriteString(stderr, "  -v, --verbose           verbose output\n")
		io.WriteString(stderr, "  -p, --portability       POSIX portable output format\n")
		io.WriteString(stderr, "      --help              display this help and exit\n\n")
		io.WriteString(stderr, "Format specifiers:\n")
		io.WriteString(stderr, "  %C    Command being timed\n")
		io.WriteString(stderr, "  %e    Elapsed real time in seconds\n")
		io.WriteString(stderr, "  %E    Elapsed time in [hours:]minutes:seconds format\n")
		io.WriteString(stderr, "  %M    Maximum resident set size (KB) - always 0\n")
		io.WriteString(stderr, "  %S    System CPU time (seconds) - always 0.00\n")
		io.WriteString(stderr, "  %U    User CPU time (seconds) - always 0.00\n")
		io.WriteString(stderr, "  %P    CPU percentage - always 0%\n")
	}

	format := "%e %M"
	outputFile := ""
	appendMode := false
	posixFormat := false

	i := 0
parseLoop:
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--format":
			i++
			if i >= len(args) {
				fmt.Fprint(stderr, "time: missing argument to '-f'\n")
				return interp.ExitStatus(1)
			}
			format = args[i]
			i++
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
			i++
		case arg == "-o" || arg == "--output":
			i++
			if i >= len(args) {
				fmt.Fprint(stderr, "time: missing argument to '-o'\n")
				return interp.ExitStatus(1)
			}
			outputFile = args[i]
			i++
		case strings.HasPrefix(arg, "--output="):
			outputFile = strings.TrimPrefix(arg, "--output=")
			i++
		case arg == "-a" || arg == "--append":
			appendMode = true
			i++
		case arg == "-v" || arg == "--verbose":
			format = "Command being timed: %C\nElapsed (wall clock) time: %e seconds\nMaximum resident set size (kbytes): %M"
			i++
		case arg == "-p" || arg == "--portability":
			posixFormat = true
			i++
		case arg == "--help":
			usage()
			return nil
		case arg == "--":
			i++
			break parseLoop
		case strings.HasPrefix(arg, "-") && arg != "-":
			// Unknown option — skip it (be permissive like GNU time).
			i++
		default:
			break parseLoop
		}
	}

	commandArgs := args[i:]

	// No command specified — print usage and exit non-zero, matching
	// GNU /usr/bin/time which errors out with "Usage: ..." in this case.
	if len(commandArgs) == 0 {
		usage()
		return interp.ExitStatus(1)
	}

	displayCommand := strings.Join(commandArgs, " ")

	startTime := stdtime.Now()
	innerErr := runInner(ctx, ec, commandArgs)
	elapsedSeconds := stdtime.Since(startTime).Seconds()

	var timingOutput string
	if posixFormat {
		timingOutput = fmt.Sprintf("real %.2f\nuser 0.00\nsys 0.00\n", elapsedSeconds)
	} else {
		timingOutput = applyFormat(format, elapsedSeconds, displayCommand)
		if !strings.HasSuffix(timingOutput, "\n") {
			timingOutput += "\n"
		}
	}

	if outputFile != "" {
		if err := writeTimingFile(ec, outputFile, timingOutput, appendMode); err != nil {
			fmt.Fprintf(stderr, "time: cannot write to '%s': %v\n", outputFile, err)
		}
	} else {
		fmt.Fprint(stderr, timingOutput)
	}

	return innerErr
}

// runInner executes the timed command in a subshell of ec.Runner. The
// command line is reassembled (with each argument shell-quoted) and parsed
// as bash so it re-enters the runner's exec-handler chain — this is what
// lets `time` dispatch to registered builtins, shell functions, or PATH
// binaries the same way the user would have invoked them directly.
func runInner(ctx context.Context, ec *command.ExecContext, commandArgs []string) error {
	if ec.Runner == nil {
		fmt.Fprint(ec.Stderr, "time: exec not available\n")
		return interp.ExitStatus(127)
	}

	var b strings.Builder
	for idx, a := range commandArgs {
		if idx > 0 {
			b.WriteByte(' ')
		}
		quoted, err := syntax.Quote(a, syntax.LangBash)
		if err != nil {
			fmt.Fprintf(ec.Stderr, "time: cannot quote argument %q: %v\n", a, err)
			return interp.ExitStatus(1)
		}
		b.WriteString(quoted)
	}

	prog, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(b.String()), "<time>")
	if err != nil {
		fmt.Fprintf(ec.Stderr, "time: cannot parse command: %v\n", err)
		return interp.ExitStatus(1)
	}

	sub := ec.Runner.Subshell()
	if err := interp.StdIO(ec.Stdin, ec.Stdout, ec.Stderr)(sub); err != nil {
		fmt.Fprintf(ec.Stderr, "time: cannot configure subshell: %v\n", err)
		return interp.ExitStatus(1)
	}
	return sub.Run(ctx, prog)
}

func writeTimingFile(ec *command.ExecContext, outputFile, timingOutput string, appendMode bool) error {
	if ec.FS == nil {
		return errors.New("no filesystem available")
	}
	full := resolvePath(ec, outputFile)

	flag := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := ec.FS.OpenFile(full, flag, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, timingOutput)
	return err
}

func applyFormat(format string, elapsedSeconds float64, displayCommand string) string {
	out := format
	out = strings.ReplaceAll(out, "%e", fmt.Sprintf("%.2f", elapsedSeconds))
	out = strings.ReplaceAll(out, "%E", formatElapsedTime(elapsedSeconds))
	out = strings.ReplaceAll(out, "%M", "0")
	out = strings.ReplaceAll(out, "%S", "0.00")
	out = strings.ReplaceAll(out, "%U", "0.00")
	out = strings.ReplaceAll(out, "%P", "0%")
	out = strings.ReplaceAll(out, "%C", displayCommand)
	return out
}

func formatElapsedTime(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int(math.Mod(seconds, 3600) / 60)
	secs := math.Mod(seconds, 60)
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%05.2f", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%05.2f", minutes, secs)
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
