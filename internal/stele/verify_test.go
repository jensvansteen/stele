package stele

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVerificationResolvesPlannedAnchors(t *testing.T) {
	root := completeFixture(t, false)
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" || report.Summary.LinkedRequirements != 1 ||
		report.Summary.LinkedScenarios != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

// @verifies scn.verify.c4db6a432869.unit
func TestRunVerificationRejectsMismatchedTarget(t *testing.T) {
	root := completeFixture(t, false)
	plan := `{"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#wrong"},` +
		`"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/demo.test.mts#wrong"}}`
	writeFixture(t, root, "artifacts/linkage-plan.json", plan)
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "fail" || !hasDiagnostic(report.Diagnostics, "LINK_TARGET_MISMATCH") {
		t.Fatalf("expected target mismatch, got %#v", report.Diagnostics)
	}
}

func TestRunVerificationLoadsExistingEvidence(t *testing.T) {
	root := completeFixture(t, false)
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{
		InputDigest:    digest,
		TestedRevision: "tested",
		Scenarios: []ScenarioOutcome{
			{ID: "scn.demo.bbbbbbbbbbbb", Outcome: "passed"},
		},
	}
	if err := writeJSON(filepath.Join(root, "artifacts", "test-results.json"), evidence); err != nil {
		t.Fatal(err)
	}
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PassedScenarios != 1 || report.Stages.Execution.Status != "passed" {
		t.Fatalf("existing evidence was not loaded: %#v", report)
	}
}

func TestRunVerificationReportsMissingAndInvalidLinks(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Works
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(t, root, "src/demo.mts", `// @implements req.demo.aaaaaaaaaaaa
const unresolved = true;
// @implements scn.demo.bbbbbbbbbbbb
function wrongKind() {}
// @implements req.demo.cccccccccccc
function dangling() {}
`)
	writeFixture(t, root, "tests/demo.test.mts", `// @verifies req.demo.aaaaaaaaaaaa
test("wrong kind", () => {});
`)
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"ANCHOR_DANGLING", "ANCHOR_KIND", "ANCHOR_TARGET_MISSING", "LINK_TEST_MISSING"} {
		if !hasDiagnostic(report.Diagnostics, code) {
			t.Errorf("missing %s in %#v", code, report.Diagnostics)
		}
	}
	if report.Verdicts.Linkage != "fail" || report.Stages.Linkage.Status != "fail" {
		t.Fatalf("unexpected failure report: %#v", report)
	}
}

// @verifies scn.verify.14c6b4fe39bc.unit
func TestRunVerificationProposalPlans(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "artifacts/linkage-plan.json", `{"requirements":{},"scenarios":{}}`)
	report, err := RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report.Diagnostics, "PLAN_CODE_MISSING") ||
		!hasDiagnostic(report.Diagnostics, "PLAN_TEST_MISSING") {
		t.Fatalf("expected missing plan diagnostics: %#v", report.Diagnostics)
	}
	if report.Requirements[0].Linkage != "missing" || report.Requirements[0].Scenarios[0].Linkage != "missing" {
		t.Fatalf("expected missing proposal linkage: %#v", report.Requirements)
	}

	plan := `{"requirements":{"req.demo.aaaaaaaaaaaa":"future.ts#value"},` +
		`"scenarios":{"scn.demo.bbbbbbbbbbbb":"future.test.ts#returns value"}}`
	writeFixture(t, root, "artifacts/linkage-plan.json", plan)
	report, err = RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" ||
		report.Stages.Linkage.Status != "planned" ||
		report.Requirements[0].Linkage != "planned" ||
		report.Requirements[0].Scenarios[0].Linkage != "planned" {
		t.Fatalf("unexpected planned report: %#v", report)
	}
}

func TestRunVerificationReturnsDependencyAndOutputErrors(t *testing.T) {
	t.Run("spec", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", string(make([]byte, 70_000)))
		if _, err := RunVerification(root, "example", "implementation", ""); err == nil {
			t.Fatal("expected spec error")
		}
	})
	t.Run("anchor", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.ts"), filepath.Join(root, "src", "broken.ts")); err != nil {
			t.Fatal(err)
		}
		if _, err := RunVerification(root, "example", "implementation", ""); err == nil {
			t.Fatal("expected anchor error")
		}
	})
	t.Run("digest", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		if err := os.Symlink(
			filepath.Join(root, "missing.yaml"),
			filepath.Join(root, "openspec", "broken.yaml"),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := RunVerification(root, "example", "implementation", ""); err == nil {
			t.Fatal("expected digest error")
		}
	})
	t.Run("report", func(t *testing.T) {
		root := completeFixture(t, false)
		writeFixture(t, root, "blocked", "file")
		if _, err := RunVerification(root, "example", "implementation", "blocked/report.json"); err == nil {
			t.Fatal("expected report write error")
		}
	})
	t.Run("absolute report", func(t *testing.T) {
		root := completeFixture(t, false)
		reportPath := filepath.Join(root, "elsewhere", "report.json")
		if _, err := RunVerification(root, "example", "implementation", reportPath); err != nil {
			t.Fatal(err)
		}
		if !fileExists(reportPath) {
			t.Fatal("absolute report was not written")
		}
	})
}

// @verifies scn.execution.a70f24b45cfe.unit
func TestBuildReportTracksEvidenceAndDiagnostics(t *testing.T) {
	req := Requirement{
		ID:     "req.demo.aaaaaaaaaaaa",
		Title:  "Demo",
		Source: Source{Path: "spec.md", Line: 1},
		Scenarios: []Scenario{
			{ID: "scn.demo.bbbbbbbbbbbb", Title: "Works", Source: Source{Path: "spec.md", Line: 2}},
		},
	}
	parsed := ParsedSpecs{
		Requirements: []Requirement{req},
		Diagnostics:  []Diagnostic{{Code: "SHAPE", Severity: "error"}},
	}
	selector, line := "demo", 2
	anchors := []Anchor{{ID: req.ID, Kind: "code", Path: "src/demo.go", Selector: &selector, DeclarationLine: &line}}
	diagnostics := []Diagnostic{{Code: "WARN", Severity: "warning"}}

	current := &Evidence{
		InputDigest:    "digest",
		TestedRevision: "abc",
		Scenarios: []ScenarioOutcome{
			{ID: req.Scenarios[0].ID, Outcome: "passed"},
		},
	}
	report := BuildReport(
		t.TempDir(),
		"example",
		"implementation",
		"digest",
		parsed,
		anchors,
		LinkagePlan{},
		diagnostics,
		current,
	)
	if report.Stages.Proposal.Status != "fail" ||
		report.Summary.Warnings != 1 ||
		report.Summary.PassedScenarios != 1 ||
		report.Stages.Execution.Status != "passed" ||
		report.Requirements[0].Scenarios[0].Linkage != "missing" {
		t.Fatalf("unexpected current report: %#v", report)
	}

	stale := BuildReport(
		t.TempDir(),
		"example",
		"implementation",
		"new",
		ParsedSpecs{Requirements: []Requirement{req}},
		nil,
		LinkagePlan{},
		[]Diagnostic{{Severity: "error"}},
		current,
	)
	if stale.Verdicts.Linkage != "fail" ||
		stale.Stages.Execution.Status != "stale" ||
		stale.Requirements[0].Linkage != "missing" {
		t.Fatalf("unexpected stale report: %#v", stale)
	}
	failed := BuildReport(
		t.TempDir(),
		"example",
		"implementation",
		"digest",
		ParsedSpecs{Requirements: []Requirement{req}},
		nil,
		LinkagePlan{},
		nil,
		&Evidence{
			InputDigest: "digest",
			Scenarios: []ScenarioOutcome{
				{ID: req.Scenarios[0].ID, Outcome: "failed"},
			},
		},
	)
	if failed.Stages.Execution.Status != "failed" {
		t.Fatalf("unexpected failed report: %#v", failed)
	}
}

func TestVerificationHelpers(t *testing.T) {
	if path, selector := plannedTarget("src/demo.go"); path != "src/demo.go" || selector != "" {
		t.Fatalf("plannedTarget without selector = %q, %q", path, selector)
	}
	for _, test := range []struct {
		outcomes []string
		want     string
	}{
		{[]string{"passed", "passed"}, "passed"},
		{[]string{"passed", "failed"}, "failed"},
		{[]string{"stale"}, "stale"},
		{nil, "not-run"},
	} {
		if got := aggregateExecution(test.outcomes); got != test.want {
			t.Fatalf("aggregateExecution(%v) = %q", test.outcomes, got)
		}
	}
	if !every(nil, "passed") {
		t.Fatal("every on empty slice should be true")
	}

	root := fixtureRoot(t)
	writeFixture(t, root, "parent", "file")
	if err := writeJSON(filepath.Join(root, "parent", "result.json"), struct{}{}); err == nil {
		t.Fatal("expected directory creation error")
	}
	directoryTarget := filepath.Join(root, "directory.json")
	if err := os.Mkdir(directoryTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(directoryTarget, struct{}{}); err == nil {
		t.Fatal("expected file write error")
	}
	originalMarshal := marshalJSON
	t.Cleanup(func() { marshalJSON = originalMarshal })
	marshalJSON = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal failed") }
	if err := writeJSON(filepath.Join(root, "result.json"), make(chan int)); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestGitState(t *testing.T) {
	root := fixtureRoot(t)
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	git := filepath.Join(bin, "git")
	writeScript := func(content string) {
		t.Helper()
		if err := os.WriteFile(git, []byte("#!/bin/sh\n"+content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	writeScript("exit 1\n")
	if revision, dirty := gitState(root); revision != "uncommitted" || !dirty {
		t.Fatalf("failed git state = %q, %v", revision, dirty)
	}
	writeScript("if [ \"$1\" = rev-parse ]; then echo abc; exit 0; fi\necho changed\n")
	if revision, dirty := gitState(root); revision != "abc" || !dirty {
		t.Fatalf("dirty git state = %q, %v", revision, dirty)
	}
	writeScript("if [ \"$1\" = rev-parse ]; then echo abc; exit 0; fi\nexit 1\n")
	if revision, dirty := gitState(root); revision != "abc" || !dirty {
		t.Fatalf("status error state = %q, %v", revision, dirty)
	}
	writeScript("if [ \"$1\" = rev-parse ]; then echo abc; fi\n")
	if revision, dirty := gitState(root); revision != "abc" || dirty {
		t.Fatalf("clean git state = %q, %v", revision, dirty)
	}
}

func TestDeterministicReportBytesAreIdentical(t *testing.T) {
	root := completeFixture(t, false)
	first, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := MarshalDeterministic(first)
	secondBytes, _ := MarshalDeterministic(second)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("deterministic reports differ")
	}
}

func TestRunScenarioTestsMapsPassAndFailure(t *testing.T) {
	for _, failing := range []bool{false, true} {
		root := completeFixture(t, failing)
		evidence, err := RunScenarioTests(root, "example", filepath.Join("artifacts", "test-results.json"))
		if err != nil {
			t.Fatal(err)
		}
		expected := "passed"
		if failing {
			expected = "failed"
		}
		if evidence.Outcome != expected || len(evidence.Scenarios) != 1 || evidence.Scenarios[0].Outcome != expected {
			t.Fatalf("expected %s evidence, got %#v", expected, evidence)
		}
	}
}

func completeFixture(t *testing.T, failing bool) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(
		t,
		root,
		"src/demo.mts",
		"// @implements "+"req.demo.aaaaaaaaaaaa\nexport function value() { return 1; }\n",
	)
	want := "1"
	if failing {
		want = "2"
	}
	testSource := "import assert from \"node:assert/strict\";\n" +
		"import test from \"node:test\";\n" +
		"// @verifies " + "scn.demo.bbbbbbbbbbbb\n" +
		"test(\"returns value\", () => assert.equal(1, " + want + "));\n"
	writeFixture(t, root, "tests/demo.test.mts", testSource)
	plan := `{"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#value"},` +
		`"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/demo.test.mts#returns value"}}`
	writeFixture(t, root, "artifacts/linkage-plan.json", plan)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"type\":\"module\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// verdictInputs returns a linked requirement with two scenarios; it passes
// no linkage diagnostics.
func verdictInputs() (ParsedSpecs, []Anchor) {
	requirement := Requirement{
		ID:     "req.demo.aaaaaaaaaaaa",
		Title:  "Demo",
		Source: Source{Path: "spec.md", Line: 1},
		Scenarios: []Scenario{
			{ID: "scn.demo.bbbbbbbbbbbb", Title: "Works", Source: Source{Path: "spec.md", Line: 2}},
			{ID: "scn.demo.cccccccccccc", Title: "Fails", Source: Source{Path: "spec.md", Line: 3}},
		},
	}
	selector := "demo"
	anchors := []Anchor{{ID: requirement.ID, Kind: "code", Path: "src/demo.go", Selector: &selector}}
	return ParsedSpecs{Requirements: []Requirement{requirement}}, anchors
}

// @verifies scn.verify.140b21cbc3f0.unit
func TestReportSeparatesExecutionVerdict(t *testing.T) {
	parsed, anchors := verdictInputs()
	evidence := &Evidence{InputDigest: "digest", Scenarios: []ScenarioOutcome{
		{ID: "scn.demo.bbbbbbbbbbbb", Outcome: "passed"},
		{ID: "scn.demo.cccccccccccc", Outcome: "failed"},
	}}
	report := BuildReport(t.TempDir(), "example", "implementation", "digest", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{}, evidence)
	want := ReportVerdicts{Linkage: "pass", Execution: "failed", Overall: "fail"}
	if report.Verdicts != want || report.Verdict != "fail" || report.SchemaVersion != "2.1" {
		t.Fatalf("verdicts = %#v, verdict = %s, schema %s", report.Verdicts, report.Verdict, report.SchemaVersion)
	}
	var output bytes.Buffer
	renderVerification(&output, report)
	for _, line := range []string{
		"✓ implementation verification pass",
		"✗ execution failed: 1/2 scenarios passed",
		"FAILED scn.demo.cccccccccccc Fails",
		"✗ overall fail",
	} {
		if !strings.Contains(output.String(), line) {
			t.Fatalf("human summary lacks %q: %q", line, output.String())
		}
	}

	proposal := BuildReport(t.TempDir(), "example", "proposal", "digest", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{}, evidence)
	if proposal.Verdicts.Overall != "pass" || proposal.Verdict != "pass" {
		t.Fatalf("the proposal stage did not follow linkage: %#v", proposal.Verdicts)
	}
	output.Reset()
	renderVerification(&output, proposal)
	if strings.Contains(output.String(), "execution") {
		t.Fatalf("the proposal summary names execution: %q", output.String())
	}
	failing := BuildReport(t.TempDir(), "example", "implementation", "digest", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{{Code: "FAIL", Severity: "error"}}, nil)
	if failing.Verdicts != (ReportVerdicts{Linkage: "fail", Execution: "not-run", Overall: "fail"}) {
		t.Fatalf("failed linkage = %#v", failing.Verdicts)
	}
}

// @verifies scn.verify.a7ae8afade0a.unit
func TestReportMarksMissingEvidenceIncomplete(t *testing.T) {
	parsed, anchors := verdictInputs()
	report := BuildReport(t.TempDir(), "example", "implementation", "digest", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{}, nil)
	want := ReportVerdicts{Linkage: "pass", Execution: "not-run", Overall: "incomplete"}
	if report.Verdicts != want || report.Verdict != "incomplete" {
		t.Fatalf("missing evidence = %#v, %s", report.Verdicts, report.Verdict)
	}
	stale := BuildReport(t.TempDir(), "example", "implementation", "digest", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{}, &Evidence{InputDigest: "old", Scenarios: []ScenarioOutcome{
			{ID: "scn.demo.bbbbbbbbbbbb", Outcome: "passed"},
			{ID: "scn.demo.cccccccccccc", Outcome: "passed"},
		}})
	if stale.Verdicts.Execution != "stale" || stale.Verdict != "incomplete" {
		t.Fatalf("stale evidence = %#v", stale.Verdicts)
	}
	passing := BuildReport(t.TempDir(), "example", "implementation", "old", parsed, anchors,
		emptyLinkagePlan(), []Diagnostic{}, &Evidence{InputDigest: "old", Scenarios: []ScenarioOutcome{
			{ID: "scn.demo.bbbbbbbbbbbb", Outcome: "passed"},
			{ID: "scn.demo.cccccccccccc", Outcome: "passed"},
		}})
	if passing.Verdicts != (ReportVerdicts{Linkage: "pass", Execution: "passed", Overall: "pass"}) {
		t.Fatalf("passing evidence = %#v", passing.Verdicts)
	}
}
