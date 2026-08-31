package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tosdan/env-setup-wizard/internal/app"
)

var (
	version = "dev"
	commit  = ""
)

const (
	exitSuccess  = 0
	exitFailure  = 1
	exitUsage    = 2
	exitCanceled = 130
)

type applicationRunner func(context.Context, app.Options) error

type parsedArguments struct {
	templatePath     string
	outputPath       string
	templateExplicit bool
	outputExplicit   bool
	force            bool
	showVersion      bool
}

func main() {
	os.Exit(run(
		context.Background(),
		os.Args[1:],
		os.Getwd,
		os.Stdout,
		os.Stderr,
		app.Run,
	))
}

func run(
	ctx context.Context,
	args []string,
	getwd func() (string, error),
	stdout io.Writer,
	stderr io.Writer,
	execute applicationRunner,
) int {
	parsed, parseResult := parseArguments(args, stderr)
	if parseResult != nil {
		return *parseResult
	}

	if parsed.showVersion {
		fmt.Fprintln(stdout, versionText())
		return exitSuccess
	}

	cwd, err := getwd()
	if err != nil {
		fmt.Fprintf(stderr, "env-wizard: determine current directory: %v\n", err)
		return exitFailure
	}

	options, err := resolveOptions(cwd, parsed)
	if err != nil {
		fmt.Fprintf(stderr, "env-wizard: %v\n", err)
		return exitUsage
	}

	if err := execute(ctx, options); err != nil {
		if errors.Is(err, app.ErrCanceled) || errors.Is(err, context.Canceled) {
			return exitCanceled
		}
		if !parsed.templateExplicit && errors.Is(err, app.ErrTemplateNotFound) {
			writeMissingDefaultTemplate(stderr, cwd)
			return exitFailure
		}

		fmt.Fprintf(stderr, "env-wizard: %v\n", err)
		return exitFailure
	}

	return exitSuccess
}

func parseArguments(args []string, stderr io.Writer) (parsedArguments, *int) {
	var parsed parsedArguments
	flags := flag.NewFlagSet("env-wizard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&parsed.templatePath, "template", ".env.example", "path to the dotenv template")
	flags.StringVar(&parsed.outputPath, "output", ".env", "path to the generated dotenv file")
	flags.BoolVar(&parsed.force, "force", false, "skip the final create or overwrite confirmation")
	flags.BoolVar(&parsed.showVersion, "version", false, "print version information and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: env-wizard [--template PATH] [--output PATH] [--force]")
		fmt.Fprintln(stderr, "       env-wizard --version")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			result := exitSuccess
			return parsedArguments{}, &result
		}

		result := exitUsage
		return parsedArguments{}, &result
	}

	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "env-wizard: unexpected argument %q\n", flags.Arg(0))
		flags.Usage()
		result := exitUsage
		return parsedArguments{}, &result
	}

	visited := 0
	flags.Visit(func(visitedFlag *flag.Flag) {
		visited++
		switch visitedFlag.Name {
		case "template":
			parsed.templateExplicit = true
		case "output":
			parsed.outputExplicit = true
		}
	})

	if parsed.showVersion {
		if visited != 1 {
			fmt.Fprintln(stderr, "env-wizard: --version cannot be combined with other flags")
			flags.Usage()
			result := exitUsage
			return parsedArguments{}, &result
		}
	}

	return parsed, nil
}

func writeMissingDefaultTemplate(stderr io.Writer, cwd string) {
	fmt.Fprintf(stderr, "env-wizard: no .env.example template found in the current directory:\n  %s\n\n", cwd)
	fmt.Fprintln(stderr, "Create a .env.example file there, or specify another template:")
	fmt.Fprintln(stderr, "  env-wizard --template path/to/file.env.example")
}

func resolveOptions(cwd string, parsed parsedArguments) (app.Options, error) {
	templatePath, err := resolvePath(cwd, "--template", parsed.templatePath)
	if err != nil {
		return app.Options{}, err
	}

	outputPath, err := resolvePath(cwd, "--output", parsed.outputPath)
	if err != nil {
		return app.Options{}, err
	}

	return app.Options{
		TemplatePath:        templatePath,
		OutputPath:          outputPath,
		SuggestedOutputPath: suggestedOutputPath(parsed, templatePath, outputPath),
		Force:               parsed.force,
	}, nil
}

func suggestedOutputPath(parsed parsedArguments, templatePath, outputPath string) string {
	if !parsed.templateExplicit || parsed.outputExplicit {
		return ""
	}

	base := filepath.Base(templatePath)
	if !strings.HasSuffix(base, ".env.example") {
		return ""
	}

	suggested := filepath.Join(filepath.Dir(templatePath), strings.TrimSuffix(base, ".example"))
	if suggested == outputPath {
		return ""
	}
	return suggested
}

func resolvePath(cwd, flagName, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s path must not be empty", flagName)
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s path: %w", flagName, err)
	}

	return filepath.Clean(absPath), nil
}

func versionText() string {
	if commit == "" {
		return "env-wizard " + version
	}

	return fmt.Sprintf("env-wizard %s (commit %s)", version, commit)
}
