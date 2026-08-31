package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrTemplateNotFound identifies a template path that does not exist.
var ErrTemplateNotFound = errors.New("template not found")

type templateNotFoundError struct {
	path  string
	cause error
}

func (err *templateNotFoundError) Error() string {
	return fmt.Sprintf("inspect template %q: %v", err.path, err.cause)
}

func (err *templateNotFoundError) Unwrap() []error {
	return []error{ErrTemplateNotFound, err.cause}
}

// Preflight verifies that templatePath can be read safely and that outputPath
// is a valid destination. Both paths must already be absolute.
//
// A template symlink is allowed when it resolves to a regular file. An existing
// output must itself be a regular file, never a symlink. The function also
// rejects paths that identify the same file through spelling, symlinks, or
// hardlinks.
func Preflight(templatePath, outputPath string) error {
	templatePath, templateInfo, err := inspectTemplate(templatePath)
	if err != nil {
		return err
	}

	outputPath, err = checkedAbsolutePath("output", outputPath)
	if err != nil {
		return err
	}

	if samePathName(templatePath, outputPath) {
		return sameFileError(templatePath, outputPath)
	}

	outputParent := filepath.Dir(outputPath)
	parentInfo, err := os.Stat(outputParent)
	if err != nil {
		return fmt.Errorf("inspect output directory %q: %w", outputParent, err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("output directory %q is not a directory", outputParent)
	}

	outputInfo, err := os.Lstat(outputPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output %q: %w", outputPath, err)
	}
	if outputInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output %q must not be a symbolic link", outputPath)
	}
	if !outputInfo.Mode().IsRegular() {
		return fmt.Errorf("output %q is not a regular file", outputPath)
	}
	if os.SameFile(templateInfo, outputInfo) {
		return sameFileError(templatePath, outputPath)
	}

	return nil
}

// PreflightTemplate verifies only that templatePath is an absolute, readable
// regular file. It is used before an interactive output destination is chosen.
func PreflightTemplate(templatePath string) error {
	_, _, err := inspectTemplate(templatePath)
	return err
}

func inspectTemplate(templatePath string) (string, os.FileInfo, error) {
	templatePath, err := checkedAbsolutePath("template", templatePath)
	if err != nil {
		return "", nil, err
	}

	templateInfo, err := os.Stat(templatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, &templateNotFoundError{path: templatePath, cause: err}
		}
		return "", nil, fmt.Errorf("inspect template %q: %w", templatePath, err)
	}
	if !templateInfo.Mode().IsRegular() {
		return "", nil, fmt.Errorf("template %q is not a regular file", templatePath)
	}
	if err := checkReadable(templatePath); err != nil {
		return "", nil, err
	}

	return templatePath, templateInfo, nil
}

func checkedAbsolutePath(role, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s path must not be empty", role)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s path %q must be absolute", role, path)
	}

	return filepath.Clean(path), nil
}

func checkReadable(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open template %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close template %q after readability check: %w", path, err)
	}

	return nil
}

func samePathName(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}

func sameFileError(templatePath, outputPath string) error {
	return fmt.Errorf("template %q and output %q identify the same file", templatePath, outputPath)
}
