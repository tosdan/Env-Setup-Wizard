package wizard_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

func TestConfirmWriteUsesCreateAndOverwriteDefaults(t *testing.T) {
	t.Setenv("TERM", "dumb")
	tests := []struct {
		name      string
		overwrite bool
		input     string
		want      bool
		prompt    string
		options   string
	}{
		{name: "create default yes", input: "\n", want: true, prompt: "Create .env?", options: "[Y/n]"},
		{name: "create explicit no", input: "n\n", want: false, prompt: "Create .env?", options: "[Y/n]"},
		{name: "overwrite default no", overwrite: true, input: "\n", want: false, prompt: "Overwrite existing .env?", options: "[y/N]"},
		{name: "overwrite explicit yes", overwrite: true, input: "y\n", want: true, prompt: "Overwrite existing .env?", options: "[y/N]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			confirmed, err := wizard.ConfirmWrite(
				context.Background(),
				".env",
				tt.overwrite,
				wizard.Terminal{
					Input:       strings.NewReader(tt.input),
					Output:      &output,
					Interactive: true,
				},
			)
			if err != nil {
				t.Fatalf("ConfirmWrite() error = %v, want nil", err)
			}
			if confirmed != tt.want {
				t.Fatalf("ConfirmWrite() = %t, want %t", confirmed, tt.want)
			}
			if !strings.Contains(output.String(), tt.prompt) || !strings.Contains(output.String(), tt.options) {
				t.Fatalf("confirmation output = %q, want prompt %q and options %q", output.String(), tt.prompt, tt.options)
			}
		})
	}
}

func TestConfirmWriteRequiresTerminalAndHonorsContext(t *testing.T) {
	if _, err := wizard.ConfirmWrite(context.Background(), ".env", false, wizard.Terminal{}); !errors.Is(err, wizard.ErrTerminalRequired) {
		t.Fatalf("ConfirmWrite() error = %v, want ErrTerminalRequired", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := wizard.ConfirmWrite(ctx, ".env", false, wizard.Terminal{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ConfirmWrite(canceled context) error = %v, want context.Canceled", err)
	}
}
