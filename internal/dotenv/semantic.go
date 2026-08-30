package dotenv

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	composedotenv "github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/tosdan/env-setup-wizard/internal/domain"
)

func parseSemanticValues(text string) (map[string]string, error) {
	values, err := composedotenv.ParseWithLookup(
		strings.NewReader(text),
		func(string) (string, bool) { return "", false },
	)
	if err != nil {
		return nil, safeComposeError(err)
	}

	return values, nil
}

func attachSemanticValues(document domain.Document, values map[string]string) (domain.Document, error) {
	for index, node := range document.Nodes {
		variable, ok := node.(domain.Variable)
		if !ok {
			continue
		}

		value, found := values[variable.Key]
		if !found {
			return domain.Document{}, fmt.Errorf(
				"line %d: semantic parser did not return variable %q",
				variable.Line,
				variable.Key,
			)
		}
		if strings.ContainsAny(value, "\x00\r\n") {
			return domain.Document{}, fmt.Errorf(
				"line %d: variable %q resolves to a value containing unsupported NUL, CR, or LF",
				variable.Line,
				variable.Key,
			)
		}

		variable.Value = value
		variable.HasValue = true
		variable.ValueSource = domain.ValueFromTemplate
		document.Nodes[index] = variable
	}

	return document, nil
}

func safeComposeError(err error) error {
	message := err.Error()
	const linePrefix = "line "
	if strings.HasPrefix(message, linePrefix) {
		if colon := strings.IndexByte(message, ':'); colon > len(linePrefix) {
			line := message[len(linePrefix):colon]
			if _, conversionError := strconv.Atoi(line); conversionError == nil {
				return fmt.Errorf("invalid Compose dotenv syntax at line %s", line)
			}
		}
	}

	return errors.New("invalid Compose dotenv syntax")
}
