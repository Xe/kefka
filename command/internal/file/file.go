package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Impl struct{}

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

func getExtension(filename string) string {
	base := path.Base(filename)
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".") {
		return base
	}
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return base[dot:]
}

var extDescriptions = map[string]string{
	".js":      "JavaScript source",
	".mjs":     "JavaScript module",
	".cjs":     "CommonJS module",
	".ts":      "TypeScript source",
	".tsx":     "TypeScript JSX source",
	".jsx":     "JavaScript JSX source",
	".py":      "Python script",
	".rb":      "Ruby script",
	".go":      "Go source",
	".rs":      "Rust source",
	".c":       "C source",
	".h":       "C header",
	".cpp":     "C++ source",
	".hpp":     "C++ header",
	".java":    "Java source",
	".sh":      "Bourne-Again shell script",
	".bash":    "Bourne-Again shell script",
	".zsh":     "Zsh shell script",
	".json":    "JSON data",
	".yaml":    "YAML data",
	".yml":     "YAML data",
	".xml":     "XML document",
	".csv":     "CSV text",
	".toml":    "TOML data",
	".html":    "HTML document",
	".htm":     "HTML document",
	".css":     "CSS stylesheet",
	".svg":     "SVG image",
	".md":      "Markdown document",
	".markdown": "Markdown document",
	".txt":     "ASCII text",
	".rst":     "reStructuredText",
}

func detectTextType(content string, filename string) string {
	// Shebang
	if strings.HasPrefix(content, "#!") {
		firstLine, _, _ := strings.Cut(content, "\n")
		switch {
		case strings.Contains(firstLine, "python"):
			return "Python script, ASCII text executable"
		case strings.Contains(firstLine, "node") || strings.Contains(firstLine, "bun") || strings.Contains(firstLine, "deno"):
			return "JavaScript script, ASCII text executable"
		case strings.Contains(firstLine, "bash"):
			return "Bourne-Again shell script, ASCII text executable"
		case strings.Contains(firstLine, "sh"):
			return "POSIX shell script, ASCII text executable"
		case strings.Contains(firstLine, "ruby"):
			return "Ruby script, ASCII text executable"
		case strings.Contains(firstLine, "perl"):
			return "Perl script, ASCII text executable"
		default:
			return "script, ASCII text executable"
		}
	}

	// XML/HTML
	trimmed := strings.TrimLeft(content, " \t\n\r")
	if strings.HasPrefix(trimmed, "<?xml") {
		return "XML document"
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
		return "HTML document"
	}

	// Line endings
	lineEnding := ""
	if strings.Contains(content, "\r\n") {
		lineEnding = ", with CRLF line terminators"
	} else if strings.Contains(content, "\r") {
		lineEnding = ", with CR line terminators"
	}

	// Extension-based
	ext := getExtension(filename)
	if desc, ok := extDescriptions[ext]; ok {
		return desc + lineEnding
	}

	// Unicode check (first 8KB)
	unicode := false
	for _, r := range content[:min(len(content), 8192)] {
		if r > 127 {
			unicode = true
			break
		}
	}
	if unicode {
		return "UTF-8 Unicode text" + lineEnding
	}
	return "ASCII text" + lineEnding
}

func detectFileType(filename string, data []byte) string {
	if len(data) == 0 {
		return "empty"
	}

	mimeType := http.DetectContentType(data)

	// For text content, use extension and content heuristics for
	// human-readable descriptions (e.g. "JSON data" vs "text/plain").
	if strings.HasPrefix(mimeType, "text/") {
		return detectTextType(string(data), filename)
	}

	// http.DetectContentType identified a concrete type (image, PDF, etc).
	if mimeType != "application/octet-stream" {
		return mimeType
	}

	// Unidentified — try text heuristics as a last resort.
	if isValidText(data) {
		return detectTextType(string(data), filename)
	}

	return "data"
}

// isValidText does a quick heuristic check: if the first 512 bytes are
// mostly printable ASCII or common whitespace, treat it as text.
func isValidText(data []byte) bool {
	check := data
	if len(check) > 512 {
		check = check[:512]
	}
	nonPrint := 0
	for _, b := range check {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
			nonPrint++
		}
	}
	return nonPrint <= len(check)/10
}

func (Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New("file: nil ExecContext")
	}
	if ec.FS == nil {
		return errors.New("file: ExecContext has no filesystem")
	}

	stderr := ec.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	stdout := ec.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	set := getopt.New()
	set.SetProgram("file")
	set.SetParameters("FILE...")

	usage := func() {
		fmt.Fprint(stderr, "Usage: file [OPTION]... FILE...\n")
		fmt.Fprint(stderr, "Determine file type.\n\n")
		fmt.Fprint(stderr, "  -b, --brief          do not prepend filenames to output\n")
		fmt.Fprint(stderr, "  -i, --mime           output MIME type strings\n")
		fmt.Fprint(stderr, "  -L, --dereference    follow symlinks\n")
		fmt.Fprint(stderr, "      --help           display this help and exit\n")
	}
	set.SetUsage(usage)

	brief := set.BoolLong("brief", 'b', "do not prepend filenames to output")
	mimeMode := set.BoolLong("mime", 'i', "output MIME type strings")
	_ = set.BoolLong("dereference", 'L', "follow symlinks")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{"file"}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "file: %s\n", err)
		usage()
		return interp.ExitStatus(1)
	}
	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		fmt.Fprint(stderr, "Usage: file [-bLi] FILE...\n")
		return interp.ExitStatus(1)
	}

	exitCode := 0

	for _, name := range files {
		p := resolvePath(ec, name)

		info, err := ec.FS.Stat(p)
		if err != nil {
			if *brief {
				fmt.Fprintln(stdout, "cannot open")
			} else {
				fmt.Fprintf(stdout, "%s: cannot open (No such file or directory)\n", name)
			}
			exitCode = 1
			continue
		}

		if info.IsDir() {
			result := "directory"
			if *mimeMode {
				result = "inode/directory"
			}
			if *brief {
				fmt.Fprintln(stdout, result)
			} else {
				fmt.Fprintf(stdout, "%s: %s\n", name, result)
			}
			continue
		}

		f, err := ec.FS.Open(p)
		if err != nil {
			if *brief {
				fmt.Fprintln(stdout, "cannot open")
			} else {
				fmt.Fprintf(stdout, "%s: cannot open (No such file or directory)\n", name)
			}
			exitCode = 1
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			if *brief {
				fmt.Fprintln(stdout, "cannot open")
			} else {
				fmt.Fprintf(stdout, "%s: cannot open (No such file or directory)\n", name)
			}
			exitCode = 1
			continue
		}

		desc := detectFileType(name, data)
		if *mimeMode {
			// When mime mode is on, convert human-readable descriptions
			// back to a MIME type where possible.
			desc = toMIME(desc, name)
		}
		if *brief {
			fmt.Fprintln(stdout, desc)
		} else {
			fmt.Fprintf(stdout, "%s: %s\n", name, desc)
		}
	}

	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

func toMIME(desc string, filename string) string {
	switch desc {
	case "empty":
		return "inode/x-empty"
	case "directory":
		return "inode/directory"
	}

	// If detectFileType already returned a MIME type via
	// http.DetectContentType, it looks like "image/png" etc.
	if strings.Contains(desc, "/") {
		return strings.SplitN(desc, ";", 2)[0]
	}

	// Text-based descriptions: try extension first.
	ext := getExtension(filename)
	if ext != "" {
		if m := mime.TypeByExtension(ext); m != "" {
			return strings.SplitN(m, ";", 2)[0]
		}
	}

	// Common text patterns.
	switch {
	case strings.Contains(desc, "text"), strings.Contains(desc, "script"), strings.Contains(desc, "source"), strings.Contains(desc, "document"):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
