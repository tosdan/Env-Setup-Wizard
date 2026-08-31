package wizard

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"charm.land/huh/v2"
)

// SelectOutputPath asks whether the generated configuration should use the
// name derived from the Template or the default .env destination.
func SelectOutputPath(
	ctx context.Context,
	suggestedPath string,
	defaultPath string,
	terminal Terminal,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !terminal.Interactive {
		return "", fmt.Errorf("%w on stdin and stderr", ErrTerminalRequired)
	}
	if terminal.Input == nil || terminal.Output == nil {
		return "", errors.New("interactive terminal input and output are required")
	}
	if suggestedPath == "" || defaultPath == "" {
		return "", errors.New("output selection paths are required")
	}
	if suggestedPath == defaultPath {
		return "", errors.New("output selection paths must be different")
	}

	selected := suggestedPath
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Where should the configuration be saved?").
			Options(
				huh.NewOption(filepath.Base(suggestedPath)+" (next to template)", suggestedPath),
				huh.NewOption(filepath.Base(defaultPath)+" (current directory)", defaultPath),
			).
			Value(&selected),
	))
	if err := form.
		WithInput(terminal.Input).
		WithOutput(terminal.Output).
		RunWithContext(ctx); err != nil {
		return "", translateFormError(ctx, err)
	}
	return selected, nil
}
