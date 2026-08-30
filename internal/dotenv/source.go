package dotenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"
)

var (
	utf8BOM    = []byte{0xef, 0xbb, 0xbf}
	utf16LEBOM = []byte{0xff, 0xfe}
	utf16BEBOM = []byte{0xfe, 0xff}
)

// LineEnding is the consistent newline sequence used by a template.
type LineEnding string

const (
	// LineEndingLF represents Unix-style line endings.
	LineEndingLF LineEnding = "\n"
	// LineEndingCRLF represents Windows-style line endings.
	LineEndingCRLF LineEnding = "\r\n"
)

// Source is a decoded template whose optional initial UTF-8 BOM has been
// removed. Text otherwise retains the original bytes, including line endings.
type Source struct {
	Text            string
	LineEnding      LineEnding
	HasFinalNewline bool
}

// LoadTemplate reads and decodes a UTF-8 dotenv template. It accepts either LF
// or CRLF, rejects mixed or isolated carriage returns, and uses LF as the
// deterministic line-ending style when the source contains no newline.
func LoadTemplate(path string) (Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, fmt.Errorf("read template %q: %w", path, err)
	}

	source, err := decodeSource(data)
	if err != nil {
		return Source{}, fmt.Errorf("decode template %q: %w", path, err)
	}

	return source, nil
}

func decodeSource(data []byte) (Source, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	if bytes.HasPrefix(data, utf16LEBOM) || bytes.HasPrefix(data, utf16BEBOM) {
		return Source{}, errors.New("UTF-16 templates are not supported; use UTF-8")
	}
	if !utf8.Valid(data) {
		return Source{}, errors.New("template is not valid UTF-8")
	}

	lineEnding, err := detectLineEnding(data)
	if err != nil {
		return Source{}, err
	}

	return Source{
		Text:            string(data),
		LineEnding:      lineEnding,
		HasFinalNewline: len(data) > 0 && data[len(data)-1] == '\n',
	}, nil
}

func detectLineEnding(data []byte) (LineEnding, error) {
	var detected LineEnding
	line := 1

	for index := 0; index < len(data); {
		var current LineEnding
		switch data[index] {
		case '\r':
			if index+1 >= len(data) || data[index+1] != '\n' {
				return "", fmt.Errorf("isolated carriage return at line %d", line)
			}
			current = LineEndingCRLF
			index += 2
		case '\n':
			current = LineEndingLF
			index++
		default:
			index++
			continue
		}

		if detected == "" {
			detected = current
		} else if detected != current {
			return "", fmt.Errorf("mixed line endings at line %d", line)
		}
		line++
	}

	if detected == "" {
		return LineEndingLF, nil
	}

	return detected, nil
}
