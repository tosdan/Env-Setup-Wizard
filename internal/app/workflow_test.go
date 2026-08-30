package app_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/tosdan/env-setup-wizard/internal/app"
)

func TestRunDeclinesCreationWithoutWriting(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_, templatePath, outputPath := workflowPaths(t, "KEY=old\n", nil)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("new\nn\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("output exists after declined creation: %v", statErr)
	}
	assertOutputContains(t, output.String(), "Summary", "KEY  new", "Create .env?", "[Y/n]", "No changes made.")
}

func TestRunAcceptsCreationBeforeSafeWriteStage(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_, templatePath, outputPath := workflowPaths(t, "KEY=old\n", nil)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("new\n\n", &output),
	})
	if err == nil || err.Error() != "safe write not available yet" {
		t.Fatalf("Run() error = %v, want safe-write placeholder", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("output exists before safe-write stage: %v", statErr)
	}
}

func TestRunUsesCompatibleExistingValueAndDeclinesOverwrite(t *testing.T) {
	t.Setenv("TERM", "dumb")
	existingContent := []byte("KEY=existing\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=template\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("\n\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, existingContent)
	assertOutputContains(t, output.String(), "KEY  existing", "Overwrite existing .env?", "[y/N]", "No changes made.")
}

func TestRunReportsByteIdenticalExistingOutputWithoutConfirmation(t *testing.T) {
	t.Setenv("TERM", "dumb")
	existingContent := []byte("KEY='same'\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=same\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Force:        true,
		Runtime:      interactiveRuntime("\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, existingContent)
	assertOutputContains(t, output.String(), "Summary", "KEY  same", "No changes detected.")
	if strings.Contains(output.String(), "Overwrite existing") {
		t.Fatalf("output = %q, want no confirmation for byte-identical result", output.String())
	}
}

func TestRunTreatsSemanticOnlyEqualityAsOverwrite(t *testing.T) {
	t.Setenv("TERM", "dumb")
	existingContent := []byte("KEY=same\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=same\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("\n\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, existingContent)
	assertOutputContains(t, output.String(), "Overwrite existing .env?", "No changes made.")
}

func TestRunPreservesExistingSecretWithoutDisplayingIt(t *testing.T) {
	t.Setenv("TERM", "dumb")
	const secret = "do-not-show"
	existingContent := []byte("TOKEN='" + secret + "'\n")
	_, templatePath, outputPath := workflowPaths(t, "# @secret\nTOKEN=template\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Force:        true,
		Runtime:      interactiveRuntime("", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil for byte-identical secret output", err)
	}
	assertFileContent(t, outputPath, existingContent)
	assertOutputContains(t, output.String(), "TOKEN  [set]", "No changes detected.")
	if strings.Contains(output.String(), secret) {
		t.Fatalf("output leaked existing secret: %q", output.String())
	}
}

func TestRunShowsRecoverableExistingValueDiagnostic(t *testing.T) {
	t.Setenv("TERM", "dumb")
	existingContent := []byte("PORT=invalid\n")
	_, templatePath, outputPath := workflowPaths(t, "# @type port\nPORT=5432\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("08080\n08080\nn\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, existingContent)
	assertOutputContains(t, output.String(), `existing value "invalid" is incompatible`, "PORT  08080", "No changes made.")
}

func TestRunForceSkipsConfirmationButNotSummaryOrNoOpDetection(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_, templatePath, outputPath := workflowPaths(t, "KEY=old\n", nil)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Force:        true,
		Runtime:      interactiveRuntime("new\n", &output),
	})
	if err == nil || err.Error() != "safe write not available yet" {
		t.Fatalf("Run() error = %v, want safe-write placeholder", err)
	}
	assertOutputContains(t, output.String(), "Summary", "KEY  new")
	if strings.Contains(output.String(), "Create .env?") {
		t.Fatalf("output = %q, want --force to skip only confirmation", output.String())
	}
}

func TestRunRejectsInvalidExistingOutputBeforeWizard(t *testing.T) {
	existingContent := []byte("KEY=first\nKEY=second\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=template\n", existingContent)

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      nonInteractiveRuntime(),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate variable") {
		t.Fatalf("Run() error = %v, want duplicate existing-variable error", err)
	}
	assertFileContent(t, outputPath, existingContent)
}

func workflowPaths(t *testing.T, template string, existing []byte) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	templatePath := filepath.Join(root, ".env.example")
	outputPath := filepath.Join(root, ".env")
	if err := os.WriteFile(templatePath, []byte(template), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}
	if existing != nil {
		if err := os.WriteFile(outputPath, existing, 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", outputPath, err)
		}
	}
	return root, templatePath, outputPath
}

func interactiveRuntime(input string, output *bytes.Buffer) *app.Runtime {
	return &app.Runtime{
		Input:       iotest.OneByteReader(strings.NewReader(input)),
		Output:      output,
		Interactive: true,
	}
}

func assertOutputContains(t *testing.T, output string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(output, fragment) {
			t.Errorf("output = %q, want it to contain %q", output, fragment)
		}
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file content = %q, want %q", got, want)
	}
}
