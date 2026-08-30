package filesystem

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceFileCreatesAndOverwritesTarget(t *testing.T) {
	tests := []struct {
		name           string
		existingTarget bool
	}{
		{name: "create"},
		{name: "overwrite", existingTarget: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			stagedPath := filepath.Join(root, ".staged-è-候補")
			targetPath := filepath.Join(root, "configurazione-è.env")
			candidate := []byte("KEY=new\r\n")
			if err := os.WriteFile(stagedPath, candidate, 0o600); err != nil {
				t.Fatalf("WriteFile(%q): %v", stagedPath, err)
			}
			if test.existingTarget {
				if err := os.WriteFile(targetPath, []byte("KEY=old\n"), 0o600); err != nil {
					t.Fatalf("WriteFile(%q): %v", targetPath, err)
				}
			}

			if err := replaceFile(stagedPath, targetPath); err != nil {
				t.Fatalf("replaceFile() error = %v, want nil", err)
			}
			got, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", targetPath, err)
			}
			if !bytes.Equal(got, candidate) {
				t.Fatalf("target content = %q, want %q", got, candidate)
			}
			if _, err := os.Lstat(stagedPath); !os.IsNotExist(err) {
				t.Fatalf("staged path still exists after replace: %v", err)
			}
		})
	}
}

func TestReplaceFileFailureLeavesExistingTargetIntact(t *testing.T) {
	root := t.TempDir()
	stagedPath := filepath.Join(root, "missing-staged-file")
	targetPath := filepath.Join(root, ".env")
	original := []byte("KEY=old\n")
	if err := os.WriteFile(targetPath, original, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", targetPath, err)
	}

	err := replaceFile(stagedPath, targetPath)
	if err == nil || !strings.Contains(err.Error(), "replace output") {
		t.Fatalf("replaceFile() error = %v, want replacement failure", err)
	}
	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q): %v", targetPath, readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("target changed after failed replace: %q", got)
	}
}
