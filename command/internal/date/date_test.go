package date

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"tangled.org/xeiaso.net/kefka/command"
)

func run(t *testing.T, args []string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	ec := &command.ExecContext{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Dir:    ".",
	}
	err := Impl{}.Exec(context.Background(), ec, args)
	return stdout.String(), stderr.String(), err
}

func TestDate(t *testing.T) {
	// 2025-01-15T12:00:00Z = Wednesday, unix 1736942400
	const fixed = "2025-01-15T12:00:00Z"

	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantErrSub string
		wantErr    bool
	}{
		{
			name:       "default format",
			args:       []string{"-d", fixed},
			wantStdout: "Wed Jan 15 12:00:00 UTC 2025\n",
		},
		{
			name:       "utc flag is accepted",
			args:       []string{"-u", "-d", fixed},
			wantStdout: "Wed Jan 15 12:00:00 UTC 2025\n",
		},
		{
			name:       "iso 8601 short form",
			args:       []string{"-I", "-d", fixed},
			wantStdout: "2025-01-15T12:00:00+0000\n",
		},
		{
			name:       "iso 8601 long form",
			args:       []string{"--iso-8601", "-d", fixed},
			wantStdout: "2025-01-15T12:00:00+0000\n",
		},
		{
			name:       "rfc email short form",
			args:       []string{"-R", "-d", fixed},
			wantStdout: "Wed, 15 Jan 2025 12:00:00 +0000\n",
		},
		{
			name:       "rfc email long form",
			args:       []string{"--rfc-email", "-d", fixed},
			wantStdout: "Wed, 15 Jan 2025 12:00:00 +0000\n",
		},
		{
			name:       "custom format year",
			args:       []string{"-d", fixed, "+%Y"},
			wantStdout: "2025\n",
		},
		{
			name:       "custom format full date",
			args:       []string{"-d", fixed, "+%F"},
			wantStdout: "2025-01-15\n",
		},
		{
			name:       "custom format time",
			args:       []string{"-d", fixed, "+%T"},
			wantStdout: "12:00:00\n",
		},
		{
			name:       "custom format hour minute",
			args:       []string{"-d", fixed, "+%R"},
			wantStdout: "12:00\n",
		},
		{
			name:       "custom format unix seconds",
			args:       []string{"-d", fixed, "+%s"},
			wantStdout: "1736942400\n",
		},
		{
			name:       "custom format weekday name",
			args:       []string{"-d", fixed, "+%a"},
			wantStdout: "Wed\n",
		},
		{
			name:       "custom format month name",
			args:       []string{"-d", fixed, "+%b"},
			wantStdout: "Jan\n",
		},
		{
			name:       "custom format month name h alias",
			args:       []string{"-d", fixed, "+%h"},
			wantStdout: "Jan\n",
		},
		{
			name:       "custom format day zero padded",
			args:       []string{"-d", "2025-01-05T00:00:00Z", "+%d"},
			wantStdout: "05\n",
		},
		{
			name:       "custom format day space padded",
			args:       []string{"-d", "2025-01-05T00:00:00Z", "+%e"},
			wantStdout: " 5\n",
		},
		{
			name:       "custom format 12-hour clock at noon",
			args:       []string{"-d", fixed, "+%I"},
			wantStdout: "12\n",
		},
		{
			name:       "custom format 12-hour clock at midnight",
			args:       []string{"-d", "2025-01-15T00:00:00Z", "+%I"},
			wantStdout: "12\n",
		},
		{
			name:       "custom format 12-hour clock at 13",
			args:       []string{"-d", "2025-01-15T13:00:00Z", "+%I"},
			wantStdout: "01\n",
		},
		{
			name:       "custom format am pm uppercase",
			args:       []string{"-d", fixed, "+%p"},
			wantStdout: "PM\n",
		},
		{
			name:       "custom format am pm lowercase",
			args:       []string{"-d", "2025-01-15T08:00:00Z", "+%P"},
			wantStdout: "am\n",
		},
		{
			name:       "custom format iso weekday",
			args:       []string{"-d", "2025-01-19T00:00:00Z", "+%u"},
			wantStdout: "7\n",
		},
		{
			name:       "custom format weekday number",
			args:       []string{"-d", "2025-01-19T00:00:00Z", "+%w"},
			wantStdout: "0\n",
		},
		{
			name:       "custom format two digit year",
			args:       []string{"-d", fixed, "+%y"},
			wantStdout: "25\n",
		},
		{
			name:       "custom format timezone offset",
			args:       []string{"-d", fixed, "+%z"},
			wantStdout: "+0000\n",
		},
		{
			name:       "custom format timezone name",
			args:       []string{"-d", fixed, "+%Z"},
			wantStdout: "UTC\n",
		},
		{
			name:       "custom format literal percent",
			args:       []string{"-d", fixed, "+%%"},
			wantStdout: "%\n",
		},
		{
			name:       "custom format newline directive",
			args:       []string{"-d", fixed, "+a%nb"},
			wantStdout: "a\nb\n",
		},
		{
			name:       "custom format tab directive",
			args:       []string{"-d", fixed, "+a%tb"},
			wantStdout: "a\tb\n",
		},
		{
			name:       "custom format unknown directive passes through",
			args:       []string{"-d", fixed, "+%Q"},
			wantStdout: "%Q\n",
		},
		{
			name:       "custom format combined",
			args:       []string{"-d", fixed, "+%Y-%m-%d %H:%M:%S"},
			wantStdout: "2025-01-15 12:00:00\n",
		},
		{
			name:       "long form date equals",
			args:       []string{"--date=" + fixed, "+%Y"},
			wantStdout: "2025\n",
		},
		{
			name:       "long form date space",
			args:       []string{"--date", fixed, "+%Y"},
			wantStdout: "2025\n",
		},
		{
			name:       "unix timestamp date string",
			args:       []string{"-d", "1736942400", "+%Y-%m-%dT%H:%M:%SZ"},
			wantStdout: "2025-01-15T12:00:00Z\n",
		},
		{
			name:       "date keyword now is accepted",
			args:       []string{"-d", "now", "+%Y"},
			wantStdout: "", // year depends on real clock; only assert non-empty below
		},
		{
			name:       "invalid date string",
			args:       []string{"-d", "not a real date"},
			wantErrSub: "invalid date 'not a real date'",
			wantErr:    true,
		},
		{
			name:    "unknown flag errors",
			args:    []string{"--no-such-flag"},
			wantErr: true,
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
			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
			if tt.wantStdout == "" && !tt.wantErr && stdout == "" {
				t.Errorf("expected non-empty stdout, got empty")
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr, tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantErrSub)
			}
		})
	}
}

func TestRelativeKeywords(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"now", "now"},
		{"today", "today"},
		{"yesterday", "yesterday"},
		{"tomorrow", "tomorrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := run(t, []string{"-d", tt.arg, "+%Y-%m-%d"})
			if err != nil {
				t.Fatalf("unexpected error: %v; stderr=%q", err, stderr)
			}
			s := strings.TrimSpace(stdout)
			if _, err := time.Parse("2006-01-02", s); err != nil {
				t.Errorf("expected ISO date, got %q (parse err: %v)", s, err)
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
	if !strings.Contains(stderr, "Usage: date [OPTION]... [+FORMAT]") {
		t.Errorf("usage line missing from stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "--iso-8601") {
		t.Errorf("--iso-8601 missing from help: %q", stderr)
	}
}
