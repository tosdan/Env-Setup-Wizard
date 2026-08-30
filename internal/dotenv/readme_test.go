package dotenv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestReadmeTemplateExampleIsValid(t *testing.T) {
	readmePath := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", readmePath, err)
	}
	readme := strings.ReplaceAll(string(data), "\r\n", "\n")
	const fence = "```dotenv\n"
	start := strings.Index(readme, fence)
	if start < 0 {
		t.Fatal("README does not contain a dotenv template example")
	}
	start += len(fence)
	end := strings.Index(readme[start:], "\n```")
	if end < 0 {
		t.Fatal("README dotenv template example has no closing fence")
	}
	example := readme[start : start+end]

	templatePath := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(templatePath, []byte(example+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", templatePath, err)
	}
	if _, err := dotenv.ParseTemplate(templatePath); err != nil {
		t.Fatalf("README dotenv template example is invalid: %v", err)
	}
}
