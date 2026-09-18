package stele

import (
	"strings"
	"testing"
)

// removedFixture has a current todo specification with one requirement and
// two scenarios, and a change that removes that requirement.
func removedFixture(t *testing.T, removal string) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/todo/spec.md", `<!-- stele: spec v1 -->
## Requirements
### Requirement: Export todos
Verification-ID: req.todo.aaaaaaaaaaaa
#### Scenario: Export as CSV
Verification-ID: scn.todo.bbbbbbbbbbbb
#### Scenario: Export as JSON
Verification-ID: scn.todo.cccccccccccc
`)
	writeFixture(t, root, "openspec/changes/drop/specs/todo/spec.md", `<!-- stele: spec v1 -->
## ADDED Requirements
### Requirement: Print todos
Verification-ID: req.todo.dddddddddddd
#### Scenario: Print a list
Verification-ID: scn.todo.eeeeeeeeeeee

## REMOVED Requirements
`+removal+`
**Reason**: Printing replaces exporting.
**Migration**: Print instead.
`)
	writeFixture(t, root, "src/print.mts", "// @implements "+"req.todo.dddddddddddd\nexport function print() {}\n")
	writeFixture(t, root, "openspec/changes/drop/linkage-plan.json", `{"schemaVersion":2,"changeId":"drop",
"scenarios":{"scn.todo.eeeeeeeeeeee":{"evidence":[{"id":"scn.todo.eeeeeeeeeeee.unit","level":"unit",
"rationale":"Pure."}]}}}`)
	writeEvidenceTest(t, root, "tests/print.test.mts", "scn.todo.eeeeeeeeeeee.unit", "prints")
	return root
}

func removedReport(t *testing.T, root, mode string) Report {
	t.Helper()
	report, err := verifyScope(verifyRequest{root: root, scope: changeScope("drop"), mode: mode})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func codesOf(report Report, code string) []Diagnostic {
	return diagnosticsWithCode(report.Diagnostics, code)
}

// @verifies scn.verify.5172c64aec19.unit
func TestRemovedBehaviorAnchorsAreReported(t *testing.T) {
	root := removedFixture(t, "### Requirement: Export todos")
	writeFixture(t, root, "src/export.mts", "// @implements "+"req.todo.aaaaaaaaaaaa\nexport function csv() {}\n")
	writeEvidenceTest(t, root, "tests/export.test.mts", "scn.todo.bbbbbbbbbbbb.unit.2", "exports csv")
	found := codesOf(removedReport(t, root, "implementation"), "LINK_REMOVED_BEHAVIOR_ANCHORED")
	if len(found) != 2 || found[0].Source.Path != "src/export.mts" || found[0].Source.Line != 1 ||
		found[1].Source.Path != "tests/export.test.mts" || *found[1].IdentityID != "scn.todo.bbbbbbbbbbbb.unit.2" ||
		!strings.Contains(found[1].Message, `"Export as CSV"`) {
		t.Fatalf("removed-behavior anchors = %#v", found)
	}
	if proposal := removedReport(t, root, "proposal"); len(codesOf(proposal, "LINK_REMOVED_BEHAVIOR_ANCHORED")) != 0 {
		t.Fatal("the proposal stage reported the code that still exists")
	}
}

// @verifies scn.verify.c03b05c1c11e.unit
func TestRemovedBehaviorWithoutAnchorsPasses(t *testing.T) {
	report := removedReport(t, removedFixture(t, "- `### Requirement: Export todos`"), "implementation")
	for _, code := range []string{
		"LINK_REMOVED_BEHAVIOR_ANCHORED", "PLAN_REMOVED_BEHAVIOR_PLANNED",
		"SPEC_REMOVED_UNMATCHED", "ID_REQUIREMENT_MISSING", "SCENARIO_MISSING",
	} {
		if len(codesOf(report, code)) != 0 {
			t.Fatalf("clean removal reported %s: %#v", code, report.Diagnostics)
		}
	}
	for _, item := range report.Diagnostics {
		if item.Code != "PLAN_UNAPPROVED" {
			t.Fatalf("clean removal reported %s: %s", item.Code, item.Message)
		}
	}
}

// @verifies scn.verify.42bc49e1f29f.unit
func TestRemovedBehaviorPlanEntriesAreReported(t *testing.T) {
	root := removedFixture(t, "### Requirement: Export todos")
	writeFixture(t, root, "openspec/changes/drop/linkage-plan.json", `{"schemaVersion":2,"changeId":"drop",
"scenarios":{"scn.todo.eeeeeeeeeeee":{"evidence":[{"id":"scn.todo.eeeeeeeeeeee.unit","level":"unit",
"rationale":"Pure."}]},"scn.todo.cccccccccccc":{"evidence":[{"id":"scn.todo.cccccccccccc.unit","level":"unit",
"rationale":"Pure."}]}}}`)
	report := removedReport(t, root, "proposal")
	found := codesOf(report, "PLAN_REMOVED_BEHAVIOR_PLANNED")
	if len(found) != 1 || *found[0].IdentityID != "scn.todo.cccccccccccc" ||
		len(codesOf(report, "PLAN_UNKNOWN_ID")) != 0 {
		t.Fatalf("plan diagnostics = %#v", report.Diagnostics)
	}
	v1 := LinkagePlan{
		Requirements: map[string]string{"req.todo.aaaaaaaaaaaa": "src/export.mts#csv"},
		Scenarios:    map[string]string{"scn.todo.bbbbbbbbbbbb": "tests/export.test.mts#csv"},
	}
	removed := map[string]removedIdentity{"req.todo.aaaaaaaaaaaa": {}, "scn.todo.bbbbbbbbbbbb": {}}
	if len(removedPlanDiagnostics(v1, removed)) != 2 {
		t.Fatal("version 1 plan entries for removed behavior were not reported")
	}
}

// @verifies scn.verify.70eaaa57724a.unit
func TestRemovedNameWithoutMatchIsRejected(t *testing.T) {
	root := removedFixture(t, "### Requirement: Export all todos")
	writeFixture(t, root, "openspec/changes/drop/specs/archive/spec.md", `<!-- stele: spec v1 -->
## REMOVED Requirements
### Requirement: Archive todos
`)
	report := removedReport(t, root, "proposal")
	found := codesOf(report, "SPEC_REMOVED_UNMATCHED")
	if len(found) != 2 || report.Verdicts.Linkage != "fail" ||
		!strings.Contains(found[0].Message, `"Archive todos"`) ||
		found[1].Source.Path != "openspec/changes/drop/specs/todo/spec.md" {
		t.Fatalf("unmatched removals = %#v", found)
	}
}

// @verifies scn.verify.ec83e3f24b0a.unit
func TestMovedBehaviorIsNotRemoved(t *testing.T) {
	root := removedFixture(t, "- `### Requirement: Export todos`")
	writeFixture(t, root, "openspec/changes/drop/specs/todo/spec.md", `<!-- stele: spec v1 -->
## REMOVED Requirements
- `+"`### Requirement: Export todos`"+`

## ADDED Requirements
### Requirement: Export the todo list
Verification-ID: req.todo.aaaaaaaaaaaa
#### Scenario: Export as CSV
Verification-ID: scn.todo.bbbbbbbbbbbb
#### Scenario: Export as JSON
Verification-ID: scn.todo.cccccccccccc
`)
	writeFixture(t, root, "src/export.mts", "// @implements "+"req.todo.aaaaaaaaaaaa\nexport function csv() {}\n")
	removed, diagnostics := removedBehavior(root, changeScope("drop"), mustParse(t, root))
	if len(removed) != 0 || len(diagnostics) != 0 {
		t.Fatalf("moved behavior counted as removed: %#v, %#v", removed, diagnostics)
	}
	current, _ := removedBehavior(root, verificationScope{currentSpecs: true}, mustParse(t, root))
	if len(current) != 0 {
		t.Fatal("the current specifications removed something")
	}
}

func mustParse(t *testing.T, root string) ParsedSpecs {
	t.Helper()
	parsed, err := parseScopeSpecs(root, changeScope("drop"))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
