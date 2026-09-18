package stele

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// published returns the latest diagnostics published for a file, and whether
// any were published, after the server handled everything sent so far.
func published(t *testing.T, client *lspTestClient, root, relative string) ([]lspDiagnostic, bool) {
	t.Helper()
	client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, "flush.mts"))},
	})
	uri := fileURI(filepath.Join(root, relative))
	var latest []lspDiagnostic
	found := false
	for _, message := range client.received {
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var params lspPublishDiagnosticsParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.URI == uri {
			latest, found = params.Diagnostics, true
		}
	}
	return latest, found
}

func withCode(diagnostics []lspDiagnostic, code string) []lspDiagnostic {
	found := make([]lspDiagnostic, 0)
	for _, item := range diagnostics {
		if item.Code == code {
			found = append(found, item)
		}
	}
	return found
}

// @verifies scn.languageserver.e6a664740343.unit
func TestLSPFlagsAnUnknownIDInAnAnchor(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "src/todo.mts", "export const first = 1;\n// @implements "+
		"req.todo.000000000000\nexport function todo() {}\n")
	client := openLSP(t, root, lspCapabilities())
	diagnostics, _ := published(t, client, root, "src/todo.mts")
	report, err := verifyScope(verifyRequest{root: root, scope: changeScope("example"), mode: "implementation"})
	if err != nil {
		t.Fatal(err)
	}
	cli := diagnosticsWithCode(report.Diagnostics, "ANCHOR_DANGLING")
	if len(diagnostics) != 1 || len(cli) != 1 || diagnostics[0].Code != cli[0].Code ||
		diagnostics[0].Severity != lspSeverityError || diagnostics[0].Range.Start.Line != 1 ||
		diagnostics[0].Source != "stele" || !strings.HasPrefix(diagnostics[0].Message, cli[0].Message+"\n") {
		t.Fatalf("diagnostics = %#v, CLI = %#v", diagnostics, cli)
	}
}

// @verifies scn.languageserver.e33a998bdf85.unit
func TestLSPFlagsApprovedEvidenceWithoutATest(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/e2e/value.test.mts", "export const nothing = 1;\n")
	client := openLSP(t, root, lspCapabilities())
	diagnostics, _ := published(t, client, root, lspChangeSpec)
	missing := withCode(diagnostics, "LINK_EVIDENCE_MISSING")
	report, err := verifyScope(verifyRequest{root: root, scope: changeScope("example"), mode: "implementation"})
	if err != nil {
		t.Fatal(err)
	}
	cli := diagnosticsWithCode(report.Diagnostics, "LINK_EVIDENCE_MISSING")
	if len(missing) != 1 || len(cli) != 1 || missing[0].Range.Start.Line != 8 ||
		missing[0].Severity != lspSeverity(cli[0].Severity) ||
		!strings.Contains(missing[0].Message, evidenceScenarioID+".e2e") {
		t.Fatalf("diagnostics = %#v, CLI = %#v", diagnostics, cli)
	}
	if len(withCode(diagnostics, "PLAN_UNAPPROVED")) != 0 {
		t.Fatalf("PLAN_UNAPPROVED was published: %#v", diagnostics)
	}
}

// @verifies scn.languageserver.72a3e3ef9ecb.unit
func TestLSPFlagsStaleResults(t *testing.T) {
	root := lspFixture(t)
	lspStoredEvidence(t, root, "passed", "an-older-digest")
	client := openLSP(t, root, lspCapabilities())
	heading, _ := published(t, client, root, lspChangeSpec)
	test, _ := published(t, client, root, "tests/value.test.mts")
	onHeading, onTest := withCode(heading, "EXECUTION_STALE"), withCode(test, "EXECUTION_STALE")
	for _, diagnostics := range [][]lspDiagnostic{onHeading, onTest} {
		if len(diagnostics) != 1 || diagnostics[0].Severity != lspSeverityInformation ||
			!strings.Contains(diagnostics[0].Message, "is stale") {
			t.Fatalf("stale diagnostics = %#v / %#v", heading, test)
		}
	}
	if onHeading[0].Range.Start.Line != 8 || onTest[0].Range.Start.Line != 1 {
		t.Fatalf("stale diagnostic lines = %#v / %#v", heading, test)
	}
}

// @verifies scn.languageserver.106cfbc87c5b.unit
func TestLSPClearsAFixedProblem(t *testing.T) {
	root := lspFixture(t)
	source := "// @implements req.demo.000000000000\nexport function value() { return 1; }\n"
	writeFixture(t, root, "src/demo.mts", source)
	client := openLSP(t, root, lspCapabilities())
	uri := fileURI(filepath.Join(root, "src/demo.mts"))
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "typescript", "version": 1, "text": source,
	}})
	diagnostics, _ := published(t, client, root, "src/demo.mts")
	if len(withCode(diagnostics, "ANCHOR_DANGLING")) != 1 {
		t.Fatalf("before the fix = %#v", diagnostics)
	}
	client.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []map[string]string{{"text": strings.Replace(source, "000000000000", "aaaaaaaaaaaa", 1)}},
	})
	if diagnostics, found := published(t, client, root, "src/demo.mts"); !found || len(diagnostics) != 0 {
		t.Fatalf("after the fix = %#v, %v", diagnostics, found)
	}
}

// @verifies scn.languageserver.3efdcf7c5cf2.unit
func TestLSPExplainsAFindingWithTheCatalogue(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/e2e/value.test.mts", "export const nothing = 1;\n")
	client := openLSP(t, root, lspCapabilities())
	diagnostics, _ := published(t, client, root, lspChangeSpec)
	missing := withCode(diagnostics, "LINK_EVIDENCE_MISSING")
	guide := diagnosticGuides["LINK_EVIDENCE_MISSING"]
	report, err := verifyScope(verifyRequest{root: root, scope: changeScope("example"), mode: "implementation"})
	if err != nil {
		t.Fatal(err)
	}
	finding := diagnosticsWithCode(report.Diagnostics, "LINK_EVIDENCE_MISSING")[0]
	want := finding.Message + "\n" + guide.meaning + "\nFix: " + guide.fixFor("--change example")
	if len(missing) != 1 || missing[0].Message != want {
		t.Fatalf("message = %#v, want %q", missing, want)
	}
	stale := lspFindings(Index{Scenarios: []IndexScenario{{
		ID: "scn.x", Scope: "example", Source: Source{Path: "a.md", Line: 3},
		Evidence: []IndexEvidence{{ID: "scn.x.unit", Execution: ExecutionState{State: "stale", Outcome: "failed"}}},
	}}}, nil)
	if messages := stale["a.md"]; len(messages) != 2 ||
		!strings.Contains(messages[0].message,
			"Fix: fix the code or the test, then run `stele test <evidence-id> --change example`") ||
		!strings.Contains(messages[1].message, "Fix: run `stele test <evidence-id> --change example`") {
		t.Fatalf("execution findings = %#v", messages)
	}
}

// @verifies scn.languageserver.2d35637c302f.unit
func TestLSPShowsAnnotationProblemsInASpecification(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, lspChangeSpec, strings.TrimPrefix(evidenceSpec, "<!-- stele: spec v1 -->\n"))
	writeFixture(t, root, "openspec/changes/example/specs/other/spec.md",
		"<!-- stele: spec v2 -->\n## ADDED Requirements\n")
	client := openLSP(t, root, lspCapabilities())
	missing, _ := published(t, client, root, lspChangeSpec)
	unsupported, _ := published(t, client, root, "openspec/changes/example/specs/other/spec.md")
	if found := withCode(missing, "SPEC_ANNOTATION_MISSING"); len(found) != 1 || found[0].Range.Start.Line != 0 ||
		found[0].Severity != lspSeverityWarning ||
		!strings.Contains(found[0].Message, "Fix: run `stele annotate --change example`") {
		t.Fatalf("missing annotation = %#v", missing)
	}
	found := withCode(unsupported, "SPEC_ANNOTATION_UNSUPPORTED")
	if len(found) != 1 || found[0].Range.Start.Line != 0 || found[0].Severity != lspSeverityError {
		t.Fatalf("unsupported annotation = %#v", unsupported)
	}
}

// @verifies scn.languageserver.8431050348d1.unit
func TestLSPFlagsATestAnchoredToRemovedBehavior(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/specs/export/spec.md", `<!-- stele: spec v1 -->
### Requirement: Export todos
Verification-ID: req.export.dddddddddddd

The system SHALL export todos.

#### Scenario: Export as JSON
Verification-ID: scn.export.eeeeeeeeeeee

- **WHEN** the user exports
- **THEN** a JSON file is written
`)
	writeFixture(t, root, "openspec/changes/example/specs/export/spec.md",
		"<!-- stele: spec v1 -->\n## REMOVED Requirements\n\n### Requirement: Export todos\n")
	writeEvidenceTest(t, root, "tests/export.test.mts", "scn.export.eeeeeeeeeeee.unit", "exports JSON")
	client := openLSP(t, root, lspCapabilities())
	diagnostics, _ := published(t, client, root, "tests/export.test.mts")
	if found := withCode(diagnostics, "LINK_REMOVED_BEHAVIOR_ANCHORED"); len(found) != 1 ||
		found[0].Range.Start.Line != 1 || found[0].Severity != lspSeverityError {
		t.Fatalf("removed behavior = %#v", diagnostics)
	}
}

// @verifies scn.languageserver.5df656e62280.unit
func TestLSPChecksAChangeInPlanningAtTheProposalStage(t *testing.T) {
	root := evidenceFixture(t)
	writeFixture(t, root, "src/demo.mts", "export function value() { return 1; }\n")
	plan := map[string][]EvidenceEntry{
		evidenceScenarioID: {entry(evidenceScenarioID, "unit", "Pure logic.")},
		otherScenarioID:    {entry(otherScenarioID, "unit", "Pure logic.")},
	}
	writeEvidencePlan(t, root, plan)
	client := openLSP(t, root, lspCapabilities())
	before, _ := published(t, client, root, lspChangeSpec)
	if len(withCode(before, "LINK_CODE_MISSING")) != 0 || len(withCode(before, "PLAN_UNAPPROVED")) != 0 {
		t.Fatalf("before approval = %#v", before)
	}
	plan[evidenceScenarioID] = []EvidenceEntry{approved(t, root, plan[evidenceScenarioID][0])}
	writeEvidencePlan(t, root, plan)
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{
		{"uri": fileURI(filepath.Join(root, "openspec/changes/example/linkage-plan.json")), "type": 2},
	}})
	after, _ := published(t, client, root, lspChangeSpec)
	if found := withCode(after, "LINK_CODE_MISSING"); len(found) != 1 || found[0].Range.Start.Line != 3 ||
		len(withCode(after, "PLAN_UNAPPROVED")) != 0 {
		t.Fatalf("after approval = %#v", after)
	}
}

func TestLSPFindingSources(t *testing.T) {
	index := Index{Requirements: []IndexRequirement{
		{ID: "req.a", Scope: "specs", Source: Source{Path: "a.md", Line: 2}},
	}}
	identity := "req.a"
	if path, line, found := lspFindingSource(index, "specs", Diagnostic{IdentityID: &identity}); !found ||
		path != "a.md" || line != 2 {
		t.Fatalf("identity source = %s:%d", path, line)
	}
	if _, _, found := lspFindingSource(index, "specs", Diagnostic{}); found {
		t.Fatal("a finding without a place was placed")
	}
	scenarioID := evidenceScenarioID + ".unit"
	withScenario := Index{Scenarios: []IndexScenario{
		{ID: evidenceScenarioID, Scope: "specs", Source: Source{Path: "b.md", Line: 4}},
	}}
	if path, line, found := lspFindingSource(withScenario, "specs", Diagnostic{IdentityID: &scenarioID}); !found ||
		path != "b.md" || line != 4 {
		t.Fatalf("scenario source = %s:%d", path, line)
	}
	other := "scn.b.unit"
	if _, _, found := lspFindingSource(index, "specs", Diagnostic{IdentityID: &other}); found {
		t.Fatal("an unknown identity was placed")
	}
	check := lspScopeCheck{name: "specs", flag: "--specs", report: Report{Diagnostics: []Diagnostic{
		{Code: "PLAN_UNAPPROVED", Severity: "warning", IdentityID: &identity},
		{Code: "LINK_CODE_MISSING", Severity: "error", Message: "m", IdentityID: &identity},
		{Code: "PLAN_ARCHIVE_ORDER_AMBIGUOUS", Severity: "warning", Message: "n", IdentityID: &identity},
		{Code: "LINK_CODE_MISSING", Severity: "error", Message: "m", IdentityID: &identity},
	}}}
	findings := lspFindings(index, []lspScopeCheck{check})
	if len(findings["a.md"]) != 2 || findings["a.md"][0].code != "LINK_CODE_MISSING" ||
		findings["a.md"][1].severity != lspSeverityWarning {
		t.Fatalf("findings = %#v", findings)
	}
}

// failingFiles fails to read files with a suffix after some reads succeeded.
type failingFiles struct {
	repoFiles
	suffix string
	after  int
	reads  *int
}

func (files failingFiles) readFile(path string) ([]byte, error) {
	if strings.HasSuffix(path, files.suffix) {
		*files.reads++
		if *files.reads > files.after {
			return nil, errors.New("read failed")
		}
	}
	return files.repoFiles.readFile(path)
}

func TestLSPBuildErrors(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/changes/empty/proposal.md", "# Empty\n")
	files := newLSPFiles(root)
	build, err := buildLSPViews(root, files.view(true), files.view(false))
	if err != nil || len(build.index.Scopes) != 2 || build.index.Scopes[0].ID != "empty" {
		t.Fatalf("build = %#v, %v", build, err)
	}
	writeFixture(t, root, "src/broken.go", "package broken\n\nfunc (\n")
	if _, err := buildLSPProject(newLSPFiles(root)); err == nil || !strings.Contains(err.Error(), "broken.go") {
		t.Fatalf("a saved file that does not parse = %v", err)
	}
	for _, failing := range []struct {
		view, saved repoFiles
	}{
		{failingFiles{repoFiles: files.view(true), suffix: "src/demo.mts", reads: new(int)}, files.view(false)},
		{files.view(true), failingFiles{repoFiles: files.view(false), suffix: "package.json", reads: new(int)}},
		{
			failingFiles{repoFiles: files.view(true), suffix: "demo/spec.md", after: 1, reads: new(int)},
			files.view(false),
		},
	} {
		if _, err := buildLSPViews(root, failing.view, failing.saved); err == nil {
			t.Fatal("a read failure did not fail the build")
		}
	}
}
