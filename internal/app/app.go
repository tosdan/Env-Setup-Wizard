package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/tosdan/env-setup-wizard/internal/dotenv"
	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
)

// ErrCanceled reports a user cancellation rather than an operational failure.
var ErrCanceled = errors.New("operation canceled")

var errNotImplemented = errors.New("semantic dotenv parsing not available yet")

// Options contains the fully resolved command inputs for one workflow run.
type Options struct {
	TemplatePath string
	OutputPath   string
	Force        bool
}

// Run executes the env-wizard workflow.
//
// The implementation will be added incrementally during Phase 1. Keeping this
// interface stable lets the command remain limited to argument parsing, path
// resolution, and exit-code mapping.
func Run(ctx context.Context, options Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := projectfs.Preflight(options.TemplatePath, options.OutputPath); err != nil {
		return fmt.Errorf("preflight paths: %w", err)
	}
	if _, err := dotenv.ParseTemplate(options.TemplatePath); err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	return errNotImplemented
}
