package dotenv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

func TestMergeExistingAppliesCompatibleValuesAndRecoversInvalidOnes(t *testing.T) {
	template := strings.Join([]string{
		"NAME=template",
		"# @type bool",
		"ENABLED=TRUE",
		"# @type bool",
		"BROKEN_BOOL=true",
		"# @options development,staging,production",
		"ENVIRONMENT=development",
		"# @type port",
		"PORT=5432",
		"# @required",
		"REQUIRED=template",
		"# @secret",
		"# @type int",
		"SECRET=123",
		"# @fixed",
		"FIXED=template",
		"MULTILINE=template",
		"NEW=template",
		"",
	}, "\n")
	document := parseMergeTemplate(t, template)
	original := document
	original.Nodes = append([]domain.Node(nil), document.Nodes...)
	values := map[string]string{
		"NAME":        "existing",
		"ENABLED":     "FALSE",
		"BROKEN_BOOL": "yes",
		"ENVIRONMENT": "testing",
		"PORT":        "invalid",
		"REQUIRED":    "   ",
		"SECRET":      "do-not-show",
		"FIXED":       "existing-fixed",
		"OBSOLETE":    "ignore",
		"MULTILINE":   "first\nsecond",
	}

	merged, err := dotenv.MergeExisting(document, values)
	if err != nil {
		t.Fatalf("MergeExisting() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(document, original) {
		t.Fatal("MergeExisting() mutated its input Document")
	}

	variables := mergeVariables(merged)
	assertMergedValue(t, variables["NAME"], "existing", domain.ValueFromExisting)
	assertMergedValue(t, variables["ENABLED"], "false", domain.ValueFromExisting)
	assertMergedValue(t, variables["BROKEN_BOOL"], "true", domain.ValueFromTemplate)
	assertMergedValue(t, variables["ENVIRONMENT"], "development", domain.ValueFromTemplate)
	assertMergedValue(t, variables["PORT"], "5432", domain.ValueFromTemplate)
	assertMergedValue(t, variables["REQUIRED"], "template", domain.ValueFromTemplate)
	assertMergedValue(t, variables["SECRET"], "123", domain.ValueFromTemplate)
	assertMergedValue(t, variables["FIXED"], "template", domain.ValueFromFixed)
	assertMergedValue(t, variables["MULTILINE"], "template", domain.ValueFromTemplate)
	assertMergedValue(t, variables["NEW"], "template", domain.ValueFromTemplate)

	if issue := variables["BROKEN_BOOL"].ExistingValueIssue; issue == nil || !strings.Contains(issue.Message, "yes") {
		t.Fatalf("BROKEN_BOOL issue = %#v, want diagnostic containing invalid boolean", issue)
	}
	if issue := variables["ENVIRONMENT"].ExistingValueIssue; issue == nil || !strings.Contains(issue.Message, "testing") {
		t.Fatalf("ENVIRONMENT issue = %#v, want diagnostic containing invalid option", issue)
	}
	if issue := variables["PORT"].ExistingValueIssue; issue == nil || !strings.Contains(issue.Message, "invalid") {
		t.Fatalf("PORT issue = %#v, want diagnostic containing non-secret invalid value", issue)
	}
	if issue := variables["REQUIRED"].ExistingValueIssue; issue == nil || !strings.Contains(issue.Message, "value is required") {
		t.Fatalf("REQUIRED issue = %#v, want required diagnostic", issue)
	}
	secretIssue := variables["SECRET"].ExistingValueIssue
	if secretIssue == nil || secretIssue.Message != "The existing secret value is incompatible with this template. Confirm or enter a valid replacement." {
		t.Fatalf("SECRET issue = %#v, want value- and length-free diagnostic", secretIssue)
	}
	if variables["FIXED"].ExistingValueIssue != nil {
		t.Fatalf("FIXED issue = %#v, want existing value ignored", variables["FIXED"].ExistingValueIssue)
	}
	if issue := variables["MULTILINE"].ExistingValueIssue; issue == nil || !strings.Contains(issue.Message, `first\nsecond`) {
		t.Fatalf("MULTILINE issue = %#v, want safely quoted unsupported multiline value", issue)
	}

	groups, err := wizard.BuildQuestionGroups(merged)
	if err != nil {
		t.Fatalf("BuildQuestionGroups() error = %v, want nil", err)
	}
	portQuestion := findQuestion(t, groups, "PORT")
	if portQuestion.ExistingValueIssue == nil || portQuestion.ExistingValueIssue == variables["PORT"].ExistingValueIssue {
		t.Fatalf("PORT Question issue = %#v, want a defensive copy", portQuestion.ExistingValueIssue)
	}
}

func TestMergeExistingLeavesObsoleteKeysOutOfRenderedConfiguration(t *testing.T) {
	document := parseMergeTemplate(t, "CURRENT=template\n")
	merged, err := dotenv.MergeExisting(document, map[string]string{
		"CURRENT":  "existing",
		"OBSOLETE": "old",
	})
	if err != nil {
		t.Fatalf("MergeExisting() error = %v, want nil", err)
	}

	rendered, err := projectfs.RenderConfiguration(merged)
	if err != nil {
		t.Fatalf("RenderConfiguration() error = %v, want nil", err)
	}
	if got, want := rendered, []byte("CURRENT='existing'\n"); !bytes.Equal(got, want) {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func parseMergeTemplate(t *testing.T, content string) domain.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	document, err := dotenv.ParseTemplate(path)
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	return document
}

func mergeVariables(document domain.Document) map[string]domain.Variable {
	variables := make(map[string]domain.Variable)
	for _, node := range document.Nodes {
		if variable, ok := node.(domain.Variable); ok {
			variables[variable.Key] = variable
		}
	}
	return variables
}

func assertMergedValue(t *testing.T, variable domain.Variable, value string, source domain.ValueSource) {
	t.Helper()
	if variable.Value != value || variable.ValueSource != source || !variable.HasValue {
		t.Errorf("Variable %q = %#v, want value %q from %q", variable.Key, variable, value, source)
	}
}

func findQuestion(t *testing.T, groups []domain.QuestionGroup, key string) domain.Question {
	t.Helper()
	for _, group := range groups {
		for _, question := range group.Questions {
			if question.Key == key {
				return question
			}
		}
	}
	t.Fatalf("question %q not found", key)
	return domain.Question{}
}
