package app

import (
	"context"
	"errors"
)

// ErrCanceled reports a user cancellation rather than an operational failure.
var ErrCanceled = errors.New("operation canceled")

var errNotImplemented = errors.New("wizard implementation not available yet")

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
func Run(context.Context, Options) error {
	return errNotImplemented
}
