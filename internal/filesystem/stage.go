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
) (stagedPath string, err error) {
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

	temporaryPath := file.Name()
	stagedPath = temporaryPath
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close staged output %q during cleanup: %w", temporaryPath, closeErr))
			}
		}
		if err != nil {
			if removeErr := dependencies.remove(temporaryPath); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("remove staged output %q: %w", temporaryPath, removeErr))
			}
			stagedPath = ""
		}
	}()

	if err := file.Chmod(mode.Perm()); err != nil {
		return "", fmt.Errorf("set staged output permissions %q: %w", temporaryPath, err)
	}
	if written, writeErr := file.Write(content); writeErr != nil {
		return "", fmt.Errorf("write staged output %q: %w", temporaryPath, writeErr)
	} else if written != len(content) {
		return "", fmt.Errorf("write staged output %q: %w", temporaryPath, io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("sync staged output %q: %w", temporaryPath, err)
	}
	if closeErr := file.Close(); closeErr != nil {
		closed = true
		return "", fmt.Errorf("close staged output %q: %w", temporaryPath, closeErr)
	}
	closed = true

	return stagedPath, nil
}
