package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

type safeWriteDependencies struct {
	preflight     func(string, string) error
	inspectOutput func(string) (bool, fs.FileMode, error)
	stage         func(string, []byte, fs.FileMode) (string, error)
	backup        func(string, time.Time) (string, error)
	replace       func(string, string) error
	remove        func(string) error
	now           func() time.Time
}

// SafeWrite revalidates the paths and atomically replaces outputPath with the
// complete candidate. Existing output is backed up before replacement. Both
// paths must be absolute, and candidate must already contain the full output.
// The returned backup path is empty when outputPath did not previously exist.
func SafeWrite(templatePath, outputPath string, candidate []byte) (string, error) {
	return safeWriteWith(templatePath, outputPath, candidate, safeWriteDependencies{
		preflight:     Preflight,
		inspectOutput: inspectOutputForWrite,
		stage:         stageCandidate,
		backup:        createBackup,
		replace:       replaceFile,
		remove:        os.Remove,
		now:           time.Now,
	})
}

func safeWriteWith(
	templatePath string,
	outputPath string,
	candidate []byte,
	dependencies safeWriteDependencies,
) (backupPath string, err error) {
	if err := dependencies.preflight(templatePath, outputPath); err != nil {
		return "", fmt.Errorf("preflight safe write: %w", err)
	}

	exists, mode, err := dependencies.inspectOutput(outputPath)
	if err != nil {
		return "", err
	}
	stagedPath, err := dependencies.stage(outputPath, candidate, mode)
	if err != nil {
		return "", err
	}

	replaced := false
	defer func() {
		if replaced {
			return
		}
		if removeErr := dependencies.remove(stagedPath); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove staged output %q after aborted write: %w", stagedPath, removeErr))
		}
	}()

	if exists {
		backupPath, err = dependencies.backup(outputPath, dependencies.now())
		if err != nil {
			return "", err
		}
	}
	if err := dependencies.replace(stagedPath, outputPath); err != nil {
		if backupPath != "" {
			return "", fmt.Errorf(
				"replace output after creating backup %q: %w",
				backupPath,
				err,
			)
		}
		return "", err
	}
	replaced = true

	return backupPath, nil
}

func inspectOutputForWrite(outputPath string) (bool, fs.FileMode, error) {
	info, err := os.Lstat(outputPath)
	if os.IsNotExist(err) {
		return false, newOutputMode(), nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("inspect output %q before write: %w", outputPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, 0, fmt.Errorf("output %q must not be a symbolic link", outputPath)
	}
	if !info.Mode().IsRegular() {
		return false, 0, fmt.Errorf("output %q is not a regular file", outputPath)
	}
	return true, overwriteOutputMode(info.Mode()), nil
}
