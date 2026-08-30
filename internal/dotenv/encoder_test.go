package dotenv_test

import (
	"strings"
	"testing"

	composedotenv "github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
)

func TestEncodeValueUsesCanonicalLiteralEncoding(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: "''"},
		{name: "plain", value: "plain", want: "'plain'"},
		{name: "surrounding whitespace", value: "  spaced  ", want: "'  spaced  '"},
		{name: "tab", value: "left\tright", want: "'left\tright'"},
		{name: "hash", value: "value # literal", want: "'value # literal'"},
		{name: "dollar name", value: "$VAR", want: "'$VAR'"},
		{name: "dollar braces", value: "${VAR}", want: "'${VAR}'"},
		{name: "double dollar", value: "$$", want: "'$$'"},
		{name: "double quote", value: `say "hello"`, want: `'say "hello"'`},
		{name: "apostrophe", value: "it's literal", want: "'it\\'s literal'"},
		{name: "backslash", value: `C:\path\file`, want: `'C:\path\file'`},
		{name: "trailing backslash", value: `trailing\`, want: `"trailing\\"`},
		{name: "backslash before apostrophe", value: "left\\'right", want: `"left\\'right"`},
		{name: "fallback escapes every special", value: "left\\'$VAR \\\"right", want: `"left\\'\$VAR \\\"right"`},
		{name: "unicode", value: "caffè ☕ 日本語", want: "'caffè ☕ 日本語'"},
		{name: "URL", value: "postgres://user:pass@db:5432/app?a=b#c", want: "'postgres://user:pass@db:5432/app?a=b#c'"},
		{name: "equals", value: "left=middle=right", want: "'left=middle=right'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := dotenv.EncodeValue(tt.value)
			if err != nil {
				t.Fatalf("EncodeValue() error = %v, want nil", err)
			}
			if encoded != tt.want {
				t.Errorf("EncodeValue() = %q, want %q", encoded, tt.want)
			}

			parsed, err := composedotenv.ParseWithLookup(
				strings.NewReader("KEY="+encoded+"\n"),
				func(string) (string, bool) { return "", false },
			)
			if err != nil {
				t.Fatalf("ParseWithLookup() error = %v", err)
			}
			if parsed["KEY"] != tt.value {
				t.Errorf("round trip value = %q, want %q", parsed["KEY"], tt.value)
			}
		})
	}
}

func TestEncodeValueRejectsUnsupportedContentWithoutLeakingIt(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "NUL", value: "do-not-show\x00rest"},
		{name: "carriage return", value: "do-not-show\rrest"},
		{name: "newline", value: "do-not-show\nrest"},
		{name: "invalid UTF-8", value: string([]byte{'d', 'o', 0xff})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dotenv.EncodeValue(tt.value)
			if err == nil {
				t.Fatal("EncodeValue() error = nil, want rejection")
			}
			if strings.Contains(err.Error(), "do-not-show") || strings.Contains(err.Error(), "rest") {
				t.Fatalf("EncodeValue() error leaked value content: %q", err)
			}
		})
	}
}

func TestEncodeValueRoundTripsSpecialCharacterCombinations(t *testing.T) {
	alphabet := []string{"a", "\\", "'", "\"", "$", "#", "="}
	var exercise func(prefix string, remaining int)
	exercise = func(prefix string, remaining int) {
		if remaining == 0 {
			encoded, err := dotenv.EncodeValue(prefix)
			if err != nil {
				t.Fatalf("EncodeValue(%q) error = %v", prefix, err)
			}
			parsed, err := composedotenv.ParseWithLookup(
				strings.NewReader("KEY="+encoded+"\n"),
				func(string) (string, bool) { return "", false },
			)
			if err != nil {
				t.Fatalf("ParseWithLookup(EncodeValue(%q)) error = %v", prefix, err)
			}
			if parsed["KEY"] != prefix {
				t.Fatalf("round trip value = %q, want %q", parsed["KEY"], prefix)
			}
			return
		}
		for _, character := range alphabet {
			exercise(prefix+character, remaining-1)
		}
	}

	for length := 1; length <= 4; length++ {
		exercise("", length)
	}
}

func TestUpdateValueStoresCanonicalAssignment(t *testing.T) {
	input := "# @type bool\nENABLED=FALSE\nNAME=old\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}
	for index, node := range document.Nodes {
		if variable, ok := node.(domain.Variable); ok && variable.Key == "NAME" {
			variable.ExistingValueIssue = &domain.ExistingValueIssue{Message: "resolved by update"}
			document.Nodes[index] = variable
		}
	}

	if err := dotenv.UpdateValue(&document, "ENABLED", "TRUE", domain.ValueFromUser); err != nil {
		t.Fatalf("UpdateValue(ENABLED) error = %v, want nil", err)
	}
	if err := dotenv.UpdateValue(&document, "NAME", "new $VALUE's", domain.ValueFromExisting); err != nil {
		t.Fatalf("UpdateValue(NAME) error = %v, want nil", err)
	}

	variables := documentVariables(document)
	enabled := variables["ENABLED"]
	if enabled.Value != "true" || enabled.RawValue != "'true'" || enabled.Raw != "ENABLED='true'" {
		t.Errorf("ENABLED = %#v, want canonical lowercase assignment", enabled)
	}
	if enabled.ValueSource != domain.ValueFromUser || !enabled.HasValue {
		t.Errorf("ENABLED source/presence = %q/%t, want user/true", enabled.ValueSource, enabled.HasValue)
	}

	name := variables["NAME"]
	if name.Value != "new $VALUE's" || name.Raw != "NAME='new $VALUE\\'s'" {
		t.Errorf("NAME = %#v, want literal canonical assignment", name)
	}
	if name.ValueSource != domain.ValueFromExisting {
		t.Errorf("NAME ValueSource = %q, want %q", name.ValueSource, domain.ValueFromExisting)
	}
	if name.ExistingValueIssue != nil {
		t.Errorf("NAME ExistingValueIssue = %#v, want nil after update", name.ExistingValueIssue)
	}
}

func TestUpdateValueRejectsInvalidUpdates(t *testing.T) {
	input := "NORMAL=value\n# @fixed\nFIXED=value\n"
	document, err := dotenv.ParseTemplate(writeTemplate(t, []byte(input)))
	if err != nil {
		t.Fatalf("ParseTemplate() error = %v, want nil", err)
	}

	tests := []struct {
		name   string
		doc    *domain.Document
		key    string
		value  string
		source domain.ValueSource
		want   string
	}{
		{name: "nil document", doc: nil, key: "NORMAL", value: "new", source: domain.ValueFromUser, want: "nil document"},
		{name: "unknown variable", doc: &document, key: "MISSING", value: "new", source: domain.ValueFromUser, want: "variable not found"},
		{name: "invalid source", doc: &document, key: "NORMAL", value: "new", source: domain.ValueSource("invalid"), want: "invalid value source"},
		{name: "change fixed as user", doc: &document, key: "FIXED", value: "new", source: domain.ValueFromUser, want: "fixed variables cannot be updated"},
		{name: "change fixed as fixed", doc: &document, key: "FIXED", value: "new", source: domain.ValueFromFixed, want: "fixed variables cannot be updated"},
		{name: "fixed source on normal", doc: &document, key: "NORMAL", value: "new", source: domain.ValueFromFixed, want: "non-fixed variables cannot"},
		{name: "unsupported value", doc: &document, key: "NORMAL", value: "do-not-show\nrest", source: domain.ValueFromUser, want: "unsupported NUL, CR, or LF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dotenv.UpdateValue(tt.doc, tt.key, tt.value, tt.source)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("UpdateValue() error = %v, want it to contain %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "do-not-show") || strings.Contains(err.Error(), "rest") {
				t.Fatalf("UpdateValue() error leaked value content: %q", err)
			}
		})
	}

	normal := documentVariables(document)["NORMAL"]
	if normal.Value != "value" || normal.Raw != "NORMAL=value" || normal.ValueSource != domain.ValueFromTemplate {
		t.Errorf("NORMAL changed after rejected updates: %#v", normal)
	}
}
