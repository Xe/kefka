package grep

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Xe/kefka/command"
	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/interp"
)

func TestPerlTranslation(t *testing.T) {
	for _, tt := range []struct{ name, pattern, input, want string }{
		{"reset before capture", `foo\K(bar)\1`, "foobar\nfoobarbar\n", "barbar\n"},
		{"reset between captures", `(foo)\K(bar)\2\1`, "foobarbarfoo\nfoobarfoo\n", "barbarfoo\n"},
		{"reset after captures", `(foo)(bar)\K\2\1`, "foobarbarfoo\n", "barfoo\n"},
		{"reset and named capture", `foo\K(?<word>bar)\1`, "foobar\nfoobarbar\n", "barbar\n"},
		{"reset and named reference", `foo\K(?<word>bar)\k<word>`, "foobar\nfoobarbar\n", "barbar\n"},
		{"decimal escape falls back to octal", `(a)\K\11`, "aa1\na\t\n", "\t\n"},
		{"decimal escape with nonoctal suffix", `(a)\K\18`, "aa8\na\x018\n", "\x018\n"},
		{"reset leaves octal escape", `foo\K(bar)\2`, "foobar\x02\nfoobarbar\n", "bar\x02\n"},
		{"hex after nul", `\0\x{31}`, "\x001\n\x01\n", "\x001\n"},
		{"hex after backreference", `(a)\1\x{31}`, "aa1\na\t\n", "aa1\n"},
		{"hex quantifier", `\x{31}+`, "111\n", "111\n"},
		{"hex digit in class", `[\0\x{31}]`, "\x001\x01\n", "\x00\n1\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader(tt.input), Stdout: &out, Stderr: &stderr}, []string{"-Po", tt.pattern})
			if err != nil || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got (%q,%q,%v), want %q", out.String(), stderr.String(), err, tt.want)
			}
		})
	}
}

func TestDiscoveredDashFile(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"glob", []string{"apple", "*"}, "apple file\n"},
		{"recursive", []string{"-r", "apple", "."}, "./-:apple file\n"},
		{"explicit stdin", []string{"apple", "-"}, "apple stdin\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := memfs.New()
			f, err := fs.Create("-")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(f, "apple file\n"); err != nil {
				t.Fatal(err)
			}
			f.Close()
			var out, stderr bytes.Buffer
			err = Impl{}.Exec(context.Background(), &command.ExecContext{FS: fs, Stdin: strings.NewReader("apple stdin\n"), Stdout: &out, Stderr: &stderr}, tt.args)
			if err != nil || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got (%q,%q,%v), want %q", out.String(), stderr.String(), err, tt.want)
			}
		})
	}
}

func TestEarlyExitOpenPipe(t *testing.T) {
	for _, tt := range []struct {
		name        string
		args        []string
		input, want string
	}{
		{"quiet", []string{"-q", "hit"}, "hit\n", ""},
		{"max count", []string{"-m1", "hit"}, "hit\n", "hit\n"},
		{"max count trailing context", []string{"-nm1", "-A2", "hit"}, "before\nhit\nhit\nafter\n", "2:hit\n3-hit\n4-after\n"},
		{"max count count mode", []string{"-cm1", "-A2", "hit"}, "hit\n", "1\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, w := io.Pipe()
			defer r.Close()
			defer w.Close()
			var out, stderr bytes.Buffer
			done := make(chan error, 1)
			go func() {
				done <- Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: r, Stdout: &out, Stderr: &stderr}, tt.args)
			}()
			if _, err := io.WriteString(w, tt.input); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil || stderr.Len() != 0 || out.String() != tt.want {
					t.Fatalf("got (%q,%q,%v), want %q", out.String(), stderr.String(), err, tt.want)
				}
			case <-time.After(time.Second):
				w.Close()
				<-done
				t.Fatal("grep waited for EOF after satisfying its match limit")
			}
		})
	}
}

func TestOccurrenceLimits(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		files   map[string]string
		wantErr string
	}{
		{"amplified output", []string{"-on", "a", strings.Repeat("f", 1024), "empty"}, map[string]string{strings.Repeat("f", 1024): strings.Repeat("a", 10000) + "\n", "empty": ""}, "output size limit exceeded"},
		{"occurrence count", []string{"-co", "a", "input"}, map[string]string{"input": strings.Repeat("a", 100001) + "\n"}, "match result limit exceeded"},
		{"aggregate across files", []string{"-co", "a", "a", "b"}, map[string]string{"a": strings.Repeat("a", 60000) + "\n", "b": strings.Repeat("a", 60000) + "\n"}, "match result limit exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := memfs.New()
			for n, s := range tt.files {
				f, e := fs.Create(n)
				if e != nil {
					t.Fatal(e)
				}
				if _, e := io.WriteString(f, s); e != nil {
					t.Fatal(e)
				}
				f.Close()
			}
			var stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{FS: fs, Stdout: io.Discard, Stderr: &stderr}, tt.args)
			if !errors.Is(err, interp.ExitStatus(2)) || !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("got (%q,%v), want %s", stderr.String(), err, tt.wantErr)
			}
		})
	}
}

func TestStreamingBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name        string
		args        []string
		input, want string
	}{
		{"repeated stdin stays claimed after max count", []string{"-cm1", "hit", "-", "-"}, "hit one\nhit two\n", "(standard input):1\n(standard input):0\n"},
		{"rolling before context", []string{"-nB2", "hit"}, "old\na\nb\nhit\nc\nd\ne\nhit\n", "2-a\n3-b\n4:hit\n--\n6-d\n7-e\n8:hit\n"},
		{"adjacent context groups", []string{"-nC1", "hit"}, "hit\na\nb\nhit\n", "1:hit\n2-a\n3-b\n4:hit\n"},
		{"max count context overlap", []string{"-nm2", "-C2", "hit"}, "a\nhit\nhit\nhit\nb\nc\n", "1-a\n2:hit\n3:hit\n4-hit\n5-b\n"},
		{"carriage return preserved", []string{"-F", "hit"}, "hit\r\nmiss\r\n", "hit\r\n"},
		{"line longer than scanner default", []string{"-x", strings.Repeat("a", 70000)}, strings.Repeat("a", 70000), strings.Repeat("a", 70000) + "\n"},
		{"utf8 across buffer boundary", []string{"-Po", `\x{1F600}`}, strings.Repeat("a", 4095) + "\U0001f600\n", "\U0001f600\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader(tt.input), Stdout: &out, Stderr: &stderr}, tt.args)
			if err != nil || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got output length %d, stderr %q, error %v; want output length %d", out.Len(), stderr.String(), err, len(tt.want))
			}
		})
	}
}

func TestInputAndContextLimits(t *testing.T) {
	for _, tt := range []struct {
		name           string
		args           []string
		input, wantErr string
	}{
		{"line size", []string{"x"}, strings.Repeat("a", (8<<20)+1), "input line size limit exceeded"},
		{"context bytes", []string{"-B10", "hit"}, strings.Repeat(strings.Repeat("a", 1<<20)+"\n", 9), "context size limit exceeded"},
		{"matching work", []string{"x"}, strings.Repeat("a\n", 1000001), "matching work limit exceeded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader(tt.input), Stdout: io.Discard, Stderr: &stderr}, tt.args)
			if !errors.Is(err, interp.ExitStatus(2)) || !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("got (%q,%v), want %s", stderr.String(), err, tt.wantErr)
			}
		})
	}
}

func TestListingIgnoresContext(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"quiet", []string{"-qB100001", "hit", "-"}, ""},
		{"list", []string{"-lB100001", "hit", "-"}, "(standard input)\n"},
		{"list without", []string{"-LB100001", "hit", "-"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			err := Impl{}.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader(strings.Repeat("\n", 100001) + "hit\n"), Stdout: &out, Stderr: &stderr}, tt.args)
			if err != nil || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got (%q,%q,%v), want %q", out.String(), stderr.String(), err, tt.want)
			}
		})
	}
}
