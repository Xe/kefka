package seq

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestSeq(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "single LAST arg counts from 1",
			args:       []string{"5"},
			wantStdout: "1\n2\n3\n4\n5\n",
		},
		{
			name:       "FIRST LAST",
			args:       []string{"3", "5"},
			wantStdout: "3\n4\n5\n",
		},
		{
			name:       "FIRST INCREMENT LAST",
			args:       []string{"1", "2", "9"},
			wantStdout: "1\n3\n5\n7\n9\n",
		},
		{
			name:       "negative increment counts down",
			args:       []string{"5", "-1", "1"},
			wantStdout: "5\n4\n3\n2\n1\n",
		},
		{
			name:       "negative FIRST passes through as positional",
			args:       []string{"-3", "0"},
			wantStdout: "-3\n-2\n-1\n0\n",
		},
		{
			name:       "FIRST greater than LAST yields nothing",
			args:       []string{"5", "1"},
			wantStdout: "",
		},
		{
			name:       "single FIRST equals LAST",
			args:       []string{"3", "3"},
			wantStdout: "3\n",
		},
		{
			name:       "custom separator -s",
			args:       []string{"-s", " ", "1", "5"},
			wantStdout: "1 2 3 4 5\n",
		},
		{
			name:       "attached separator -sSTRING",
			args:       []string{"-s,", "1", "3"},
			wantStdout: "1,2,3\n",
		},
		{
			name:       "equalize width -w",
			args:       []string{"-w", "8", "10"},
			wantStdout: "08\n09\n10\n",
		},
		{
			name:       "equalize width pads negatives with leading zeros",
			args:       []string{"-w", "-1", "10"},
			wantStdout: "-01\n00\n01\n02\n03\n04\n05\n06\n07\n08\n09\n10\n",
		},
		{
			name:       "float precision matches widest input",
			args:       []string{"1", "0.5", "3"},
			wantStdout: "1.0\n1.5\n2.0\n2.5\n3.0\n",
		},
		{
			name:       "double dash terminates option parsing",
			args:       []string{"--", "1", "3"},
			wantStdout: "1\n2\n3\n",
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantErrSub: "seq: missing operand",
			wantErr:    true,
		},
		{
			name:       "zero increment is rejected",
			args:       []string{"1", "0", "5"},
			wantErrSub: "seq: invalid Zero increment value: '0'",
			wantErr:    true,
		},
		{
			name:       "non-numeric argument is rejected",
			args:       []string{"abc"},
			wantErrSub: "seq: invalid floating point argument: 'abc'",
			wantErr:    true,
		},
		{
			name:       "non-numeric increment is rejected",
			args:       []string{"1", "x", "5"},
			wantErrSub: "seq: invalid floating point argument: 'x'",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, tt.wantStdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: seq") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "-s STRING") {
		t.Errorf("-s flag missing from help: %q", stderr)
	}
	if !strings.Contains(stderr, "-w") {
		t.Errorf("-w flag missing from help: %q", stderr)
	}
}
