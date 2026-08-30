//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package filesystem

import (
	"fmt"
	"os"
)

// replaceFile atomically renames a closed staged file over its target. The
// caller guarantees that both paths are in the same directory.
func replaceFile(stagedPath, targetPath string) error {
	if err := os.Rename(stagedPath, targetPath); err != nil {
		return fmt.Errorf("replace output %q: %w", targetPath, err)
	}
	return nil
}
