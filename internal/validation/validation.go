package validation

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

// String accepts any valid UTF-8 value that can be represented on one dotenv
// line. Errors never contain the value.
func String(value string) error {
	if !utf8.ValidString(value) {
		return errors.New("value must be valid UTF-8")
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return errors.New("value must not contain NUL, CR, or LF")
	}
	return nil
}

// Required rejects empty and whitespace-only values.
func Required(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is required")
	}
	return nil
}

// Integer accepts an empty value or a base-10 integer in the signed 64-bit
// range. It does not normalize the accepted text.
func Integer(value string) error {
	if value == "" {
		return nil
	}

	digits := value
	if value[0] == '+' || value[0] == '-' {
		digits = value[1:]
	}
	if digits == "" || !decimalDigits(digits) {
		return errors.New("value must be a decimal integer")
	}
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return errors.New("value must be within the signed 64-bit integer range")
	}

	return nil
}

// Boolean accepts only true or false, case-insensitively. Unlike the other
// typed validators, an empty value is always invalid.
func Boolean(value string) error {
	if !strings.EqualFold(value, "true") && !strings.EqualFold(value, "false") {
		return errors.New("value must be true or false")
	}
	return nil
}

// Port accepts an empty value or decimal digits representing a port in the
// inclusive range 1..65535. It does not normalize leading zeroes.
func Port(value string) error {
	if value == "" {
		return nil
	}
	if !decimalDigits(value) {
		return errors.New("value must be a decimal port number")
	}

	port, err := strconv.ParseUint(value, 10, 64)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("value must be a port in the range 1..65535")
	}
	return nil
}

// URL accepts an empty value or an absolute generic URI with a scheme and
// meaningful host, path, or opaque content. It performs no normalization or
// network access.
func URL(value string) error {
	if value == "" {
		return nil
	}
	if !utf8.ValidString(value) {
		return errors.New("value must be a valid absolute URI")
	}
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return errors.New("value must be a valid absolute URI without whitespace or control characters")
		}
	}

	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Scheme == "" {
		return errors.New("value must be a valid absolute URI")
	}
	if parsed.Host == "" && parsed.Path == "" && parsed.Opaque == "" {
		return errors.New("absolute URI must contain a host, path, or opaque part")
	}

	return nil
}

// ValidateQuestion applies all final-value rules carried by a Question.
func ValidateQuestion(question domain.Question, value string) error {
	if err := String(value); err != nil {
		return err
	}
	if question.Required {
		if err := Required(value); err != nil {
			return err
		}
	}

	switch question.Type {
	case "", domain.VariableTypeString:
	case domain.VariableTypeInt:
		if err := Integer(value); err != nil {
			return err
		}
	case domain.VariableTypeBool:
		if err := Boolean(value); err != nil {
			return err
		}
	case domain.VariableTypePort:
		if err := Port(value); err != nil {
			return err
		}
	case domain.VariableTypeURL:
		if err := URL(value); err != nil {
			return err
		}
	default:
		return errors.New("question has an unsupported variable type")
	}

	if len(question.Options) > 0 && !contains(question.Options, value) {
		return errors.New("value must be one of the allowed options")
	}

	return nil
}

func decimalDigits(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return value != ""
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
