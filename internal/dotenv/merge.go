package dotenv

import (
	"fmt"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/validation"
)

// MergeExisting returns a copy of the template Document with compatible
// existing values applied. Incompatible values retain the valid template
// default and receive a presentation-safe recovery diagnostic.
func MergeExisting(document domain.Document, values map[string]string) (domain.Document, error) {
	merged := document
	merged.Nodes = append([]domain.Node(nil), document.Nodes...)

	for index, node := range merged.Nodes {
		variable, ok := node.(domain.Variable)
		if !ok || variable.Annotations.Fixed {
			continue
		}

		existingValue, found := values[variable.Key]
		if !found {
			continue
		}
		if err := validateExistingValue(variable, existingValue); err != nil {
			variable.ExistingValueIssue = incompatibleExistingValueIssue(variable, existingValue, err)
			merged.Nodes[index] = variable
			continue
		}

		if err := UpdateValue(&merged, variable.Key, existingValue, domain.ValueFromExisting); err != nil {
			return domain.Document{}, fmt.Errorf("merge existing variable %q: %w", variable.Key, err)
		}
	}

	return merged, nil
}

func validateExistingValue(variable domain.Variable, value string) error {
	if err := validation.ValidateQuestion(domain.Question{
		Key:      variable.Key,
		Type:     variable.Annotations.Type,
		Required: variable.Annotations.Required,
		Secret:   variable.Annotations.Secret,
		Options:  variable.Annotations.Options,
	}, value); err != nil {
		return err
	}
	_, err := EncodeValue(value)
	return err
}

func incompatibleExistingValueIssue(
	variable domain.Variable,
	value string,
	cause error,
) *domain.ExistingValueIssue {
	if variable.Annotations.Secret {
		return &domain.ExistingValueIssue{
			Message: "The existing secret value is incompatible with this template. Confirm or enter a valid replacement.",
		}
	}
	return &domain.ExistingValueIssue{
		Message: fmt.Sprintf(
			"The existing value %q is incompatible with this template: %v. Confirm or enter a valid replacement.",
			value,
			cause,
		),
	}
}
