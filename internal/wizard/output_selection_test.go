package wizard_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

func TestSelectOutputPathOffersDerivedNameFirst(t *testing.T) {
	t.Setenv("TERM", "dumb")
	root := t.TempDir()
	suggested := filepath.Join(root, "typed-values.env")
	fallback := filepath.Join(root, ".env")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "matching template", input: "1\n", want: suggested},
		{name: "project default", input: "2\n", want: fallback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := wizard.SelectOutputPath(
				context.Background(),
				suggested,
				fallback,
				wizard.Terminal{
					Input:       iotest.OneByteReader(strings.NewReader(tt.input)),
					Output:      &output,
					Interactive: true,
				},
			)
			if err != nil {
				t.Fatalf("SelectOutputPath() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("SelectOutputPath() = %q, want %q", got, tt.want)
			}
			for _, text := range []string{
				"Where should the configuration be saved?",
				"typed-values.env (next to template)",
				".env (current directory)",
			} {
				if !strings.Contains(output.String(), text) {
					t.Errorf("selection output = %q, want %q", output.String(), text)
				}
			}
		})
	}
}

func TestSelectOutputPathRequiresTerminalAndHonorsContext(t *testing.T) {
	if _, err := wizard.SelectOutputPath(context.Background(), "named.env", ".env", wizard.Terminal{}); !errors.Is(err, wizard.ErrTerminalRequired) {
		t.Fatalf("SelectOutputPath() error = %v, want ErrTerminalRequired", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := wizard.SelectOutputPath(ctx, "named.env", ".env", wizard.Terminal{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("SelectOutputPath(canceled context) error = %v, want context.Canceled", err)
	}
}
