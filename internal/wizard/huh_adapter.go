package wizard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"charm.land/huh/v2"
	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/validation"
)

var (
	// ErrCanceled reports that the user left the form before submitting it.
	ErrCanceled = errors.New("wizard canceled")
	// ErrTerminalRequired reports that the interactive terminal is unavailable.
	ErrTerminalRequired = errors.New("interactive terminal is required")
)

// Terminal contains the injected terminal dependencies for one wizard run.
type Terminal struct {
	Input       io.Reader
	Output      io.Writer
	Interactive bool
}

type answerBinding struct {
	question             *domain.Question
	initialValue         string
	textValue            string
	boolValue            bool
	existingIssuePending string
}

// Run presents the Question groups and returns an updated copy. The input
// groups are never mutated, including on cancellation or validation failure.
func Run(
	ctx context.Context,
	groups []domain.QuestionGroup,
	terminal Terminal,
) ([]domain.QuestionGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !terminal.Interactive {
		return nil, fmt.Errorf("%w on stdin and stderr", ErrTerminalRequired)
	}

	result := cloneQuestionGroups(groups)
	if questionCount(result) == 0 {
		return result, nil
	}
	if terminal.Input == nil || terminal.Output == nil {
		return nil, errors.New("interactive terminal input and output are required")
	}

	form, bindings, err := buildForm(result)
	if err != nil {
		return nil, err
	}
	err = applyEnvWizardTheme(form).
		WithInput(terminal.Input).
		WithOutput(terminal.Output).
		RunWithContext(ctx)
	if err != nil {
		return nil, translateFormError(ctx, err)
	}

	for _, binding := range bindings {
		if err := binding.commit(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func buildForm(groups []domain.QuestionGroup) (*huh.Form, []*answerBinding, error) {
	huhGroups := make([]*huh.Group, 0, len(groups))
	bindings := make([]*answerBinding, 0, questionCount(groups))

	for groupIndex := range groups {
		group := &groups[groupIndex]
		fields := make([]huh.Field, 0, len(group.Questions))
		for questionIndex := range group.Questions {
			question := &group.Questions[questionIndex]
			binding, field, err := fieldForQuestion(question)
			if err != nil {
				return nil, nil, err
			}
			bindings = append(bindings, binding)
			fields = append(fields, field)
		}
		if len(fields) == 0 {
			continue
		}
		huhGroups = append(huhGroups, huh.NewGroup(fields...).Title(group.Section))
	}

	return huh.NewForm(huhGroups...), bindings, nil
}

func fieldForQuestion(question *domain.Question) (*answerBinding, huh.Field, error) {
	if !question.HasValue {
		return nil, nil, fmt.Errorf("question %q has no resolved value", question.Key)
	}
	initialQuestion := *question
	// A configurable required field may intentionally start empty: the form is
	// the recovery path that collects its required final value.
	initialQuestion.Required = false
	if err := validation.ValidateQuestion(initialQuestion, question.Value); err != nil {
		return nil, nil, fmt.Errorf("question %q has an invalid initial value: %w", question.Key, err)
	}
	if question.Kind == domain.QuestionKindSelect && len(question.Options) == 0 {
		return nil, nil, fmt.Errorf("question %q is a selection without options", question.Key)
	}

	binding := &answerBinding{
		question:             question,
		initialValue:         question.Value,
		textValue:            question.Value,
		existingIssuePending: "",
	}
	if question.ExistingValueIssue != nil {
		binding.existingIssuePending = question.ExistingValueIssue.Message
	}
	validateText := func(value string) error {
		return binding.validateText(value)
	}

	switch question.Kind {
	case domain.QuestionKindInput:
		field := huh.NewInput().
			Key(question.Key).
			Title(question.Prompt).
			Description(question.Description).
			Value(&binding.textValue).
			Validate(validateText)
		if question.Placeholder != "" {
			field.Placeholder(question.Placeholder)
		}
		if question.Secret {
			field.EchoMode(huh.EchoModePassword)
		}
		return binding, field, nil

	case domain.QuestionKindSelect:
		field := huh.NewSelect[string]().
			Key(question.Key).
			Title(question.Prompt).
			Description(question.Description).
			Options(huh.NewOptions(question.Options...)...).
			Value(&binding.textValue).
			Validate(validateText)
		return binding, field, nil

	case domain.QuestionKindConfirm:
		boolValue, err := strconv.ParseBool(question.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("question %q has an invalid boolean value", question.Key)
		}
		binding.boolValue = boolValue
		validateBool := func(value bool) error {
			return binding.validateText(strconv.FormatBool(value))
		}
		field := huh.NewConfirm().
			Key(question.Key).
			Title(question.Prompt).
			Description(question.Description).
			Value(&binding.boolValue).
			Validate(validateBool)
		if question.ExistingValueIssue != nil {
			return binding, &validatedAccessibleConfirm{
				Confirm:  field,
				validate: validateBool,
			}, nil
		}
		return binding, field, nil

	default:
		return nil, nil, fmt.Errorf("question %q has unsupported kind %q", question.Key, question.Kind)
	}
}

// validatedAccessibleConfirm compensates for Huh's accessible Confirm path,
// which does not invoke the field validator itself.
type validatedAccessibleConfirm struct {
	*huh.Confirm
	validate func(bool) error
}

func (field *validatedAccessibleConfirm) RunAccessible(output io.Writer, input io.Reader) error {
	for {
		if err := field.Confirm.RunAccessible(output, input); err != nil {
			return err
		}
		value := field.GetValue().(bool)
		if err := field.validate(value); err != nil {
			if _, writeErr := fmt.Fprintln(output, err); writeErr != nil {
				return writeErr
			}
			continue
		}
		return nil
	}
}

func (binding *answerBinding) validateText(value string) error {
	if binding.existingIssuePending != "" {
		message := binding.existingIssuePending
		binding.existingIssuePending = ""
		return errors.New(message)
	}
	return validation.ValidateQuestion(*binding.question, value)
}

func (binding *answerBinding) commit() error {
	value := binding.textValue
	if binding.question.Kind == domain.QuestionKindConfirm {
		value = strconv.FormatBool(binding.boolValue)
	}
	if err := validation.ValidateQuestion(*binding.question, value); err != nil {
		return fmt.Errorf("question %q returned an invalid value: %w", binding.question.Key, err)
	}

	binding.question.Value = value
	binding.question.HasValue = true
	if value != binding.initialValue || binding.question.ExistingValueIssue != nil {
		binding.question.ValueSource = domain.ValueFromUser
	}
	binding.question.ExistingValueIssue = nil
	return nil
}

func translateFormError(ctx context.Context, err error) error {
	if contextError := ctx.Err(); contextError != nil {
		return contextError
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCanceled
	}
	return fmt.Errorf("run interactive form: %w", err)
}

func cloneQuestionGroups(groups []domain.QuestionGroup) []domain.QuestionGroup {
	cloned := make([]domain.QuestionGroup, len(groups))
	for groupIndex, group := range groups {
		cloned[groupIndex].Section = group.Section
		cloned[groupIndex].Questions = make([]domain.Question, len(group.Questions))
		for questionIndex, question := range group.Questions {
			question.Options = append([]string(nil), question.Options...)
			if question.ExistingValueIssue != nil {
				issue := *question.ExistingValueIssue
				question.ExistingValueIssue = &issue
			}
			cloned[groupIndex].Questions[questionIndex] = question
		}
	}
	return cloned
}

func questionCount(groups []domain.QuestionGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Questions)
	}
	return count
}
