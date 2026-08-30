package dotenv

import (
	"strings"
	"testing"

	composedotenv "github.com/compose-spec/compose-go/v2/dotenv"
)

func TestComposeGoControlledLiteralParsing(t *testing.T) {
	const source = "DOLLAR='$VAR ${VAR} $$'\nAPOSTROPHE='single\\'quote'\n"

	values, err := composedotenv.ParseWithLookup(
		strings.NewReader(source),
		func(string) (string, bool) { return "", false },
	)
	if err != nil {
		t.Fatalf("ParseWithLookup() error = %v", err)
	}

	if got, want := values["DOLLAR"], "$VAR ${VAR} $$"; got != want {
		t.Fatalf("DOLLAR = %q, want %q", got, want)
	}
	if got, want := values["APOSTROPHE"], "single'quote"; got != want {
		t.Fatalf("APOSTROPHE = %q, want %q", got, want)
	}
}

func TestSemanticAdapterNeverReadsProcessEnvironment(t *testing.T) {
	t.Setenv("OUTSIDE_VALUE", "must-not-be-used")

	values, err := parseSemanticValues("VALUE=$OUTSIDE_VALUE\n")
	if err != nil {
		t.Fatalf("parseSemanticValues() error = %v", err)
	}
	if got := values["VALUE"]; got != "" {
		t.Fatalf("VALUE = %q, want empty value from controlled lookup", got)
	}
}

func TestSemanticAdapterCanResolveEarlierTemplateVariables(t *testing.T) {
	values, err := parseSemanticValues("BASE=inside\nVALUE=$BASE\n")
	if err != nil {
		t.Fatalf("parseSemanticValues() error = %v", err)
	}
	if got, want := values["VALUE"], "inside"; got != want {
		t.Fatalf("VALUE = %q, want %q", got, want)
	}
}

func TestSafeComposeErrorDoesNotExposeSourceContent(t *testing.T) {
	err := safeComposeError(&composeTestError{message: "line 7: invalid super-secret value"})
	if got, want := err.Error(), "invalid Compose dotenv syntax at line 7"; got != want {
		t.Fatalf("safeComposeError() = %q, want %q", got, want)
	}
}

type composeTestError struct {
	message string
}

func (err *composeTestError) Error() string {
	return err.message
}
