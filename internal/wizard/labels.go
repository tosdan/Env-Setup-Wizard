package wizard

import (
	"fmt"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

// variableLabel identifies a variable consistently in questions and summaries.
func variableLabel(variable domain.Variable) string {
	prompt := variable.Annotations.Prompt
	if prompt == "" || prompt == variable.Key {
		return variable.Key
	}
	return fmt.Sprintf("%s (%s)", prompt, variable.Key)
}
