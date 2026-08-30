package wizard

import (
	"context"
	"errors"
	"fmt"

	"charm.land/huh/v2"
)

// ConfirmWrite asks the single final create or overwrite question. Creation is
// affirmative by default; overwrite is negative by default.
func ConfirmWrite(
	ctx context.Context,
	target string,
	overwrite bool,
	terminal Terminal,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !terminal.Interactive {
		return false, fmt.Errorf("%w on stdin and stderr", ErrTerminalRequired)
	}
	if terminal.Input == nil || terminal.Output == nil {
		return false, errors.New("interactive terminal input and output are required")
	}
	if target == "" {
		return false, errors.New("confirmation target is required")
	}

	confirmed := !overwrite
	action := "Create"
	if overwrite {
		action = "Overwrite existing"
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s %s?", action, target)).
			Value(&confirmed),
	))
	if err := form.
		WithInput(terminal.Input).
		WithOutput(terminal.Output).
		RunWithContext(ctx); err != nil {
		return false, translateFormError(ctx, err)
	}
	return confirmed, nil
}
