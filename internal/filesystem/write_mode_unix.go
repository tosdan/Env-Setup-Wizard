//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package filesystem

import "io/fs"

func newOutputMode() fs.FileMode {
	return 0o600
}

func overwriteOutputMode(existing fs.FileMode) fs.FileMode {
	return existing.Perm()
}
