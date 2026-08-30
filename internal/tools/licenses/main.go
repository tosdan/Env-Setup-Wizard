// Command licenses generates and verifies THIRD_PARTY_NOTICES for the release
// targets supported by Env Setup Wizard.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	modulePath = "github.com/tosdan/env-setup-wizard"
	noticeName = "THIRD_PARTY_NOTICES"
)

var releaseTargets = []target{
	{goos: "windows", goarch: "amd64"},
	{goos: "linux", goarch: "amd64"},
	{goos: "linux", goarch: "arm64"},
	{goos: "darwin", goarch: "amd64"},
	{goos: "darwin", goarch: "arm64"},
}

type target struct {
	goos   string
	goarch string
}

func (t target) String() string {
	return t.goos + "/" + t.goarch
}

type listedPackage struct {
	Standard bool          `json:"Standard"`
	Module   *listedModule `json:"Module"`
}

type listedModule struct {
	Path    string        `json:"Path"`
	Version string        `json:"Version"`
	Dir     string        `json:"Dir"`
	Replace *listedModule `json:"Replace"`
}

type dependency struct {
	path        string
	version     string
	replacement string
	dir         string
	targets     map[string]bool
	licenses    []legalFile
	notices     []legalFile
}

type legalFile struct {
	name    string
	text    string
	license string
}

func main() {
	check := flag.Bool("check", false, "verify that THIRD_PARTY_NOTICES is current")
	write := flag.Bool("write", false, "regenerate THIRD_PARTY_NOTICES")
	flag.Parse()

	if *check == *write || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/tools/licenses (-check|-write)")
		os.Exit(2)
	}

	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}
	dependencies, err := collectDependencies(root)
	if err != nil {
		fatal(err)
	}
	content, err := renderNotice(dependencies)
	if err != nil {
		fatal(err)
	}
	noticePath := filepath.Join(root, noticeName)

	if *write {
		if err := os.WriteFile(noticePath, content, 0o644); err != nil {
			fatal(fmt.Errorf("write %s: %w", noticeName, err))
		}
		fmt.Printf("wrote %s for %d modules\n", noticeName, len(dependencies))
		return
	}

	actual, err := os.ReadFile(noticePath)
	if err != nil {
		fatal(fmt.Errorf("read %s: %w (run with -write to generate it)", noticeName, err))
	}
	if !bytes.Equal(normalizeNewlines(actual), content) {
		fatal(fmt.Errorf("%s is out of date; run with -write and review the result", noticeName))
	}
	fmt.Printf("%s is current; %d modules use approved licenses\n", noticeName, len(dependencies))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "license check:", err)
	os.Exit(1)
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect go.mod: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find go.mod in this directory or its parents")
		}
		dir = parent
	}
}

func collectDependencies(root string) ([]*dependency, error) {
	byID := make(map[string]*dependency)
	for _, target := range releaseTargets {
		modules, err := listModules(root, target)
		if err != nil {
			return nil, err
		}
		for _, module := range modules {
			if module.Path == modulePath {
				continue
			}

			effective := module
			replacement := ""
			if module.Replace != nil {
				effective = module.Replace
				replacement = moduleID(module.Replace)
			}
			if effective.Dir == "" {
				return nil, fmt.Errorf("module %s has no source directory", moduleID(module))
			}

			id := moduleID(module) + "=>" + replacement
			item, ok := byID[id]
			if !ok {
				item = &dependency{
					path:        module.Path,
					version:     module.Version,
					replacement: replacement,
					dir:         effective.Dir,
					targets:     make(map[string]bool),
				}
				byID[id] = item
			} else if filepath.Clean(item.dir) != filepath.Clean(effective.Dir) {
				return nil, fmt.Errorf("module %s resolved to multiple directories", moduleID(module))
			}
			item.targets[target.String()] = true
		}
	}

	dependencies := make([]*dependency, 0, len(byID))
	for _, item := range byID {
		licenses, notices, err := inspectLegalFiles(item.dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.label(), err)
		}
		item.licenses = licenses
		item.notices = notices
		dependencies = append(dependencies, item)
	}
	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].label() < dependencies[j].label()
	})
	return dependencies, nil
}

func listModules(root string, target target) ([]*listedModule, error) {
	command := exec.Command("go", "list", "-buildvcs=false", "-deps", "-json", "./cmd/env-wizard")
	command.Dir = root
	command.Env = withEnvironment(
		os.Environ(),
		"CGO_ENABLED", "0",
		"GOOS", target.goos,
		"GOARCH", target.goarch,
	)
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("list dependencies for %s: %w: %s", target, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("list dependencies for %s: %w", target, err)
	}

	seen := make(map[string]*listedModule)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode dependencies for %s: %w", target, err)
		}
		if pkg.Standard || pkg.Module == nil {
			continue
		}
		id := moduleID(pkg.Module)
		if pkg.Module.Replace != nil {
			id += "=>" + moduleID(pkg.Module.Replace)
		}
		seen[id] = pkg.Module
	}

	modules := make([]*listedModule, 0, len(seen))
	for _, module := range seen {
		modules = append(modules, module)
	}
	return modules, nil
}

func withEnvironment(current []string, pairs ...string) []string {
	replacements := make(map[string]string)
	for i := 0; i < len(pairs); i += 2 {
		replacements[strings.ToUpper(pairs[i])] = pairs[i] + "=" + pairs[i+1]
	}

	result := make([]string, 0, len(current)+len(replacements))
	for _, item := range current {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, replaced := replacements[strings.ToUpper(key)]; replaced {
				continue
			}
		}
		result = append(result, item)
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, replacements[key])
	}
	return result
}

func moduleID(module *listedModule) string {
	if module == nil {
		return ""
	}
	if module.Version == "" {
		return module.Path
	}
	return module.Path + " " + module.Version
}

func (dependency *dependency) label() string {
	label := dependency.path
	if dependency.version != "" {
		label += " " + dependency.version
	}
	if dependency.replacement != "" {
		label += " => " + dependency.replacement
	}
	return label
}

func inspectLegalFiles(dir string) ([]legalFile, []legalFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("read module directory: %w", err)
	}

	var licenses []legalFile
	var notices []legalFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		isLicense := hasLegalPrefix(name, "LICENSE", "LICENCE", "COPYING")
		isNotice := hasLegalPrefix(name, "NOTICE")
		if !isLicense && !isNotice {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", name, err)
		}
		file := legalFile{name: name, text: string(normalizeNewlines(content))}
		if isLicense {
			file.license, err = detectLicense(file.text)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", name, err)
			}
			licenses = append(licenses, file)
		} else {
			notices = append(notices, file)
		}
	}
	if len(licenses) == 0 {
		return nil, nil, errors.New("no top-level license file found")
	}
	sort.Slice(licenses, func(i, j int) bool { return licenses[i].name < licenses[j].name })
	sort.Slice(notices, func(i, j int) bool { return notices[i].name < notices[j].name })
	return licenses, notices, nil
}

func hasLegalPrefix(name string, prefixes ...string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range prefixes {
		if upper == prefix || strings.HasPrefix(upper, prefix+".") ||
			strings.HasPrefix(upper, prefix+"-") || strings.HasPrefix(upper, prefix+"_") {
			return true
		}
	}
	return false
}

func detectLicense(text string) (string, error) {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "apache license") && strings.Contains(lower, "version 2.0"):
		return "Apache-2.0", nil
	case strings.Contains(lower, "permission is hereby granted, free of charge"):
		return "MIT", nil
	case strings.Contains(lower, "permission to use, copy, modify, and/or distribute"):
		return "ISC", nil
	case strings.Contains(lower, "redistribution and use in source and binary forms") &&
		strings.Contains(lower, "neither the name"):
		return "BSD-3-Clause", nil
	case strings.Contains(lower, "redistribution and use in source and binary forms"):
		return "BSD-2-Clause", nil
	default:
		return "", errors.New("unrecognized or unapproved license")
	}
}

func renderNotice(dependencies []*dependency) ([]byte, error) {
	var output strings.Builder
	output.WriteString("THIRD-PARTY NOTICES\n")
	output.WriteString("===================\n\n")
	output.WriteString("This distribution includes packages from the modules listed below. ")
	output.WriteString("The inventory is the union of packages linked into ./cmd/env-wizard for ")
	output.WriteString("the Windows, Linux, and macOS release targets.\n\n")
	output.WriteString("Generated with: go run ./internal/tools/licenses -write\n")
	output.WriteString("Verified with:  go run ./internal/tools/licenses -check\n\n")
	output.WriteString("DEPENDENCY SUMMARY\n")
	output.WriteString("------------------\n")
	for _, dependency := range dependencies {
		licenseNames := make(map[string]bool)
		for _, file := range dependency.licenses {
			licenseNames[file.license] = true
		}
		licenses := sortedKeys(licenseNames)
		fmt.Fprintf(&output, "- %s [%s]\n", dependency.label(), strings.Join(licenses, ", "))
	}

	for _, dependency := range dependencies {
		output.WriteString("\n======================================================================\n")
		output.WriteString(dependency.label())
		output.WriteByte('\n')
		fmt.Fprintf(&output, "Release targets: %s\n", strings.Join(sortedKeys(dependency.targets), ", "))
		for _, file := range dependency.licenses {
			fmt.Fprintf(&output, "\n--- %s (%s) ---\n", file.name, file.license)
			writeLegalText(&output, file.text)
		}
		for _, file := range dependency.notices {
			fmt.Fprintf(&output, "\n--- %s ---\n", file.name)
			writeLegalText(&output, file.text)
		}
	}
	return []byte(output.String()), nil
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeLegalText(output *strings.Builder, text string) {
	output.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		output.WriteByte('\n')
	}
}

func normalizeNewlines(content []byte) []byte {
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))
}
