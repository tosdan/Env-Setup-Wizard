package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/app"
)

func TestRunRejectsInvalidTypedTemplateBeforeInteractiveStage(t *testing.T) {
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte("# @type port\nPORT=70000\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
	})
	if err == nil || !strings.Contains(err.Error(), "parse template") || !strings.Contains(err.Error(), "range 1..65535") {
		t.Fatalf("Run() error = %v, want pre-wizard port validation error", err)
	}
}

func TestRunAcceptsAllFixedTemplateBeforeInteractiveStage(t *testing.T) {
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte("# @fixed\nONE=1\n# @fixed\nTWO=2\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
	})
	if err == nil || err.Error() != "interactive wizard not available yet" {
		t.Fatalf("Run() error = %v, want next-stage placeholder", err)
	}
}
