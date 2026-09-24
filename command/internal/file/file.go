package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-billy/v6"
	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
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

// statPath stats p, optionally without following the final symlink. When
// follow is false and the underlying filesystem implements
// billy.Symlink, Lstat is used so that the symlink itself is reported.
// When the filesystem doesn't expose Lstat the call gracefully degrades
// to Stat (links are always followed); this matches kefka's behavior in
// other commands like cp.
func statPath(fsys billy.Filesystem, p string, follow bool) (os.FileInfo, error) {
	if !follow {
		if sl, ok := fsys.(billy.Symlink); ok {
			return sl.Lstat(p)
		}
	}
	return fsys.Stat(p)
}

// readlink returns the target of a symlink at p, or an empty string if
// the underlying filesystem doesn't support it or the call fails.
func readlink(fsys billy.Filesystem, p string) string {
	sl, ok := fsys.(billy.Symlink)
	if !ok {
		return ""
	}
	target, err := sl.Readlink(p)
	if err != nil {
		return ""
	}
	return target
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

// detectMagic inspects the leading bytes of data and returns a
// human-readable description of well-known binary formats. The empty
// string means no magic-byte signature matched.
//
// This is a deliberately small subset of libmagic; it covers the most
// frequent formats encountered in practice. The descriptions are kept
// close to BSD `file(1)` output where reasonable, but we do not try to
// extract version numbers or sub-format details that would require a
// proper parser.
func detectMagic(data []byte) string {
	switch {
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x7f, 'E', 'L', 'F'}):
		// ELFCLASS at offset 4: 1 = 32-bit, 2 = 64-bit.
		bits := "32-bit"
		if len(data) > 4 && data[4] == 2 {
			bits = "64-bit"
		}
		// EI_DATA at offset 5: 1 = LSB, 2 = MSB.
		endian := "LSB"
		if len(data) > 5 && data[5] == 2 {
			endian = "MSB"
		}
		return "ELF " + bits + " " + endian + " executable"
	case len(data) >= 2 && data[0] == 'M' && data[1] == 'Z':
		return "PE32 executable (MS-DOS)"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0x1f, 0x8b, 0x08}):
		return "gzip compressed data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("PK\x03\x04")):
		return "Zip archive data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("PK\x05\x06")):
		return "Zip archive data (empty)"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		return "JPEG image data"
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "PNG image data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("%PDF")):
		return "PDF document"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "GIF image data"
	case len(data) >= 2 && bytes.Equal(data[:2], []byte("BZ")) && len(data) >= 3 && data[2] == 'h':
		return "bzip2 compressed data"
	case len(data) >= 6 && bytes.Equal(data[:6], []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		return "XZ compressed data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("Rar!")):
		return "RAR archive data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("\x7fELF")):
		// Covered by first case, but kept here so the reader sees it.
		return "ELF executable"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0xca, 0xfe, 0xba, 0xbe}):
		return "Java class data"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x00, 0x61, 0x73, 0x6d}):
		return "WebAssembly (wasm) binary module"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("OggS")):
		return "Ogg data"
	}
	return ""
}

// magicMIME maps the human-readable magic descriptions returned by
// detectMagic to MIME types. Keys are matched as prefixes so that
// suffixes like ", with extra info" do not break the mapping.
var magicMIME = []struct {
	prefix string
	mime   string
}{
	{"ELF", "application/x-executable"},
	{"PE32 executable", "application/vnd.microsoft.portable-executable"},
	{"gzip compressed", "application/gzip"},
	{"Zip archive", "application/zip"},
	{"JPEG image", "image/jpeg"},
	{"PNG image", "image/png"},
	{"GIF image", "image/gif"},
	{"PDF document", "application/pdf"},
	{"bzip2 compressed", "application/x-bzip2"},
	{"XZ compressed", "application/x-xz"},
	{"RAR archive", "application/vnd.rar"},
	{"Java class", "application/x-java-applet"},
	{"WebAssembly", "application/wasm"},
	{"Ogg data", "application/ogg"},
}

func detectFileType(filename string, data []byte) string {
	if len(data) == 0 {
		return "empty"
	}

	// Magic-byte detection takes precedence: it is more reliable than
	// extension hints for common binary formats and catches files where
	// http.DetectContentType returns "application/octet-stream".
	if magic := detectMagic(data); magic != "" {
		return magic
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
		fmt.Fprint(stderr, "  -b, --brief             do not prepend filenames to output\n")
		fmt.Fprint(stderr, "  -i, --mime              output MIME type strings\n")
		fmt.Fprint(stderr, "  -h, --no-dereference    do not follow symlinks (default)\n")
		fmt.Fprint(stderr, "  -L, --dereference       follow symlinks\n")
		fmt.Fprint(stderr, "      --help              display this help and exit\n")
	}
	set.SetUsage(usage)

	brief := set.BoolLong("brief", 'b', "do not prepend filenames to output")
	mimeMode := set.BoolLong("mime", 'i', "output MIME type strings")
	noDeref := set.BoolLong("no-dereference", 'h', "do not follow symlinks")
	deref := set.BoolLong("dereference", 'L', "follow symlinks")
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

	// `-L` forces dereferencing; `-h` disables it. When both are supplied
	// the last-wins behavior matches BSD `file(1)`. By default we follow
	// symlinks, matching the BSD convention.
	follow := true
	if *noDeref {
		follow = false
	}
	if *deref {
		follow = true
	}

	exitCode := 0

	for _, name := range files {
		p := resolvePath(ec, name)

		info, err := statPath(ec.FS, p, follow)
		if err != nil {
			if *brief {
				fmt.Fprintln(stdout, "cannot open")
			} else {
				fmt.Fprintf(stdout, "%s: cannot open (No such file or directory)\n", name)
			}
			exitCode = 1
			continue
		}

		// Symlink that we are not dereferencing.
		if info.Mode()&os.ModeSymlink != 0 {
			target := readlink(ec.FS, p)
			result := "symbolic link"
			if target != "" {
				result = "symbolic link to " + target
			}
			if *mimeMode {
				result = "inode/symlink"
			}
			if *brief {
				fmt.Fprintln(stdout, result)
			} else {
				fmt.Fprintf(stdout, "%s: %s\n", name, result)
			}
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
	case "symbolic link":
		return "inode/symlink"
	}

	// Magic-byte descriptions map to specific MIME types.
	for _, m := range magicMIME {
		if strings.HasPrefix(desc, m.prefix) {
			return m.mime
		}
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
