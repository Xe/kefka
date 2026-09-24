package wasmcommand

import (
	"bytes"
	"context"
	"testing"

	"github.com/Xe/kefka/command"
	"github.com/go-git/go-billy/v6/memfs"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wasm    []byte
		wantErr bool
	}{
		{
			name: "valid wasm",
			// Minimal module with an empty _start function.
			wasm: []byte{
				0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
				0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
				0x03, 0x02, 0x01, 0x00,
				0x07, 0x0a, 0x01, 0x06, '_', 's', 't', 'a', 'r', 't', 0x00, 0x00,
				0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
			},
		},
		{
			name:    "invalid wasm",
			wasm:    []byte("not wasm"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl, err := New("testcmd", tt.wasm)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			var stdout, stderr bytes.Buffer
			err = impl.Exec(context.Background(), &command.ExecContext{
				Stdin:  bytes.NewReader(nil),
				Stdout: &stdout,
				Stderr: &stderr,
				FS:     memfs.New(),
			}, []string{"argument"})
			if err != nil {
				t.Fatalf("Exec() error = %v, want nil", err)
			}
			if got := stdout.String(); got != "" {
				t.Errorf("stdout = %q, want empty", got)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}
