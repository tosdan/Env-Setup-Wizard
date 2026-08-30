package wizard_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	"github.com/tosdan/env-setup-wizard/internal/validation"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

func TestBuildQuestionGroupsMapsAndMergesDocumentVariables(t *testing.T) {
	input := strings.Join([]string{
		"ROOT_NAME=root",
		"# @section Empty",
		"# @fixed",
		"FIXED_ONLY=keep",
		"# @section Database",
		"# @prompt Database port",
		"# @description Port exposed by the database",
		"# @required",
		"# @type port",
		"# @placeholder 5432",
		"DB_PORT=05432",
		"# @section Application",
		"# @options development,staging,production",
		"ENVIRONMENT=staging",
		"# @secret",
		"API_TOKEN='token'",
		"# @section Database",
		"# @type bool",
		"ENABLED=TRUE",
		"# @section Configuration",
		"TAIL=",
	}, "\n")
	document := parseQuestionTemplate(t, input)

	groups, err := wizard.BuildQuestionGroups(document)
	if err != nil {
		t.Fatalf("BuildQuestionGroups() error = %v, want nil", err)
	}
	if got, want := groupNames(groups), []string{"Configuration", "Database", "Application"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("group order = %#v, want %#v", got, want)
	}
	if got, want := questionKeys(groups[0]), []string{"ROOT_NAME", "TAIL"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Configuration questions = %#v, want %#v", got, want)
	}
	if got, want := questionKeys(groups[1]), []string{"DB_PORT", "ENABLED"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Database questions = %#v, want %#v", got, want)
	}
	if got, want := questionKeys(groups[2]), []string{"ENVIRONMENT", "API_TOKEN"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Application questions = %#v, want %#v", got, want)
	}

	root := groups[0].Questions[0]
	if root.Prompt != "ROOT_NAME" || root.Kind != domain.QuestionKindInput || root.Type != domain.VariableTypeString {
		t.Errorf("ROOT_NAME Question = %#v, want fallback prompt and string input", root)
	}
	if root.Value != "root" || !root.HasValue || root.ValueSource != domain.ValueFromTemplate {
		t.Errorf("ROOT_NAME value state = %#v, want resolved template value", root)
	}

	port := groups[1].Questions[0]
	if port.Prompt != "Database port" || port.Description != "Port exposed by the database" {
		t.Errorf("DB_PORT labels = %#v, want annotation values", port)
	}
	if port.Type != domain.VariableTypePort || port.Kind != domain.QuestionKindInput || !port.Required || port.Placeholder != "5432" {
		t.Errorf("DB_PORT semantics = %#v, want required port input with placeholder", port)
	}
	if port.Value != "05432" {
		t.Errorf("DB_PORT Value = %q, want literal leading zeroes preserved", port.Value)
	}

	enabled := groups[1].Questions[1]
	if enabled.Type != domain.VariableTypeBool || enabled.Kind != domain.QuestionKindConfirm || enabled.Value != "true" {
		t.Errorf("ENABLED Question = %#v, want normalized boolean confirmation", enabled)
	}

	environment := groups[2].Questions[0]
	if environment.Kind != domain.QuestionKindSelect || !reflect.DeepEqual(environment.Options, []string{"development", "staging", "production"}) {
		t.Errorf("ENVIRONMENT Question = %#v, want closed selection", environment)
	}
	secret := groups[2].Questions[1]
	if !secret.Secret || secret.Kind != domain.QuestionKindInput || secret.ExistingValueIssue != nil {
		t.Errorf("API_TOKEN Question = %#v, want secret input without existing-value issue", secret)
	}
}

func TestBuildQuestionGroupsAllowsAllFixedDocument(t *testing.T) {
	document := parseQuestionTemplate(t, "# @fixed\nONE=1\n# @fixed\n# @secret\nTWO=2\n")
	groups, err := wizard.BuildQuestionGroups(document)
	if err != nil {
		t.Fatalf("BuildQuestionGroups() error = %v, want nil", err)
	}
	if len(groups) != 0 {
		t.Fatalf("len(groups) = %d, want zero for all-fixed document", len(groups))
	}
}

func TestBuildQuestionGroupsUsesFirstSectionOccurrenceForGroupOrder(t *testing.T) {
	input := strings.Join([]string{
		"# @section Reopened",
		"# @fixed",
		"FIXED=keep",
		"# @section Empty",
		"# @section Later",
		"LATER=value",
		"# @section Reopened",
		"EARLIER_SECTION=value",
	}, "\n")
	groups, err := wizard.BuildQuestionGroups(parseQuestionTemplate(t, input))
	if err != nil {
		t.Fatalf("BuildQuestionGroups() error = %v, want nil", err)
	}
	if got, want := groupNames(groups), []string{"Reopened", "Later"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("group order = %#v, want first section occurrence order %#v", got, want)
	}
}

func TestBuildQuestionGroupsAllowsEmptyConfigurableDefaults(t *testing.T) {
	input := strings.Join([]string{
		"# @required",
		"REQUIRED_TEXT=",
		"# @type int",
		"OPTIONAL_INT=",
		"# @type port",
		"OPTIONAL_PORT=",
		"# @type url",
		"OPTIONAL_URL=",
	}, "\n")
	groups, err := wizard.BuildQuestionGroups(parseQuestionTemplate(t, input))
	if err != nil {
		t.Fatalf("BuildQuestionGroups() error = %v, want nil", err)
	}
	if len(groups) != 1 || len(groups[0].Questions) != 4 {
		t.Fatalf("groups = %#v, want four configurable questions", groups)
	}
	if err := validation.ValidateQuestion(groups[0].Questions[0], ""); err == nil {
		t.Fatal("ValidateQuestion(required empty default) error = nil, want final-value rejection")
	}
}

func TestBuildQuestionGroupsRejectsInvalidTemplateDefaults(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMessage string
	}{
		{name: "integer", input: "# @type int\nVALUE=1.5\n", wantMessage: "decimal integer"},
		{name: "integer overflow", input: "# @type int\nVALUE=9223372036854775808\n", wantMessage: "signed 64-bit"},
		{name: "port", input: "# @type port\nVALUE=0\n", wantMessage: "range 1..65535"},
		{name: "URL", input: "# @type url\nVALUE=example.com/path\n", wantMessage: "absolute URI"},
		{name: "fixed integer", input: "# @fixed\n# @type int\nVALUE=invalid\n", wantMessage: "decimal integer"},
		{name: "fixed required", input: "# @fixed\n# @required\nVALUE=\n", wantMessage: "nonempty template value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeQuestionTemplate(t, tt.input)
			document, parseErr := dotenv.ParseTemplate(path)
			if parseErr != nil {
				if !strings.Contains(parseErr.Error(), tt.wantMessage) {
					t.Fatalf("ParseTemplate() error = %q, want it to contain %q", parseErr, tt.wantMessage)
				}
				return
			}

			_, err := wizard.BuildQuestionGroups(document)
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("BuildQuestionGroups() error = %v, want it to contain %q", err, tt.wantMessage)
			}
		})
	}
}

func TestBuildQuestionGroupsDoesNotLeakInvalidSecretDefault(t *testing.T) {
	document := domain.Document{
		Nodes: []domain.Node{domain.Variable{
			Key:         "SECRET",
			Value:       "do-not-show",
			HasValue:    true,
			ValueSource: domain.ValueFromTemplate,
			Annotations: domain.Annotations{Type: domain.VariableTypeInt, Secret: true},
			Section:     "Configuration",
			Raw:         "SECRET=do-not-show",
			Line:        1,
		}},
		LineEnding: domain.LineEndingLF,
	}
	_, err := wizard.BuildQuestionGroups(document)
	if err == nil {
		t.Fatal("BuildQuestionGroups() error = nil, want invalid integer error")
	}
	if strings.Contains(err.Error(), "do-not-show") {
		t.Fatalf("BuildQuestionGroups() error leaked secret value: %q", err)
	}
}

func TestBuildQuestionGroupsRejectsDocumentWithoutVariables(t *testing.T) {
	_, err := wizard.BuildQuestionGroups(domain.Document{LineEnding: domain.LineEndingLF})
	if err == nil || !strings.Contains(err.Error(), "at least one variable") {
		t.Fatalf("BuildQuestionGroups() error = %v, want missing variable error", err)
	}
}

func TestBuildQuestionGroupsDefensivelyRejectsInvalidDocumentValue(t *testing.T) {
	document := domain.Document{
		Nodes: []domain.Node{domain.Variable{
			Key:         "PORT",
			Value:       "70000",
			HasValue:    true,
			ValueSource: domain.ValueFromTemplate,
			Annotations: domain.Annotations{Type: domain.VariableTypePort},
			Section:     "Configuration",
			Raw:         "PORT=70000",
			Line:        1,
		}},
		LineEnding: domain.LineEndingLF,
	}

	_, err := wizard.BuildQuestionGroups(document)
	if err == nil || !strings.Contains(err.Error(), "range 1..65535") {
		t.Fatalf("BuildQuestionGroups() error = %v, want invalid port error", err)
	}
}

func parseQuestionTemplate(t *testing.T, input string) domain.Document {
	t.Helper()
	path := writeQuestionTemplate(t, input)
	document, err := dotenv.ParseTemplate(path)
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	return document
}

func writeQuestionTemplate(t *testing.T, input string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

func groupNames(groups []domain.QuestionGroup) []string {
	names := make([]string, len(groups))
	for index, group := range groups {
		names[index] = group.Section
	}
	return names
}

func questionKeys(group domain.QuestionGroup) []string {
	keys := make([]string, len(group.Questions))
	for index, question := range group.Questions {
		keys[index] = question.Key
	}
	return keys
}
