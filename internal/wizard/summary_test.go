package wizard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

func TestRenderSummaryMergesSectionsAndRedactsSecrets(t *testing.T) {
	input := strings.Join([]string{
		"ROOT=root",
		"# @section Secrets",
		"# @prompt API token",
		"# @secret",
		"TOKEN='do-not-show'",
		"# @secret",
		"EMPTY=",
		"# @section Empty",
		"# @section Database",
		"# @prompt Database port",
		"PORT=5432",
		"# @section Secrets",
		"# @fixed",
		"# @secret",
		"FIXED_SECRET='also-do-not-show'",
		"# @section Configuration",
		"CONTROL='before\tafter'",
		"",
	}, "\n")
	document := parseSummaryTemplate(t, input)

	summary, err := wizard.RenderSummary(document)
	if err != nil {
		t.Fatalf("RenderSummary() error = %v, want nil", err)
	}
	want := strings.Join([]string{
		"Summary",
		"",
		"[Configuration]",
		"  ROOT     root",
		`  CONTROL  before\tafter`,
		"",
		"[Secrets]",
		"  API token (TOKEN)  [set]",
		"  EMPTY              [not set]",
		"  FIXED_SECRET       [set]",
		"",
		"[Database]",
		"  Database port (PORT)  5432",
		"",
	}, "\n")
	if summary != want {
		t.Fatalf("RenderSummary() =\n%s\nwant:\n%s", summary, want)
	}
	for _, secret := range []string{"do-not-show", "also-do-not-show"} {
		if strings.Contains(summary, secret) {
			t.Fatalf("RenderSummary() leaked secret %q", secret)
		}
	}
	if strings.Contains(summary, "[Empty]") {
		t.Fatal("RenderSummary() emitted an empty section")
	}
}

func TestRenderSummaryRejectsMissingResolvedValue(t *testing.T) {
	_, err := wizard.RenderSummary(domain.Document{
		Nodes: []domain.Node{domain.Variable{Key: "MISSING", Section: "Configuration"}},
	})
	if err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Fatalf("RenderSummary() error = %v, want missing-value error", err)
	}
}

func parseSummaryTemplate(t *testing.T, input string) domain.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	document, err := dotenv.ParseTemplate(path)
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	return document
}
