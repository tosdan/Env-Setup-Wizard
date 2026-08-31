package filesystem

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

// RenderTemplate reconstructs the normalized Template source, including its
// annotation lines and the original representation of unchanged values.
func RenderTemplate(document domain.Document) []byte {
	lines := make([]string, 0, len(document.Nodes))
	for _, node := range document.Nodes {
		lines = append(lines, node.RawLine())
	}

	return joinDocumentLines(lines, document)
}

// RenderConfiguration produces a Generated configuration in memory. It keeps
// the Template structure and comments intact while validating resolved values.
func RenderConfiguration(document domain.Document) ([]byte, error) {
	if document.LineEnding != domain.LineEndingLF && document.LineEnding != domain.LineEndingCRLF {
		return nil, errors.New("render configuration: document has an unsupported line ending")
	}

	lines := make([]string, 0, len(document.Nodes))
	for _, node := range document.Nodes {
		switch node := node.(type) {
		case domain.Variable:
			line, err := renderVariable(node)
			if err != nil {
				return nil, err
			}
			lines = append(lines, line)
		default:
			lines = append(lines, node.RawLine())
		}
	}

	return joinDocumentLines(lines, document), nil
}

func renderVariable(variable domain.Variable) (string, error) {
	if !variable.HasValue {
		return "", fmt.Errorf(
			"render variable %q at line %d: resolved value is not assigned",
			variable.Key,
			variable.Line,
		)
	}
	if !renderableValueSource(variable) {
		return "", fmt.Errorf(
			"render variable %q at line %d: inconsistent value source",
			variable.Key,
			variable.Line,
		)
	}

	value := normalizeRenderedBoolean(variable)
	encoded, err := dotenv.EncodeValue(value)
	if err != nil {
		return "", fmt.Errorf("render variable %q at line %d: %w", variable.Key, variable.Line, err)
	}

	if variable.Annotations.Type == domain.VariableTypeBool ||
		variable.ValueSource == domain.ValueFromExisting ||
		variable.ValueSource == domain.ValueFromUser {
		return variable.Key + "=" + encoded, nil
	}

	return variable.Raw, nil
}

func renderableValueSource(variable domain.Variable) bool {
	if variable.Annotations.Fixed {
		return variable.ValueSource == domain.ValueFromFixed
	}
	if variable.ValueSource == domain.ValueFromFixed {
		return false
	}

	switch variable.ValueSource {
	case domain.ValueFromTemplate, domain.ValueFromExisting, domain.ValueFromUser:
		return true
	default:
		return false
	}
}

func normalizeRenderedBoolean(variable domain.Variable) string {
	if variable.Annotations.Type != domain.VariableTypeBool {
		return variable.Value
	}
	if strings.EqualFold(variable.Value, "true") {
		return "true"
	}
	if strings.EqualFold(variable.Value, "false") {
		return "false"
	}
	return variable.Value
}

func joinDocumentLines(lines []string, document domain.Document) []byte {
	text := strings.Join(lines, string(document.LineEnding))
	if document.HasFinalNewline {
		text += string(document.LineEnding)
	}
	return []byte(text)
}
