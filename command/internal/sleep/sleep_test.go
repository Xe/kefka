package sleep

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, ctx context.Context, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(ctx, ec, args)
	return stdout.String(), stderr.String(), err
}

func TestSleep(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErrSub string
		wantErr    bool
	}{
		{
			name: "default seconds suffix",
			args: []string{"0.001"},
		},
		{
			name: "explicit seconds",
			args: []string{"0.001s"},
		},
		{
			name: "zero seconds",
			args: []string{"0"},
		},
		{
			name: "multiple args sum",
			args: []string{"0.001s", "0.001s"},
		},
		{
			name: "decimal number",
			args: []string{"0.0001"},
		},
		{
			name:       "missing operand",
			args:       []string{},
			wantErrSub: "missing operand",
			wantErr:    true,
		},
		{
			name:       "invalid time interval",
			args:       []string{"5x"},
			wantErrSub: "invalid time interval '5x'",
			wantErr:    true,
		},
		{
			name:       "negative number",
			args:       []string{"-5"},
			wantErr:    true,
		},
		{
			name:       "empty string",
			args:       []string{""},
			wantErrSub: "invalid time interval ''",
			wantErr:    true,
		},
		{
			name:       "trailing junk",
			args:       []string{"5sm"},
			wantErrSub: "invalid time interval '5sm'",
			wantErr:    true,
		},
		{
			name:       "scientific notation rejected",
			args:       []string{"1e3"},
			wantErrSub: "invalid time interval '1e3'",
			wantErr:    true,
		},
		{
			name:       "second arg invalid stops sum",
			args:       []string{"0.001", "bogus"},
			wantErrSub: "invalid time interval 'bogus'",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; stdout=%q stderr=%q", stdout, stderr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			if stdout != "" {
				t.Errorf("expected empty stdout, got %q", stdout)
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

func TestSuffixes(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want time.Duration
	}{
		{"seconds default", "1", time.Second},
		{"seconds explicit", "1s", time.Second},
		{"minutes", "1m", time.Minute},
		{"hours", "1h", time.Hour},
		{"days clamped to max", "1d", time.Hour},
		{"decimal seconds", "0.5s", 500 * time.Millisecond},
		{"trailing dot", "5.", 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, ok := parseDuration(tt.arg)
			if !ok {
				t.Fatalf("parseDuration(%q) returned !ok", tt.arg)
			}
			// Days exceeds the cap; the cap logic lives in Exec, not parseDuration,
			// so for parsing we just verify the raw conversion.
			if tt.name == "days clamped to max" {
				if d != 24*time.Hour {
					t.Errorf("parseDuration(%q) = %v, want %v", tt.arg, d, 24*time.Hour)
				}
				return
			}
			if d != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.arg, d, tt.want)
			}
		})
	}
}

func TestCancelAlreadyDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, _, err := run(t, ctx, []string{"1d"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("pre-cancelled ctx slept for %v, expected immediate return", elapsed)
	}
}

func TestCancelDuringSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := run(t, ctx, []string{"1h"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("cancel during sleep took %v, expected ~10ms", elapsed)
	}
}

func TestCapAtOneHour(t *testing.T) {
	// Combined duration > 1h. With an immediately-cancelled ctx the call returns
	// fast; if the cap weren't in place the timer would still be set to the
	// uncapped duration, but that's not observable here. The point of this
	// test is that we don't reject "over cap" inputs as errors.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, stderr, err := run(t, ctx, []string{"2h", "30m"})
	if err != nil {
		t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	stdout, stderr, err := run(t, context.Background(), []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Usage: sleep NUMBER[SUFFIX]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Errorf("help flag missing from help: %q", stderr)
	}
	for _, suffix := range []string{"s - seconds", "m - minutes", "h - hours", "d - days"} {
		if !strings.Contains(stderr, suffix) {
			t.Errorf("suffix line %q missing from help: %q", suffix, stderr)
		}
	}
}

func TestUnknownFlag(t *testing.T) {
	_, stderr, err := run(t, context.Background(), []string{"--no-such-flag", "1s"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(stderr, "sleep:") {
		t.Errorf("expected sleep prefix in stderr, got %q", stderr)
	}
}
