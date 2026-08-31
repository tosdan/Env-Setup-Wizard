package filesystem_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
)

func TestPreflightAcceptsValidPaths(t *testing.T) {
	tests := []struct {
		name           string
		existingOutput bool
	}{
		{name: "new output"},
		{name: "existing regular output", existingOutput: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			templatePath := writeTestFile(t, filepath.Join(root, ".env.example"))
			outputPath := filepath.Join(root, ".env")
			if tt.existingOutput {
				writeTestFile(t, outputPath)
			}

			if err := projectfs.Preflight(templatePath, outputPath); err != nil {
				t.Fatalf("Preflight() error = %v, want nil", err)
			}
		})
	}
}

func TestPreflightTemplateDoesNotRequireAnOutputPath(t *testing.T) {
	templatePath := writeTestFile(t, filepath.Join(t.TempDir(), ".env.example"))
	if err := projectfs.PreflightTemplate(templatePath); err != nil {
		t.Fatalf("PreflightTemplate() error = %v, want nil", err)
	}
}

func TestPreflightAcceptsTemplateSymlink(t *testing.T) {
	root := t.TempDir()
	targetPath := writeTestFile(t, filepath.Join(root, "template-source"))
	templatePath := filepath.Join(root, ".env.example")
	createSymlinkOrSkip(t, targetPath, templatePath)

	if err := projectfs.Preflight(templatePath, filepath.Join(root, ".env")); err != nil {
		t.Fatalf("Preflight() error = %v, want nil", err)
	}
}

func TestPreflightRejectsInvalidTemplate(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name        string
		template    string
		wantMessage string
	}{
		{
			name:        "relative path",
			template:    ".env.example",
			wantMessage: "template path \".env.example\" must be absolute",
		},
		{
			name:        "missing file",
			template:    filepath.Join(root, "missing.env.example"),
			wantMessage: "inspect template",
		},
		{
			name:        "directory",
			template:    root,
			wantMessage: "is not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := projectfs.Preflight(tt.template, filepath.Join(root, ".env"))
			requireErrorContaining(t, err, tt.wantMessage)
		})
	}
}

func TestPreflightClassifiesMissingTemplate(t *testing.T) {
	root := t.TempDir()
	err := projectfs.Preflight(
		filepath.Join(root, "missing.env.example"),
		filepath.Join(root, ".env"),
	)

	if !errors.Is(err, projectfs.ErrTemplateNotFound) {
		t.Fatalf("Preflight() error = %v, want ErrTemplateNotFound", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Preflight() error = %v, want it to preserve os.ErrNotExist", err)
	}
}

func TestPreflightRejectsInvalidOutputParent(t *testing.T) {
	root := t.TempDir()
	templatePath := writeTestFile(t, filepath.Join(root, ".env.example"))
	notDirectory := writeTestFile(t, filepath.Join(root, "parent-file"))

	tests := []struct {
		name        string
		output      string
		wantMessage string
	}{
		{
			name:        "relative path",
			output:      ".env",
			wantMessage: "output path \".env\" must be absolute",
		},
		{
			name:        "missing directory",
			output:      filepath.Join(root, "missing", ".env"),
			wantMessage: "inspect output directory",
		},
		{
			name:        "parent is a file",
			output:      filepath.Join(notDirectory, ".env"),
			wantMessage: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := projectfs.Preflight(templatePath, tt.output)
			requireErrorContaining(t, err, tt.wantMessage)
		})
	}
}

func TestPreflightRejectsInvalidExistingOutput(t *testing.T) {
	root := t.TempDir()
	templatePath := writeTestFile(t, filepath.Join(root, ".env.example"))

	t.Run("directory", func(t *testing.T) {
		outputPath := filepath.Join(root, "output-directory")
		if err := os.Mkdir(outputPath, 0o755); err != nil {
			t.Fatalf("Mkdir(%q): %v", outputPath, err)
		}

		err := projectfs.Preflight(templatePath, outputPath)
		requireErrorContaining(t, err, "is not a regular file")
	})

	t.Run("symbolic link", func(t *testing.T) {
		targetPath := writeTestFile(t, filepath.Join(root, "output-target"))
		outputPath := filepath.Join(root, "output-link")
		createSymlinkOrSkip(t, targetPath, outputPath)

		err := projectfs.Preflight(templatePath, outputPath)
		requireErrorContaining(t, err, "must not be a symbolic link")
	})
}

func TestPreflightRejectsSameFile(t *testing.T) {
	t.Run("same spelling after cleaning", func(t *testing.T) {
		root := t.TempDir()
		templatePath := writeTestFile(t, filepath.Join(root, ".env.example"))
		outputPath := filepath.Join(root, "nested", "..", ".env.example")

		err := projectfs.Preflight(templatePath, outputPath)
		requireErrorContaining(t, err, "identify the same file")
	})

	t.Run("hardlink", func(t *testing.T) {
		root := t.TempDir()
		templatePath := writeTestFile(t, filepath.Join(root, ".env.example"))
		outputPath := filepath.Join(root, ".env")
		if err := os.Link(templatePath, outputPath); err != nil {
			t.Skipf("hardlinks are unavailable: %v", err)
		}

		err := projectfs.Preflight(templatePath, outputPath)
		requireErrorContaining(t, err, "identify the same file")
	})

	t.Run("template symlink to output", func(t *testing.T) {
		root := t.TempDir()
		outputPath := writeTestFile(t, filepath.Join(root, ".env"))
		templatePath := filepath.Join(root, ".env.example")
		createSymlinkOrSkip(t, outputPath, templatePath)

		err := projectfs.Preflight(templatePath, outputPath)
		requireErrorContaining(t, err, "identify the same file")
	})

	if runtime.GOOS == "windows" {
		t.Run("case-insensitive spelling on Windows", func(t *testing.T) {
			root := t.TempDir()
			templatePath := writeTestFile(t, filepath.Join(root, "Template.env"))
			outputPath := filepath.Join(root, "template.env")

			err := projectfs.Preflight(templatePath, outputPath)
			requireErrorContaining(t, err, "identify the same file")
		})
	}
}

func writeTestFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte("KEY=value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}

	return path
}

func createSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
}

func requireErrorContaining(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Preflight() error = nil, want it to contain %q", message)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("Preflight() error = %q, want it to contain %q", err, message)
	}
}
