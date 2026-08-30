package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	charmterm "github.com/charmbracelet/x/term"
	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

// ErrCanceled reports a user cancellation rather than an operational failure.
var ErrCanceled = errors.New("operation canceled")

var errNotImplemented = errors.New("summary and confirmation not available yet")

// Runtime contains process resources that are injected for testability.
type Runtime struct {
	Input       io.Reader
	Output      io.Writer
	Interactive bool
}

// Options contains the fully resolved command inputs for one workflow run.
type Options struct {
	TemplatePath string
	OutputPath   string
	Force        bool
	Runtime      *Runtime
}

// Run executes the env-wizard workflow.
//
// The workflow is assembled incrementally behind this stable interface so the
// command remains limited to argument parsing, path resolution, and exit-code
// mapping.
func Run(ctx context.Context, options Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := projectfs.Preflight(options.TemplatePath, options.OutputPath); err != nil {
		return fmt.Errorf("preflight paths: %w", err)
	}
	document, err := dotenv.ParseTemplate(options.TemplatePath)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	groups, err := wizard.BuildQuestionGroups(document)
	if err != nil {
		return fmt.Errorf("build questions: %w", err)
	}
	answeredGroups, err := wizard.Run(ctx, groups, terminalFor(options.Runtime))
	if err != nil {
		if errors.Is(err, wizard.ErrCanceled) {
			return ErrCanceled
		}
		return fmt.Errorf("run wizard: %w", err)
	}
	for _, group := range answeredGroups {
		for _, question := range group.Questions {
			if question.ValueSource != domain.ValueFromUser {
				continue
			}
			if err := dotenv.UpdateValue(&document, question.Key, question.Value, question.ValueSource); err != nil {
				return fmt.Errorf("apply wizard answer for %q: %w", question.Key, err)
			}
		}
	}
	if _, err := projectfs.RenderConfiguration(document); err != nil {
		return fmt.Errorf("render configuration: %w", err)
	}

	return errNotImplemented
}

func terminalFor(runtime *Runtime) wizard.Terminal {
	if runtime != nil {
		return wizard.Terminal{
			Input:       runtime.Input,
			Output:      runtime.Output,
			Interactive: runtime.Interactive,
		}
	}

	return wizard.Terminal{
		Input:       os.Stdin,
		Output:      os.Stderr,
		Interactive: charmterm.IsTerminal(os.Stdin.Fd()) && charmterm.IsTerminal(os.Stderr.Fd()),
	}
}
