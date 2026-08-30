package dotenv

import (
	"fmt"
	"strings"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

const implicitSection = "Configuration"

type pendingAnnotations struct {
	annotations domain.Annotations
	lines       map[domain.AnnotationName]int
	firstLine   int
}

func bindAnnotations(document domain.Document) (domain.Document, error) {
	section := implicitSection
	pending := newPendingAnnotations()
	variableCount := 0

	for index, node := range document.Nodes {
		switch node := node.(type) {
		case domain.AnnotationLine:
			annotation, err := parseAnnotation(node)
			if err != nil {
				return domain.Document{}, err
			}
			document.Nodes[index] = annotation

			if annotation.Name == domain.AnnotationSection {
				if !pending.empty() {
					return domain.Document{}, orphanedAnnotationError(pending)
				}
				section = annotation.Value
				continue
			}

			if err := pending.add(annotation); err != nil {
				return domain.Document{}, err
			}

		case domain.Variable:
			node.Annotations = pending.annotations
			node.Section = section
			if err := validateVariableAnnotations(node, pending.lines); err != nil {
				return domain.Document{}, err
			}
			if node.Annotations.Type == domain.VariableTypeBool {
				node.Value = strings.ToLower(node.Value)
			}
			if node.Annotations.Fixed {
				node.ValueSource = domain.ValueFromFixed
			}
			document.Nodes[index] = node
			variableCount++
			pending = newPendingAnnotations()

		case domain.Comment, domain.BlankLine:
			if !pending.empty() {
				return domain.Document{}, orphanedAnnotationError(pending)
			}
		}
	}

	if !pending.empty() {
		return domain.Document{}, orphanedAnnotationError(pending)
	}
	if variableCount == 0 {
		return domain.Document{}, fmt.Errorf("template must define at least one variable")
	}

	return document, nil
}

func newPendingAnnotations() pendingAnnotations {
	return pendingAnnotations{
		annotations: domain.Annotations{Type: domain.VariableTypeString},
		lines:       make(map[domain.AnnotationName]int),
	}
}

func parseAnnotation(line domain.AnnotationLine) (domain.AnnotationLine, error) {
	content := strings.TrimLeft(line.Raw, " \t")
	content = strings.TrimLeft(content[1:], " \t")
	content = content[1:]

	separator := strings.IndexFunc(content, func(character rune) bool {
		return character == ' ' || character == '\t'
	})
	nameText := content
	value := ""
	hasValue := separator >= 0
	if hasValue {
		nameText = content[:separator]
		value = content[separator+1:]
	}
	name := domain.AnnotationName(nameText)
	value = strings.TrimSpace(value)

	if !knownAnnotation(name) {
		return domain.AnnotationLine{}, fmt.Errorf("line %d: unknown annotation @%s", line.Line, nameText)
	}

	if flagAnnotation(name) {
		if hasValue && value != "" {
			return domain.AnnotationLine{}, fmt.Errorf("line %d: annotation @%s does not accept a value", line.Line, name)
		}
		value = ""
	} else if !hasValue || value == "" {
		return domain.AnnotationLine{}, fmt.Errorf("line %d: annotation @%s requires a nonempty value", line.Line, name)
	}

	line.Name = name
	line.Value = value
	return line, nil
}

func knownAnnotation(name domain.AnnotationName) bool {
	switch name {
	case domain.AnnotationPrompt,
		domain.AnnotationDescription,
		domain.AnnotationRequired,
		domain.AnnotationSecret,
		domain.AnnotationType,
		domain.AnnotationOptions,
		domain.AnnotationPlaceholder,
		domain.AnnotationFixed,
		domain.AnnotationSection:
		return true
	default:
		return false
	}
}

func flagAnnotation(name domain.AnnotationName) bool {
	switch name {
	case domain.AnnotationRequired, domain.AnnotationSecret, domain.AnnotationFixed:
		return true
	default:
		return false
	}
}

func (pending *pendingAnnotations) empty() bool {
	return pending.firstLine == 0
}

func (pending *pendingAnnotations) add(annotation domain.AnnotationLine) error {
	if previousLine, duplicate := pending.lines[annotation.Name]; duplicate {
		return fmt.Errorf(
			"line %d: duplicate annotation @%s; first declared at line %d",
			annotation.Line,
			annotation.Name,
			previousLine,
		)
	}
	if pending.firstLine == 0 {
		pending.firstLine = annotation.Line
	}
	pending.lines[annotation.Name] = annotation.Line

	switch annotation.Name {
	case domain.AnnotationPrompt:
		pending.annotations.Prompt = annotation.Value
	case domain.AnnotationDescription:
		pending.annotations.Description = annotation.Value
	case domain.AnnotationRequired:
		pending.annotations.Required = true
	case domain.AnnotationSecret:
		pending.annotations.Secret = true
	case domain.AnnotationType:
		variableType := domain.VariableType(annotation.Value)
		if !knownVariableType(variableType) {
			return fmt.Errorf(
				"line %d: annotation @type has unsupported value %q",
				annotation.Line,
				annotation.Value,
			)
		}
		pending.annotations.Type = variableType
	case domain.AnnotationOptions:
		options, err := parseOptions(annotation)
		if err != nil {
			return err
		}
		pending.annotations.Options = options
	case domain.AnnotationPlaceholder:
		pending.annotations.Placeholder = annotation.Value
	case domain.AnnotationFixed:
		pending.annotations.Fixed = true
	}

	return nil
}

func knownVariableType(variableType domain.VariableType) bool {
	switch variableType {
	case domain.VariableTypeString,
		domain.VariableTypeInt,
		domain.VariableTypeBool,
		domain.VariableTypePort,
		domain.VariableTypeURL:
		return true
	default:
		return false
	}
}

func parseOptions(annotation domain.AnnotationLine) ([]string, error) {
	parts := strings.Split(annotation.Value, ",")
	options := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		option := strings.TrimSpace(part)
		if option == "" {
			return nil, fmt.Errorf("line %d: annotation @options contains an empty option", annotation.Line)
		}
		if _, duplicate := seen[option]; duplicate {
			return nil, fmt.Errorf("line %d: annotation @options contains duplicate option %q", annotation.Line, option)
		}
		seen[option] = struct{}{}
		options = append(options, option)
	}

	return options, nil
}

func validateVariableAnnotations(variable domain.Variable, lines map[domain.AnnotationName]int) error {
	annotations := variable.Annotations

	if len(annotations.Options) > 0 && annotations.Type != domain.VariableTypeString {
		return incompatibleAnnotationsError(
			lines[domain.AnnotationOptions],
			variable.Key,
			"@options",
			"@type "+string(annotations.Type),
		)
	}
	if len(annotations.Options) > 0 && annotations.Secret {
		return incompatibleAnnotationsError(lines[domain.AnnotationOptions], variable.Key, "@options", "@secret")
	}
	if len(annotations.Options) > 0 && annotations.Placeholder != "" {
		return incompatibleAnnotationsError(lines[domain.AnnotationPlaceholder], variable.Key, "@placeholder", "@options")
	}
	if annotations.Placeholder != "" && annotations.Type == domain.VariableTypeBool {
		return incompatibleAnnotationsError(lines[domain.AnnotationPlaceholder], variable.Key, "@placeholder", "@type bool")
	}
	if annotations.Fixed && annotations.Prompt != "" {
		return incompatibleAnnotationsError(lines[domain.AnnotationFixed], variable.Key, "@fixed", "@prompt")
	}
	if annotations.Fixed && annotations.Placeholder != "" {
		return incompatibleAnnotationsError(lines[domain.AnnotationFixed], variable.Key, "@fixed", "@placeholder")
	}

	if len(annotations.Options) > 0 {
		if variable.Value == "" {
			return fmt.Errorf(
				"line %d: variable %q with @options requires a nonempty template value",
				variable.Line,
				variable.Key,
			)
		}
		if !containsOption(annotations.Options, variable.Value) {
			return fmt.Errorf(
				"line %d: template value for variable %q is not one of its @options",
				variable.Line,
				variable.Key,
			)
		}
	}

	if annotations.Type == domain.VariableTypeBool {
		if variable.Value == "" || (!strings.EqualFold(variable.Value, "true") && !strings.EqualFold(variable.Value, "false")) {
			return fmt.Errorf(
				"line %d: template value for @type bool variable %q must be true or false",
				variable.Line,
				variable.Key,
			)
		}
	}

	if annotations.Fixed && annotations.Required && strings.TrimSpace(variable.Value) == "" {
		return fmt.Errorf(
			"line %d: fixed required variable %q must have a nonempty template value",
			variable.Line,
			variable.Key,
		)
	}

	return nil
}

func containsOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}

func incompatibleAnnotationsError(line int, key, first, second string) error {
	return fmt.Errorf(
		"line %d: variable %q has incompatible annotations %s and %s",
		line,
		key,
		first,
		second,
	)
}

func orphanedAnnotationError(pending pendingAnnotations) error {
	return fmt.Errorf(
		"line %d: field annotations must be followed immediately by a variable",
		pending.firstLine,
	)
}
