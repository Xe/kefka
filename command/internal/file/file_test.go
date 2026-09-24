package file

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/Xe/kefka/command"
)

func newFS(t *testing.T) billy.Filesystem {
	t.Helper()
	fs := memfs.New()
	write := func(name string, data []byte) {
		f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Close()
	}
	write("hello.txt", []byte("hello world"))
	write("empty.txt", []byte{})
	write("script.sh", []byte("#!/bin/bash\necho hi\n"))
	write("python.py", []byte("#!/usr/bin/env python3\nprint('hi')\n"))
	write("data.json", []byte("{\"key\": \"value\"}\n"))
	write("index.html", []byte("<!doctype html><html></html>"))
	write("style.css", []byte("body { margin: 0; }"))
	write("readme.md", []byte("# Hello\n\nWorld"))
	write("crlf.txt", []byte("line1\r\nline2\r\n"))
	write("unicode.txt", []byte("Héllo wörld"))
	write("unicode.dat", []byte("Héllo wörld"))
	write("binary.bin", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}) // PNG header
	write("subdir/inner.txt", []byte("inner"))

	// ELF: \x7fELF, ELFCLASS64, EI_DATA LSB. Padded to 16 bytes (e_ident).
	write("hello.elf", []byte{
		0x7f, 'E', 'L', 'F',
		2, 1, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	})
	// gzip magic + minimal header.
	write("data.gz", []byte{
		0x1f, 0x8b, 0x08, 0x00,
		0, 0, 0, 0, 0, 0,
	})
	// PNG bytes saved to a name with no extension hint, to verify magic
	// detection runs even when extensions don't help.
	write("unknown.bin", []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0, 0, 0, 0,
	})
	// PDF.
	write("doc.pdf", []byte("%PDF-1.4\n"))
	// JPEG.
	write("photo.jpg", []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0})
	// Zip.
	write("archive.zip", []byte{'P', 'K', 0x03, 0x04, 0, 0, 0, 0})

	// Create directory manually
	fs.MkdirAll("mydir", 0o755)

	// Symlink (best-effort: memfs implements billy.Symlink).
	if sl, ok := fs.(billy.Symlink); ok {
		_ = sl.Symlink("hello.txt", "link.txt")
	}
	return fs
}

func TestExec(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "text file detection",
			args:       []string{"hello.txt"},
			wantStdout: "hello.txt: ASCII text\n",
		},
		{
			name:       "empty file",
			args:       []string{"empty.txt"},
			wantStdout: "empty.txt: empty\n",
		},
		{
			name:       "shell script with shebang",
			args:       []string{"script.sh"},
			wantStdout: "script.sh: Bourne-Again shell script, ASCII text executable\n",
		},
		{
			name:       "python script with shebang",
			args:       []string{"python.py"},
			wantStdout: "python.py: Python script, ASCII text executable\n",
		},
		{
			name:       "json file",
			args:       []string{"data.json"},
			wantStdout: "data.json: JSON data\n",
		},
		{
			name:       "html file",
			args:       []string{"index.html"},
			wantStdout: "index.html: HTML document\n",
		},
		{
			name:       "css file",
			args:       []string{"style.css"},
			wantStdout: "style.css: CSS stylesheet\n",
		},
		{
			name:       "markdown file",
			args:       []string{"readme.md"},
			wantStdout: "readme.md: Markdown document\n",
		},
		{
			name:       "crlf text",
			args:       []string{"crlf.txt"},
			wantStdout: "crlf.txt: ASCII text, with CRLF line terminators\n",
		},
		{
			name:       "unicode text no extension hint",
			args:       []string{"unicode.dat"},
			wantStdout: "unicode.dat: UTF-8 Unicode text\n",
		},
		{
			name:       "png binary",
			args:       []string{"binary.bin"},
			wantStdout: "binary.bin: PNG image data\n",
		},
		{
			name:       "directory",
			args:       []string{"mydir"},
			wantStdout: "mydir: directory\n",
		},
		{
			name:       "directory mime mode",
			args:       []string{"-i", "mydir"},
			wantStdout: "mydir: inode/directory\n",
		},
		{
			name:       "brief mode",
			args:       []string{"-b", "hello.txt"},
			wantStdout: "ASCII text\n",
		},
		{
			name:       "brief mode empty",
			args:       []string{"-b", "empty.txt"},
			wantStdout: "empty\n",
		},
		{
			name:       "mime mode text",
			args:       []string{"-i", "hello.txt"},
			wantStdout: "hello.txt: text/plain\n",
		},
		{
			name:       "mime mode json",
			args:       []string{"-i", "data.json"},
			wantStdout: "data.json: application/json\n",
		},
		{
			name:       "mime mode html",
			args:       []string{"-i", "index.html"},
			wantStdout: "index.html: text/html\n",
		},
		{
			name:       "brief mime combined",
			args:       []string{"-bi", "data.json"},
			wantStdout: "application/json\n",
		},
		{
			name:       "multiple files",
			args:       []string{"hello.txt", "empty.txt"},
			wantStdout: "hello.txt: ASCII text\nempty.txt: empty\n",
		},
		{
			name:       "missing file",
			args:       []string{"nope.txt"},
			wantStdout: "nope.txt: cannot open (No such file or directory)\n",
			wantErr:    true,
		},
		{
			name:       "missing file brief",
			args:       []string{"-b", "nope.txt"},
			wantStdout: "cannot open\n",
			wantErr:    true,
		},
		{
			name:       "no files prints usage",
			args:       []string{},
			wantStderr: "Usage: file [-bLi] FILE...\n",
			wantErr:    true,
		},
		{
			name: "help flag",
			args: []string{"--help"},
			wantStderr: "Usage: file [OPTION]... FILE...\n" +
				"Determine file type.\n\n" +
				"  -b, --brief             do not prepend filenames to output\n" +
				"  -i, --mime              output MIME type strings\n" +
				"  -h, --no-dereference    do not follow symlinks (default)\n" +
				"  -L, --dereference       follow symlinks\n" +
				"      --help              display this help and exit\n",
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: true,
			wantStderr: "file: unknown option: --nope\n" +
				"Usage: file [OPTION]... FILE...\n" +
				"Determine file type.\n\n" +
				"  -b, --brief             do not prepend filenames to output\n" +
				"  -i, --mime              output MIME type strings\n" +
				"  -h, --no-dereference    do not follow symlinks (default)\n" +
				"  -L, --dereference       follow symlinks\n" +
				"      --help              display this help and exit\n",
		},
		{
			name:       "subdirectory file",
			args:       []string{"subdir/inner.txt"},
			wantStdout: "subdir/inner.txt: ASCII text\n",
		},
		{
			name:       "dereference flag accepted",
			args:       []string{"-L", "hello.txt"},
			wantStdout: "hello.txt: ASCII text\n",
		},
		{
			name:       "elf magic",
			args:       []string{"hello.elf"},
			wantStdout: "hello.elf: ELF 64-bit LSB executable\n",
		},
		{
			name:       "gzip magic",
			args:       []string{"data.gz"},
			wantStdout: "data.gz: gzip compressed data\n",
		},
		{
			name:       "pdf magic",
			args:       []string{"doc.pdf"},
			wantStdout: "doc.pdf: PDF document\n",
		},
		{
			name:       "jpeg magic",
			args:       []string{"photo.jpg"},
			wantStdout: "photo.jpg: JPEG image data\n",
		},
		{
			name:       "zip magic",
			args:       []string{"archive.zip"},
			wantStdout: "archive.zip: Zip archive data\n",
		},
		{
			name:       "binary content no extension",
			args:       []string{"unknown.bin"},
			wantStdout: "unknown.bin: PNG image data\n",
		},
		{
			name:       "mime mode elf",
			args:       []string{"-i", "hello.elf"},
			wantStdout: "hello.elf: application/x-executable\n",
		},
		{
			name:       "mime mode gzip",
			args:       []string{"-i", "data.gz"},
			wantStdout: "data.gz: application/gzip\n",
		},
		{
			name:       "mime mode png unknown extension",
			args:       []string{"-i", "unknown.bin"},
			wantStdout: "unknown.bin: image/png\n",
		},
		{
			name:       "brief mime gzip",
			args:       []string{"-bi", "data.gz"},
			wantStdout: "application/gzip\n",
		},
		{
			name:       "symlink no-dereference",
			args:       []string{"-h", "link.txt"},
			wantStdout: "link.txt: symbolic link to hello.txt\n",
		},
		{
			name:       "symlink no-dereference brief",
			args:       []string{"-bh", "link.txt"},
			wantStdout: "symbolic link to hello.txt\n",
		},
		{
			name:       "symlink no-dereference mime",
			args:       []string{"-hi", "link.txt"},
			wantStdout: "link.txt: inode/symlink\n",
		},
		{
			name:       "symlink follows by default",
			args:       []string{"link.txt"},
			wantStdout: "link.txt: ASCII text\n",
		},
		{
			name:       "L overrides h when both given",
			args:       []string{"-h", "-L", "link.txt"},
			wantStdout: "link.txt: ASCII text\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			ec := &command.ExecContext{
				Stdout: &stdout,
				Stderr: &stderr,
				Dir:    ".",
				FS:     newFS(t),
			}
			err := Impl{}.Exec(context.Background(), ec, tc.args)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("stdout mismatch\nwant: %q\ngot:  %q", tc.wantStdout, got)
			}
			if got := stderr.String(); got != tc.wantStderr {
				t.Errorf("stderr mismatch\nwant: %q\ngot:  %q", tc.wantStderr, got)
			}
		})
	}
}

func TestExec_NilContext(t *testing.T) {
	if err := (Impl{}).Exec(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil ExecContext")
	}
}

func TestExec_NoFS(t *testing.T) {
	ec := &command.ExecContext{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	if err := (Impl{}).Exec(context.Background(), ec, nil); err == nil {
		t.Fatal("expected error for missing filesystem")
	}
}
