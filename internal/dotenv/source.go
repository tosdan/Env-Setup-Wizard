package dotenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/tosdan/env-setup-wizard/internal/domain"
)

var (
	utf8BOM    = []byte{0xef, 0xbb, 0xbf}
	utf16LEBOM = []byte{0xff, 0xfe}
	utf16BEBOM = []byte{0xfe, 0xff}
)

type source struct {
	Text            string
	LineEnding      domain.LineEnding
	HasFinalNewline bool
}

func loadTemplate(path string) (source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return source{}, fmt.Errorf("read template %q: %w", path, err)
	}

	decoded, err := decodeSource(data)
	if err != nil {
		return source{}, fmt.Errorf("decode template %q: %w", path, err)
	}

	return decoded, nil
}

func decodeSource(data []byte) (source, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	if bytes.HasPrefix(data, utf16LEBOM) || bytes.HasPrefix(data, utf16BEBOM) {
		return source{}, errors.New("UTF-16 templates are not supported; use UTF-8")
	}
	if !utf8.Valid(data) {
		return source{}, errors.New("template is not valid UTF-8")
	}

	lineEnding, err := detectLineEnding(data)
	if err != nil {
		return source{}, err
	}

	return source{
		Text:            string(data),
		LineEnding:      lineEnding,
		HasFinalNewline: len(data) > 0 && data[len(data)-1] == '\n',
	}, nil
}

func detectLineEnding(data []byte) (domain.LineEnding, error) {
	var detected domain.LineEnding
	line := 1

	for index := 0; index < len(data); {
		var current domain.LineEnding
		switch data[index] {
		case '\r':
			if index+1 >= len(data) || data[index+1] != '\n' {
				return "", fmt.Errorf("isolated carriage return at line %d", line)
			}
			current = domain.LineEndingCRLF
			index += 2
		case '\n':
			current = domain.LineEndingLF
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
		return domain.LineEndingLF, nil
	}

	return detected, nil
}
