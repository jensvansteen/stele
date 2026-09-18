//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type commandResult struct {
	stdout string
	stderr string
	err    error
}

// @verifies scn.init.cbf5781012fa.e2e
// @verifies scn.validate.10388560c2dc.e2e
// @verifies scn.terminalreport.cd40642c50e6.e2e
// @verifies scn.validate.ddff25a243df.e2e
func TestPackedPackageInitializesAndValidatesSeparateConsumer(t *testing.T) {
	repositoryRoot := testRepositoryRoot(t)
	temporaryRoot, err := os.MkdirTemp("", "stele-package-")
	if err != nil {
		t.Fatalf("create temporary package directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(temporaryRoot); err != nil {
			t.Errorf("remove temporary package directory: %v", err)
		}
	})

	npmEnvironment := setEnvironmentVariable(
		os.Environ(),
		"NPM_CONFIG_CACHE",
		filepath.Join(temporaryRoot, "npm-cache"),
	)
	pack := runCommand(
		repositoryRoot,
		npmEnvironment,
		"npm",
		"pack",
		"--json",
		"--pack-destination",
		temporaryRoot,
	)
	requireCommandSuccess(t, "pack Stele", pack)

	var packedFiles []struct {
		Filename string `json:"filename"`
		Files    []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(pack.stdout), &packedFiles); err != nil {
		t.Fatalf("decode npm pack output %q: %v", pack.stdout, err)
	}
	if len(packedFiles) != 1 || packedFiles[0].Filename == "" {
		t.Fatalf("expected one packed filename, got %#v", packedFiles)
	}
	packagedPaths := make(map[string]bool, len(packedFiles[0].Files))
	for _, file := range packedFiles[0].Files {
		packagedPaths[file.Path] = true
	}
	for _, path := range []string{
		"dist/stele-darwin-arm64",
		"dist/stele-darwin-amd64",
		"dist/stele-linux-arm64",
		"dist/stele-linux-amd64",
		"scripts/select-binary.mjs",
	} {
		if !packagedPaths[path] {
			t.Errorf("packed package is missing %s", path)
		}
	}

	consumerRoot := filepath.Join(temporaryRoot, "consumer")
	mustCreateDirectory(t, consumerRoot)
	mustWriteFile(
		t,
		filepath.Join(consumerRoot, "package.json"),
		"{\"name\":\"consumer\",\"private\":true,\"type\":\"module\"}\n",
	)

	install := runCommand(
		consumerRoot,
		npmEnvironment,
		"npm",
		"install",
		"--no-audit",
		"--no-fund",
		filepath.Join(temporaryRoot, packedFiles[0].Filename),
	)
	requireCommandSuccess(t, "install packed Stele package", install)

	// The consumer has no OpenSpec setup: stele init runs the bundled OpenSpec.
	steleExecutable := packageExecutable(consumerRoot, "stele")
	initResult := runCommand(consumerRoot, os.Environ(), steleExecutable, "init")
	requireCommandSuccess(t, "initialize consumer project", initResult)

	var config struct {
		Change string `json:"change"`
	}
	readJSONFile(t, filepath.Join(consumerRoot, "stele.config.json"), &config)
	if config.Change != "" {
		t.Fatalf("expected no default change, got %q", config.Change)
	}
	for _, skill := range []string{
		"stele-propose",
		"stele-apply",
		"stele-archive",
		"stele-plan",
		"stele-verify",
		"openspec-propose",
	} {
		requireFile(t, filepath.Join(consumerRoot, ".agents", "skills", skill, "SKILL.md"))
	}
	requireFile(t, filepath.Join(consumerRoot, ".claude", "skills", "stele-propose", "SKILL.md"))
	requireFile(t, filepath.Join(consumerRoot, "openspec", "schemas", "stele", "schema.yaml"))

	openspecExecutable := packageExecutable(consumerRoot, "openspec")
	requireFile(t, openspecExecutable)
	requireCommandSuccess(t, "create OpenSpec change", runCommand(
		consumerRoot,
		os.Environ(),
		openspecExecutable,
		"new",
		"change",
		"example",
	))
	status := runCommand(consumerRoot, os.Environ(), openspecExecutable, "status", "--change", "example")
	requireCommandSuccess(t, "show OpenSpec status", status)
	if !strings.Contains(status.stdout, "Schema: stele") || !strings.Contains(status.stdout, "verification") {
		t.Fatalf("new change does not use the stele schema:\n%s", status.stdout)
	}

	writeConsumerFixture(t, consumerRoot)

	validateEnvironment := removeEnvironmentVariable(os.Environ(), "NODE_TEST_CONTEXT")
	validation := runCommand(
		consumerRoot,
		validateEnvironment,
		steleExecutable,
		"validate",
		"--change",
		"example",
		"--json",
	)
	evidencePath := filepath.Join(consumerRoot, "artifacts", "test-results.json")
	evidence, evidenceErr := os.ReadFile(evidencePath)
	if evidenceErr != nil {
		evidence = []byte("no evidence file")
	}
	if validation.err != nil {
		t.Fatalf(
			"validate installed package: %v\nstderr:\n%s\nstdout:\n%s\nevidence:\n%s",
			validation.err,
			validation.stderr,
			validation.stdout,
			evidence,
		)
	}

	var report struct {
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(validation.stdout), &report); err != nil {
		t.Fatalf("decode validation report %q: %v", validation.stdout, err)
	}
	if report.Verdict != "pass" {
		t.Fatalf("expected validation verdict %q, got %q", "pass", report.Verdict)
	}

	requirePackageVersion(t, repositoryRoot, runCommand(consumerRoot, os.Environ(), steleExecutable, "--version"))
	requireCheckGate(t, consumerRoot, validateEnvironment, steleExecutable)
}

// requirePackageVersion checks that the installed binary names the version of
// the package it was packed from.
func requirePackageVersion(t *testing.T, repositoryRoot string, result commandResult) {
	t.Helper()
	requireCommandSuccess(t, "print the installed version", result)
	var manifest struct {
		Version string `json:"version"`
	}
	readJSONFile(t, filepath.Join(repositoryRoot, "package.json"), &manifest)
	if strings.TrimSpace(result.stdout) != manifest.Version || manifest.Version == "" {
		t.Fatalf("installed stele --version = %q, package version %q", result.stdout, manifest.Version)
	}
}

// requireCheckGate runs `stele check --all` as CI does, without a terminal,
// after the change gets a plan that nobody approved.
func requireCheckGate(t *testing.T, consumerRoot string, environment []string, steleExecutable string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(consumerRoot, "openspec", "changes", "example", "linkage-plan.json"),
		`{"schemaVersion":2,"changeId":"example","scenarios":{"scn.example.abcdef012345":{"evidence":[`+
			`{"id":"scn.example.abcdef012345.unit","level":"unit","rationale":"Pure function."}]}}}`)
	check := runCommand(consumerRoot, environment, steleExecutable, "check", "--all")
	var exitError *exec.ExitError
	if !errors.As(check.err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("stele check --all: %v\nstderr:\n%s\nstdout:\n%s", check.err, check.stderr, check.stdout)
	}
	if !strings.Contains(check.stdout, "PLAN_UNAPPROVED") ||
		!strings.HasSuffix(check.stdout, "\n") ||
		!strings.HasPrefix(lastLine(check.stdout), "✗ FAILED  check · all scopes — ") {
		t.Fatalf("stele check --all summary:\n%s", check.stdout)
	}
	if strings.Contains(check.stdout+check.stderr, "\x1b") || !strings.Contains(check.stderr, "test execution:") {
		t.Fatalf("stele check --all progress without a terminal:\n%q", check.stderr)
	}
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	return lines[len(lines)-1]
}

func packageExecutable(consumerRoot, name string) string {
	executable := filepath.Join(consumerRoot, "node_modules", ".bin", name)
	if runtime.GOOS == "windows" {
		executable += ".cmd"
	}
	return executable
}

func testRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("determine integration test source path")
	}
	return filepath.Dir(filepath.Dir(filename))
}

func writeConsumerFixture(t *testing.T, consumerRoot string) {
	t.Helper()
	changeRoot := filepath.Join(consumerRoot, "openspec", "changes", "example")
	mustCreateDirectory(t, filepath.Join(changeRoot, "specs", "example"))
	mustCreateDirectory(t, filepath.Join(consumerRoot, "src"))
	mustCreateDirectory(t, filepath.Join(consumerRoot, "tests"))

	files := map[string]string{
		filepath.Join(changeRoot, ".openspec.yaml"): "schema: stele\n",
		filepath.Join(changeRoot, "proposal.md"): "# Proposal: Package boundary\n\n" +
			"## Why\n\nProve the installed package.\n\n" +
			"## What Changes\n\n- Add a deterministic example.\n\n" +
			"## Capabilities\n\n- `example`: Return a value.\n\n" +
			"## Impact\n\nNo production impact.\n",
		filepath.Join(changeRoot, "design.md"): "# Design\n\nUse one pure function and one exact test.\n",
		filepath.Join(changeRoot, "tasks.md"):  "# Tasks\n\n- [x] Implement the example.\n",
		filepath.Join(changeRoot, "specs", "example", "spec.md"): strings.Join([]string{
			"## ADDED Requirements",
			"",
			"### Requirement: Return a value",
			"Verification-ID: req.example.0123456789ab",
			"",
			"The system SHALL return the supplied value.",
			"",
			"#### Scenario: Value supplied",
			"Verification-ID: scn.example.abcdef012345",
			"",
			"- **WHEN** a value is supplied",
			"- **THEN** the same value is returned",
			"",
		}, "\n"),
		filepath.Join(consumerRoot, "src", "example.mts"): "// @implements req.example.0123456789ab\n" +
			"export function value(input: string): string { return input; }\n",
		filepath.Join(consumerRoot, "tests", "example.test.mts"): strings.Join([]string{
			`import assert from "node:assert/strict";`,
			`import test from "node:test";`,
			`import { value } from "../src/example.mts";`,
			"// @verifies scn.example.abcdef012345",
			`test("returns the supplied value", () => assert.equal(value("proof"), "proof"));`,
			"",
		}, "\n"),
	}

	for filename, content := range files {
		mustWriteFile(t, filename, content)
	}
}

func runCommand(directory string, environment []string, name string, arguments ...string) commandResult {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = environment
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func requireCommandSuccess(t *testing.T, description string, result commandResult) {
	t.Helper()
	if result.err == nil {
		return
	}
	t.Fatalf(
		"%s: %v\nstderr:\n%s\nstdout:\n%s",
		description,
		result.err,
		result.stderr,
		result.stdout,
	)
}

func mustCreateDirectory(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create directory %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON file %s: %v", path, err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("decode JSON file %s: %v", path, err)
	}
}

func requireFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat expected file %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file at %s, found directory", path)
	}
}

func setEnvironmentVariable(environment []string, name string, value string) []string {
	return append(removeEnvironmentVariable(environment, name), fmt.Sprintf("%s=%s", name, value))
}

func removeEnvironmentVariable(environment []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment))
	for _, variable := range environment {
		if !strings.HasPrefix(variable, prefix) {
			filtered = append(filtered, variable)
		}
	}
	return filtered
}
