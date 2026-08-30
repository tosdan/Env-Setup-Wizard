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
)

func TestStageCandidateWritesSyncedClosedFileBesideOutput(t *testing.T) {
	root := t.TempDir()
	outputPath := filepath.Join(root, ".env")
	content := []byte("SECRET=do-not-show\n")

	stagedPath, err := stageCandidate(outputPath, content, 0o600)
	if err != nil {
		t.Fatalf("stageCandidate() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = os.Remove(stagedPath) })

	if filepath.Dir(stagedPath) != root {
		t.Fatalf("staged directory = %q, want %q", filepath.Dir(stagedPath), root)
	}
	if !strings.HasPrefix(filepath.Base(stagedPath), ".env.tmp-") {
		t.Fatalf("staged name = %q, want .env.tmp-*", filepath.Base(stagedPath))
	}
	got, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", stagedPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("staged content = %q, want %q", got, content)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(stagedPath)
		if err != nil {
			t.Fatalf("Stat(%q): %v", stagedPath, err)
		}
		if gotMode := info.Mode().Perm(); gotMode != 0o600 {
			t.Fatalf("staged mode = %04o, want 0600", gotMode)
		}
	}

	renamedPath := stagedPath + ".closed"
	if err := os.Rename(stagedPath, renamedPath); err != nil {
		t.Fatalf("Rename closed staged file: %v", err)
	}
	stagedPath = renamedPath
}

func TestStageCandidateCleansUpAfterFileOperationFailures(t *testing.T) {
	tests := []struct {
		name           string
		failure        string
		wantError      string
		wantEvents     []string
		cleanupFailure bool
	}{
		{
			name:       "permissions",
			failure:    "chmod",
			wantError:  "set staged output permissions",
			wantEvents: []string{"create", "chmod", "close", "remove"},
		},
		{
			name:       "write",
			failure:    "write",
			wantError:  "write staged output",
			wantEvents: []string{"create", "chmod", "write", "close", "remove"},
		},
		{
			name:       "short write",
			failure:    "short-write",
			wantError:  "short write",
			wantEvents: []string{"create", "chmod", "write", "close", "remove"},
		},
		{
			name:       "sync",
			failure:    "sync",
			wantError:  "sync staged output",
			wantEvents: []string{"create", "chmod", "write", "sync", "close", "remove"},
		},
		{
			name:       "close",
			failure:    "close",
			wantError:  "close staged output",
			wantEvents: []string{"create", "chmod", "write", "sync", "close", "remove"},
		},
		{
			name:           "cleanup",
			failure:        "write",
			wantError:      "remove staged output",
			wantEvents:     []string{"create", "chmod", "write", "close", "remove"},
			cleanupFailure: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := make([]string, 0)
			removedPath := ""
			file := &fakeStagingFile{
				name:    filepath.Join(t.TempDir(), ".env.tmp-test"),
				failure: test.failure,
				events:  &events,
			}
			dependencies := stagingDependencies{
				createTemp: func(string, string) (stagingFile, error) {
					events = append(events, "create")
					return file, nil
				},
				remove: func(path string) error {
					events = append(events, "remove")
					removedPath = path
					if test.cleanupFailure {
						return errors.New("injected remove failure")
					}
					return nil
				},
			}

			stagedPath, err := stageCandidateWith(
				filepath.Join(t.TempDir(), ".env"),
				[]byte("do-not-show"),
				0o600,
				dependencies,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("stageCandidateWith() error = %v, want %q", err, test.wantError)
			}
			if stagedPath != "" {
				t.Fatalf("stageCandidateWith() path = %q after failure, want empty", stagedPath)
			}
			if removedPath != file.name {
				t.Fatalf("removed path = %q, want staged path %q", removedPath, file.name)
			}
			if strings.Contains(err.Error(), "do-not-show") {
				t.Fatalf("stageCandidateWith() error leaked content: %q", err)
			}
			if got := strings.Join(events, ","); got != strings.Join(test.wantEvents, ",") {
				t.Fatalf("events = %q, want %q", got, strings.Join(test.wantEvents, ","))
			}
		})
	}
}

type fakeStagingFile struct {
	name    string
	failure string
	events  *[]string
}

func (file *fakeStagingFile) Name() string { return file.name }

func (file *fakeStagingFile) Chmod(fs.FileMode) error {
	*file.events = append(*file.events, "chmod")
	return file.operationError("chmod")
}

func (file *fakeStagingFile) Write(content []byte) (int, error) {
	*file.events = append(*file.events, "write")
	if file.failure == "short-write" {
		return len(content) - 1, nil
	}
	if err := file.operationError("write"); err != nil {
		return 0, err
	}
	return len(content), nil
}

func (file *fakeStagingFile) Sync() error {
	*file.events = append(*file.events, "sync")
	return file.operationError("sync")
}

func (file *fakeStagingFile) Close() error {
	*file.events = append(*file.events, "close")
	return file.operationError("close")
}

func (file *fakeStagingFile) operationError(operation string) error {
	if file.failure == operation {
		return errors.New("injected " + operation + " failure")
	}
	return nil
}
