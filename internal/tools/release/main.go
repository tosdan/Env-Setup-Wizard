// Command release builds and verifies the distributable Env Setup Wizard artifacts.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	checksumName  = "SHA256SUMS"
	defaultOutput = "dist"
)

var (
	versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	commitPattern  = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	fixedTime      = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	releaseTargets = []target{
		{name: "windows-amd64", goos: "windows", goarch: "amd64", binaryName: "env-wizard.exe", archiveExtension: ".zip"},
		{name: "linux-amd64", goos: "linux", goarch: "amd64", binaryName: "env-wizard", archiveExtension: ".tar.gz"},
		{name: "linux-arm64", goos: "linux", goarch: "arm64", binaryName: "env-wizard", archiveExtension: ".tar.gz"},
		{name: "darwin-amd64", goos: "darwin", goarch: "amd64", binaryName: "env-wizard", archiveExtension: ".tar.gz"},
		{name: "darwin-arm64", goos: "darwin", goarch: "arm64", binaryName: "env-wizard", archiveExtension: ".tar.gz"},
	}
)

type target struct {
	name             string
	goos             string
	goarch           string
	binaryName       string
	archiveExtension string
}

type buildOptions struct {
	version   string
	commit    string
	target    string
	outputDir string
}

type finalizeOptions struct {
	version   string
	inputDir  string
	outputDir string
}

type archiveEntry struct {
	name       string
	content    []byte
	executable bool
	binary     bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("expected build or finalize subcommand")
	}

	root, err := findModuleRoot()
	if err != nil {
		return err
	}

	switch args[0] {
	case "build":
		options, err := parseBuildOptions(args[1:], stderr)
		if err != nil {
			return err
		}
		path, err := buildRelease(root, options, stdout, stderr)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created %s\n", path)
		return nil
	case "finalize":
		options, err := parseFinalizeOptions(args[1:], stderr)
		if err != nil {
			return err
		}
		path, err := finalizeRelease(root, options)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created and verified %s\n", path)
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q; expected build or finalize", args[0])
	}
}

func parseBuildOptions(args []string, stderr io.Writer) (buildOptions, error) {
	flags := flag.NewFlagSet("release build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := buildOptions{}
	flags.StringVar(&options.version, "version", os.Getenv("RELEASE_VERSION"), "semantic version prefixed with v")
	flags.StringVar(&options.commit, "commit", os.Getenv("RELEASE_COMMIT"), "Git commit (7 to 40 hexadecimal characters)")
	flags.StringVar(&options.target, "target", os.Getenv("RELEASE_TARGET"), "native release target")
	flags.StringVar(&options.outputDir, "output", defaultOutput, "artifact output directory")
	if err := flags.Parse(args); err != nil {
		return buildOptions{}, err
	}
	if flags.NArg() != 0 {
		return buildOptions{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return options, nil
}

func parseFinalizeOptions(args []string, stderr io.Writer) (finalizeOptions, error) {
	flags := flag.NewFlagSet("release finalize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := finalizeOptions{}
	flags.StringVar(&options.version, "version", os.Getenv("RELEASE_VERSION"), "semantic version prefixed with v")
	flags.StringVar(&options.inputDir, "input", defaultOutput, "directory containing all release archives")
	flags.StringVar(&options.outputDir, "output", defaultOutput, "checksum output directory")
	if err := flags.Parse(args); err != nil {
		return finalizeOptions{}, err
	}
	if flags.NArg() != 0 {
		return finalizeOptions{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return options, nil
}

func buildRelease(root string, options buildOptions, stdout, stderr io.Writer) (string, error) {
	if err := validateVersion(options.version); err != nil {
		return "", err
	}
	if options.outputDir == "" {
		return "", errors.New("artifact output directory must not be empty")
	}
	shortCommit, err := normalizeCommit(options.commit)
	if err != nil {
		return "", err
	}
	releaseTarget, err := targetByName(options.target)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != releaseTarget.goos || runtime.GOARCH != releaseTarget.goarch {
		return "", fmt.Errorf("target %s requires a native %s/%s runner; current runner is %s/%s", releaseTarget.name, releaseTarget.goos, releaseTarget.goarch, runtime.GOOS, runtime.GOARCH)
	}

	outputDir := resolveFromRoot(root, options.outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory %s: %w", outputDir, err)
	}
	archivePath := filepath.Join(outputDir, archiveName(options.version, releaseTarget))
	if _, err := os.Lstat(archivePath); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing artifact %s", archivePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect artifact path %s: %w", archivePath, err)
	}

	temporaryDir, err := os.MkdirTemp("", "env-wizard-release-")
	if err != nil {
		return "", fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(temporaryDir)

	binaryPath := filepath.Join(temporaryDir, releaseTarget.binaryName)
	command := exec.Command(
		"go", "build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags", linkerFlags(options.version, shortCommit),
		"-o", binaryPath,
		"./cmd/env-wizard",
	)
	command.Dir = root
	command.Env = withEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        releaseTarget.goos,
		"GOARCH":      releaseTarget.goarch,
	})
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("build %s: %w", releaseTarget.name, err)
	}

	expectedVersion := fmt.Sprintf("env-wizard %s (commit %s)", options.version, shortCommit)
	if err := smokeTest(binaryPath, root, expectedVersion); err != nil {
		return "", err
	}
	if err := createArchive(root, binaryPath, archivePath, releaseTarget); err != nil {
		return "", err
	}
	if err := verifyArchive(root, archivePath, releaseTarget); err != nil {
		_ = os.Remove(archivePath)
		return "", fmt.Errorf("verify newly created archive: %w", err)
	}
	return archivePath, nil
}

func finalizeRelease(root string, options finalizeOptions) (string, error) {
	if err := validateVersion(options.version); err != nil {
		return "", err
	}
	if options.inputDir == "" || options.outputDir == "" {
		return "", errors.New("archive input and checksum output directories must not be empty")
	}
	inputDir := resolveFromRoot(root, options.inputDir)
	outputDir := resolveFromRoot(root, options.outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create checksum output directory %s: %w", outputDir, err)
	}

	names := make([]string, 0, len(releaseTargets))
	for _, releaseTarget := range releaseTargets {
		name := archiveName(options.version, releaseTarget)
		path := filepath.Join(inputDir, name)
		if err := verifyArchive(root, path, releaseTarget); err != nil {
			return "", fmt.Errorf("verify %s: %w", name, err)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	checksumPath := filepath.Join(outputDir, checksumName)
	if _, err := os.Lstat(checksumPath); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing checksum file %s", checksumPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect checksum path %s: %w", checksumPath, err)
	}

	var contents strings.Builder
	for _, name := range names {
		digest, err := fileSHA256(filepath.Join(inputDir, name))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&contents, "%s  %s\n", digest, name)
	}
	if err := writeFileAtomically(checksumPath, []byte(contents.String()), 0o644); err != nil {
		return "", err
	}
	if err := verifyChecksums(checksumPath, inputDir, names); err != nil {
		_ = os.Remove(checksumPath)
		return "", fmt.Errorf("verify generated checksums: %w", err)
	}
	return checksumPath, nil
}

func validateVersion(version string) error {
	matches := versionPattern.FindStringSubmatch(version)
	if matches == nil {
		return fmt.Errorf("version %q must be Semantic Versioning prefixed with v and without build metadata", version)
	}
	if matches[4] != "" {
		for _, identifier := range strings.Split(matches[4], ".") {
			if identifier != "0" && identifier[0] == '0' && isDecimal(identifier) {
				return fmt.Errorf("version %q contains a numeric prerelease identifier with a leading zero", version)
			}
		}
	}
	return nil
}

func normalizeCommit(commit string) (string, error) {
	if !commitPattern.MatchString(commit) {
		return "", fmt.Errorf("commit %q must contain 7 to 40 hexadecimal characters", commit)
	}
	return strings.ToLower(commit[:7]), nil
}

func isDecimal(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func targetByName(name string) (target, error) {
	for _, releaseTarget := range releaseTargets {
		if releaseTarget.name == name {
			return releaseTarget, nil
		}
	}
	valid := make([]string, 0, len(releaseTargets))
	for _, releaseTarget := range releaseTargets {
		valid = append(valid, releaseTarget.name)
	}
	return target{}, fmt.Errorf("unknown target %q; expected one of %s", name, strings.Join(valid, ", "))
}

func archiveName(version string, releaseTarget target) string {
	return fmt.Sprintf("env-wizard_%s_%s_%s%s", strings.TrimPrefix(version, "v"), releaseTarget.goos, releaseTarget.goarch, releaseTarget.archiveExtension)
}

func linkerFlags(version, commit string) string {
	return strings.Join([]string{
		"-s",
		"-w",
		"-buildid=",
		"-X", "main.version=" + version,
		"-X", "main.commit=" + commit,
	}, " ")
}

func smokeTest(binaryPath, root, expected string) error {
	command := exec.Command(binaryPath, "--version")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run native --version smoke test: %w: %s", err, strings.TrimSpace(string(output)))
	}
	actual := strings.TrimSuffix(string(output), "\n")
	actual = strings.TrimSuffix(actual, "\r")
	if actual != expected {
		return fmt.Errorf("native --version smoke test returned %q, expected %q", actual, expected)
	}
	return nil
}

func createArchive(root, binaryPath, archivePath string, releaseTarget target) error {
	entries, err := loadArchiveEntries(root, binaryPath, releaseTarget)
	if err != nil {
		return err
	}
	data, err := archiveBytes(entries, releaseTarget)
	if err != nil {
		return err
	}
	if err := writeFileAtomically(archivePath, data, 0o644); err != nil {
		return fmt.Errorf("write archive %s: %w", archivePath, err)
	}
	return nil
}

func loadArchiveEntries(root, binaryPath string, releaseTarget target) ([]archiveEntry, error) {
	entries := []archiveEntry{{name: releaseTarget.binaryName, executable: true, binary: true}}
	paths := []string{"README.md", "LICENSE", "THIRD_PARTY_NOTICES"}
	for _, name := range paths {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("read archive input %s: %w", name, err)
		}
		entries = append(entries, archiveEntry{name: name, content: content})
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("read release binary: %w", err)
	}
	if len(binary) == 0 {
		return nil, errors.New("release binary is empty")
	}
	entries[0].content = binary
	return entries, nil
}

func archiveBytes(entries []archiveEntry, releaseTarget target) ([]byte, error) {
	var buffer bytes.Buffer
	if releaseTarget.archiveExtension == ".zip" {
		writer := zip.NewWriter(&buffer)
		for _, entry := range entries {
			header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
			header.SetModTime(fixedTime)
			if entry.executable {
				header.SetMode(0o755)
			} else {
				header.SetMode(0o644)
			}
			file, err := writer.CreateHeader(header)
			if err != nil {
				_ = writer.Close()
				return nil, fmt.Errorf("create zip entry %s: %w", entry.name, err)
			}
			if _, err := file.Write(entry.content); err != nil {
				_ = writer.Close()
				return nil, fmt.Errorf("write zip entry %s: %w", entry.name, err)
			}
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close zip archive: %w", err)
		}
		return buffer.Bytes(), nil
	}

	gzipWriter, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}
	gzipWriter.Header.ModTime = fixedTime
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		mode := int64(0o644)
		if entry.executable {
			mode = 0o755
		}
		header := &tar.Header{
			Name:     entry.name,
			Mode:     mode,
			Size:     int64(len(entry.content)),
			ModTime:  fixedTime,
			Typeflag: tar.TypeReg,
			Format:   tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("create tar entry %s: %w", entry.name, err)
		}
		if _, err := tarWriter.Write(entry.content); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			return nil, fmt.Errorf("write tar entry %s: %w", entry.name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, fmt.Errorf("close tar archive: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func verifyArchive(root, archivePath string, releaseTarget target) error {
	expected, err := expectedEntries(root, releaseTarget)
	if err != nil {
		return err
	}
	if releaseTarget.archiveExtension == ".zip" {
		return verifyZip(archivePath, expected)
	}
	return verifyTarGzip(archivePath, expected)
}

func expectedEntries(root string, releaseTarget target) (map[string]archiveEntry, error) {
	expected := map[string]archiveEntry{
		releaseTarget.binaryName: {name: releaseTarget.binaryName, executable: true, binary: true},
	}
	for _, name := range []string{"README.md", "LICENSE", "THIRD_PARTY_NOTICES"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("read expected archive input %s: %w", name, err)
		}
		expected[name] = archiveEntry{name: name, content: content}
	}
	return expected, nil
}

func verifyZip(path string, expected map[string]archiveEntry) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	seen := make(map[string]bool, len(reader.File))
	for _, file := range reader.File {
		entry, ok := expected[file.Name]
		if !ok {
			return fmt.Errorf("unexpected zip entry %q", file.Name)
		}
		if seen[file.Name] {
			return fmt.Errorf("duplicate zip entry %q", file.Name)
		}
		if !file.Mode().IsRegular() {
			return fmt.Errorf("zip entry %q is not a regular file", file.Name)
		}
		if entry.executable && file.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("zip binary %q is not executable", file.Name)
		}
		content, err := readZipFile(file)
		if err != nil {
			return err
		}
		if err := verifyEntryContent(entry, content); err != nil {
			return err
		}
		seen[file.Name] = true
	}
	return verifyAllEntriesSeen(expected, seen)
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open zip entry %q: %w", file.Name, err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read zip entry %q: %w", file.Name, err)
	}
	return content, nil
}

func verifyTarGzip(path string, expected map[string]archiveEntry) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	seen := make(map[string]bool, len(expected))
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}
		entry, ok := expected[header.Name]
		if !ok {
			return fmt.Errorf("unexpected tar entry %q", header.Name)
		}
		if seen[header.Name] {
			return fmt.Errorf("duplicate tar entry %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("tar entry %q is not a regular file", header.Name)
		}
		if entry.executable && header.Mode&0o111 == 0 {
			return fmt.Errorf("tar binary %q is not executable", header.Name)
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return fmt.Errorf("read tar entry %q: %w", header.Name, err)
		}
		if err := verifyEntryContent(entry, content); err != nil {
			return err
		}
		seen[header.Name] = true
	}
	return verifyAllEntriesSeen(expected, seen)
}

func verifyEntryContent(entry archiveEntry, actual []byte) error {
	if entry.binary {
		if len(actual) == 0 {
			return fmt.Errorf("binary entry %q is empty", entry.name)
		}
		return nil
	}
	if !bytes.Equal(actual, entry.content) {
		return fmt.Errorf("archive entry %q does not match the repository file", entry.name)
	}
	return nil
}

func verifyAllEntriesSeen(expected map[string]archiveEntry, seen map[string]bool) error {
	if len(seen) != len(expected) {
		missing := make([]string, 0)
		for name := range expected {
			if !seen[name] {
				missing = append(missing, name)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("archive is missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func verifyChecksums(checksumPath, archiveDir string, expectedNames []string) error {
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksum file: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	if len(lines) != len(expectedNames) {
		return fmt.Errorf("checksum file has %d rows, expected %d", len(lines), len(expectedNames))
	}
	for index, name := range expectedNames {
		parts := strings.SplitN(lines[index], "  ", 2)
		if len(parts) != 2 || parts[1] != name {
			return fmt.Errorf("invalid checksum row %d for %s", index+1, name)
		}
		if len(parts[0]) != sha256.Size*2 {
			return fmt.Errorf("invalid SHA-256 digest on row %d", index+1)
		}
		if _, err := hex.DecodeString(parts[0]); err != nil {
			return fmt.Errorf("invalid SHA-256 digest on row %d: %w", index+1, err)
		}
		actual, err := fileSHA256(filepath.Join(archiveDir, name))
		if err != nil {
			return err
		}
		if parts[0] != actual {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s for checksum: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeFileAtomically(path string, content []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".env-wizard-release-")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}

func resolveFromRoot(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}

func findModuleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine current directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect go.mod in %s: %w", directory, err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("could not find go.mod in current directory or its parents")
		}
		directory = parent
	}
}

func withEnvironment(environment []string, replacements map[string]string) []string {
	result := make([]string, 0, len(environment)+len(replacements))
	for _, entry := range environment {
		key, _, found := strings.Cut(entry, "=")
		if found {
			if _, replace := replacements[strings.ToUpper(key)]; replace {
				continue
			}
		}
		result = append(result, entry)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}
