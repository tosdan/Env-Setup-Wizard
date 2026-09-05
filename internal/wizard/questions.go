package wizard

import (
	"fmt"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/validation"
)

const implicitSection = "Configuration"

// BuildQuestionGroups derives configurable Questions from a validated
// Document. It merges repeated Sections without changing document order.
func BuildQuestionGroups(document domain.Document) ([]domain.QuestionGroup, error) {
	sectionOrder := make([]string, 0)
	seenSections := make(map[string]struct{})
	questionsBySection := make(map[string][]domain.Question)
	variableCount := 0

	rememberSection := func(section string) string {
		if section == "" {
			section = implicitSection
		}
		if _, seen := seenSections[section]; !seen {
			seenSections[section] = struct{}{}
			sectionOrder = append(sectionOrder, section)
		}
		return section
	}

	for _, node := range document.Nodes {
		switch node := node.(type) {
		case domain.AnnotationLine:
			if node.Name == domain.AnnotationSection {
				rememberSection(node.Value)
			}

		case domain.Variable:
			variableCount++
			section := rememberSection(node.Section)
			question := questionFromVariable(node, section)
			if err := validateTemplateValue(node, question); err != nil {
				return nil, fmt.Errorf(
					"line %d: template value for variable %q: %w",
					node.Line,
					node.Key,
					err,
				)
			}
			if node.Annotations.Fixed {
				continue
			}
			questionsBySection[section] = append(questionsBySection[section], question)
		}
	}

	if variableCount == 0 {
		return nil, fmt.Errorf("document must contain at least one variable")
	}

	groups := make([]domain.QuestionGroup, 0, len(sectionOrder))
	for _, section := range sectionOrder {
		questions := questionsBySection[section]
		if len(questions) == 0 {
			continue
		}
		groups = append(groups, domain.QuestionGroup{
			Section:   section,
			Questions: questions,
		})
	}

	return groups, nil
}

func questionFromVariable(variable domain.Variable, section string) domain.Question {
	variableType := variable.Annotations.Type
	if variableType == "" {
		variableType = domain.VariableTypeString
	}

	return domain.Question{
		Key:                variable.Key,
		Prompt:             variableLabel(variable),
		Description:        variable.Annotations.Description,
		Value:              variable.Value,
		HasValue:           variable.HasValue,
		ValueSource:        variable.ValueSource,
		Type:               variableType,
		Kind:               questionKind(variableType, variable.Annotations.Options),
		Required:           variable.Annotations.Required,
		Secret:             variable.Annotations.Secret,
		Options:            append([]string(nil), variable.Annotations.Options...),
		Placeholder:        variable.Annotations.Placeholder,
		Section:            section,
		ExistingValueIssue: cloneExistingValueIssue(variable.ExistingValueIssue),
	}
}

func cloneExistingValueIssue(issue *domain.ExistingValueIssue) *domain.ExistingValueIssue {
	if issue == nil {
		return nil
	}
	cloned := *issue
	return &cloned
}

func questionKind(variableType domain.VariableType, options []string) domain.QuestionKind {
	if len(options) > 0 {
		return domain.QuestionKindSelect
	}
	if variableType == domain.VariableTypeBool {
		return domain.QuestionKindConfirm
	}
	return domain.QuestionKindInput
}

func validateTemplateValue(variable domain.Variable, question domain.Question) error {
	if !variable.HasValue {
		return fmt.Errorf("resolved value is not assigned")
	}

	// A configurable required Variable may start empty because the wizard can
	// collect it. A fixed Variable has no such recovery path.
	if !variable.Annotations.Fixed {
		question.Required = false
	}
	return validation.ValidateQuestion(question, variable.Value)
}
