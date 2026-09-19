package stele

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const scopePlanForExample = `{"changeId":"example",` +
	`"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#value"},` +
	`"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/demo.test.mts#returns value"}}`

// @verifies scn.verificationscope.20fde975089b.unit
func TestRunVerificationPrefersChangePlan(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "artifacts/linkage-plan.json", `{"changeId":"example","requirements":{},"scenarios":{}}`)
	writeFixture(t, root, "openspec/changes/example/linkage-plan.json", scopePlanForExample)
	report, err := RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" || report.Requirements[0].Linkage != "planned" {
		t.Fatalf("change plan was not used: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationscope.d477fd8e9980.unit
func TestRunVerificationFallsBackToSharedPlan(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "artifacts/linkage-plan.json", scopePlanForExample)
	report, err := RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" {
		t.Fatalf("shared plan was not used: %#v", report.Diagnostics)
	}

	writeFixture(t, root, "artifacts/linkage-plan.json", `{"changeId":"example","requirements":null,"scenarios":null}`)
	report, err = RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report.Diagnostics, "PLAN_CODE_MISSING") {
		t.Fatalf("expected an empty plan to leave targets missing: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationscope.784db68ce025.unit
func TestRunVerificationRejectsPlanForOtherChange(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "artifacts/linkage-plan.json", strings.Replace(scopePlanForExample, "example", "other", 1))
	report, err := RunVerification(root, "example", "proposal", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "fail" || !hasDiagnostic(report.Diagnostics, "PLAN_CHANGE_MISMATCH") {
		t.Fatalf("expected a plan mismatch: %#v", report.Diagnostics)
	}
	if !hasDiagnostic(report.Diagnostics, "PLAN_CODE_MISSING") {
		t.Fatalf("a mismatched plan must not provide targets: %#v", report.Diagnostics)
	}
	for _, item := range report.Diagnostics {
		if item.Code == "PLAN_CHANGE_MISMATCH" &&
			(item.Source == nil || item.Source.Path != "artifacts/linkage-plan.json" ||
				!strings.Contains(item.Message, "other") || !strings.Contains(item.Message, "example")) {
			t.Fatalf("mismatch diagnostic does not name both changes and the plan: %#v", item)
		}
	}
}

// @verifies scn.verificationscope.ee5d7b261d62.unit
func TestRunVerificationIgnoresOtherChangeAnchors(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "openspec/changes/other/specs/other/spec.md", `### Requirement: Other
Verification-ID: req.other.cccccccccccc
#### Scenario: Other scenario
Verification-ID: scn.other.dddddddddddd
`)
	writeFixture(t, root, "src/other.mts", "// @verifies "+"req.other.cccccccccccc\nconst unrelated = 1;\n")
	writeFixture(t, root, "tests/other.test.mts", "// @verifies "+"scn.other.dddddddddddd.e2e\nconst unrelated = 2;\n")
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" || report.Summary.Errors != 0 {
		t.Fatalf("anchors of another change affected the verdict: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationscope.101e06b07f39.unit
func TestRunVerificationReportsUndeclaredAnchors(t *testing.T) {
	root := completeFixture(t, false)
	writeFixture(t, root, "openspec/specs/archived/spec.md", "### Requirement: Archived\n"+
		"Verification-ID: req.archived.eeeeeeeeeeee\r\n")
	writeFixture(t, root, "src/archived.mts",
		"// @implements "+"req.archived.eeeeeeeeeeee\nexport function kept() {}\n"+
			"// @implements "+"req.typo.ffffffffffff\nexport function typo() {}\n")
	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	dangling := 0
	for _, item := range report.Diagnostics {
		if item.Code == "ANCHOR_DANGLING" {
			dangling++
			if item.IdentityID == nil || *item.IdentityID != "req.typo.ffffffffffff" {
				t.Fatalf("wrong dangling identity: %#v", item)
			}
		}
	}
	if report.Verdicts.Linkage != "fail" || dangling != 1 {
		t.Fatalf("expected exactly the undeclared anchor to dangle: %#v", report.Diagnostics)
	}

	broken := filepath.Join(root, "openspec", "specs", "broken.md")
	if err := os.Symlink(filepath.Join(root, "missing.md"), broken); err != nil {
		t.Fatal(err)
	}
	if _, err := RunVerification(root, "example", "implementation", ""); err == nil {
		t.Fatal("expected an error for an unreadable specification")
	}
}

func archivedFixture(t *testing.T) string {
	t.Helper()
	root := completeFixture(t, false)
	if err := os.RemoveAll(filepath.Join(root, "openspec", "changes", "example")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "artifacts", "linkage-plan.json")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/specs/demo/spec.md", `# demo Specification

## Purpose
Return values.

## Requirements
### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	return root
}

// @verifies scn.verificationscope.8b577e9712e7.unit
func TestRunVerificationVerifiesCurrentSpecs(t *testing.T) {
	root := archivedFixture(t)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-example/linkage-plan.json", scopePlanForExample)
	report, err := verifyScope(verifyRequest{
		root: root, scope: verificationScope{currentSpecs: true}, mode: "implementation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" || report.OpenSpec.ChangeID != "" || report.Summary.Requirements != 1 {
		t.Fatalf("current specifications did not verify: %#v", report)
	}

	run, err := runScopeTests(testRequest{root: root, scope: verificationScope{currentSpecs: true}})
	evidence := run.evidence
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Outcome != "passed" || len(evidence.Scenarios) != 1 {
		t.Fatalf("current specification scenarios did not run: %#v", evidence)
	}
}

// @verifies scn.verificationscope.70c3ee060846.unit
func TestCombinedArchivePlanPrefersLatest(t *testing.T) {
	root := archivedFixture(t)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-example/linkage-plan.json",
		`{"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#old"},`+
			`"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/demo.test.mts#old"}}`)
	writeFixture(t, root, "openspec/changes/archive/2026-02-01-rework/linkage-plan.json", scopePlanForExample)
	writeFixture(t, root, "openspec/changes/archive/2026-03-01-broken/linkage-plan.json", "{")
	plan, _ := loadArchivedPlans(root, verificationScope{currentSpecs: true})
	if plan.Requirements["req.demo.aaaaaaaaaaaa"] != "src/demo.mts#value" ||
		plan.Scenarios["scn.demo.bbbbbbbbbbbb"] != "tests/demo.test.mts#returns value" {
		t.Fatalf("latest archived plan did not win: %#v", plan)
	}
	report, err := verifyScope(verifyRequest{
		root: root, scope: verificationScope{currentSpecs: true}, mode: "implementation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdicts.Linkage != "pass" {
		t.Fatalf("combined plan did not verify: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationscope.f8ab6e2e1812.integration
func TestRunOpenSpecValidatesCurrentSpecs(t *testing.T) {
	repositoryOpenSpec, err := filepath.Abs(filepath.Join("..", "..", "node_modules", "@fission-ai", "openspec"))
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(repositoryOpenSpec, "bin", "openspec.js")) {
		t.Fatalf("bundled OpenSpec CLI is missing at %s; run npm ci", repositoryOpenSpec)
	}
	root := fixtureRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "@fission-ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "node_modules", "@fission-ai", "openspec")
	if err := os.Symlink(repositoryOpenSpec, linked); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/config.yaml", "schema: spec-driven\n")
	writeFixture(t, root, "openspec/specs/demo/spec.md", `# demo Specification

## Purpose
Return stored values to callers of the demonstration capability.

## Requirements
### Requirement: Return value
The system SHALL return the stored value.

#### Scenario: Value is returned
- **WHEN** a value is stored
- **THEN** it is returned
`)
	passed, err := runOpenSpec(root, verificationScope{currentSpecs: true})
	if err != nil || !passed {
		t.Fatalf("valid specifications failed validation: %v", err)
	}

	writeFixture(t, root, "openspec/specs/demo/spec.md", "# demo Specification\n\n## Requirements\n"+
		"### Requirement: Return value\nNo normative statement.\n")
	passed, err = runOpenSpec(root, verificationScope{currentSpecs: true})
	if err == nil || passed {
		t.Fatal("expected strict validation of the current specifications to fail")
	}
}

// @verifies scn.verificationscope.79a83cf82aba.unit
func TestRunRejectsSpecsWithChange(t *testing.T) {
	root := fixtureRoot(t)
	for _, command := range []string{"verify", "test", "validate"} {
		var stderr bytes.Buffer
		code := Run([]string{command, "--root", root, "--specs", "--change", "example"}, &bytes.Buffer{}, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "--specs cannot be combined with --change") {
			t.Fatalf("%s = %d, %q", command, code, stderr.String())
		}
	}
	var stderr bytes.Buffer
	if code := Run([]string{"init", "--root", root, "--specs"}, &bytes.Buffer{}, &stderr); code != 2 {
		t.Fatalf("init accepted --specs: %q", stderr.String())
	}

	writeFixture(t, root, "stele.config.json", `{"change":"configured"}`)
	parsed, err := parseOptions("verify", []string{"--root", root, "--specs"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = withConfig(parsed)
	if scope := resolveScope(parsed); err != nil || !scope.currentSpecs || scope.changeID != "" ||
		scope.spec().Name() != "openspec" {
		t.Fatalf("--specs did not select the current specifications: %#v, %v", parsed, err)
	}
	if scope := resolveScope(options{changeID: "example"}); !reflect.DeepEqual(scope, changeScope("example")) {
		t.Fatal("a change option did not select the change scope")
	}
}

// @verifies scn.verificationscope.0246717fcb77.unit
func TestRunRejectsMissingChange(t *testing.T) {
	root := fixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"verify", "--root", root, "--change", "missing"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "change missing has no delta specs") || stdout.Len() != 0 {
		t.Fatalf("verify missing change = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	if _, err := RunVerification(root, "missing", "proposal", ""); err == nil {
		t.Fatal("RunVerification accepted a missing change")
	}
}

// @verifies scn.verificationscope.2af83d65e804.unit
func TestRunRejectsChangeWithoutSpecs(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/empty/proposal.md", "## Why\n")
	writeFixture(t, root, "openspec/changes/empty/specs/notes.txt", "not a spec\n")
	originalOpenSpec := validateProjectOpenSpec
	t.Cleanup(func() { validateProjectOpenSpec = originalOpenSpec })
	openSpecRan := false
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) {
		openSpecRan = true
		return true, nil
	}
	for _, command := range []string{"verify", "test", "validate"} {
		var stderr bytes.Buffer
		code := Run([]string{command, "--root", root, "--change", "empty"}, &bytes.Buffer{}, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "change empty has no delta specs") {
			t.Fatalf("%s empty change = %d, %q", command, code, stderr.String())
		}
	}
	if openSpecRan {
		t.Fatal("OpenSpec validation ran for an empty change")
	}
	if fileExists(filepath.Join(root, "artifacts", "test-results.json")) {
		t.Fatal("evidence was written for an empty change")
	}
}

// @verifies scn.verificationscope.6f3a1ce24f05.unit
func TestRunRejectsEmptyCurrentSpecs(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/.gitkeep", "")
	var stderr bytes.Buffer
	code := Run([]string{"verify", "--root", root, "--specs"}, &bytes.Buffer{}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "no current specifications in openspec/specs") {
		t.Fatalf("verify --specs without specs = %d, %q", code, stderr.String())
	}
	writeFixture(t, root, "openspec/specs/demo/spec.md", "### Requirement: Demo\n")
	if code := Run([]string{"verify", "--root", root, "--specs"}, &bytes.Buffer{}, &stderr); code == 2 {
		t.Fatalf("verify --specs with a specification was still rejected: %q", stderr.String())
	}
}

// sameDayArchives archives two changes on the same date that both plan
// scn.demo.bbbbbbbbbbbb; alpha declares the scenario with the given text and
// beta with an older one.
func sameDayArchives(t *testing.T, alphaText, betaText string) string {
	t.Helper()
	root := archivedFixture(t)
	spec := func(text string) string {
		return "## ADDED Requirements\n### Requirement: Return value\nVerification-ID: req.demo.aaaaaaaaaaaa\n" +
			"#### Scenario: " + text + "\nVerification-ID: scn.demo.bbbbbbbbbbbb\n"
	}
	plan := func(change, level string) string {
		return `{"schemaVersion":2,"changeId":"` + change + `","scenarios":{"scn.demo.bbbbbbbbbbbb":{"evidence":[` +
			`{"id":"scn.demo.bbbbbbbbbbbb.` + level + `","level":"` + level +
			`","rationale":"Plan of ` + change + `."}]}}}`
	}
	writeFixture(t, root, "openspec/changes/archive/2026-09-18-alpha/specs/demo/spec.md", spec(alphaText))
	writeFixture(t, root, "openspec/changes/archive/2026-09-18-alpha/linkage-plan.json", plan("alpha", "unit"))
	writeFixture(t, root, "openspec/changes/archive/2026-09-18-beta/specs/demo/spec.md", spec(betaText))
	writeFixture(t, root, "openspec/changes/archive/2026-09-18-beta/linkage-plan.json", plan("beta", "e2e"))
	return root
}

// @verifies scn.verificationscope.4ad1b478a310.unit
func TestArchivedPlanPrefersMatchingSpecification(t *testing.T) {
	root := sameDayArchives(t, "Value is returned", "Value was returned")
	plan, diagnostics := loadArchivedPlans(root, verificationScope{currentSpecs: true})
	entries := plan.Evidence["scn.demo.bbbbbbbbbbbb"]
	if len(entries) != 1 || entries[0].Level != "unit" || len(diagnostics) != 0 {
		t.Fatalf("the plan whose specification matches did not win: %#v, %#v", entries, diagnostics)
	}
	if archiveDate("undated") != "undated" || archiveDate("2026-09-18-alpha") != "2026-09-18" {
		t.Fatal("archive dates are not read from directory names")
	}
}

// @verifies scn.verificationscope.1c6a44335f91.unit
func TestArchivedPlanWarnsWhenOrderIsAmbiguous(t *testing.T) {
	root := sameDayArchives(t, "Value is returned", "Value is returned")
	plan, diagnostics := loadArchivedPlans(root, verificationScope{currentSpecs: true})
	entries := plan.Evidence["scn.demo.bbbbbbbbbbbb"]
	if len(entries) != 1 || entries[0].Level != "e2e" || len(diagnostics) != 1 ||
		diagnostics[0].Code != "PLAN_ARCHIVE_ORDER_AMBIGUOUS" || diagnostics[0].Severity != "warning" ||
		!strings.Contains(diagnostics[0].Message, "2026-09-18-alpha/linkage-plan.json") ||
		!strings.Contains(diagnostics[0].Message, "2026-09-18-beta/linkage-plan.json") {
		t.Fatalf("an ambiguous order was not reported: %#v, %#v", entries, diagnostics)
	}
	writeFixture(t, root, "openspec/changes/archive/2026-09-18-beta/linkage-plan.json",
		`{"schemaVersion":2,"changeId":"beta","scenarios":{"scn.demo.bbbbbbbbbbbb":{"evidence":[`+
			`{"id":"scn.demo.bbbbbbbbbbbb.unit","level":"unit","rationale":"Same."}]}}}`)
	if _, diagnostics := loadArchivedPlans(root, verificationScope{currentSpecs: true}); len(diagnostics) != 0 {
		t.Fatalf("agreeing plans were reported: %#v", diagnostics)
	}
}

// @verifies scn.verificationscope.9c3ef2025b56.unit
func TestSpecsReportAnchorsToRetiredBehavior(t *testing.T) {
	root := archivedFixture(t)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-example/linkage-plan.json", scopePlanForExample)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-legacy/specs/legacy/spec.md",
		"### Requirement: Legacy\nVerification-ID: req.legacy.111111111111\n"+
			"#### Scenario: Legacy works\nVerification-ID: scn.legacy.222222222222\n")
	writeFixture(t, root, "openspec/changes/active/specs/other/spec.md",
		"### Requirement: Other\nVerification-ID: req.other.333333333333\n")
	writeFixture(t, root, "src/legacy.mts", "// @implements "+"req.other.333333333333\nexport function other() {}\n")
	writeEvidenceTest(t, root, "tests/legacy.test.mts", "scn.legacy.222222222222.unit", "legacy works")
	report, err := verifyScope(verifyRequest{
		root: root, scope: verificationScope{currentSpecs: true},
		mode: "implementation",
	})
	if err != nil {
		t.Fatal(err)
	}
	found := diagnosticsWithCode(report.Diagnostics, "LINK_REMOVED_BEHAVIOR_ANCHORED")
	if len(found) != 1 || found[0].Source.Path != "tests/legacy.test.mts" ||
		*found[0].IdentityID != "scn.legacy.222222222222.unit" || hasDiagnostic(report.Diagnostics, "ANCHOR_DANGLING") {
		t.Fatalf("retired anchors = %#v", report.Diagnostics)
	}
	change, err := verifyScope(verifyRequest{root: root, scope: changeScope("active"), mode: "implementation"})
	if err != nil || len(diagnosticsWithCode(change.Diagnostics, "LINK_REMOVED_BEHAVIOR_ANCHORED")) != 0 {
		t.Fatalf("a change scope reported retired anchors: %v, %#v", err, change.Diagnostics)
	}
}
