package dotenv_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestParseTemplateBuildsOrderedDocument(t *testing.T) {
	input := "# Application\n\n# @prompt Application name\nAPP_NAME='example'\nAPP_URL=https://example.test/path?a=b\nEMPTY=\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	if len(document.Nodes) != 6 {
		t.Fatalf("len(Nodes) = %d, want 6", len(document.Nodes))
	}
	assertNode[domain.Comment](t, document.Nodes[0], 1, "# Application")
	assertNode[domain.BlankLine](t, document.Nodes[1], 2, "")
	assertNode[domain.AnnotationLine](t, document.Nodes[2], 3, "# @prompt Application name")

	appName := assertNode[domain.Variable](t, document.Nodes[3], 4, "APP_NAME='example'")
	if appName.Key != "APP_NAME" || appName.RawValue != "'example'" {
		t.Errorf("APP_NAME Variable = %#v", appName)
	}
	appURL := assertNode[domain.Variable](t, document.Nodes[4], 5, "APP_URL=https://example.test/path?a=b")
	if appURL.Key != "APP_URL" || appURL.RawValue != "https://example.test/path?a=b" {
		t.Errorf("APP_URL Variable = %#v", appURL)
	}
	empty := assertNode[domain.Variable](t, document.Nodes[5], 6, "EMPTY=")
	if empty.Key != "EMPTY" || empty.RawValue != "" {
		t.Errorf("EMPTY Variable = %#v", empty)
	}
	if got := reconstruct(document); got != input {
		t.Errorf("structural round trip = %q, want %q", got, input)
	}
}

func TestParseTemplateRecognizesSupportedVariableForms(t *testing.T) {
	input := strings.Join([]string{
		"EMPTY=",
		"PLAIN=value",
		"DOUBLE=\"value\"",
		"SINGLE='value'",
		"EQUALS=left=middle=right",
		"INTERNAL_SPACE=hello world",
		"HASH=value#fragment",
		"LITERAL_DOLLAR='$VAR ${VAR} $$'",
		"DOUBLE_ESCAPED=\"\\$VAR\"",
		"DOUBLE_DOLLAR=$$",
	}, "\n")

	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	if len(document.Nodes) != 10 {
		t.Fatalf("len(Nodes) = %d, want 10", len(document.Nodes))
	}
	for index, node := range document.Nodes {
		if _, ok := node.(domain.Variable); !ok {
			t.Errorf("Nodes[%d] type = %T, want domain.Variable", index, node)
		}
	}
}

func TestParseTemplateDistinguishesAnnotationsFromComments(t *testing.T) {
	input := "  # normal comment\n#@required\n\t#  @required\nKEY=value\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	assertNode[domain.Comment](t, document.Nodes[0], 1, "  # normal comment")
	assertNode[domain.Comment](t, document.Nodes[1], 2, "#@required")
	annotation := assertNode[domain.AnnotationLine](t, document.Nodes[2], 3, "\t#  @required")
	if annotation.Name != domain.AnnotationRequired || annotation.Value != "" {
		t.Errorf("AnnotationLine = %#v, want parsed @required flag", annotation)
	}
}

func TestParseTemplateRejectsDuplicateVariables(t *testing.T) {
	path := writeTemplate(t, []byte("DUPLICATE=first\nDUPLICATE=second\n"))

	_, err := dotenv.ParseTemplate(path)
	requireScanError(t, err, "line 2: duplicate variable \"DUPLICATE\"; first declared at line 1")
}

func TestParseTemplateProjectFixtures(t *testing.T) {
	for _, name := range []string{"basic.env.example", "annotations.env.example", "quoting.env.example"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", name)
			if _, err := dotenv.ParseTemplate(path); err != nil {
				t.Fatalf("ParseTemplate(%q) error = %v, want nil", path, err)
			}
		})
	}

	duplicatePath := filepath.Join("..", "..", "testdata", "invalid", "duplicate.env.example")
	_, err := dotenv.ParseTemplate(duplicatePath)
	requireScanError(t, err, "line 2: duplicate variable \"DUPLICATE\"; first declared at line 1")
}

func TestParseTemplateRejectsUnsupportedStructure(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantMessage string
	}{
		{name: "missing assignment", line: "KEY", wantMessage: "expected a KEY=value assignment or comment"},
		{name: "colon assignment", line: "KEY: value", wantMessage: "':' assignments are not supported"},
		{name: "export prefix", line: "export KEY=value", wantMessage: "export prefixes are not supported"},
		{name: "invalid leading digit", line: "1KEY=value", wantMessage: "invalid variable key \"1KEY\""},
		{name: "invalid key space", line: "BAD KEY=value", wantMessage: "invalid variable key \"BAD KEY\""},
		{name: "leading value whitespace", line: "KEY= value", wantMessage: "leading or trailing whitespace must be quoted"},
		{name: "trailing value whitespace", line: "KEY=value ", wantMessage: "leading or trailing whitespace must be quoted"},
		{name: "unterminated single quote", line: "KEY='value", wantMessage: "unterminated quoted value"},
		{name: "unterminated double quote", line: "KEY=\"value", wantMessage: "unterminated quoted value"},
		{name: "trailing quoted content", line: "KEY='value' extra", wantMessage: "quoted values cannot have trailing characters"},
		{name: "partial quote", line: "KEY=value'other", wantMessage: "quotes must surround the complete value"},
		{name: "inline comment", line: "KEY=value # comment", wantMessage: "unquoted inline comments are not supported"},
		{name: "unquoted interpolation", line: "KEY=$OTHER", wantMessage: "active interpolation is not supported"},
		{name: "braced interpolation", line: "KEY=${OTHER}", wantMessage: "active interpolation is not supported"},
		{name: "double quoted interpolation", line: "KEY=\"$OTHER\"", wantMessage: "active interpolation is not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.ParseTemplate(writeTemplate(t, []byte(tt.line+"\n")))
			requireScanError(t, err, "line 1:")
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("ParseTemplate() error = %q, want it to contain %q", err, tt.wantMessage)
			}
		})
	}
}

func assertNode[T domain.Node](t *testing.T, node domain.Node, line int, raw string) T {
	t.Helper()
	typed, ok := node.(T)
	if !ok {
		t.Fatalf("node type = %T, want requested type", node)
	}
	if typed.LineNumber() != line || typed.RawLine() != raw {
		t.Errorf("node line/raw = %d/%q, want %d/%q", typed.LineNumber(), typed.RawLine(), line, raw)
	}

	return typed
}

func requireScanError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ParseTemplate() error = nil, want it to contain %q", message)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("ParseTemplate() error = %q, want it to contain %q", err, message)
	}
}
