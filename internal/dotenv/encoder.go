package dotenv

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

const encodedValueKey = "ENV_WIZARD_ENCODED_VALUE"

// EncodeValue returns the canonical Compose-compatible representation of one
// literal, single-line resolved value. Errors never include the value.
func EncodeValue(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("value is not valid UTF-8")
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("value contains unsupported NUL, CR, or LF")
	}

	singleQuoted := "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
	if encodingRoundTrips(singleQuoted, value) {
		return singleQuoted, nil
	}

	// compose-go cannot represent an odd number of backslashes immediately
	// before an escaped apostrophe inside single quotes. Double quotes are the
	// deterministic fallback; backslashes, quotes, and dollars must all be
	// escaped so the resolved value remains literal.
	doubleQuotedContent := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
	).Replace(value)
	doubleQuoted := "\"" + doubleQuotedContent + "\""
	if encodingRoundTrips(doubleQuoted, value) {
		return doubleQuoted, nil
	}

	return "", errors.New("encoded value failed semantic round trip")
}

func encodingRoundTrips(encoded, value string) bool {
	parsed, err := parseSemanticValues(encodedValueKey + "=" + encoded + "\n")
	return err == nil && parsed[encodedValueKey] == value
}

// UpdateValue assigns a resolved value to a Variable and replaces its raw
// assignment with the canonical encoding. The Document is unchanged on error.
func UpdateValue(document *domain.Document, key, value string, source domain.ValueSource) error {
	if document == nil {
		return errors.New("cannot update a nil document")
	}
	if !knownValueSource(source) {
		return fmt.Errorf("update variable %q: invalid value source", key)
	}

	for index, node := range document.Nodes {
		variable, ok := node.(domain.Variable)
		if !ok || variable.Key != key {
			continue
		}

		if variable.Annotations.Fixed {
			return fmt.Errorf("update variable %q: fixed variables cannot be updated", key)
		}
		if source == domain.ValueFromFixed {
			return fmt.Errorf("update variable %q: non-fixed variables cannot use the fixed value source", key)
		}

		value = normalizeBooleanValue(variable, value)
		encoded, err := EncodeValue(value)
		if err != nil {
			return fmt.Errorf("update variable %q: %w", key, err)
		}

		variable.Value = value
		variable.HasValue = true
		variable.ValueSource = source
		variable.RawValue = encoded
		variable.Raw = variable.Key + "=" + encoded
		document.Nodes[index] = variable
		return nil
	}

	return fmt.Errorf("update variable %q: variable not found", key)
}

func knownValueSource(source domain.ValueSource) bool {
	switch source {
	case domain.ValueFromTemplate,
		domain.ValueFromExisting,
		domain.ValueFromUser,
		domain.ValueFromFixed:
		return true
	default:
		return false
	}
}

func normalizeBooleanValue(variable domain.Variable, value string) string {
	if variable.Annotations.Type != domain.VariableTypeBool {
		return value
	}
	if strings.EqualFold(value, "true") {
		return "true"
	}
	if strings.EqualFold(value, "false") {
		return "false"
	}
	return value
}
