package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// useRepositoryOpenSpec makes the bundled OpenSpec CLI resolve to this
// repository's pinned installation, as it does from an installed package.
func useRepositoryOpenSpec(t *testing.T) {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(repository, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js")) {
		t.Fatalf("bundled OpenSpec CLI is missing in %s; run npm ci", repository)
	}
	original := executablePath
	t.Cleanup(func() { executablePath = original })
	executablePath = func() (string, error) { return filepath.Join(repository, "dist", "stele"), nil }
}

func openSpec(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	output, err := runBundledOpenSpec(root, arguments...)
	if err != nil {
		t.Fatalf("openspec %v: %v\n%s", arguments, err, output)
	}
	return output
}

func openSpecJSON(t *testing.T, root string, into any, arguments ...string) {
	t.Helper()
	output := openSpec(t, root, append(arguments, "--json")...)
	// OpenSpec may print a progress line before the JSON document.
	_, document, _ := strings.Cut(output, "{")
	if err := json.Unmarshal([]byte("{"+document), into); err != nil {
		t.Fatalf("openspec %v returned invalid JSON: %v\n%s", arguments, err, output)
	}
}

func steleInit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"init", "--root", root}, arguments...), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stele init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// openSpecOnlyProject returns a project initialized by OpenSpec alone.
func openSpecOnlyProject(t *testing.T) string {
	t.Helper()
	useRepositoryOpenSpec(t)
	root := fixtureRoot(t)
	openSpec(t, root, "init", "--tools", "agents", "--no-animation", ".")
	return root
}

func writeReadyChange(t *testing.T, root, change string, withPlan bool) {
	t.Helper()
	openSpec(t, root, "new", "change", change)
	directory := "openspec/changes/" + change + "/"
	writeFixture(t, root, directory+"proposal.md", "# Proposal\n")
	writeFixture(t, root, directory+"design.md", "# Design\n")
	writeFixture(t, root, directory+"specs/demo/spec.md", "## ADDED Requirements\n")
	writeFixture(t, root, directory+"tasks.md", "- [ ] 1.1 Implement it\n")
	if withPlan {
		writeFixture(t, root, directory+"linkage-plan.json", "{}\n")
	}
}

type openSpecStatus struct {
	SchemaName    string   `json:"schemaName"`
	ApplyRequires []string `json:"applyRequires"`
	Artifacts     []struct {
		ID string `json:"id"`
	} `json:"artifacts"`
}

// @verifies scn.workflowschema.b4a054dbe512.integration
func TestInitInstallsStelePipelineSchema(t *testing.T) {
	root := openSpecOnlyProject(t)
	if !strings.Contains(readTestFile(t, root, "openspec/config.yaml"), "schema: spec-driven") {
		t.Fatal("OpenSpec did not select spec-driven")
	}
	output := steleInit(t, root)
	if !strings.Contains(output, "Installed the stele workflow schema") {
		t.Fatalf("init did not report the schema: %q", output)
	}
	schema := readTestFile(t, root, "openspec/schemas/stele/schema.yaml")
	if !strings.Contains(schema, "forked from OpenSpec "+OpenSpecVersion+" spec-driven by stele "+Version) {
		t.Fatalf("schema does not record its versions:\n%s", schema[:200])
	}
	if !strings.HasPrefix(readTestFile(t, root, "openspec/config.yaml"), "schema: stele\n") {
		t.Fatal("stele is not the default schema")
	}

	openSpec(t, root, "new", "change", "demo")
	var status openSpecStatus
	openSpecJSON(t, root, &status, "status", "--change", "demo")
	order := make([]string, 0, len(status.Artifacts))
	for _, artifact := range status.Artifacts {
		order = append(order, artifact.ID)
	}
	if status.SchemaName != "stele" ||
		!slices.Equal(order, []string{"proposal", "specs", "design", "verification", "tasks"}) {
		t.Fatalf("unexpected status: %#v", status)
	}
	if !slices.Contains(status.ApplyRequires, "verification") {
		t.Fatalf("apply does not require the plan: %#v", status.ApplyRequires)
	}
	if text := openSpec(t, root, "status", "--change", "demo"); !strings.Contains(text, "Schema: stele") {
		t.Fatalf("status does not show the stele schema: %s", text)
	}
}

// @verifies scn.workflowschema.270587f7a4c3.integration
func TestVerificationInstructionsReachOpenSpec(t *testing.T) {
	root := openSpecOnlyProject(t)
	steleInit(t, root)
	openSpec(t, root, "new", "change", "demo")
	var instructions struct {
		OutputPath  string   `json:"outputPath"`
		Instruction string   `json:"instruction"`
		Rules       []string `json:"rules"`
		Template    string   `json:"template"`
	}
	openSpecJSON(t, root, &instructions, "instructions", "verification", "--change", "demo")
	if instructions.OutputPath != "linkage-plan.json" {
		t.Fatalf("verification generates %q", instructions.OutputPath)
	}
	assertOrdered(t, instructions.Instruction,
		"`stele ids --change <change>`",
		"level",
		"rationale",
		"advisory placement",
		"stele-plan skill",
		"unapproved",
		"`stele verify --stage proposal --change <change>`",
		`"Plan ready. Ask me to apply the change; I'll show the levels to confirm first."`,
	)
	if len(instructions.Rules) == 0 || !strings.Contains(instructions.Rules[0], "stele-plan") {
		t.Fatalf("verification rules are missing: %#v", instructions.Rules)
	}
	if !strings.Contains(instructions.Template, `"schemaVersion": 2`) {
		t.Fatalf("plan template is missing: %q", instructions.Template)
	}
}

type applyInstructions struct {
	State            string         `json:"state"`
	MissingArtifacts []string       `json:"missingArtifacts"`
	ContextFiles     map[string]any `json:"contextFiles"`
	Instruction      string         `json:"instruction"`
}

// @verifies scn.workflowschema.a444787573ca.integration
func TestApplyWaitsForVerificationPlan(t *testing.T) {
	root := openSpecOnlyProject(t)
	steleInit(t, root)
	writeReadyChange(t, root, "demo", false)

	var blocked applyInstructions
	openSpecJSON(t, root, &blocked, "instructions", "apply", "--change", "demo")
	if blocked.State != "blocked" || !slices.Equal(blocked.MissingArtifacts, []string{"verification"}) {
		t.Fatalf("apply is not blocked by the plan: %#v", blocked)
	}

	writeFixture(t, root, "openspec/changes/demo/linkage-plan.json", "{}\n")
	var ready applyInstructions
	openSpecJSON(t, root, &ready, "instructions", "apply", "--change", "demo")
	if ready.State == "blocked" {
		t.Fatalf("apply is still blocked: %#v", ready)
	}
	if _, listed := ready.ContextFiles["verification"]; !listed {
		t.Fatalf("the plan is not among the files to read: %#v", ready.ContextFiles)
	}
	assertOrdered(t, ready.Instruction,
		"Stele apply steps:",
		"confirm the verification levels",
		"stele-plan skill",
		"anchors",
		"`stele validate --change <change>`",
	)
}

// @verifies scn.workflowschema.3aef78a653b4.integration
func TestInitKeepsExistingSchemas(t *testing.T) {
	root := openSpecOnlyProject(t)
	openSpec(t, root, "new", "change", "active")
	steleInit(t, root)
	if metadata := readTestFile(t, root, "openspec/changes/active/.openspec.yaml"); !strings.Contains(
		metadata, "schema: spec-driven") {
		t.Fatalf("the active change was switched:\n%s", metadata)
	}

	schemaPath := "openspec/schemas/stele/schema.yaml"
	customized := readTestFile(t, root, schemaPath) + "# customized by the project\n"
	writeFixture(t, root, schemaPath, customized)
	if output := steleInit(t, root); !strings.Contains(output, "Kept the existing stele schema") {
		t.Fatalf("init did not report the kept schema: %q", output)
	}
	if readTestFile(t, root, schemaPath) != customized {
		t.Fatal("init changed a customized schema")
	}
	if output := steleInit(t, root, "--refresh-schema"); !strings.Contains(output, "Installed the stele workflow") {
		t.Fatalf("refresh did not re-fork: %q", output)
	}
	refreshed := readTestFile(t, root, schemaPath)
	if strings.Contains(refreshed, "customized by the project") ||
		strings.Count(refreshed, "- id: verification") != 1 ||
		strings.Count(refreshed, "Stele apply steps:") != 1 {
		t.Fatalf("refresh did not produce one clean patch:\n%s", refreshed)
	}

	other := openSpecOnlyProject(t)
	writeFixture(t, other, "openspec/config.yaml", "schema: team-flow\n")
	output := steleInit(t, other)
	if !strings.Contains(output, "team-flow") || !strings.Contains(output, "schema: stele") {
		t.Fatalf("init did not explain how to switch: %q", output)
	}
	if !strings.HasPrefix(readTestFile(t, other, "openspec/config.yaml"), "schema: team-flow\n") {
		t.Fatal("init replaced a custom default schema")
	}
}

// @verifies scn.workflowschema.779f1d7cebcb.integration
func TestInitInitializesOpenSpecWhenMissing(t *testing.T) {
	useRepositoryOpenSpec(t)
	root := fixtureRoot(t)
	output := steleInit(t, root)
	for _, note := range []string{"Initialized OpenSpec " + OpenSpecVersion, "Linked .claude/skills"} {
		if !strings.Contains(output, note) {
			t.Fatalf("init output lacks %q: %q", note, output)
		}
	}
	for _, skill := range []string{"openspec-propose", "stele-propose", "stele-plan"} {
		if !fileExists(filepath.Join(root, ".agents", "skills", skill, "SKILL.md")) {
			t.Fatalf("skill %s is missing", skill)
		}
	}
	if target, err := os.Readlink(filepath.Join(root, ".claude", "skills")); err != nil ||
		target != filepath.Join("..", ".agents", "skills") {
		t.Fatalf(".claude/skills link = %q, %v", target, err)
	}
	if !fileExists(filepath.Join(root, ".claude", "skills", "stele-apply", "SKILL.md")) {
		t.Fatal("the link does not expose the Stele skills")
	}
	config := readTestFile(t, root, "openspec/config.yaml")
	if !strings.HasPrefix(config, "schema: stele\n") || !strings.Contains(config, "Stele:") {
		t.Fatalf("OpenSpec was not extended:\n%s", config)
	}

	existing := fixtureRoot(t)
	writeFixture(t, existing, ".claude/skills/own/SKILL.md", "own skill\n")
	output = steleInit(t, existing)
	if !strings.Contains(output, "Warning: .claude/skills already exists") {
		t.Fatalf("init did not warn about .claude/skills: %q", output)
	}
	if info, err := os.Lstat(filepath.Join(existing, ".claude", "skills")); err != nil || !info.IsDir() {
		t.Fatalf(".claude/skills was replaced: %v", err)
	}

	claude := fixtureRoot(t)
	output = steleInit(t, claude, "--tools", "claude")
	if !strings.Contains(output, "for claude") || strings.Contains(output, "Linked .claude/skills") {
		t.Fatalf("init did not pass the tools through: %q", output)
	}
	if info, err := os.Lstat(filepath.Join(claude, ".claude", "skills")); err != nil || !info.IsDir() {
		t.Fatalf("OpenSpec did not write Claude skills: %v", err)
	}
	if !fileExists(filepath.Join(claude, ".claude", "skills", "openspec-propose", "SKILL.md")) {
		t.Fatal("OpenSpec skills for claude are missing")
	}
	if !fileExists(filepath.Join(claude, "openspec", "schemas", "stele", "schema.yaml")) {
		t.Fatal("the stele schema is missing")
	}
}

const userConfiguration = `# Team configuration
schema: spec-driven # chosen by the team

context: |
  Tech stack: Go

rules:
  proposal:
    - Keep proposals short # team rule
operations:
  apply:
    guidance:
      - Run the linters first
`

// @verifies scn.workflowschema.6ffeeaa11cce.integration
func TestMergeKeepsUserConfiguration(t *testing.T) {
	root := openSpecOnlyProject(t)
	writeFixture(t, root, "openspec/config.yaml", userConfiguration)
	output := steleInit(t, root)
	if !strings.Contains(output, "Added Stele guidance to openspec/config.yaml.") {
		t.Fatalf("init did not report the merge: %q", output)
	}
	config := readTestFile(t, root, "openspec/config.yaml")
	for _, kept := range []string{
		"# Team configuration",
		"# chosen by the team",
		"Tech stack: Go",
		"- Keep proposals short # team rule",
		"- Run the linters first",
	} {
		if !strings.Contains(config, kept) {
			t.Fatalf("merge lost %q:\n%s", kept, config)
		}
	}
	for _, added := range []string{
		"schema: stele",
		"confirm the verification levels",
		"must pass before archiving",
		"verification:",
	} {
		if !strings.Contains(config, added) {
			t.Fatalf("merge did not add %q:\n%s", added, config)
		}
	}
	if strings.Index(config, "Run the linters first") > strings.Index(config, "confirm the verification levels") {
		t.Fatalf("Stele guidance was not appended after the user's:\n%s", config)
	}
}

// @verifies scn.workflowschema.61671da05c73.integration
func TestMergeIsIdempotent(t *testing.T) {
	root := openSpecOnlyProject(t)
	writeFixture(t, root, "openspec/config.yaml", userConfiguration)
	steleInit(t, root)
	merged := readTestFile(t, root, "openspec/config.yaml")
	info, err := os.Stat(filepath.Join(root, "openspec", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	output := steleInit(t, root)
	if strings.Contains(output, "Added Stele guidance") {
		t.Fatalf("second init merged again: %q", output)
	}
	if readTestFile(t, root, "openspec/config.yaml") != merged {
		t.Fatal("second init changed the configuration")
	}
	again, err := os.Stat(filepath.Join(root, "openspec", "config.yaml"))
	if err != nil || !again.ModTime().Equal(info.ModTime()) {
		t.Fatalf("second init rewrote the configuration: %v", err)
	}
}

// @verifies scn.workflowschema.b184c4d2986a.integration
func TestOpenSpecInstructionsShowGuidance(t *testing.T) {
	root := openSpecOnlyProject(t)
	steleInit(t, root)
	writeReadyChange(t, root, "demo", true)
	for operation, want := range map[string]string{
		"apply":   "Stele: before the first task, confirm the verification levels",
		"archive": "Stele: `stele validate --change <change>` must pass before archiving.",
	} {
		var instructions struct {
			OperationGuidance []string `json:"operationGuidance"`
		}
		openSpecJSON(t, root, &instructions, "instructions", operation, "--change", "demo")
		if !slices.ContainsFunc(instructions.OperationGuidance, func(item string) bool {
			return strings.HasPrefix(item, want)
		}) {
			t.Fatalf("%s guidance lacks %q: %#v", operation, want, instructions.OperationGuidance)
		}
	}
}

func TestInstallWorkflowSchemaReportsFailures(t *testing.T) {
	failure := errors.New("setup failed")
	t.Run("missing OpenSpec CLI", func(t *testing.T) {
		original := executablePath
		t.Cleanup(func() { executablePath = original })
		executablePath = func() (string, error) { return "", failure }
		_, err := installWorkflowSchema(fixtureRoot(t), openSpecSetup{tools: defaultOpenSpecTools})
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "openspec init failed") {
			t.Fatalf("expected the process error, got %v", err)
		}
	})
	t.Run("OpenSpec init output", func(t *testing.T) {
		useRepositoryOpenSpec(t)
		_, err := installWorkflowSchema(fixtureRoot(t), openSpecSetup{tools: "not-a-tool"})
		if err == nil || !strings.Contains(err.Error(), "openspec init failed: ") ||
			!strings.Contains(err.Error(), "not-a-tool") {
			t.Fatalf("expected the OpenSpec output, got %v", err)
		}
	})
	t.Run("claude link", func(t *testing.T) {
		useRepositoryOpenSpec(t)
		root := fixtureRoot(t)
		writeFixture(t, root, ".claude", "blocking file")
		if _, err := installWorkflowSchema(root, openSpecSetup{tools: defaultOpenSpecTools}); err == nil {
			t.Fatal("expected a link error")
		}
	})
	t.Run("missing configuration", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/project.md", "legacy\n")
		notes, err := installWorkflowSchema(root, openSpecSetup{tools: defaultOpenSpecTools})
		if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "config.yaml was not found") {
			t.Fatalf("notes = %#v, %v", notes, err)
		}
	})
	t.Run("fork", func(t *testing.T) {
		useRepositoryOpenSpec(t)
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/config.yaml", "schema: spec-driven\n")
		writeFixture(t, root, "openspec/schemas", "blocking file")
		_, err := installWorkflowSchema(root, openSpecSetup{tools: defaultOpenSpecTools})
		if err == nil || !strings.Contains(err.Error(), "openspec schema fork failed") {
			t.Fatalf("expected a fork error, got %v", err)
		}
	})
	t.Run("guidance", func(t *testing.T) {
		useRepositoryOpenSpec(t)
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/config.yaml", "schema: spec-driven\n")
		writeFixture(t, root, "openspec/schemas/stele/schema.yaml", "name: stele\n")
		original := runNodeScript
		t.Cleanup(func() { runNodeScript = original })
		runNodeScript = func(string, string, string) ([]byte, error) { return nil, failure }
		if _, err := installWorkflowSchema(root, openSpecSetup{tools: defaultOpenSpecTools}); !errors.Is(err, failure) {
			t.Fatalf("expected the guidance error, got %v", err)
		}
	})
}

func TestMergeOpenSpecGuidanceReportsFailures(t *testing.T) {
	root := fixtureRoot(t)
	original := executablePath
	t.Cleanup(func() { executablePath = original })
	executablePath = func() (string, error) { return filepath.Join(root, "bin", "stele"), nil }
	if _, err := mergeOpenSpecGuidance(root, false); err == nil {
		t.Fatal("expected a missing OpenSpec error")
	}

	useRepositoryOpenSpec(t)
	originalNode := runNodeScript
	t.Cleanup(func() { runNodeScript = originalNode })
	runNodeScript = func(string, string, string) ([]byte, error) { return []byte("not json"), nil }
	if _, err := mergeOpenSpecGuidance(root, false); err == nil || !strings.Contains(err.Error(), "invalid output") {
		t.Fatalf("expected an invalid output error, got %v", err)
	}

	// The real script fails without a configuration and reports its error.
	runNodeScript = originalNode
	_, err := mergeOpenSpecGuidance(root, false)
	if err == nil || !strings.Contains(err.Error(), "updating the OpenSpec setup failed") ||
		strings.TrimSpace(strings.TrimPrefix(err.Error(), "updating the OpenSpec setup failed:")) == "" {
		t.Fatalf("expected the script error, got %v", err)
	}
}

func TestRunNodeAndLinkFailures(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := runNode(fixtureRoot(t), "", ""); err == nil {
		t.Fatal("expected a missing node error")
	}

	root := fixtureRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, ".claude"), 0o755) })
	if _, err := linkClaudeSkills(root); err == nil {
		t.Fatal("expected a symlink error")
	}
}

func TestInitCommandReportsSetupFailures(t *testing.T) {
	stubOpenSpecSetup(t, nil, errors.New("setup failed"))
	var stderr bytes.Buffer
	if code := Run([]string{"init", "--root", fixtureRoot(t)}, &bytes.Buffer{}, &stderr); code != 2 ||
		!strings.Contains(stderr.String(), "setup failed") {
		t.Fatalf("init with a failing setup = %d, %q", code, stderr.String())
	}
}
