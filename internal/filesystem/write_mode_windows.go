//go:build windows

package filesystem

import "io/fs"

func newOutputMode() fs.FileMode {
	return 0o600
}

func overwriteOutputMode(fs.FileMode) fs.FileMode {
	// Windows CreateTemp derives ACLs from the output directory. Chmod(0600)
	// keeps the staged file writable but does not promise preservation of the
	// previous target ACLs.
	return 0o600
}
