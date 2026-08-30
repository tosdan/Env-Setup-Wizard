package dotenv

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ExistingFile is the parsed value state and original content of an output
// file. Content is retained for byte-identical no-op detection.
type ExistingFile struct {
	Exists  bool
	Values  map[string]string
	Content []byte
}

// LoadExisting loads an optional existing output with the full dotenv syntax
// supported by compose-go. Process environment values are never consulted.
func LoadExisting(path string) (ExistingFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ExistingFile{}, nil
	}
	if err != nil {
		return ExistingFile{}, fmt.Errorf("read existing output %q: %w", path, err)
	}

	decoded := bytes.TrimPrefix(data, utf8BOM)
	if !utf8.Valid(decoded) {
		return ExistingFile{}, fmt.Errorf("decode existing output %q: file is not valid UTF-8", path)
	}

	values, err := parseSemanticValues(string(decoded))
	if err != nil {
		return ExistingFile{}, fmt.Errorf("parse existing output %q: %w", path, err)
	}
	if err := rejectExistingDuplicateKeys(string(decoded)); err != nil {
		return ExistingFile{}, fmt.Errorf("parse existing output %q: %w", path, err)
	}

	return ExistingFile{
		Exists:  true,
		Values:  values,
		Content: append([]byte(nil), data...),
	}, nil
}

type existingKey struct {
	name string
	line int
}

func rejectExistingDuplicateKeys(text string) error {
	seen := make(map[string]int)
	for _, key := range existingKeys(text) {
		if firstLine, duplicate := seen[key.name]; duplicate {
			return fmt.Errorf(
				"line %d: duplicate variable %q; first declared at line %d",
				key.line,
				key.name,
				firstLine,
			)
		}
		seen[key.name] = key.line
	}
	return nil
}

// existingKeys walks only statement boundaries. compose-go performs the
// syntax validation; this pass exists solely because its map result otherwise
// loses duplicate declarations.
func existingKeys(text string) []existingKey {
	keys := make([]existingKey, 0)
	for index := 0; index < len(text); {
		index = skipExistingTrivia(text, index)
		if index >= len(text) {
			break
		}

		statementStart := index
		index = skipExportPrefix(text, index)
		keyStart := index
		delimiterIndex, delimiter := existingKeyDelimiter(text, index)

		name := strings.TrimRightFunc(text[keyStart:delimiterIndex], unicode.IsSpace)
		if name != "" {
			keys = append(keys, existingKey{
				name: name,
				line: 1 + strings.Count(text[:statementStart], "\n"),
			})
		}

		if delimiter == 0 {
			break
		}
		index = delimiterIndex + 1
		if delimiter == '\n' {
			continue
		}
		index = skipExistingHorizontalSpace(text, index)
		if index >= len(text) {
			break
		}
		if text[index] == '\'' || text[index] == '"' {
			index = skipExistingQuotedValue(text, index, text[index])
			continue
		}
		if newline := strings.IndexByte(text[index:], '\n'); newline >= 0 {
			index += newline + 1
		} else {
			break
		}
	}
	return keys
}

func skipExistingTrivia(text string, index int) int {
	for index < len(text) {
		r, size := utf8.DecodeRuneInString(text[index:])
		if unicode.IsSpace(r) {
			index += size
			continue
		}
		if r != '#' {
			return index
		}
		if newline := strings.IndexByte(text[index:], '\n'); newline >= 0 {
			index += newline + 1
			continue
		}
		return len(text)
	}
	return index
}

func skipExportPrefix(text string, index int) int {
	const prefix = "export"
	if !strings.HasPrefix(text[index:], prefix) {
		return index
	}
	after := index + len(prefix)
	if after >= len(text) {
		return index
	}
	r, _ := utf8.DecodeRuneInString(text[after:])
	if !unicode.IsSpace(r) || r == '\n' {
		return index
	}
	return skipExistingHorizontalSpace(text, after)
}

func skipExistingHorizontalSpace(text string, index int) int {
	for index < len(text) {
		r, size := utf8.DecodeRuneInString(text[index:])
		if r == '\n' || !unicode.IsSpace(r) {
			break
		}
		index += size
	}
	return index
}

func existingKeyDelimiter(text string, index int) (int, byte) {
	for offset, r := range text[index:] {
		switch r {
		case '=', ':', '\n':
			position := index + offset
			return position, text[position]
		}
	}
	return len(text), 0
}

func skipExistingQuotedValue(text string, index int, quote byte) int {
	escaped := false
	for index++; index < len(text); index++ {
		character := text[index]
		if character == quote && !escaped {
			return index + 1
		}
		if character == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false
	}
	return len(text)
}
