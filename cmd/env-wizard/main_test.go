package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tosdan/env-setup-wizard/internal/app"
)

func TestVersionTextDevelopmentBuild(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() {
		version, commit = originalVersion, originalCommit
	})

	version, commit = "dev", ""

	if got, want := versionText(), "env-wizard dev"; got != want {
		t.Fatalf("versionText() = %q, want %q", got, want)
	}
}

func TestVersionTextReleaseBuild(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() {
		version, commit = originalVersion, originalCommit
	})

	version, commit = "v1.0.0", "abc1234"

	if got, want := versionText(), "env-wizard v1.0.0 (commit abc1234)"; got != want {
		t.Fatalf("versionText() = %q, want %q", got, want)
	}
}

func TestRunUsesPathsRelativeToCurrentDirectory(t *testing.T) {
	cwd := t.TempDir()
	tests := []struct {
		name string
		args []string
		want app.Options
	}{
		{
			name: "defaults",
			want: app.Options{
				TemplatePath: filepath.Join(cwd, ".env.example"),
				OutputPath:   filepath.Join(cwd, ".env"),
			},
		},
		{
			name: "independent overrides",
			args: []string{
				"--template", filepath.Join("config", "app.env.example"),
				"--output", filepath.Join("generated", ".env"),
				"--force",
			},
			want: app.Options{
				TemplatePath: filepath.Join(cwd, "config", "app.env.example"),
				OutputPath:   filepath.Join(cwd, "generated", ".env"),
				Force:        true,
			},
		},
		{
			name: "named template suggests matching output",
			args: []string{"--template", "typed-values.env.example"},
			want: app.Options{
				TemplatePath:        filepath.Join(cwd, "typed-values.env.example"),
				OutputPath:          filepath.Join(cwd, ".env"),
				SuggestedOutputPath: filepath.Join(cwd, "typed-values.env"),
			},
		},
		{
			name: "template in another directory suggests output beside it",
			args: []string{"--template", filepath.Join("config", ".env.example")},
			want: app.Options{
				TemplatePath:        filepath.Join(cwd, "config", ".env.example"),
				OutputPath:          filepath.Join(cwd, ".env"),
				SuggestedOutputPath: filepath.Join(cwd, "config", ".env"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got app.Options
			executed := false
			execute := func(_ context.Context, options app.Options) error {
				executed = true
				got = options
				return nil
			}

			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			code := run(context.Background(), tt.args, fixedWorkingDirectory(cwd), stdout, stderr, execute)

			if code != exitSuccess {
				t.Fatalf("run() exit code = %d, want %d; stderr = %q", code, exitSuccess, stderr.String())
			}
			if !executed {
				t.Fatal("run() did not execute the application")
			}
			if got != tt.want {
				t.Fatalf("application options = %#v, want %#v", got, tt.want)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("run() wrote stdout %q and stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunPreservesAbsolutePaths(t *testing.T) {
	cwd := t.TempDir()
	templatePath := filepath.Join(t.TempDir(), "config", "..", ".env.example")
	outputPath := filepath.Join(t.TempDir(), "output", "..", ".env")
	var got app.Options

	code := run(
		context.Background(),
		[]string{"--template", templatePath, "--output", outputPath},
		fixedWorkingDirectory(cwd),
		&bytes.Buffer{},
		&bytes.Buffer{},
		func(_ context.Context, options app.Options) error {
			got = options
			return nil
		},
	)

	if code != exitSuccess {
		t.Fatalf("run() exit code = %d, want %d", code, exitSuccess)
	}
	if got.TemplatePath != filepath.Clean(templatePath) {
		t.Errorf("TemplatePath = %q, want %q", got.TemplatePath, filepath.Clean(templatePath))
	}
	if got.OutputPath != filepath.Clean(outputPath) {
		t.Errorf("OutputPath = %q, want %q", got.OutputPath, filepath.Clean(outputPath))
	}
}

func TestRunVersionDoesNotNeedCurrentDirectoryOrApplication(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() {
		version, commit = originalVersion, originalCommit
	})
	version, commit = "v1.2.3", "def5678"

	cwdCalled, executeCalled := false, false
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	code := run(
		context.Background(),
		[]string{"--version"},
		func() (string, error) {
			cwdCalled = true
			return "", errors.New("unavailable")
		},
		stdout,
		stderr,
		func(context.Context, app.Options) error {
			executeCalled = true
			return nil
		},
	)

	if code != exitSuccess {
		t.Fatalf("run() exit code = %d, want %d", code, exitSuccess)
	}
	if cwdCalled || executeCalled {
		t.Fatalf("cwdCalled = %t, executeCalled = %t; want both false", cwdCalled, executeCalled)
	}
	if got, want := stdout.String(), "env-wizard v1.2.3 (commit def5678)\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelpReturnsSuccessWithoutExecuting(t *testing.T) {
	executeCalled := false
	stderr := &bytes.Buffer{}
	code := run(
		context.Background(),
		[]string{"--help"},
		fixedWorkingDirectory(t.TempDir()),
		&bytes.Buffer{},
		stderr,
		func(context.Context, app.Options) error {
			executeCalled = true
			return nil
		},
	)

	if code != exitSuccess {
		t.Fatalf("run() exit code = %d, want %d", code, exitSuccess)
	}
	if executeCalled {
		t.Fatal("application executed for --help")
	}
	if !strings.Contains(stderr.String(), "Usage: env-wizard") {
		t.Fatalf("help output = %q, want usage", stderr.String())
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "unknown flag", args: []string{"--unknown"}, wantMessage: "flag provided but not defined"},
		{name: "positional argument", args: []string{"file.env"}, wantMessage: "unexpected argument"},
		{name: "empty template path", args: []string{"--template", ""}, wantMessage: "--template path must not be empty"},
		{name: "empty output path", args: []string{"--output", ""}, wantMessage: "--output path must not be empty"},
		{name: "version with other flag", args: []string{"--version", "--force"}, wantMessage: "--version cannot be combined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executeCalled := false
			stderr := &bytes.Buffer{}
			code := run(
				context.Background(),
				tt.args,
				fixedWorkingDirectory(t.TempDir()),
				&bytes.Buffer{},
				stderr,
				func(context.Context, app.Options) error {
					executeCalled = true
					return nil
				},
			)

			if code != exitUsage {
				t.Fatalf("run() exit code = %d, want %d", code, exitUsage)
			}
			if executeCalled {
				t.Fatal("application executed for invalid arguments")
			}
			if !strings.Contains(stderr.String(), tt.wantMessage) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tt.wantMessage)
			}
		})
	}
}

func TestRunMapsApplicationErrorsToExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStderr string
	}{
		{name: "operational failure", err: errors.New("load template: denied"), wantCode: exitFailure, wantStderr: "env-wizard: load template: denied\n"},
		{name: "user cancellation", err: app.ErrCanceled, wantCode: exitCanceled},
		{name: "context cancellation", err: context.Canceled, wantCode: exitCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			code := run(
				context.Background(),
				nil,
				fixedWorkingDirectory(t.TempDir()),
				&bytes.Buffer{},
				stderr,
				func(context.Context, app.Options) error { return tt.err },
			)

			if code != tt.wantCode {
				t.Fatalf("run() exit code = %d, want %d", code, tt.wantCode)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunExplainsMissingDefaultTemplate(t *testing.T) {
	cwd := t.TempDir()
	stderr := &bytes.Buffer{}
	code := run(
		context.Background(),
		nil,
		fixedWorkingDirectory(cwd),
		&bytes.Buffer{},
		stderr,
		func(context.Context, app.Options) error {
			return fmt.Errorf("preflight paths: %w", app.ErrTemplateNotFound)
		},
	)

	if code != exitFailure {
		t.Fatalf("run() exit code = %d, want %d", code, exitFailure)
	}
	want := fmt.Sprintf(
		"env-wizard: no .env.example template found in the current directory:\n  %s\n\n"+
			"Create a .env.example file there, or specify another template:\n"+
			"  env-wizard --template path/to/file.env.example\n",
		cwd,
	)
	if got := stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunPreservesDetailsForExplicitMissingTemplate(t *testing.T) {
	cwd := t.TempDir()
	stderr := &bytes.Buffer{}
	detail := "preflight paths: inspect template custom.env.example: file not found"
	code := run(
		context.Background(),
		[]string{"--template", "custom.env.example"},
		fixedWorkingDirectory(cwd),
		&bytes.Buffer{},
		stderr,
		func(context.Context, app.Options) error {
			return fmt.Errorf("%s: %w", detail, app.ErrTemplateNotFound)
		},
	)

	if code != exitFailure {
		t.Fatalf("run() exit code = %d, want %d", code, exitFailure)
	}
	if got, want := stderr.String(), "env-wizard: "+detail+": template not found\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunReportsCurrentDirectoryFailure(t *testing.T) {
	stderr := &bytes.Buffer{}
	code := run(
		context.Background(),
		nil,
		func() (string, error) { return "", errors.New("directory was removed") },
		&bytes.Buffer{},
		stderr,
		func(context.Context, app.Options) error {
			t.Fatal("application executed after current-directory failure")
			return nil
		},
	)

	if code != exitFailure {
		t.Fatalf("run() exit code = %d, want %d", code, exitFailure)
	}
	if got, want := stderr.String(), "env-wizard: determine current directory: directory was removed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func fixedWorkingDirectory(path string) func() (string, error) {
	return func() (string, error) {
		return path, nil
	}
}
