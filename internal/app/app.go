package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
	"github.com/tosdan/env-setup-wizard/internal/wizard"
)

// ErrCanceled reports a user cancellation rather than an operational failure.
var ErrCanceled = errors.New("operation canceled")

var errNotImplemented = errors.New("interactive wizard not available yet")

// Options contains the fully resolved command inputs for one workflow run.
type Options struct {
	TemplatePath string
	OutputPath   string
	Force        bool
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
	if _, err := wizard.BuildQuestionGroups(document); err != nil {
		return fmt.Errorf("build questions: %w", err)
	}
	if _, err := projectfs.RenderConfiguration(document); err != nil {
		return fmt.Errorf("render configuration: %w", err)
	}

	return errNotImplemented
}
