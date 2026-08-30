package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

const backupTimestampLayout = "20060102T150405Z"

type backupDependencies struct {
	readFile func(string) ([]byte, error)
	openFile func(string, int, fs.FileMode) (stagingFile, error)
	remove   func(string) error
}

// createBackup creates a byte-identical, synced backup without overwriting any
// existing backup. The returned path uses a UTC timestamp and a numeric suffix
// when the timestamp name is already occupied.
func createBackup(outputPath string, at time.Time) (string, error) {
	return createBackupWith(outputPath, at, backupDependencies{
		readFile: os.ReadFile,
		openFile: func(path string, flag int, mode fs.FileMode) (stagingFile, error) {
			return os.OpenFile(path, flag, mode)
		},
		remove: os.Remove,
	})
}

func createBackupWith(
	outputPath string,
	at time.Time,
	dependencies backupDependencies,
) (string, error) {
	content, err := dependencies.readFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read existing output %q for backup: %w", outputPath, err)
	}

	basePath := outputPath + ".backup-" + at.UTC().Format(backupTimestampLayout)
	for collision := 0; ; collision++ {
		backupPath := basePath
		if collision > 0 {
			backupPath = fmt.Sprintf("%s-%d", basePath, collision)
		}

		file, err := dependencies.openFile(
			backupPath,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0o600,
		)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create backup %q: %w", backupPath, err)
		}
		if err := persistCreatedFile(file, content, 0o600, "backup", dependencies.remove); err != nil {
			return "", err
		}
		return backupPath, nil
	}
}
