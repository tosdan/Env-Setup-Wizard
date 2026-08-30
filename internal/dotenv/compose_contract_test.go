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
