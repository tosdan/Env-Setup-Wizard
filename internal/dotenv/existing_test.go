package dotenv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestLoadExistingReturnsAbsentState(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	existing, err := dotenv.LoadExisting(path)
	if err != nil {
		t.Fatalf("LoadExisting() error = %v, want nil", err)
	}
	if existing.Exists || existing.Values != nil || existing.Content != nil {
		t.Fatalf("ExistingFile = %#v, want absent state", existing)
	}
}

func TestLoadExistingAcceptsComposeSyntaxWithControlledLookup(t *testing.T) {
	t.Setenv("ENV_WIZARD_PROCESS_ONLY", "do-not-use")
	content := append(
		[]byte{0xef, 0xbb, 0xbf},
		[]byte(strings.Join([]string{
			"export NAME = \"existing value\"",
			"PORT: 05432",
			"FROM_LOOKUP=${ENV_WIZARD_PROCESS_ONLY:-fallback}",
			"MULTILINE='first",
			"second'",
			"",
		}, "\n"))...,
	)
	path := writeExistingFile(t, content)

	existing, err := dotenv.LoadExisting(path)
	if err != nil {
		t.Fatalf("LoadExisting() error = %v, want nil", err)
	}
	if !existing.Exists {
		t.Fatal("ExistingFile.Exists = false, want true")
	}
	if !bytes.Equal(existing.Content, content) {
		t.Fatalf("ExistingFile.Content = %q, want original bytes %q", existing.Content, content)
	}
	want := map[string]string{
		"NAME":        "existing value",
		"PORT":        "05432",
		"FROM_LOOKUP": "fallback",
		"MULTILINE":   "first\nsecond",
	}
	if len(existing.Values) != len(want) {
		t.Fatalf("ExistingFile.Values = %#v, want %#v", existing.Values, want)
	}
	for key, value := range want {
		if existing.Values[key] != value {
			t.Errorf("ExistingFile.Values[%q] = %q, want %q", key, existing.Values[key], value)
		}
	}
}

func TestLoadExistingRejectsInvalidFilesWithoutLeakingValues(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		wantMessage string
	}{
		{name: "invalid UTF-8", content: []byte{0xff, 'K'}, wantMessage: "not valid UTF-8"},
		{
			name:        "invalid Compose syntax",
			content:     []byte("SECRET='do-not-show\n"),
			wantMessage: "invalid Compose dotenv syntax",
		},
		{
			name:        "duplicate",
			content:     []byte("export KEY=first\nKEY: second\n"),
			wantMessage: "line 2: duplicate variable \"KEY\"; first declared at line 1",
		},
		{
			name:        "duplicate inherited key at end of file",
			content:     []byte("KEY=first\nKEY"),
			wantMessage: "line 2: duplicate variable \"KEY\"; first declared at line 1",
		},
		{
			name: "duplicate after multiline",
			content: []byte(strings.Join([]string{
				"KEY='first",
				"NOT_A_KEY=value",
				"last'",
				"KEY=second",
				"",
			}, "\n")),
			wantMessage: "line 4: duplicate variable \"KEY\"; first declared at line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.LoadExisting(writeExistingFile(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("LoadExisting() error = %v, want it to contain %q", err, tt.wantMessage)
			}
			if strings.Contains(err.Error(), "do-not-show") {
				t.Fatalf("LoadExisting() error leaked value: %q", err)
			}
		})
	}
}

func writeExistingFile(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}
