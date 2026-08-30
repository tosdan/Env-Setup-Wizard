//go:build windows

package filesystem

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type moveFileExFunc func(*uint16, *uint16, uint32) error

// replaceFile atomically moves a closed staged file over its target and asks
// Windows not to return before the move has been flushed to disk.
func replaceFile(stagedPath, targetPath string) error {
	return replaceFileWith(stagedPath, targetPath, windows.MoveFileEx)
}

func replaceFileWith(stagedPath, targetPath string, move moveFileExFunc) error {
	stagedPathPointer, err := windows.UTF16PtrFromString(stagedPath)
	if err != nil {
		return fmt.Errorf("encode staged output path: %w", err)
	}
	targetPathPointer, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("encode output path: %w", err)
	}

	flags := uint32(windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH)
	if err := move(stagedPathPointer, targetPathPointer, flags); err != nil {
		return fmt.Errorf("replace output %q: %w", targetPath, err)
	}
	return nil
}
