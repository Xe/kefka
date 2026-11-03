package coreutils

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Xe/kefka/command"
	"github.com/Xe/kefka/command/registry"
)

func TestGrepCommands(t *testing.T) {
	for _, tt := range []struct{ name, pattern, want string }{
		{"grep", "a.b", "a.b\naxb\n"},
		{"egrep", "a|b", "a.b\naxb\n"},
		{"fgrep", "a.b", "a.b\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reg := registry.New()
			Register(reg)
			cmd, ok := reg.Get(tt.name)
			if !ok {
				t.Fatalf("%s is not registered", tt.name)
			}
			var out, stderr bytes.Buffer
			err := cmd.Exec(context.Background(), &command.ExecContext{Stdin: strings.NewReader("a.b\naxb\n"), Stdout: &out, Stderr: &stderr}, []string{tt.pattern})
			if err != nil || out.String() != tt.want || stderr.Len() != 0 {
				t.Fatalf("got (%q, %q, %v), want %q", out.String(), stderr.String(), err, tt.want)
			}
		})
	}
}
