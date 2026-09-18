package stele

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const driftedSkill = "---\nname: openspec-propose\nmetadata:\n  generatedBy: \"1.14.0\"\n---\n# Propose\n"

func pinnedSkill(name string) string {
	return "---\nname: " + name + "\nmetadata:\n  author: openspec\n  generatedBy: \"" + OpenSpecVersion + "\"\n---\n"
}

func runCommand(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(arguments, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// withoutOpenSpecOnPath makes the PATH lookup find no openspec executable.
func withoutOpenSpecOnPath(t *testing.T) {
	t.Helper()
	original := lookPathFunction
	t.Cleanup(func() { lookPathFunction = original })
	lookPathFunction = func(string) (string, error) { return "", errors.New("not found") }
}

// @verifies scn.backend.17bc5c526b79.unit
func TestBackendDefaultsToOpenSpec(t *testing.T) {
	for _, adapter := range []string{"", "openspec"} {
		backend, err := resolveBackend(adapter)
		if err != nil || backend.Name() != "openspec" {
			t.Fatalf("resolveBackend(%q) = %v, %v", adapter, backend, err)
		}
	}
	if _, err := (openSpecBackend{}).Validate(fixtureRoot(t), changeScope("example")); err == nil {
		t.Fatal("OpenSpec validation ran without the OpenSpec CLI")
	}
	root := completeFixture(t, false)
	if backend, err := configuredBackend(root); err != nil || backend.Name() != "openspec" {
		t.Fatalf("configuredBackend without a configuration = %v, %v", backend, err)
	}
	withoutOpenSpecOnPath(t)
	code, stdout, stderr := runCommand(t, "verify", "--root", root, "--change", "example")
	if code != 0 || !strings.Contains(stdout, "implementation verification pass") || stderr != "" {
		t.Fatalf("verify without a configuration = %d, %q, %q", code, stdout, stderr)
	}
	writeFixture(t, root, "stele.config.json", `{"schemaVersion":1,"adapter":"","change":"example"}`)
	if code, _, stderr := runCommand(t, "verify", "--root", root); code != 0 {
		t.Fatalf("verify with an empty adapter = %d, %q", code, stderr)
	}
	stubOpenSpecSetup(t, nil, nil)
	fresh := fixtureRoot(t)
	if code, _, _ := runCommand(t, "init", "--root", fresh); code != 0 {
		t.Fatalf("init = %d", code)
	}
	if config, err := readConfig(fresh); err != nil || config.Adapter != "openspec" {
		t.Fatalf("init wrote %#v, %v", config, err)
	}
}

// @verifies scn.backend.e20f52b594b9.unit
func TestUnknownAdapterIsRejected(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "stele.config.json", `{"schemaVersion":1,"adapter":"specmatic","change":"example"}`)
	stubOpenSpecSetup(t, nil, nil)
	for _, command := range [][]string{
		{"init"}, {"ids"}, {"verify"}, {"test"}, {"validate"}, {"approve", "--confirmed-in-chat"}, {"plan", "migrate"},
	} {
		code, stdout, stderr := runCommand(t, append(command, "--root", root)...)
		if code != 2 || stdout != "" ||
			!strings.Contains(stderr, `unsupported adapter "specmatic"`) ||
			!strings.Contains(stderr, "supported adapters: openspec") {
			t.Fatalf("%v = %d, %q, %q", command, code, stdout, stderr)
		}
	}
	writeFixture(t, root, "stele.config.json", "{")
	if code, _, stderr := runCommand(t, "init", "--root", root); code != 2 ||
		!strings.Contains(stderr, "invalid stele.config.json") {
		t.Fatalf("init with invalid configuration = %d, %q", code, stderr)
	}
	if err := os.Remove(filepath.Join(root, "stele.config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "stele.config.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfig(root); err == nil {
		t.Fatal("expected a configuration read error")
	}
}

// memoryBackend serves parsed specifications from memory and keeps its files
// outside openspec/, recording every call.
type memoryBackend struct {
	calls *[]string
}

func (backend memoryBackend) record(call string) {
	*backend.calls = append(*backend.calls, call)
}

func (memoryBackend) Name() string { return "memory" }

func (backend memoryBackend) SpecFiles(root string, _ verificationScope) []string {
	backend.record("SpecFiles")
	return []string{filepath.Join(root, "memory", "draft.md")}
}

func (backend memoryBackend) ParseSpecs(string, verificationScope) (ParsedSpecs, error) {
	backend.record("ParseSpecs")
	source := Source{Path: "memory/demo", Line: 1}
	return ParsedSpecs{
		Requirements: []Requirement{{
			ID:        "req.demo.aaaaaaaaaaaa",
			Title:     "Return value",
			Source:    source,
			Scenarios: []Scenario{{ID: "scn.demo.bbbbbbbbbbbb", Title: "Value is returned", Source: source}},
		}},
		Diagnostics: []Diagnostic{},
		Files:       []string{"memory/demo"},
	}, nil
}

func (backend memoryBackend) DeclaredIdentities(string) (map[string]bool, error) {
	backend.record("DeclaredIdentities")
	return map[string]bool{"req.demo.aaaaaaaaaaaa": true, "scn.demo.bbbbbbbbbbbb": true}, nil
}

func (backend memoryBackend) IdentityFiles(root string) []string {
	backend.record("IdentityFiles")
	return []string{filepath.Join(root, "memory", "draft.md")}
}

func (backend memoryBackend) Capability(verificationScope, string) string {
	backend.record("Capability")
	return "demo"
}

func (backend memoryBackend) CurrentSpecFile(root, capability string) string {
	backend.record("CurrentSpecFile")
	return filepath.Join(root, "memory", "current", capability+".md")
}

func (backend memoryBackend) Changes(string) []string {
	backend.record("Changes")
	return []string{"example"}
}

func (backend memoryBackend) PlanPaths(root string, _ verificationScope) []string {
	backend.record("PlanPaths")
	return []string{filepath.Join(root, "memory", "plan.json")}
}

func (backend memoryBackend) Validate(string, verificationScope) (bool, error) {
	backend.record("Validate")
	return true, nil
}

func (backend memoryBackend) Install(string, openSpecSetup) ([]string, error) {
	backend.record("Install")
	return []string{"Prepared the memory backend."}, nil
}

func (backend memoryBackend) VersionDrift(string, bool) []string {
	backend.record("VersionDrift")
	return nil
}

// @verifies scn.backend.ad74565b613f.unit
func TestCommandsUseSelectedBackend(t *testing.T) {
	calls := make([]string, 0)
	specificationBackends["memory"] = memoryBackend{calls: &calls}
	t.Cleanup(func() { delete(specificationBackends, "memory") })
	originalRun := runExactTest
	t.Cleanup(func() { runExactTest = originalRun })
	runExactTest = func(string, string, string) (bool, bool, error) { return true, true, nil }

	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"schemaVersion":1,"adapter":"memory","change":"example"}`)
	writeFixture(t, root, "memory/draft.md", "## MODIFIED Requirements\n### Requirement: Drafted\n")
	writeFixture(t, root, "memory/current/demo.md",
		"### Requirement: Drafted\nVerification-ID: req.demo.cccccccccccc\n")
	writeFixture(t, root, "memory/plan.json", `{"changeId":"example",`+
		`"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#value"},`+
		`"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/demo.test.mts#returns value"}}`)
	writeFixture(t, root, "src/demo.mts",
		"// @implements "+"req.demo.aaaaaaaaaaaa\nexport function value() { return 1; }\n")
	writeEvidenceTest(t, root, "tests/demo.test.mts", "scn.demo.bbbbbbbbbbbb", "returns value")

	expectations := []struct {
		arguments []string
		calls     []string
		output    string
	}{
		{[]string{"init"}, []string{"Install", "VersionDrift"}, "Prepared the memory backend."},
		{
			[]string{"ids"},
			[]string{"SpecFiles", "IdentityFiles", "Capability", "CurrentSpecFile"},
			"req.demo.cccccccccccc",
		},
		{
			[]string{"verify"},
			[]string{"SpecFiles", "ParseSpecs", "DeclaredIdentities", "PlanPaths", "VersionDrift"},
			"implementation verification pass",
		},
		{[]string{"test"}, []string{"SpecFiles", "ParseSpecs"}, "1/1 passed"},
		{[]string{"validate"}, []string{"Validate", "VersionDrift"}, "deterministic validation passed"},
	}
	for _, expectation := range expectations {
		calls = calls[:0]
		code, stdout, stderr := runCommand(t, append(expectation.arguments, "--root", root)...)
		if code != 0 || !strings.Contains(stdout, expectation.output) {
			t.Fatalf("%v = %d, %q, %q", expectation.arguments, code, stdout, stderr)
		}
		for _, call := range expectation.calls {
			if !strings.Contains(strings.Join(calls, ","), call) {
				t.Fatalf("%v did not call %s: %v", expectation.arguments, call, calls)
			}
		}
	}
	if fileExists(filepath.Join(root, "openspec")) {
		t.Fatal("a command created OpenSpec files")
	}
	if config, _ := readConfig(root); config.Adapter != "memory" {
		t.Fatalf("configuration adapter = %q", config.Adapter)
	}
	if !strings.Contains(readTestFile(t, root, "memory/draft.md"), "Verification-ID: req.demo.cccccccccccc") {
		t.Fatal("ids did not reuse the identity from the backend's current specification")
	}
}

// driftFixture returns a verifiable project with OpenSpec skills from another
// version, a linked Claude skills directory, and a stale schema fork.
func driftFixture(t *testing.T) string {
	t.Helper()
	root := completeFixture(t, false)
	writeFixture(t, root, ".agents/skills/openspec-apply-change/SKILL.md", pinnedSkill("openspec-apply-change"))
	writeFixture(t, root, ".agents/skills/openspec-propose/SKILL.md", driftedSkill)
	writeFixture(t, root, ".agents/skills/stele-plan/SKILL.md", "---\nname: stele-plan\n---\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, ".claude", "skills")
	if err := os.Symlink(filepath.Join("..", ".agents", "skills"), link); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, ".cursor/skills/openspec-explore/SKILL.md", "---\nname: openspec-explore\n---\n")
	if err := os.MkdirAll(filepath.Join(root, ".cursor", "skills", "openspec-broken", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, ".stale", "skills")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/schemas/stele/schema.yaml",
		"# Stele workflow schema, forked from OpenSpec 1.12.0 spec-driven by stele 0.1.0.\nname: stele\n")
	return root
}

// @verifies scn.backend.c077e67d1e58.unit
func TestDriftWarnsForOtherSkillVersions(t *testing.T) {
	root := driftFixture(t)
	withoutOpenSpecOnPath(t)
	code, stdout, stderr := runCommand(t, "verify", "--root", root, "--change", "example")
	if code != 0 || !strings.Contains(stdout, "verification pass") {
		t.Fatalf("verify = %d, %q, %q", code, stdout, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected one skills and one schema warning, got:\n%s", stderr)
	}
	for _, want := range []string{
		"stele: warning: OpenSpec skills in .agents/skills were generated by OpenSpec 1.14.0, " +
			"but Stele pins OpenSpec " + OpenSpecVersion + ". Run `npx openspec update`",
		"stele: warning: The stele schema was forked from OpenSpec 1.12.0, but Stele pins OpenSpec " +
			OpenSpecVersion + ". Run `stele init --refresh-schema`",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr)
		}
	}

	stubOpenSpecSetup(t, nil, nil)
	code, _, stderr = runCommand(t, "init", "--root", root)
	if code != 0 || strings.Count(stderr, "stele: warning:") != 2 {
		t.Fatalf("init = %d, %q", code, stderr)
	}
}

// @verifies scn.backend.124f95bf55a3.integration
func TestInitWarnsForOtherOpenSpecOnPath(t *testing.T) {
	stubOpenSpecSetup(t, nil, nil)
	bin := filepath.Join(fixtureRoot(t), "bin")
	writeFixture(t, bin, "openspec",
		"#!/bin/sh\necho \"${STELE_FAKE_OPENSPEC_VERSION}\"\nexit \"${STELE_FAKE_EXIT:-0}\"\n")
	if err := os.Chmod(filepath.Join(bin, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("STELE_FAKE_OPENSPEC_VERSION", "1.14.0")

	root := fixtureRoot(t)
	code, _, stderr := runCommand(t, "init", "--root", root)
	want := "stele: warning: The openspec on PATH (" + filepath.Join(bin, "openspec") + ") is version 1.14.0, " +
		"but Stele uses its bundled OpenSpec " + OpenSpecVersion + "."
	if code != 0 || !strings.Contains(stderr, want) {
		t.Fatalf("init = %d, %q", code, stderr)
	}
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md",
		"### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n")
	_, _, stderr = runCommand(t, "verify", "--root", root, "--change", "example")
	if strings.Contains(stderr, "PATH") {
		t.Fatalf("verify looked up openspec on PATH: %q", stderr)
	}

	for name, environment := range map[string][2]string{
		"same version":   {OpenSpecVersion, "0"},
		"failing binary": {"1.14.0", "3"},
	} {
		t.Setenv("STELE_FAKE_OPENSPEC_VERSION", environment[0])
		t.Setenv("STELE_FAKE_EXIT", environment[1])
		if findings := pathOpenSpecDrift(root); len(findings) != 0 {
			t.Fatalf("%s: unexpected findings %v", name, findings)
		}
	}

	// The bundled CLI on PATH is never reported.
	useRepositoryOpenSpec(t)
	bundled, err := openSpecCLI(root)
	if err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(fixtureRoot(t), "openspec")
	if err := os.Symlink(bundled, linked); err != nil {
		t.Fatal(err)
	}
	original := lookPathFunction
	t.Cleanup(func() { lookPathFunction = original })
	lookPathFunction = func(string) (string, error) { return linked, nil }
	if findings := pathOpenSpecDrift(root); len(findings) != 0 {
		t.Fatalf("the bundled CLI was reported: %v", findings)
	}
	lookPathFunction = func(string) (string, error) { return filepath.Join(root, "vanished"), nil }
	if findings := pathOpenSpecDrift(root); len(findings) != 0 {
		t.Fatalf("a vanished executable was reported: %v", findings)
	}
}

// @verifies scn.backend.08f190c98ed9.unit
func TestStrictVersionsFailOnDrift(t *testing.T) {
	root := driftFixture(t)
	withoutOpenSpecOnPath(t)
	stubOpenSpecSetup(t, nil, nil)
	originalValidate, originalTests := validateProjectOpenSpec, runProjectScenarios
	t.Cleanup(func() { validateProjectOpenSpec, runProjectScenarios = originalValidate, originalTests })
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	runProjectScenarios = func(testRequest) (testRun, error) {
		return testRun{evidence: Evidence{Outcome: "passed"}}, nil
	}

	if code, _, stderr := runCommand(t, "verify", "--root", root, "--change", "example"); code != 0 ||
		!strings.Contains(stderr, "stele: warning:") {
		t.Fatalf("verify without the flag = %d, %q", code, stderr)
	}
	for _, command := range [][]string{
		{"verify", "--change", "example"},
		{"validate", "--change", "example"},
		{"init"},
	} {
		code, _, stderr := runCommand(t, append(command, "--root", root, "--strict-versions")...)
		if code != 1 || !strings.Contains(stderr, "stele: error: OpenSpec skills in .agents/skills") ||
			strings.Contains(stderr, "warning") {
			t.Fatalf("%v --strict-versions = %d, %q", command, code, stderr)
		}
	}
	if code, _, _ := runCommand(t, "test", "--root", root, "--change", "example", "--strict-versions"); code != 2 {
		t.Fatalf("test accepted --strict-versions: %d", code)
	}

	writeFixture(t, root, "src/demo.mts", "export function value() { return 1; }\n")
	if code, _, _ := runCommand(t, "verify", "--root", root, "--change", "example", "--strict-versions"); code != 1 {
		t.Fatalf("failing strict verify = %d", code)
	}
	clean := completeFixture(t, false)
	code, _, stderr := runCommand(t, "verify", "--root", clean, "--change", "example", "--strict-versions")
	if code != 0 || stderr != "" {
		t.Fatalf("strict verify without drift = %d, %q", code, stderr)
	}
}

// @verifies scn.backend.0ee7b914816a.unit
func TestDriftSilentWhenVersionsMatch(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, ".agents/skills/openspec-propose/SKILL.md", pinnedSkill("openspec-propose"))
	writeFixture(t, root, ".claude/skills/openspec-propose/SKILL.md", pinnedSkill("openspec-propose"))
	writeFixture(t, root, "openspec/schemas/stele/schema.yaml",
		"# Stele workflow schema, forked from OpenSpec "+OpenSpecVersion+" spec-driven by stele "+Version+".\n")
	t.Setenv("PATH", fixtureRoot(t))
	stubOpenSpecSetup(t, nil, nil)
	for _, command := range [][]string{{"verify", "--change", "example"}, {"init"}} {
		code, _, stderr := runCommand(t, append(command, "--root", root)...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v = %d, %q", command, code, stderr)
		}
	}
	if findings := openSpecVersionDrift(root, true); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
	if findings := openSpecVersionDrift(fixtureRoot(t), true); len(findings) != 0 {
		t.Fatalf("an empty project reported drift: %v", findings)
	}
}
