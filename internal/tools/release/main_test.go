package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestValidateVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		valid   bool
	}{
		{version: "v1.0.0", valid: true},
		{version: "v1.0.0-rc.1", valid: true},
		{version: "v0.12.3-alpha-beta.9", valid: true},
		{version: "1.0.0"},
		{version: "v1.0"},
		{version: "v01.0.0"},
		{version: "v1.0.0-rc.01"},
		{version: "v1.0.0+build.1"},
		{version: "v1.0.0-"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.version, func(t *testing.T) {
			t.Parallel()
			err := validateVersion(test.version)
			if test.valid && err != nil {
				t.Fatalf("validateVersion(%q) returned %v", test.version, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("validateVersion(%q) unexpectedly succeeded", test.version)
			}
		})
	}
}

func TestNormalizeCommit(t *testing.T) {
	t.Parallel()

	commit, err := normalizeCommit("ABCDEF0123456789ABCDEF0123456789ABCDEF01")
	if err != nil {
		t.Fatalf("normalizeCommit returned %v", err)
	}
	if commit != "abcdef0" {
		t.Fatalf("normalizeCommit returned %q", commit)
	}
	for _, invalid := range []string{"", "abcdef", "abcdefg", "123456!", strings.Repeat("a", 41)} {
		if _, err := normalizeCommit(invalid); err == nil {
			t.Fatalf("normalizeCommit(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestArchiveName(t *testing.T) {
	t.Parallel()

	windows, err := targetByName("windows-amd64")
	if err != nil {
		t.Fatal(err)
	}
	if actual := archiveName("v1.0.0-rc.2", windows); actual != "env-wizard_1.0.0-rc.2_windows_amd64.zip" {
		t.Fatalf("archiveName returned %q", actual)
	}
	linux, err := targetByName("linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if actual := archiveName("v1.0.0", linux); actual != "env-wizard_1.0.0_linux_arm64.tar.gz" {
		t.Fatalf("archiveName returned %q", actual)
	}
}

func TestLinkerFlagsTargetMainPackageVariables(t *testing.T) {
	t.Parallel()

	flags := linkerFlags("v1.0.0-rc.1", "abcdef0")
	for _, expected := range []string{
		"-buildid=",
		"-X main.version=v1.0.0-rc.1",
		"-X main.commit=abcdef0",
	} {
		if !strings.Contains(flags, expected) {
			t.Fatalf("linkerFlags returned %q without %q", flags, expected)
		}
	}
}

func TestArchivesAreDeterministicAndVerifiable(t *testing.T) {
	t.Parallel()

	root := testReleaseRoot(t)
	binary := []byte("test executable")
	for _, targetName := range []string{"windows-amd64", "linux-amd64"} {
		releaseTarget, err := targetByName(targetName)
		if err != nil {
			t.Fatal(err)
		}
		entries := testArchiveEntries(t, root, releaseTarget, binary)
		first, err := archiveBytes(entries, releaseTarget)
		if err != nil {
			t.Fatalf("archiveBytes(%s) returned %v", targetName, err)
		}
		second, err := archiveBytes(entries, releaseTarget)
		if err != nil {
			t.Fatalf("archiveBytes(%s) returned %v", targetName, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("%s archive is not deterministic", targetName)
		}

		path := filepath.Join(root, "artifact"+releaseTarget.archiveExtension)
		if err := os.WriteFile(path, first, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := verifyArchive(root, path, releaseTarget); err != nil {
			t.Fatalf("verifyArchive(%s) returned %v", targetName, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVerifyArchiveDetectsChangedRepositoryDocument(t *testing.T) {
	t.Parallel()

	root := testReleaseRoot(t)
	releaseTarget, err := targetByName("linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := archiveBytes(testArchiveEntries(t, root, releaseTarget, []byte("binary")), releaseTarget)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifact.tar.gz")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(root, path, releaseTarget); err == nil {
		t.Fatal("verifyArchive unexpectedly accepted stale README.md content")
	}
}

func TestFinalizeReleaseWritesAndVerifiesAllChecksums(t *testing.T) {
	t.Parallel()

	root := testReleaseRoot(t)
	dist := filepath.Join(root, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, releaseTarget := range releaseTargets {
		data, err := archiveBytes(testArchiveEntries(t, root, releaseTarget, []byte("binary for "+releaseTarget.name)), releaseTarget)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dist, archiveName("v1.0.0-rc.1", releaseTarget))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	checksumPath, err := finalizeRelease(root, finalizeOptions{
		version:   "v1.0.0-rc.1",
		inputDir:  dist,
		outputDir: dist,
	})
	if err != nil {
		t.Fatalf("finalizeRelease returned %v", err)
	}
	names := make([]string, 0, len(releaseTargets))
	for _, releaseTarget := range releaseTargets {
		names = append(names, archiveName("v1.0.0-rc.1", releaseTarget))
	}
	sort.Strings(names)
	if err := verifyChecksums(checksumPath, dist, names); err != nil {
		t.Fatalf("verifyChecksums returned %v", err)
	}
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(content), "\n"); lines != len(releaseTargets) {
		t.Fatalf("SHA256SUMS has %d rows, expected %d", lines, len(releaseTargets))
	}
}

func testReleaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, file := range []struct {
		name    string
		content string
	}{
		{name: "README.md", content: "readme\n"},
		{name: "LICENSE", content: "license\n"},
		{name: "THIRD_PARTY_NOTICES", content: "notices\n"},
	} {
		if err := os.WriteFile(filepath.Join(root, file.name), []byte(file.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func testArchiveEntries(t *testing.T, root string, releaseTarget target, binary []byte) []archiveEntry {
	t.Helper()
	entries := []archiveEntry{{name: releaseTarget.binaryName, content: binary, executable: true, binary: true}}
	for _, name := range []string{"README.md", "LICENSE", "THIRD_PARTY_NOTICES"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, archiveEntry{name: name, content: content})
	}
	return entries
}
