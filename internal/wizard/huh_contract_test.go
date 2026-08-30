package wizard

import (
	"errors"
	"io"
	"strings"
	"testing"

	"charm.land/huh/v2"
)

func TestHuhV2FieldContract(t *testing.T) {
	var (
		name    = "existing"
		secret  = "secret"
		choice  = "development"
		enabled = true
	)

	required := func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New("value is required")
		}
		return nil
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Description("Regular input").
				Placeholder("example").
				Value(&name).
				Validate(required),
			huh.NewInput().
				Title("Secret").
				Value(&secret).
				EchoMode(huh.EchoModePassword),
			huh.NewSelect[string]().
				Title("Environment").
				Options(huh.NewOptions("development", "staging", "production")...).
				Value(&choice),
			huh.NewConfirm().
				Title("Enabled?").
				Value(&enabled),
		).Title("Configuration"),
	).
		WithInput(strings.NewReader("")).
		WithOutput(io.Discard)

	if form == nil {
		t.Fatal("huh.NewForm returned nil")
	}
	if !errors.Is(huh.ErrUserAborted, huh.ErrUserAborted) {
		t.Fatal("huh.ErrUserAborted cannot be matched with errors.Is")
	}
}
