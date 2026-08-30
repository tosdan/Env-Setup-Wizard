package dotenv_test

import (
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestParseTemplateAssignsSemanticValues(t *testing.T) {
	input := strings.Join([]string{
		"EMPTY=",
		"PLAIN=value",
		"SINGLE='value with # and $VAR ${VAR} $$'",
		`DOUBLE="tab\tvalue"`,
		`APOSTROPHE='single\'quote'`,
		"EQUALS=left=right",
		"HASH=value#fragment",
	}, "\n")
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	want := map[string]string{
		"EMPTY":      "",
		"PLAIN":      "value",
		"SINGLE":     "value with # and $VAR ${VAR} $$",
		"DOUBLE":     "tab\tvalue",
		"APOSTROPHE": "single'quote",
		"EQUALS":     "left=right",
		"HASH":       "value#fragment",
	}
	variables := documentVariables(document)
	if len(variables) != len(want) {
		t.Fatalf("variable count = %d, want %d", len(variables), len(want))
	}
	for key, wantValue := range want {
		variable, found := variables[key]
		if !found {
			t.Errorf("Variable %q not found", key)
			continue
		}
		if variable.Value != wantValue {
			t.Errorf("Variable %q Value = %q, want %q", key, variable.Value, wantValue)
		}
		if !variable.HasValue {
			t.Errorf("Variable %q HasValue = false, want true", key)
		}
		if variable.ValueSource != domain.ValueFromTemplate {
			t.Errorf("Variable %q ValueSource = %q, want %q", key, variable.ValueSource, domain.ValueFromTemplate)
		}
	}
}

func TestParseTemplateRejectsResolvedMultilineAndNULWithoutLeakingValue(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "escaped newline", input: []byte(`SECRET="do-not-show\nrest"` + "\n")},
		{name: "escaped carriage return", input: []byte(`SECRET="do-not-show\rrest"` + "\n")},
		{name: "NUL", input: []byte("SECRET=do-not-show\x00rest\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, tt.input))
			if err == nil {
				t.Fatal("ParseTemplate() error = nil, want unsupported resolved value error")
			}
			if strings.Contains(err.Error(), "do-not-show") || strings.Contains(err.Error(), "rest") {
				t.Fatalf("ParseTemplate() error leaked value content: %q", err)
			}
			if !strings.Contains(err.Error(), "unsupported NUL, CR, or LF") {
				t.Fatalf("ParseTemplate() error = %q, want unsupported resolved value diagnostic", err)
			}
		})
	}
}

func documentVariables(document domain.Document) map[string]domain.Variable {
	variables := make(map[string]domain.Variable)
	for _, node := range document.Nodes {
		if variable, ok := node.(domain.Variable); ok {
			variables[variable.Key] = variable
		}
	}

	return variables
}
