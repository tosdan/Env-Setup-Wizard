package filesystem_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	projectfs "github.com/tosdan/env-setup-wizard/internal/filesystem"
)

func TestSafeWriteCreatesNewOutputWithoutBackup(t *testing.T) {
	root := t.TempDir()
	templatePath := writeSafeWriteFile(t, filepath.Join(root, ".env.example"), []byte("KEY=template\n"), 0o600)
	outputPath := filepath.Join(root, ".env")
	candidate := []byte("KEY='new value'\n")

	backupPath, err := projectfs.SafeWrite(templatePath, outputPath, candidate)
	if err != nil {
		t.Fatalf("SafeWrite() error = %v, want nil", err)
	}
	if backupPath != "" {
		t.Fatalf("SafeWrite() backup = %q for new output, want empty", backupPath)
	}
	assertSafeWriteContent(t, outputPath, candidate)
	assertNoStagedFiles(t, root)
	backups, err := filepath.Glob(outputPath + ".backup-*")
	if err != nil {
		t.Fatalf("Glob backups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("new output created backups: %v", backups)
	}
	if runtime.GOOS != "windows" {
		assertSafeWriteMode(t, outputPath, 0o600)
	}
}

func TestSafeWriteOverwritesWithByteIdenticalBackup(t *testing.T) {
	root := t.TempDir()
	templatePath := writeSafeWriteFile(t, filepath.Join(root, ".env.example"), []byte("KEY=template\n"), 0o600)
	outputPath := writeSafeWriteFile(t, filepath.Join(root, ".env"), []byte("KEY=old\r\n"), 0o640)
	original, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", outputPath, err)
	}
	candidate := []byte("KEY='new'\n")

	backupPath, err := projectfs.SafeWrite(templatePath, outputPath, candidate)
	if err != nil {
		t.Fatalf("SafeWrite() error = %v, want nil", err)
	}
	if !strings.HasPrefix(backupPath, outputPath+".backup-") {
		t.Fatalf("SafeWrite() backup = %q, want timestamped path", backupPath)
	}
	assertSafeWriteContent(t, outputPath, candidate)
	assertSafeWriteContent(t, backupPath, original)
	assertNoStagedFiles(t, root)
	if runtime.GOOS != "windows" {
		assertSafeWriteMode(t, outputPath, 0o640)
		assertSafeWriteMode(t, backupPath, 0o600)
	}
}

func TestSafeWriteOwnsFinalPreflight(t *testing.T) {
	root := t.TempDir()
	templatePath := writeSafeWriteFile(t, filepath.Join(root, ".env.example"), []byte("KEY=template\n"), 0o600)
	protectedPath := writeSafeWriteFile(t, filepath.Join(root, "protected.env"), []byte("PROTECTED=unchanged\n"), 0o600)
	outputPath := filepath.Join(root, ".env")
	if err := os.Symlink(protectedPath, outputPath); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	_, err := projectfs.SafeWrite(templatePath, outputPath, []byte("KEY=new\n"))
	if err == nil || !strings.Contains(err.Error(), "preflight safe write") || !strings.Contains(err.Error(), "must not be a symbolic link") {
		t.Fatalf("SafeWrite() error = %v, want final preflight rejection", err)
	}
	assertSafeWriteContent(t, protectedPath, []byte("PROTECTED=unchanged\n"))
	assertNoStagedFiles(t, root)
}

func writeSafeWriteFile(t *testing.T, path string, content []byte, mode os.FileMode) string {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
	return path
}

func assertSafeWriteContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content of %q = %q, want %q", path, got, want)
	}
}

func assertNoStagedFiles(t *testing.T, root string) {
	t.Helper()
	staged, err := filepath.Glob(filepath.Join(root, ".env.tmp-*"))
	if err != nil {
		t.Fatalf("Glob staged files: %v", err)
	}
	if len(staged) != 0 {
		t.Fatalf("staged files remain: %v", staged)
	}
}

func assertSafeWriteMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %q = %04o, want %04o", path, got, want)
	}
}
