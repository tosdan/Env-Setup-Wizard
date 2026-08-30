package dotenv

import (
	"fmt"
	"strings"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

// ParseTemplate loads a Template and returns its ordered Document with Compose
// dotenv semantics assigned to each Variable. Annotation meaning is assigned
// by a later stage.
func ParseTemplate(path string) (domain.Document, error) {
	source, err := loadTemplate(path)
	if err != nil {
		return domain.Document{}, err
	}

	document, err := scanSource(source)
	if err != nil {
		return domain.Document{}, fmt.Errorf("scan template %q: %w", path, err)
	}
	values, err := parseSemanticValues(source.Text)
	if err != nil {
		return domain.Document{}, fmt.Errorf("parse template values %q: %w", path, err)
	}
	document, err = attachSemanticValues(document, values)
	if err != nil {
		return domain.Document{}, fmt.Errorf("assign template values %q: %w", path, err)
	}

	return document, nil
}

func scanSource(source source) (domain.Document, error) {
	document := domain.Document{
		LineEnding:      source.LineEnding,
		HasFinalNewline: source.HasFinalNewline,
	}
	seenVariables := make(map[string]int)

	for index, rawLine := range splitSourceLines(source) {
		lineNumber := index + 1
		node, err := scanLine(rawLine, lineNumber)
		if err != nil {
			return domain.Document{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		if variable, ok := node.(domain.Variable); ok {
			if previousLine, duplicate := seenVariables[variable.Key]; duplicate {
				return domain.Document{}, fmt.Errorf(
					"line %d: duplicate variable %q; first declared at line %d",
					lineNumber,
					variable.Key,
					previousLine,
				)
			}
			seenVariables[variable.Key] = lineNumber
		}

		document.Nodes = append(document.Nodes, node)
	}

	return document, nil
}

func splitSourceLines(source source) []string {
	if source.Text == "" {
		return nil
	}

	text := source.Text
	separator := string(source.LineEnding)
	if source.HasFinalNewline {
		text = strings.TrimSuffix(text, separator)
	}

	return strings.Split(text, separator)
}

func scanLine(rawLine string, lineNumber int) (domain.Node, error) {
	if strings.Trim(rawLine, " \t") == "" {
		return domain.BlankLine{Raw: rawLine, Line: lineNumber}, nil
	}

	leftTrimmed := strings.TrimLeft(rawLine, " \t")
	if strings.HasPrefix(leftTrimmed, "#") {
		if isAnnotationLine(leftTrimmed) {
			return domain.AnnotationLine{Raw: rawLine, Line: lineNumber}, nil
		}
		return domain.Comment{Raw: rawLine, Line: lineNumber}, nil
	}

	if hasExportPrefix(rawLine) {
		return nil, fmt.Errorf("export prefixes are not supported")
	}

	equalsIndex := strings.IndexByte(rawLine, '=')
	colonIndex := strings.IndexByte(rawLine, ':')
	if colonIndex >= 0 && (equalsIndex < 0 || colonIndex < equalsIndex) && validKey(rawLine[:colonIndex]) {
		return nil, fmt.Errorf("':' assignments are not supported")
	}
	if equalsIndex < 0 {
		return nil, fmt.Errorf("expected a KEY=value assignment or comment")
	}

	key := rawLine[:equalsIndex]
	if !validKey(key) {
		return nil, fmt.Errorf("invalid variable key %q", key)
	}

	rawValue := rawLine[equalsIndex+1:]
	if err := validateRawValue(rawValue); err != nil {
		return nil, fmt.Errorf("variable %q: %w", key, err)
	}

	return domain.Variable{
		Key:      key,
		RawValue: rawValue,
		Raw:      rawLine,
		Line:     lineNumber,
	}, nil
}

func isAnnotationLine(comment string) bool {
	afterMarker := comment[1:]
	if len(afterMarker) == 0 || (afterMarker[0] != ' ' && afterMarker[0] != '\t') {
		return false
	}

	return strings.HasPrefix(strings.TrimLeft(afterMarker, " \t"), "@")
}

func hasExportPrefix(line string) bool {
	return line == "export" || strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t")
}

func validKey(key string) bool {
	if key == "" || !keyStart(key[0]) {
		return false
	}
	for index := 1; index < len(key); index++ {
		character := key[index]
		if !keyStart(character) && (character < '0' || character > '9') && character != '.' && character != '-' {
			return false
		}
	}

	return true
}

func keyStart(character byte) bool {
	return character == '_' ||
		(character >= 'A' && character <= 'Z') ||
		(character >= 'a' && character <= 'z')
}

func validateRawValue(rawValue string) error {
	if rawValue == "" {
		return nil
	}

	if rawValue[0] == '\'' || rawValue[0] == '"' {
		return validateQuotedValue(rawValue, rawValue[0])
	}
	if strings.ContainsAny(rawValue, "'\"") {
		return fmt.Errorf("quotes must surround the complete value")
	}
	if strings.Trim(rawValue, " \t") != rawValue {
		return fmt.Errorf("leading or trailing whitespace must be quoted")
	}
	if hasInlineComment(rawValue) {
		return fmt.Errorf("unquoted inline comments are not supported")
	}
	if hasActiveInterpolation(rawValue, false) {
		return fmt.Errorf("active interpolation is not supported; use single quotes for a literal value")
	}

	return nil
}

func hasInlineComment(rawValue string) bool {
	for index := 1; index < len(rawValue); index++ {
		if rawValue[index] == '#' && (rawValue[index-1] == ' ' || rawValue[index-1] == '\t') {
			return true
		}
	}

	return false
}

func validateQuotedValue(rawValue string, quote byte) error {
	closingQuote := -1
	escaped := false
	for index := 1; index < len(rawValue); index++ {
		character := rawValue[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == quote {
			closingQuote = index
			break
		}
	}

	if closingQuote < 0 {
		return fmt.Errorf("unterminated quoted value")
	}
	if closingQuote != len(rawValue)-1 {
		return fmt.Errorf("quoted values cannot have trailing characters")
	}
	if quote == '"' && hasActiveInterpolation(rawValue[1:closingQuote], true) {
		return fmt.Errorf("active interpolation is not supported; use single quotes for a literal value")
	}

	return nil
}

func hasActiveInterpolation(value string, honorEscapes bool) bool {
	for index := 0; index < len(value); index++ {
		if honorEscapes && value[index] == '\\' {
			index++
			continue
		}
		if value[index] != '$' || index+1 >= len(value) {
			continue
		}

		next := value[index+1]
		if next == '{' || keyStart(next) {
			return true
		}
	}

	return false
}
