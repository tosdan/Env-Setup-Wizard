package app_test

import (
	"bytes"
	"context"
	"io"
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

func TestRunAcceptsCreationAndWritesOutput(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_, templatePath, outputPath := workflowPaths(t, "KEY=old\n", nil)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("new\n\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, []byte("KEY='new'\n"))
	assertOutputContains(t, output.String(), "Summary", "KEY  new", "Create .env?", "Created .env.")
	assertNoWorkflowBackups(t, outputPath)
}

func TestRunLetsUserChooseOutputForNamedTemplate(t *testing.T) {
	t.Setenv("TERM", "dumb")
	tests := []struct {
		name         string
		selection    string
		selectedName string
		otherName    string
	}{
		{name: "matching template", selection: "1\n", selectedName: "typed-values.env", otherName: ".env"},
		{name: "project default", selection: "2\n", selectedName: ".env", otherName: "typed-values.env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			templatePath := filepath.Join(root, "typed-values.env.example")
			if err := os.WriteFile(templatePath, []byte("KEY=old\n"), 0o600); err != nil {
				t.Fatalf("WriteFile(%q): %v", templatePath, err)
			}
			var output bytes.Buffer
			err := app.Run(context.Background(), app.Options{
				TemplatePath:        templatePath,
				OutputPath:          filepath.Join(root, ".env"),
				SuggestedOutputPath: filepath.Join(root, "typed-values.env"),
				Force:               true,
				Runtime:             interactiveRuntime(tt.selection+"new\n", &output),
			})
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			assertFileContent(t, filepath.Join(root, tt.selectedName), []byte("KEY='new'\n"))
			if _, statErr := os.Stat(filepath.Join(root, tt.otherName)); !os.IsNotExist(statErr) {
				t.Fatalf("unselected output %q exists: %v", tt.otherName, statErr)
			}
			assertOutputContains(
				t,
				output.String(),
				"Where should the configuration be saved?",
				"Created "+tt.selectedName+".",
			)
		})
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
	assertNoWorkflowBackups(t, outputPath)
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
	assertNoWorkflowBackups(t, outputPath)
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

func TestRunAcceptsOverwriteAndReportsBackup(t *testing.T) {
	t.Setenv("TERM", "dumb")
	existingContent := []byte("KEY=old\r\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=template\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Runtime:      interactiveRuntime("new\ny\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, []byte("KEY='new'\n"))
	backupPath := singleWorkflowBackup(t, outputPath)
	assertFileContent(t, backupPath, existingContent)
	assertOutputContains(t, output.String(), "Overwrite existing .env?", "Updated .env.", "Backup created: "+backupPath)
}

func TestRunPreservesExistingSecretWithoutDisplayingIt(t *testing.T) {
	t.Setenv("TERM", "dumb")
	const secret = "do-not-show"
	existingContent := []byte("# @secret\nTOKEN='" + secret + "'\n")
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
	assertNoWorkflowBackups(t, outputPath)
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
	existingContent := []byte("KEY=old\n")
	_, templatePath, outputPath := workflowPaths(t, "KEY=template\n", existingContent)
	var output bytes.Buffer

	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Force:        true,
		Runtime:      interactiveRuntime("new\n", &output),
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	assertFileContent(t, outputPath, []byte("KEY='new'\n"))
	backupPath := singleWorkflowBackup(t, outputPath)
	assertFileContent(t, backupPath, existingContent)
	assertOutputContains(t, output.String(), "Summary", "KEY  new", "Updated .env.", "Backup created: "+backupPath)
	if strings.Contains(output.String(), "Overwrite existing .env?") {
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

func TestRunRevalidatesOutputAfterWizard(t *testing.T) {
	t.Setenv("TERM", "dumb")
	root, templatePath, outputPath := workflowPaths(t, "KEY=old\n", nil)
	protectedPath := filepath.Join(root, "protected.env")
	protectedContent := []byte("PROTECTED=unchanged\n")
	if err := os.WriteFile(protectedPath, protectedContent, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", protectedPath, err)
	}

	var mutationErr error
	input := &mutatingReader{
		reader: iotest.OneByteReader(strings.NewReader("new\n")),
		mutate: func() {
			mutationErr = os.Symlink(protectedPath, outputPath)
		},
	}
	var output bytes.Buffer
	err := app.Run(context.Background(), app.Options{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Force:        true,
		Runtime: &app.Runtime{
			Input:       input,
			Output:      &output,
			Interactive: true,
		},
	})
	if mutationErr != nil {
		t.Skipf("symbolic links are unavailable: %v", mutationErr)
	}
	if err == nil || !strings.Contains(err.Error(), "write output safely") || !strings.Contains(err.Error(), "preflight safe write") || !strings.Contains(err.Error(), "must not be a symbolic link") {
		t.Fatalf("Run() error = %v, want pre-write symlink rejection", err)
	}
	assertFileContent(t, protectedPath, protectedContent)
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

type mutatingReader struct {
	reader io.Reader
	mutate func()
	done   bool
}

func (reader *mutatingReader) Read(buffer []byte) (int, error) {
	if !reader.done {
		reader.done = true
		reader.mutate()
	}
	return reader.reader.Read(buffer)
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

func singleWorkflowBackup(t *testing.T, outputPath string) string {
	t.Helper()
	backups, err := filepath.Glob(outputPath + ".backup-*")
	if err != nil {
		t.Fatalf("Glob backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want exactly one", backups)
	}
	return backups[0]
}

func assertNoWorkflowBackups(t *testing.T, outputPath string) {
	t.Helper()
	backups, err := filepath.Glob(outputPath + ".backup-*")
	if err != nil {
		t.Fatalf("Glob backups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("backups = %v, want none", backups)
	}
}
