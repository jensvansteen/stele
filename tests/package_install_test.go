//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
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

	openspecExecutable := packageExecutable(consumerRoot, "openspec")
	requireFile(t, openspecExecutable)
	requireCommandSuccess(t, "initialize OpenSpec workspace", runCommand(
		consumerRoot,
		os.Environ(),
		openspecExecutable,
		"init",
		".",
		"--tools",
		"none",
		"--no-animation",
	))
	requireCommandSuccess(t, "create OpenSpec change", runCommand(
		consumerRoot,
		os.Environ(),
		openspecExecutable,
		"new",
		"change",
		"example",
	))

	steleExecutable := packageExecutable(consumerRoot, "stele")
	initResult := runCommand(
		consumerRoot,
		os.Environ(),
		steleExecutable,
		"init",
		"--change",
		"example",
	)
	requireCommandSuccess(t, "initialize consumer project", initResult)

	var config struct {
		Change string `json:"change"`
	}
	readJSONFile(t, filepath.Join(consumerRoot, "stele.config.json"), &config)
	if config.Change != "example" {
		t.Fatalf("expected initialized change %q, got %q", "example", config.Change)
	}
	requireFile(t, filepath.Join(consumerRoot, ".agents", "skills", "stele-plan", "SKILL.md"))
	requireFile(t, filepath.Join(consumerRoot, ".agents", "skills", "stele-verify", "SKILL.md"))

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
		filepath.Join(changeRoot, ".openspec.yaml"): "schema: spec-driven\n",
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
