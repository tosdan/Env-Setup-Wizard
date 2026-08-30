package filesystem

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCreateBackupUsesUTCExclusiveNameAndPreservesContent(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(root, ".env")
	content := []byte("SECRET=do-not-show\r\nEMPTY=\r\n")
	if err := os.WriteFile(outputPath, content, 0o640); err != nil {
		t.Fatalf("WriteFile(%q): %v", outputPath, err)
	}

	at := time.Date(2026, time.August, 30, 15, 4, 5, 0, time.FixedZone("UTC+2", 2*60*60))
	basePath := outputPath + ".backup-20260830T130405Z"
	for _, collision := range []string{basePath, basePath + "-1"} {
		if err := os.WriteFile(collision, []byte("keep-existing"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", collision, err)
		}
	}

	backupPath, err := createBackup(outputPath, at)
	if err != nil {
		t.Fatalf("createBackup() error = %v, want nil", err)
	}
	if want := basePath + "-2"; backupPath != want {
		t.Fatalf("createBackup() path = %q, want %q", backupPath, want)
	}
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", backupPath, err)
	}
	if !bytes.Equal(backupContent, content) {
		t.Fatalf("backup content = %q, want byte-identical %q", backupContent, content)
	}
	for _, collision := range []string{basePath, basePath + "-1"} {
		got, err := os.ReadFile(collision)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", collision, err)
		}
		if string(got) != "keep-existing" {
			t.Fatalf("existing backup %q was overwritten: %q", collision, got)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(backupPath)
		if err != nil {
			t.Fatalf("Stat(%q): %v", backupPath, err)
		}
		if gotMode := info.Mode().Perm(); gotMode != 0o600 {
			t.Fatalf("backup mode = %04o, want 0600", gotMode)
		}
	}
}

func TestCreateBackupCleansFailedBackupAndLeavesOutputIntact(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(root, ".env")
	content := []byte("SECRET=do-not-show\n")
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", outputPath, err)
	}
	at := time.Date(2026, time.August, 30, 13, 4, 5, 0, time.UTC)
	wantBackupPath := outputPath + ".backup-20260830T130405Z"
	events := make([]string, 0)
	removedPath := ""
	file := &fakeStagingFile{
		name:    wantBackupPath,
		failure: "sync",
		events:  &events,
	}
	dependencies := backupDependencies{
		readFile: os.ReadFile,
		openFile: func(path string, flag int, mode fs.FileMode) (stagingFile, error) {
			events = append(events, "open")
			if path != wantBackupPath {
				t.Fatalf("open path = %q, want %q", path, wantBackupPath)
			}
			if flag != os.O_WRONLY|os.O_CREATE|os.O_EXCL {
				t.Fatalf("open flags = %d, want exclusive create flags", flag)
			}
			if mode != 0o600 {
				t.Fatalf("open mode = %04o, want 0600", mode)
			}
			return file, nil
		},
		remove: func(path string) error {
			events = append(events, "remove")
			removedPath = path
			return nil
		},
	}

	backupPath, err := createBackupWith(outputPath, at, dependencies)
	if err == nil || !strings.Contains(err.Error(), "sync backup") {
		t.Fatalf("createBackupWith() error = %v, want sync failure", err)
	}
	if backupPath != "" {
		t.Fatalf("createBackupWith() path = %q after failure, want empty", backupPath)
	}
	if removedPath != wantBackupPath {
		t.Fatalf("removed path = %q, want %q", removedPath, wantBackupPath)
	}
	if strings.Contains(err.Error(), "do-not-show") {
		t.Fatalf("createBackupWith() error leaked content: %q", err)
	}
	if got := strings.Join(events, ","); got != "open,chmod,write,sync,close,remove" {
		t.Fatalf("events = %q, want full cleanup sequence", got)
	}
	gotOutput, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", outputPath, err)
	}
	if !bytes.Equal(gotOutput, content) {
		t.Fatalf("output changed after backup failure: %q", gotOutput)
	}
}

func TestCreateBackupReportsReadFailureWithoutCreatingBackup(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), ".env")
	_, err := createBackupWith(outputPath, time.Time{}, backupDependencies{
		readFile: func(string) ([]byte, error) {
			return nil, errors.New("injected read failure")
		},
		openFile: func(string, int, fs.FileMode) (stagingFile, error) {
			t.Fatal("openFile called after read failure")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "read existing output") {
		t.Fatalf("createBackupWith() error = %v, want read failure", err)
	}
}
