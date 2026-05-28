package registry

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// TestExec_CommandNotFound checks that an unknown command produces the
// diagnostic on stderr exactly once and returns interp.ExitStatus(127) whose
// Error() string does not repeat the diagnostic.
//
// If the returned error also carries the "command not found" text, any caller
// that prints the error from sh.Run (as cmd/sophia does) ends up printing the
// message twice.
func TestExec_CommandNotFound(t *testing.T) {
	reg := New()
	fsys := memfs.New()

	var sh *interp.Runner
	var stderr bytes.Buffer
	middleware := func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			return reg.Exec(ctx, fsys, sh, args)
		}
	}
	var err error
	sh, err = interp.New(
		interp.StdIO(nil, nil, &stderr),
		interp.ExecHandlers(middleware),
	)
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}

	prog, err := syntax.NewParser().Parse(strings.NewReader("nope-not-here"), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	runErr := sh.Run(context.Background(), prog)

	var status interp.ExitStatus
	if !errors.As(runErr, &status) {
		t.Fatalf("err = %v, want ExitStatus", runErr)
	}
	if uint8(status) != 127 {
		t.Errorf("exit = %d, want 127", uint8(status))
	}

	if !strings.Contains(stderr.String(), "kefka: command not found: nope-not-here") {
		t.Errorf("stderr missing not-found diagnostic: %q", stderr.String())
	}

	if strings.Contains(runErr.Error(), "command not found") {
		t.Errorf("returned error duplicates stderr diagnostic; err.Error() = %q", runErr.Error())
	}
}
