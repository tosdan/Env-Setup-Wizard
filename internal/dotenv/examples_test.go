package dotenv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestManualExamplesAreValidAndCoverEveryAnnotation(t *testing.T) {
	t.Parallel()

	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.env.example"))
	if err != nil {
		t.Fatalf("Glob examples: %v", err)
	}
	if len(examples) != 4 {
		t.Fatalf("found %d manual examples, want 4", len(examples))
	}

	var corpus strings.Builder
	for _, path := range examples {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			_, err := dotenv.ParseTemplate(path)
			if err != nil {
				t.Fatalf("ParseTemplate(%q): %v", path, err)
			}
		})

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", path, err)
		}
		corpus.Write(content)
		corpus.WriteByte('\n')
	}

	for _, marker := range []string{
		"# @prompt ",
		"# @description ",
		"# @required",
		"# @secret",
		"# @type string",
		"# @type int",
		"# @type bool",
		"# @type port",
		"# @type url",
		"# @options ",
		"# @placeholder ",
		"# @fixed",
		"# @section ",
	} {
		if !strings.Contains(corpus.String(), marker) {
			t.Errorf("manual examples do not cover %q", marker)
		}
	}
}
