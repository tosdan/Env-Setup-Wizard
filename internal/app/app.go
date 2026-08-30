package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	charmterm "github.com/charmbracelet/x/term"
	"github.com/tosdan/env-setup-wizard/internal/domain"
	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

// ErrCanceled reports a user cancellation rather than an operational failure.
var ErrCanceled = errors.New("operation canceled")

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
	existing, err := dotenv.LoadExisting(options.OutputPath)
	if err != nil {
		return fmt.Errorf("load existing output: %w", err)
	}
	document, err = dotenv.MergeExisting(document, existing.Values)
	if err != nil {
		return fmt.Errorf("merge existing output: %w", err)
	}
	groups, err := wizard.BuildQuestionGroups(document)
	if err != nil {
		return fmt.Errorf("build questions: %w", err)
	}
	terminal := terminalFor(options.Runtime)
	answeredGroups, err := wizard.Run(ctx, groups, terminal)
	if err != nil {
		return mapWizardError("run wizard", err)
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
	summary, err := wizard.RenderSummary(document)
	if err != nil {
		return fmt.Errorf("render summary: %w", err)
	}
	if terminal.Output == nil {
		return errors.New("write summary: interactive terminal output is required")
	}
	if _, err := io.WriteString(terminal.Output, summary); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	candidate, err := projectfs.RenderConfiguration(document)
	if err != nil {
		return fmt.Errorf("render configuration: %w", err)
	}
	if existing.Exists && bytes.Equal(candidate, existing.Content) {
		if _, err := fmt.Fprintln(terminal.Output, "No changes detected."); err != nil {
			return fmt.Errorf("report unchanged output: %w", err)
		}
		return nil
	}

	if !options.Force {
		confirmed, err := wizard.ConfirmWrite(
			ctx,
			filepath.Base(options.OutputPath),
			existing.Exists,
			terminal,
		)
		if err != nil {
			return mapWizardError("confirm output", err)
		}
		if !confirmed {
			if _, err := fmt.Fprintln(terminal.Output, "No changes made."); err != nil {
				return fmt.Errorf("report declined confirmation: %w", err)
			}
			return nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backupPath, err := projectfs.SafeWrite(options.TemplatePath, options.OutputPath, candidate)
	if err != nil {
		return fmt.Errorf("write output safely: %w", err)
	}
	action := "Created"
	if backupPath != "" {
		action = "Updated"
	}
	message := fmt.Sprintf("%s %s.\n", action, filepath.Base(options.OutputPath))
	if backupPath != "" {
		message += fmt.Sprintf("Backup created: %s\n", backupPath)
	}
	if _, err := io.WriteString(terminal.Output, message); err != nil {
		return fmt.Errorf("report successful write: %w", err)
	}
	return nil
}

func mapWizardError(action string, err error) error {
	if errors.Is(err, wizard.ErrCanceled) {
		return ErrCanceled
	}
	return fmt.Errorf("%s: %w", action, err)
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
