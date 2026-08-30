package filesystem_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	composedotenv "github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
)

func TestRenderConfigurationMatchesGoldenFile(t *testing.T) {
	templatePath := filepath.Join("..", "..", "testdata", "render.env.example")
	document, err := dotenv.ParseTemplate(templatePath)
	if err != nil {
		t.Fatalf("ParseTemplate(%q) error = %v, want nil", templatePath, err)
	}
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", templatePath, err)
	}
	if got := projectfs.RenderTemplate(document); !bytes.Equal(got, templateBytes) {
		t.Errorf("RenderTemplate() = %q, want byte-identical template %q", got, templateBytes)
	}

	if err := dotenv.UpdateValue(&document, "APP_NAME", "new name with $VAR and #hash", domain.ValueFromUser); err != nil {
		t.Fatalf("UpdateValue(APP_NAME) error = %v", err)
	}
	if err := dotenv.UpdateValue(&document, "TOKEN", "${TOKEN} $$", domain.ValueFromUser); err != nil {
		t.Fatalf("UpdateValue(TOKEN) error = %v", err)
	}

	got, err := projectfs.RenderConfiguration(document)
	if err != nil {
		t.Fatalf("RenderConfiguration() error = %v, want nil", err)
	}
	wantPath := filepath.Join("..", "..", "testdata", "render.env.golden")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", wantPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("RenderConfiguration() =\n%s\nwant golden output:\n%s", got, want)
	}
	if bytes.Contains(got, []byte("# @")) {
		t.Errorf("RenderConfiguration() retained annotation lines: %q", got)
	}

	values, err := composedotenv.ParseWithLookup(
		bytes.NewReader(got),
		func(string) (string, bool) { return "", false },
	)
	if err != nil {
		t.Fatalf("generated configuration is not Compose-compatible: %v", err)
	}
	wantValues := map[string]string{
		"APP_NAME":    "new name with $VAR and #hash",
		"TOKEN":       "${TOKEN} $$",
		"ENABLED":     "true",
		"FIXED_VALUE": "keep-original-style",
	}
	for key, wantValue := range wantValues {
		if values[key] != wantValue {
			t.Errorf("generated %s = %q, want %q", key, values[key], wantValue)
		}
	}
}

func TestRenderersPreserveLineEndingsAndFinalNewlineState(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{
			name: "CRLF without final newline",
			in:   []byte("# header\r\n# @prompt Name\r\nNAME=value"),
			want: []byte("# header\r\nNAME=value"),
		},
		{
			name: "LF with final newline",
			in:   []byte("# @section App\nNAME=value\n"),
			want: []byte("NAME=value\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFilesystemTemplate(t, tt.in)
			document, err := dotenv.ParseTemplate(path)
			if err != nil {
				t.Fatalf("ParseTemplate() error = %v, want nil", err)
			}
			if source := projectfs.RenderTemplate(document); !bytes.Equal(source, tt.in) {
				t.Errorf("RenderTemplate() = %q, want %q", source, tt.in)
			}
			got, err := projectfs.RenderConfiguration(document)
			if err != nil {
				t.Fatalf("RenderConfiguration() error = %v, want nil", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("RenderConfiguration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTemplateNormalizesInitialBOM(t *testing.T) {
	path := writeFilesystemTemplate(t, []byte("\xef\xbb\xbfKEY=value\n"))
	document, err := dotenv.ParseTemplate(path)
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	if got, want := projectfs.RenderTemplate(document), []byte("KEY=value\n"); !bytes.Equal(got, want) {
		t.Errorf("RenderTemplate() = %q, want normalized %q", got, want)
	}
}

func TestRenderConfigurationRejectsInvalidDocumentStateWithoutLeakingValues(t *testing.T) {
	base := domain.Document{
		Nodes: []domain.Node{domain.Variable{
			Key:         "SECRET",
			RawValue:    "old",
			Value:       "do-not-show\nrest",
			HasValue:    true,
			ValueSource: domain.ValueFromTemplate,
			Annotations: domain.Annotations{Secret: true, Type: domain.VariableTypeString},
			Raw:         "SECRET=old",
			Line:        4,
		}},
		LineEnding: domain.LineEndingLF,
	}

	_, err := projectfs.RenderConfiguration(base)
	if err == nil || !strings.Contains(err.Error(), "unsupported NUL, CR, or LF") {
		t.Fatalf("RenderConfiguration() error = %v, want unsupported value error", err)
	}
	if strings.Contains(err.Error(), "do-not-show") || strings.Contains(err.Error(), "rest") {
		t.Fatalf("RenderConfiguration() error leaked secret value: %q", err)
	}

	missing := base
	variable := missing.Nodes[0].(domain.Variable)
	variable.Value = "safe"
	variable.HasValue = false
	missing.Nodes = []domain.Node{variable}
	if _, err := projectfs.RenderConfiguration(missing); err == nil || !strings.Contains(err.Error(), "not assigned") {
		t.Fatalf("RenderConfiguration() error = %v, want unassigned value error", err)
	}

	badEnding := base
	badEnding.LineEnding = domain.LineEnding("invalid")
	if _, err := projectfs.RenderConfiguration(badEnding); err == nil || !strings.Contains(err.Error(), "unsupported line ending") {
		t.Fatalf("RenderConfiguration() error = %v, want line-ending error", err)
	}
}

func writeFilesystemTemplate(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env.example")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}
