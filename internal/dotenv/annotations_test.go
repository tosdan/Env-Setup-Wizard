package dotenv_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestParseTemplateBindsAllAnnotations(t *testing.T) {
	input := strings.Join([]string{
		"PLAIN=value",
		"# @section Application",
		"# @prompt Public endpoint",
		"# @description Absolute address used by clients",
		"# @required",
		"# @type url",
		"# @placeholder https://example.test",
		"PUBLIC_URL=https://example.test",
		"# @options development, staging,production",
		"ENVIRONMENT=staging",
		"# @section Secrets",
		"# @secret",
		"# @fixed",
		"TOKEN='template-token'",
		"# @type bool",
		"ENABLED=TRUE",
	}, "\n")

	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	variables := documentVariables(document)
	plain := variables["PLAIN"]
	if plain.Section != "Configuration" || plain.Annotations.Type != domain.VariableTypeString {
		t.Errorf("PLAIN = %#v, want implicit section and string type", plain)
	}

	publicURL := variables["PUBLIC_URL"]
	wantURLAnnotations := domain.Annotations{
		Prompt:      "Public endpoint",
		Description: "Absolute address used by clients",
		Required:    true,
		Type:        domain.VariableTypeURL,
		Placeholder: "https://example.test",
	}
	if publicURL.Section != "Application" || !reflect.DeepEqual(publicURL.Annotations, wantURLAnnotations) {
		t.Errorf("PUBLIC_URL = %#v, want section Application and annotations %#v", publicURL, wantURLAnnotations)
	}

	environment := variables["ENVIRONMENT"]
	if environment.Section != "Application" || !equalStrings(environment.Annotations.Options, []string{"development", "staging", "production"}) {
		t.Errorf("ENVIRONMENT = %#v, want trimmed options in Application", environment)
	}

	token := variables["TOKEN"]
	if token.Section != "Secrets" || !token.Annotations.Secret || !token.Annotations.Fixed {
		t.Errorf("TOKEN = %#v, want fixed secret in Secrets", token)
	}
	if token.ValueSource != domain.ValueFromFixed {
		t.Errorf("TOKEN ValueSource = %q, want %q", token.ValueSource, domain.ValueFromFixed)
	}

	enabled := variables["ENABLED"]
	if enabled.Value != "true" || enabled.Annotations.Type != domain.VariableTypeBool {
		t.Errorf("ENABLED = %#v, want normalized bool value", enabled)
	}

	section := assertNode[domain.AnnotationLine](t, document.Nodes[1], 2, "# @section Application")
	if section.Name != domain.AnnotationSection || section.Value != "Application" {
		t.Errorf("section AnnotationLine = %#v, want parsed section", section)
	}
}

func TestParseTemplateTrimsAnnotationValues(t *testing.T) {
	input := "\t#  @prompt\t  Display name  \t\nKEY=value\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	variable := documentVariables(document)["KEY"]
	if variable.Annotations.Prompt != "Display name" {
		t.Errorf("Prompt = %q, want %q", variable.Annotations.Prompt, "Display name")
	}
}

func TestParseTemplateRejectsInvalidAnnotationSyntax(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		wantMessage string
	}{
		{name: "unknown", annotation: "# @unknown value", wantMessage: "unknown annotation @unknown"},
		{name: "case sensitive", annotation: "# @Required", wantMessage: "unknown annotation @Required"},
		{name: "flag value", annotation: "# @required yes", wantMessage: "annotation @required does not accept a value"},
		{name: "prompt missing value", annotation: "# @prompt", wantMessage: "annotation @prompt requires a nonempty value"},
		{name: "description missing value", annotation: "# @description   ", wantMessage: "annotation @description requires a nonempty value"},
		{name: "options missing value", annotation: "# @options", wantMessage: "annotation @options requires a nonempty value"},
		{name: "placeholder missing value", annotation: "# @placeholder", wantMessage: "annotation @placeholder requires a nonempty value"},
		{name: "section missing value", annotation: "# @section", wantMessage: "annotation @section requires a nonempty value"},
		{name: "unsupported type", annotation: "# @type float", wantMessage: "annotation @type has unsupported value \"float\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(tt.annotation+"\nKEY=value\n")))
			requireAnnotationError(t, err, "line 1:")
			requireAnnotationError(t, err, tt.wantMessage)
		})
	}
}

func TestParseTemplateRejectsDuplicateFieldAnnotations(t *testing.T) {
	input := "# @prompt First\n# @prompt Second\nKEY=value\n"
	_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	requireAnnotationError(t, err, "line 2: duplicate annotation @prompt; first declared at line 1")
}

func TestParseTemplateRejectsInterruptedAnnotationBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int
	}{
		{name: "blank line", input: "# @required\n\nKEY=value\n", wantLine: 1},
		{name: "normal comment", input: "# @required\n# explanation\nKEY=value\n", wantLine: 1},
		{name: "section", input: "# @required\n# @section Other\nKEY=value\n", wantLine: 1},
		{name: "end of file", input: "KEY=value\n# @required\n", wantLine: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(tt.input)))
			requireAnnotationError(t, err, fmt.Sprintf("line %d: field annotations must be followed immediately by a variable", tt.wantLine))
		})
	}
}

func TestParseTemplateValidatesOptions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMessage string
	}{
		{name: "empty entry", input: "# @options one,,two\nKEY=one\n", wantMessage: "@options contains an empty option"},
		{name: "trailing comma", input: "# @options one,two,\nKEY=one\n", wantMessage: "@options contains an empty option"},
		{name: "duplicate", input: "# @options one, two,one\nKEY=one\n", wantMessage: "@options contains duplicate option"},
		{name: "empty default", input: "# @options one,two\nKEY=\n", wantMessage: "requires a nonempty template value"},
		{name: "unknown default", input: "# @options one,two\nKEY=three\n", wantMessage: "is not one of its @options"},
		{name: "case sensitive default", input: "# @options one,two\nKEY=One\n", wantMessage: "is not one of its @options"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(tt.input)))
			requireAnnotationError(t, err, tt.wantMessage)
		})
	}
}

func TestParseTemplateRejectsIncompatibleAnnotations(t *testing.T) {
	tests := []struct {
		name  string
		block string
	}{
		{name: "options bool", block: "# @options true,false\n# @type bool"},
		{name: "options int", block: "# @options 1,2\n# @type int"},
		{name: "options port", block: "# @options 80,443\n# @type port"},
		{name: "options url", block: "# @options https://one.test,https://two.test\n# @type url"},
		{name: "options secret", block: "# @options one,two\n# @secret"},
		{name: "placeholder options", block: "# @placeholder choose\n# @options one,two"},
		{name: "placeholder bool", block: "# @placeholder true or false\n# @type bool"},
		{name: "fixed prompt", block: "# @fixed\n# @prompt Value"},
		{name: "fixed placeholder", block: "# @fixed\n# @placeholder Value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(tt.block+"\nKEY=true\n")))
			requireAnnotationError(t, err, "incompatible annotations")
		})
	}
}

func TestParseTemplateValidatesBooleanTemplateValuesWithoutLeakingThem(t *testing.T) {
	for _, value := range []string{"", "yes", "do-not-show"} {
		t.Run(value, func(t *testing.T) {
			input := "# @secret\n# @type bool\nKEY=" + value + "\n"
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
			requireAnnotationError(t, err, "must be true or false")
			if value != "" && strings.Contains(err.Error(), value) {
				t.Fatalf("ParseTemplate() error leaked secret value: %q", err)
			}
		})
	}
}

func TestParseTemplateRejectsEmptyFixedRequiredValue(t *testing.T) {
	input := "# @required\n# @fixed\nKEY='   '\n"
	_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	requireAnnotationError(t, err, "fixed required variable \"KEY\" must have a nonempty template value")
}

func TestParseTemplateRequiresAtLeastOneVariable(t *testing.T) {
	for _, input := range []string{"", "# comment\n", "# @section Empty\n"} {
		_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
		requireAnnotationError(t, err, "template must define at least one variable")
		if !strings.Contains(err.Error(), ".env.example") {
			t.Fatalf("ParseTemplate() error = %q, want template path", err)
		}
	}
}

func TestParseTemplateAcceptsAllFixedVariables(t *testing.T) {
	input := "# @fixed\nFIRST=one\n# @fixed\n# @secret\nSECOND=two\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	for key, variable := range documentVariables(document) {
		if !variable.Annotations.Fixed || variable.ValueSource != domain.ValueFromFixed {
			t.Errorf("Variable %q = %#v, want fixed value source", key, variable)
		}
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func requireAnnotationError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ParseTemplate() error = nil, want it to contain %q", message)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("ParseTemplate() error = %q, want it to contain %q", err, message)
	}
}
