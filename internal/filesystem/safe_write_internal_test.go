package filesystem

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestSafeWriteCleansStageAndKeepsBackupWhenReplaceFails(t *testing.T) {
	injected := errors.New("injected replace failure")
	events := make([]string, 0)
	dependencies := safeWriteDependencies{
		preflight: func(string, string) error {
			events = append(events, "preflight")
			return nil
		},
		inspectOutput: func(string) (bool, fs.FileMode, error) {
			events = append(events, "inspect")
			return true, 0o640, nil
		},
		stage: func(_ string, _ []byte, mode fs.FileMode) (string, error) {
			events = append(events, "stage")
			if mode != 0o640 {
				t.Fatalf("stage mode = %04o, want preserved 0640", mode)
			}
			return "staged-path", nil
		},
		backup: func(string, time.Time) (string, error) {
			events = append(events, "backup")
			return "backup-path", nil
		},
		replace: func(string, string) error {
			events = append(events, "replace")
			return injected
		},
		remove: func(path string) error {
			events = append(events, "remove "+path)
			return nil
		},
		now: func() time.Time { return time.Time{} },
	}

	backupPath, err := safeWriteWith("template", "output", []byte("candidate"), dependencies)
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), `after creating backup "backup-path"`) {
		t.Fatalf("safeWriteWith() error = %v, want wrapped replace failure with backup path", err)
	}
	if backupPath != "" {
		t.Fatalf("safeWriteWith() backup = %q after failure, want empty", backupPath)
	}
	if got := strings.Join(events, ","); got != "preflight,inspect,stage,backup,replace,remove staged-path" {
		t.Fatalf("events = %q, want ordered safe-write cleanup", got)
	}
}

func TestSafeWriteCleansStageAndSkipsReplaceWhenBackupFails(t *testing.T) {
	injected := errors.New("injected backup failure")
	events := make([]string, 0)
	dependencies := safeWriteDependencies{
		preflight: func(string, string) error {
			events = append(events, "preflight")
			return nil
		},
		inspectOutput: func(string) (bool, fs.FileMode, error) {
			events = append(events, "inspect")
			return true, 0o600, nil
		},
		stage: func(string, []byte, fs.FileMode) (string, error) {
			events = append(events, "stage")
			return "staged-path", nil
		},
		backup: func(string, time.Time) (string, error) {
			events = append(events, "backup")
			return "", injected
		},
		replace: func(string, string) error {
			t.Fatal("replace called after backup failure")
			return nil
		},
		remove: func(path string) error {
			events = append(events, "remove "+path)
			return nil
		},
		now: func() time.Time { return time.Time{} },
	}

	backupPath, err := safeWriteWith("template", "output", []byte("candidate"), dependencies)
	if !errors.Is(err, injected) {
		t.Fatalf("safeWriteWith() error = %v, want backup failure", err)
	}
	if backupPath != "" {
		t.Fatalf("safeWriteWith() backup = %q after failure, want empty", backupPath)
	}
	if got := strings.Join(events, ","); got != "preflight,inspect,stage,backup,remove staged-path" {
		t.Fatalf("events = %q, want cleanup without replace", got)
	}
}

func TestSafeWriteReportsStageCleanupFailure(t *testing.T) {
	replaceFailure := errors.New("injected replace failure")
	cleanupFailure := errors.New("injected cleanup failure")
	dependencies := safeWriteDependencies{
		preflight:     func(string, string) error { return nil },
		inspectOutput: func(string) (bool, fs.FileMode, error) { return false, 0o600, nil },
		stage:         func(string, []byte, fs.FileMode) (string, error) { return "staged-path", nil },
		backup:        func(string, time.Time) (string, error) { t.Fatal("backup called for new output"); return "", nil },
		replace:       func(string, string) error { return replaceFailure },
		remove:        func(string) error { return cleanupFailure },
		now:           func() time.Time { return time.Time{} },
	}

	_, err := safeWriteWith("template", "output", nil, dependencies)
	if !errors.Is(err, replaceFailure) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("safeWriteWith() error = %v, want replace and cleanup failures", err)
	}
}
