package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/app"
)

func TestRunPerformsPathPreflight(t *testing.T) {
	root := t.TempDir()
	err := app.Run(context.Background(), app.Options{
		TemplatePath: filepath.Join(root, "missing.env.example"),
		OutputPath:   filepath.Join(root, ".env"),
	})

	if err == nil {
		t.Fatal("Run() error = nil, want path preflight error")
	}
	if !strings.Contains(err.Error(), "preflight paths: inspect template") {
		t.Fatalf("Run() error = %q, want contextual path preflight error", err)
	}
}

func TestRunReachesNextPipelineStageAfterSuccessfulPreflight(t *testing.T) {
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte("KEY=value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
	})

	if err == nil || err.Error() != "validation and question model not available yet" {
		t.Fatalf("Run() error = %v, want next-stage placeholder", err)
	}
}

func TestRunLoadsTemplateBeforeNextPipelineStage(t *testing.T) {
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	if err := os.WriteFile(templatePath, []byte{0xff, 'K'}, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   filepath.Join(root, ".env"),
	})

	if err == nil {
		t.Fatal("Run() error = nil, want template decoding error")
	}
	if !strings.Contains(err.Error(), "parse template: decode template") {
		t.Fatalf("Run() error = %q, want contextual template decoding error", err)
	}
}

func TestRunHonorsCanceledContextBeforeFilesystemAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx, app.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}
