package wizard

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"charm.land/huh/v2"
	"github.com/tosdan/env-setup-wizard/internal/domain"
)

func TestRunCollectsAndValidatesAnswers(t *testing.T) {
	t.Setenv("TERM", "dumb")
	issue := &domain.ExistingValueIssue{Message: "Confirm the replacement value."}
	groups := []domain.QuestionGroup{{
		Section: "Configuration",
		Questions: []domain.Question{
			{
				Key:                "NAME",
				Prompt:             "Name",
				Description:        "Public name",
				Value:              "old",
				HasValue:           true,
				ValueSource:        domain.ValueFromTemplate,
				Type:               domain.VariableTypeString,
				Kind:               domain.QuestionKindInput,
				Required:           true,
				ExistingValueIssue: issue,
			},
			{
				Key:         "PORT",
				Prompt:      "Port",
				Value:       "3000",
				HasValue:    true,
				ValueSource: domain.ValueFromTemplate,
				Type:        domain.VariableTypePort,
				Kind:        domain.QuestionKindInput,
			},
			{
				Key:         "ENVIRONMENT",
				Prompt:      "Environment",
				Value:       "development",
				HasValue:    true,
				ValueSource: domain.ValueFromTemplate,
				Type:        domain.VariableTypeString,
				Kind:        domain.QuestionKindSelect,
				Options:     []string{"development", "production"},
			},
			{
				Key:         "ENABLED",
				Prompt:      "Enabled",
				Value:       "true",
				HasValue:    true,
				ValueSource: domain.ValueFromTemplate,
				Type:        domain.VariableTypeBool,
				Kind:        domain.QuestionKindConfirm,
			},
		},
	}}
	original := cloneQuestionGroups(groups)
	var output bytes.Buffer

	answered, err := Run(
		context.Background(),
		groups,
		Terminal{
			Input: iotest.OneByteReader(
				strings.NewReader("new name\ninvalid\n08080\n2\nn\n"),
			),
			Output:      &output,
			Interactive: true,
		},
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !strings.Contains(output.String(), "decimal port number") {
		t.Fatalf("wizard output = %q, want validation diagnostic", output.String())
	}
	if !reflect.DeepEqual(groups, original) {
		t.Fatalf("Run() mutated input groups:\ngot  %#v\nwant %#v", groups, original)
	}

	wantValues := []string{"new name", "08080", "production", "false"}
	for index, question := range answered[0].Questions {
		if question.Value != wantValues[index] {
			t.Errorf("Question[%d] Value = %q, want %q", index, question.Value, wantValues[index])
		}
		if question.ValueSource != domain.ValueFromUser || !question.HasValue {
			t.Errorf("Question[%d] source/presence = %q/%t, want user/true", index, question.ValueSource, question.HasValue)
		}
		if question.ExistingValueIssue != nil {
			t.Errorf("Question[%d] retained ExistingValueIssue after confirmation", index)
		}
	}
}

func TestRunPreservesUnchangedDefaultAndPlaceholderIsNotAValue(t *testing.T) {
	t.Setenv("TERM", "dumb")
	groups := []domain.QuestionGroup{{
		Section: "Configuration",
		Questions: []domain.Question{{
			Key:         "OPTIONAL",
			Prompt:      "Optional",
			Value:       "",
			HasValue:    true,
			ValueSource: domain.ValueFromTemplate,
			Type:        domain.VariableTypeString,
			Kind:        domain.QuestionKindInput,
			Placeholder: "visual hint only",
		}},
	}}

	answered, err := Run(
		context.Background(),
		groups,
		Terminal{
			Input:       strings.NewReader("\n"),
			Output:      io.Discard,
			Interactive: true,
		},
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	question := answered[0].Questions[0]
	if question.Value != "" || question.ValueSource != domain.ValueFromTemplate {
		t.Fatalf("Question = %#v, want unchanged empty template value", question)
	}
}

func TestRunCollectsAnInitiallyEmptyRequiredValue(t *testing.T) {
	t.Setenv("TERM", "dumb")
	groups := []domain.QuestionGroup{{
		Section: "Configuration",
		Questions: []domain.Question{{
			Key:         "REQUIRED",
			Prompt:      "Required",
			Value:       "",
			HasValue:    true,
			ValueSource: domain.ValueFromTemplate,
			Type:        domain.VariableTypeString,
			Kind:        domain.QuestionKindInput,
			Required:    true,
		}},
	}}
	var output bytes.Buffer

	answered, err := Run(
		context.Background(),
		groups,
		Terminal{
			Input: iotest.OneByteReader(
				strings.NewReader("\nprovided\n"),
			),
			Output:      &output,
			Interactive: true,
		},
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !strings.Contains(output.String(), "value is required") {
		t.Fatalf("wizard output = %q, want required diagnostic", output.String())
	}
	question := answered[0].Questions[0]
	if question.Value != "provided" || question.ValueSource != domain.ValueFromUser {
		t.Fatalf("Question = %#v, want collected user value", question)
	}
}

func TestRunRequiresTerminalEvenWhenThereAreNoQuestions(t *testing.T) {
	_, err := Run(context.Background(), nil, Terminal{})
	if !errors.Is(err, ErrTerminalRequired) {
		t.Fatalf("Run() error = %v, want ErrTerminalRequired", err)
	}

	answered, err := Run(context.Background(), nil, Terminal{Interactive: true})
	if err != nil {
		t.Fatalf("Run(all fixed) error = %v, want nil", err)
	}
	if len(answered) != 0 {
		t.Fatalf("len(answered) = %d, want zero", len(answered))
	}
}

func TestRunHonorsCanceledContextBeforeTerminalCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, nil, Terminal{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestFieldForQuestionMapsEveryQuestionKind(t *testing.T) {
	tests := []struct {
		name     string
		question domain.Question
		wantType any
	}{
		{
			name: "input",
			question: domain.Question{
				Key: "NAME", Prompt: "Name", Value: "value", HasValue: true,
				Type: domain.VariableTypeString, Kind: domain.QuestionKindInput,
			},
			wantType: (*huh.Input)(nil),
		},
		{
			name: "select",
			question: domain.Question{
				Key: "ENV", Prompt: "Environment", Value: "dev", HasValue: true,
				Type: domain.VariableTypeString, Kind: domain.QuestionKindSelect,
				Options: []string{"dev", "prod"},
			},
			wantType: (*huh.Select[string])(nil),
		},
		{
			name: "confirm",
			question: domain.Question{
				Key: "ENABLED", Prompt: "Enabled", Value: "true", HasValue: true,
				Type: domain.VariableTypeBool, Kind: domain.QuestionKindConfirm,
			},
			wantType: (*huh.Confirm)(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, field, err := fieldForQuestion(&tt.question)
			if err != nil {
				t.Fatalf("fieldForQuestion() error = %v, want nil", err)
			}
			if reflect.TypeOf(field) != reflect.TypeOf(tt.wantType) {
				t.Fatalf("field type = %T, want %T", field, tt.wantType)
			}
		})
	}
}

func TestSecretInputIsMaskedByHuh(t *testing.T) {
	question := domain.Question{
		Key:         "SECRET",
		Prompt:      "Secret",
		Value:       "do-not-show",
		HasValue:    true,
		ValueSource: domain.ValueFromTemplate,
		Type:        domain.VariableTypeString,
		Kind:        domain.QuestionKindInput,
		Secret:      true,
	}
	_, field, err := fieldForQuestion(&question)
	if err != nil {
		t.Fatalf("fieldForQuestion() error = %v, want nil", err)
	}
	if err := field.RunAccessible(io.Discard, strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "needs a tty") {
		t.Fatalf("secret RunAccessible() error = %v, want password TTY requirement", err)
	}
	field.Init()
	field.Focus()
	if view := field.View(); strings.Contains(view, "do-not-show") {
		t.Fatalf("secret field view leaked value: %q", view)
	}
}

func TestFieldForQuestionRejectsInvalidQuestionStateWithoutLeakingValue(t *testing.T) {
	tests := []domain.Question{
		{Key: "MISSING", Kind: domain.QuestionKindInput},
		{
			Key: "BAD", Value: "do-not-show", HasValue: true, Secret: true,
			Type: domain.VariableTypeInt, Kind: domain.QuestionKindInput,
		},
		{
			Key: "SELECT", Value: "", HasValue: true,
			Type: domain.VariableTypeString, Kind: domain.QuestionKindSelect,
		},
		{
			Key: "UNKNOWN", Value: "value", HasValue: true,
			Type: domain.VariableTypeString, Kind: domain.QuestionKind("unknown"),
		},
	}

	for _, question := range tests {
		_, _, err := fieldForQuestion(&question)
		if err == nil {
			t.Fatalf("fieldForQuestion(%q) error = nil, want rejection", question.Key)
		}
		if strings.Contains(err.Error(), "do-not-show") {
			t.Fatalf("fieldForQuestion() error leaked secret value: %q", err)
		}
	}
}

func TestQuestionDescriptionIncludesSafeExistingValueIssue(t *testing.T) {
	question := domain.Question{
		Description:        "Template help.",
		ExistingValueIssue: &domain.ExistingValueIssue{Message: "Current value is incompatible."},
	}
	if got, want := questionDescription(question), "Current value is incompatible.\nTemplate help."; got != want {
		t.Fatalf("questionDescription() = %q, want %q", got, want)
	}
}

func TestTranslateFormErrorMapsCancellation(t *testing.T) {
	if err := translateFormError(context.Background(), huh.ErrUserAborted); !errors.Is(err, ErrCanceled) {
		t.Fatalf("translateFormError() = %v, want ErrCanceled", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := translateFormError(ctx, huh.ErrUserAborted); !errors.Is(err, context.Canceled) {
		t.Fatalf("translateFormError(canceled context) = %v, want context.Canceled", err)
	}
}
