// Package checksum implements the shared logic for the md5sum, sha1sum,
// and sha256sum coreutils.
package checksum

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/pborman/getopt/v2"
	"mvdan.cc/sh/v3/interp"
	"tangled.org/xeiaso.net/kefka/command"
)

type Algorithm int

const (
	MD5 Algorithm = iota
	SHA1
	SHA256
)

func (a Algorithm) new() hash.Hash {
	switch a {
	case MD5:
		return md5.New()
	case SHA1:
		return sha1.New()
	case SHA256:
		return sha256.New()
	}
	panic("checksum: unknown algorithm")
}

// Config configures a single checksum command implementation.
type Config struct {
	Name      string
	Algorithm Algorithm
	Summary   string
}

func (c Config) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	if ec == nil {
		return errors.New(c.Name + ": nil ExecContext")
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
	set.SetProgram(c.Name)
	set.SetParameters("[FILE]...")

	usage := func() {
		fmt.Fprintf(stderr, "Usage: %s [OPTION]... [FILE]...\n", c.Name)
		fmt.Fprintf(stderr, "%s.\n\n", c.Summary)
		fmt.Fprint(stderr, "  -c, --check    read checksums from FILEs and check them\n")
		fmt.Fprint(stderr, "      --help     display this help and exit\n")
	}
	set.SetUsage(usage)

	check := set.BoolLong("check", 'c', "read checksums from FILEs and check them")
	set.BoolLong("binary", 'b', "read in binary mode (ignored)")
	set.BoolLong("text", 't', "read in text mode (ignored)")
	help := set.BoolLong("help", 0, "display this help and exit")

	if err := set.Getopt(append([]string{c.Name}, args...), nil); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", c.Name, err)
		usage()
		return interp.ExitStatus(1)
	}

	if *help {
		usage()
		return nil
	}

	files := set.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	if *check {
		return runCheck(c, ec, files, stdout, stderr)
	}
	return runHash(c, ec, files, stdout, stderr)
}

func runHash(c Config, ec *command.ExecContext, files []string, stdout, stderr io.Writer) error {
	exitCode := 0
	for _, file := range files {
		data, err := readBinary(ec, file)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: No such file or directory\n", c.Name, file)
			exitCode = 1
			continue
		}
		h := c.Algorithm.new()
		h.Write(data)
		fmt.Fprintf(stdout, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), file)
	}
	if exitCode != 0 {
		return interp.ExitStatus(uint8(exitCode))
	}
	return nil
}

var checksumLineRE = regexp.MustCompile(`^([a-fA-F0-9]+)\s+[* ]?(.+)$`)

func runCheck(c Config, ec *command.ExecContext, files []string, stdout, stderr io.Writer) error {
	failed := 0
	for _, file := range files {
		var content []byte
		var err error
		if file == "-" {
			content, err = readStdin(ec)
		} else {
			content, err = readFile(ec, file)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: No such file or directory\n", c.Name, file)
			return interp.ExitStatus(1)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			match := checksumLineRE.FindStringSubmatch(scanner.Text())
			if match == nil {
				continue
			}
			expected := strings.ToLower(match[1])
			target := match[2]

			data, err := readBinary(ec, target)
			if err != nil {
				fmt.Fprintf(stdout, "%s: FAILED open or read\n", target)
				failed++
				continue
			}
			h := c.Algorithm.new()
			h.Write(data)
			if hex.EncodeToString(h.Sum(nil)) == expected {
				fmt.Fprintf(stdout, "%s: OK\n", target)
			} else {
				fmt.Fprintf(stdout, "%s: FAILED\n", target)
				failed++
			}
		}
	}

	if failed > 0 {
		plural := ""
		if failed > 1 {
			plural = "s"
		}
		fmt.Fprintf(stdout, "%s: WARNING: %d computed checksum%s did NOT match\n", c.Name, failed, plural)
		return interp.ExitStatus(1)
	}
	return nil
}

func readBinary(ec *command.ExecContext, file string) ([]byte, error) {
	if file == "-" {
		return readStdin(ec)
	}
	return readFile(ec, file)
}

func readStdin(ec *command.ExecContext) ([]byte, error) {
	if ec.Stdin == nil {
		return nil, nil
	}
	return io.ReadAll(ec.Stdin)
}

func readFile(ec *command.ExecContext, file string) ([]byte, error) {
	if ec.FS == nil {
		return nil, errors.New("no filesystem")
	}
	f, err := ec.FS.Open(resolvePath(ec, file))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
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
