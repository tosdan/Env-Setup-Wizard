package wizard

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

// RenderSummary returns a terminal-safe summary grouped by the merged section
// order. Secret values are represented only by their presence state.
func RenderSummary(document domain.Document) (string, error) {
	groups := collectSummaryGroups(document)
	if len(groups) == 0 {
		return "", errors.New("summary requires at least one variable")
	}

	var summary strings.Builder
	summary.WriteString("Summary\n")
	for _, group := range groups {
		summary.WriteString("\n[")
		summary.WriteString(group.section)
		summary.WriteString("]\n")

		width := 0
		for _, variable := range group.variables {
			label := variableLabel(variable)
			if len(label) > width {
				width = len(label)
			}
		}
		for _, variable := range group.variables {
			if !variable.HasValue {
				return "", fmt.Errorf("summary variable %q has no resolved value", variable.Key)
			}
			fmt.Fprintf(
				&summary,
				"  %-*s  %s\n",
				width,
				variableLabel(variable),
				summaryValue(variable),
			)
		}
	}
	return summary.String(), nil
}

type summaryGroup struct {
	section   string
	variables []domain.Variable
}

func collectSummaryGroups(document domain.Document) []summaryGroup {
	sectionOrder := make([]string, 0)
	variablesBySection := make(map[string][]domain.Variable)
	seenSections := make(map[string]struct{})
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
			section := rememberSection(node.Section)
			variablesBySection[section] = append(variablesBySection[section], node)
		}
	}

	groups := make([]summaryGroup, 0, len(sectionOrder))
	for _, section := range sectionOrder {
		variables := variablesBySection[section]
		if len(variables) == 0 {
			continue
		}
		groups = append(groups, summaryGroup{section: section, variables: variables})
	}
	return groups
}

func summaryValue(variable domain.Variable) string {
	if variable.Annotations.Secret {
		if variable.Value == "" {
			return "[not set]"
		}
		return "[set]"
	}
	return terminalSafeValue(variable.Value)
}

func terminalSafeValue(value string) string {
	var safe strings.Builder
	for _, character := range value {
		if !unicode.IsControl(character) {
			safe.WriteRune(character)
			continue
		}
		switch character {
		case '\t':
			safe.WriteString(`\t`)
		default:
			fmt.Fprintf(&safe, `\u%04X`, character)
		}
	}
	return safe.String()
}
