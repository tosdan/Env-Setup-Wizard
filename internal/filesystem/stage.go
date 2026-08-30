package filesystem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type stagingFile interface {
	Name() string
	Chmod(fs.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type stagingDependencies struct {
	createTemp func(string, string) (stagingFile, error)
	remove     func(string) error
}

// stageCandidate writes, syncs, and closes a candidate in the output
// directory. On success the caller owns the returned temporary path and must
// replace or remove it. Every failure attempts to close and remove the file.
func stageCandidate(outputPath string, content []byte, mode fs.FileMode) (string, error) {
	return stageCandidateWith(outputPath, content, mode, stagingDependencies{
		createTemp: func(directory, pattern string) (stagingFile, error) {
			return os.CreateTemp(directory, pattern)
		},
		remove: os.Remove,
	})
}

func stageCandidateWith(
	outputPath string,
	content []byte,
	mode fs.FileMode,
	dependencies stagingDependencies,
) (string, error) {
	directory := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	pattern := base + ".tmp-*"
	if base[0] != '.' {
		pattern = "." + pattern
	}
	file, err := dependencies.createTemp(directory, pattern)
	if err != nil {
		return "", fmt.Errorf("create staged output in %q: %w", directory, err)
	}

	stagedPath := file.Name()
	if err := persistCreatedFile(file, content, mode, "staged output", dependencies.remove); err != nil {
		return "", err
	}
	return stagedPath, nil
}

func persistCreatedFile(
	file stagingFile,
	content []byte,
	mode fs.FileMode,
	role string,
	remove func(string) error,
) (err error) {
	path := file.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close %s %q during cleanup: %w", role, path, closeErr))
			}
		}
		if err != nil {
			if removeErr := remove(path); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("remove %s %q: %w", role, path, removeErr))
			}
		}
	}()

	if err := file.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("set %s permissions %q: %w", role, path, err)
	}
	if written, writeErr := file.Write(content); writeErr != nil {
		return fmt.Errorf("write %s %q: %w", role, path, writeErr)
	} else if written != len(content) {
		return fmt.Errorf("write %s %q: %w", role, path, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync %s %q: %w", role, path, err)
	}
	if closeErr := file.Close(); closeErr != nil {
		closed = true
		return fmt.Errorf("close %s %q: %w", role, path, closeErr)
	}
	closed = true

	return nil
}
