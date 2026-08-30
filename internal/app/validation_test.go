package app_test

import (
	"context"
	"io"
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
		Force:        true,
		Runtime: &app.Runtime{
			Output:      io.Discard,
			Interactive: true,
		},
	})
	if err == nil || err.Error() != "safe write not available yet" {
		t.Fatalf("Run() error = %v, want next-stage placeholder", err)
	}
}

func TestRunForceDoesNotBypassTerminalRequirement(t *testing.T) {
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte("KEY=value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
		Force:        true,
		Runtime:      nonInteractiveRuntime(),
	})
	if err == nil || !strings.Contains(err.Error(), "interactive terminal is required") {
		t.Fatalf("Run() error = %v, want terminal requirement", err)
	}
}

func TestRunCompletesWizardBeforeNextStage(t *testing.T) {
	t.Setenv("TERM", "dumb")
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte("# @required\nKEY=old\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
		Force:        true,
		Runtime: &app.Runtime{
			Input:       strings.NewReader("new\n"),
			Output:      &strings.Builder{},
			Interactive: true,
		},
	})
	if err == nil || err.Error() != "safe write not available yet" {
		t.Fatalf("Run() error = %v, want next-stage placeholder", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".env")); !os.IsNotExist(statErr) {
		t.Fatalf("output was created before summary and confirmation: %v", statErr)
	}
}
