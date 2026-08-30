package main

import "testing"

func TestVersionTextDevelopmentBuild(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() {
		version, commit = originalVersion, originalCommit
	})

	version, commit = "dev", ""

	if got, want := versionText(), "env-wizard dev"; got != want {
		t.Fatalf("versionText() = %q, want %q", got, want)
	}
}

func TestVersionTextReleaseBuild(t *testing.T) {
	originalVersion, originalCommit := version, commit
	t.Cleanup(func() {
		version, commit = originalVersion, originalCommit
	})

	version, commit = "v1.0.0", "abc1234"

	if got, want := versionText(), "env-wizard v1.0.0 (commit abc1234)"; got != want {
		t.Fatalf("versionText() = %q, want %q", got, want)
	}
}
