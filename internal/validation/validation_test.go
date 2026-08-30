package validation_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/validation"
)

func TestRequired(t *testing.T) {
	for _, value := range []string{"value", "  value  ", "0", "false"} {
		if err := validation.Required(value); err != nil {
			t.Errorf("Required(%q) error = %v, want nil", value, err)
		}
	}
	for _, value := range []string{"", " ", "\t", "\u00a0"} {
		if err := validation.Required(value); err == nil {
			t.Errorf("Required(%q) error = nil, want rejection", value)
		}
	}
}

func TestInteger(t *testing.T) {
	valid := []string{
		"",
		"0",
		"-0",
		"+1",
		"00042",
		strconv.FormatInt(math.MinInt64, 10),
		strconv.FormatInt(math.MaxInt64, 10),
	}
	for _, value := range valid {
		if err := validation.Integer(value); err != nil {
			t.Errorf("Integer(%q) error = %v, want nil", value, err)
		}
	}

	invalid := []string{
		"+",
		"-",
		" 1",
		"1 ",
		"1.0",
		"1e3",
		"0x10",
		"one",
		"１２",
		"9223372036854775808",
		"-9223372036854775809",
	}
	for _, value := range invalid {
		if err := validation.Integer(value); err == nil {
			t.Errorf("Integer(%q) error = nil, want rejection", value)
		}
	}
}

func TestBoolean(t *testing.T) {
	for _, value := range []string{"true", "false", "TRUE", "False"} {
		if err := validation.Boolean(value); err != nil {
			t.Errorf("Boolean(%q) error = %v, want nil", value, err)
		}
	}
	for _, value := range []string{"", " true", "false ", "yes", "0", "1"} {
		if err := validation.Boolean(value); err == nil {
			t.Errorf("Boolean(%q) error = nil, want rejection", value)
		}
	}
}

func TestPort(t *testing.T) {
	for _, value := range []string{"", "1", "5432", "65535", "00080"} {
		if err := validation.Port(value); err != nil {
			t.Errorf("Port(%q) error = %v, want nil", value, err)
		}
	}
	for _, value := range []string{"0", "00000", "65536", "+80", "-1", " 80", "80 ", "8.0", "http", "０８０"} {
		if err := validation.Port(value); err == nil {
			t.Errorf("Port(%q) error = nil, want rejection", value)
		}
	}
}

func TestURL(t *testing.T) {
	valid := []string{
		"",
		"https://example.com",
		"https://user:pass@example.com:8443/path?a=b#fragment",
		"postgres://user:pass@db:5432/app",
		"unix:///var/run/app.sock",
		"mailto:user@example.com",
		"urn:isbn:9780141036144",
		"custom+scheme://host/path",
		"file:///",
	}
	for _, value := range valid {
		if err := validation.URL(value); err != nil {
			t.Errorf("URL(%q) error = %v, want nil", value, err)
		}
	}

	invalid := []string{
		"example.com/api",
		"plain text",
		"://example.com",
		"https://",
		"mailto:",
		"scheme:?query=yes",
		"https://exa mple.com",
		"https://example.com/line\nbreak",
		"https://example.com/%zz",
		"https://[::1",
		string([]byte{'h', 't', 't', 'p', ':', 0xff}),
	}
	for _, value := range invalid {
		if err := validation.URL(value); err == nil {
			t.Errorf("URL(%q) error = nil, want rejection", value)
		}
	}
}

func TestString(t *testing.T) {
	for _, value := range []string{"", "plain", "caffè ☕", "\t"} {
		if err := validation.String(value); err != nil {
			t.Errorf("String(%q) error = %v, want nil", value, err)
		}
	}
	for _, value := range []string{"bad\x00value", "bad\rvalue", "bad\nvalue", string([]byte{0xff})} {
		if err := validation.String(value); err == nil {
			t.Errorf("String(%q) error = nil, want rejection", value)
		}
	}
}

func TestValidateQuestionCombinesRules(t *testing.T) {
	tests := []struct {
		name     string
		question domain.Question
		value    string
		wantErr  bool
	}{
		{name: "required present", question: domain.Question{Required: true}, value: "value"},
		{name: "required empty", question: domain.Question{Required: true}, value: "  ", wantErr: true},
		{name: "optional integer empty", question: domain.Question{Type: domain.VariableTypeInt}, value: ""},
		{name: "integer invalid", question: domain.Question{Type: domain.VariableTypeInt}, value: "1.5", wantErr: true},
		{name: "boolean empty", question: domain.Question{Type: domain.VariableTypeBool}, value: "", wantErr: true},
		{name: "option selected", question: domain.Question{Options: []string{"one", "two"}}, value: "two"},
		{name: "option case sensitive", question: domain.Question{Options: []string{"one", "two"}}, value: "Two", wantErr: true},
		{name: "unsupported type", question: domain.Question{Type: domain.VariableType("other")}, value: "value", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateQuestion(tt.question, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuestion() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestValidationErrorsNeverContainTheValue(t *testing.T) {
	tests := []struct {
		question domain.Question
		value    string
		secret   string
	}{
		{question: domain.Question{Secret: true}, value: "do-not-show\nrest", secret: "do-not-show"},
		{question: domain.Question{Type: domain.VariableTypeInt, Secret: true}, value: "do-not-show", secret: "do-not-show"},
		{question: domain.Question{Type: domain.VariableTypeBool, Secret: true}, value: "do-not-show", secret: "do-not-show"},
		{question: domain.Question{Type: domain.VariableTypePort, Secret: true}, value: "do-not-show", secret: "do-not-show"},
		{question: domain.Question{Type: domain.VariableTypeURL, Secret: true}, value: "do-not-show", secret: "do-not-show"},
		{question: domain.Question{Options: []string{"allowed"}, Secret: true}, value: "do-not-show", secret: "do-not-show"},
	}

	for _, tt := range tests {
		err := validation.ValidateQuestion(tt.question, tt.value)
		if err == nil {
			t.Fatalf("ValidateQuestion(%#v) error = nil, want rejection", tt.question)
		}
		if strings.Contains(err.Error(), tt.secret) {
			t.Fatalf("ValidateQuestion() error leaked secret value: %q", err)
		}
	}
}
