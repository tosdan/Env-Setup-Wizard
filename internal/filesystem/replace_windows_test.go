//go:build windows

package filesystem

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestReplaceFileWithUsesReplaceAndWriteThroughFlags(t *testing.T) {
	stagedPath := `C:\temp\staged-è`
	targetPath := `C:\temp\.env`
	called := false
	err := replaceFileWith(stagedPath, targetPath, func(from, to *uint16, flags uint32) error {
		called = true
		if got := windows.UTF16PtrToString(from); got != stagedPath {
			t.Errorf("MoveFileExW source = %q, want %q", got, stagedPath)
		}
		if got := windows.UTF16PtrToString(to); got != targetPath {
			t.Errorf("MoveFileExW target = %q, want %q", got, targetPath)
		}
		wantFlags := uint32(windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH)
		if flags != wantFlags {
			t.Errorf("MoveFileExW flags = %#x, want %#x", flags, wantFlags)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replaceFileWith() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("replaceFileWith() did not call MoveFileExW adapter")
	}
}

func TestReplaceFileWithRejectsInvalidPathsAndWrapsMoveErrors(t *testing.T) {
	called := false
	err := replaceFileWith("invalid\x00path", `C:\temp\.env`, func(*uint16, *uint16, uint32) error {
		called = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "encode staged output path") {
		t.Fatalf("replaceFileWith(invalid path) error = %v, want encoding failure", err)
	}
	if called {
		t.Fatal("MoveFileExW adapter called with invalid source path")
	}

	injected := errors.New("injected move failure")
	err = replaceFileWith(`C:\temp\staged`, `C:\temp\.env`, func(*uint16, *uint16, uint32) error {
		return injected
	})
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), "replace output") {
		t.Fatalf("replaceFileWith(move failure) error = %v, want wrapped move failure", err)
	}
}
