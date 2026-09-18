package stele

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const evidenceScenarioID = "scn.demo.bbbbbbbbbbbb"

const evidenceSpec = `## ADDED Requirements

### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa

The system SHALL return the stored value.

#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb

- **WHEN** a value is stored
- **THEN** it is returned

#### Scenario: Missing value is reported
Verification-ID: scn.demo.cccccccccccc

- **WHEN** no value is stored
- **THEN** an error is reported
`

// evidenceFixture returns a change "example" with two scenarios and an
// implementation anchor, without a plan or tests.
func evidenceFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", evidenceSpec)
	writeFixture(t, root, "src/demo.mts",
		"// @implements "+"req.demo.aaaaaaaaaaaa\nexport function value() { return 1; }\n")
	writeFixture(t, root, "package.json", "{\"type\":\"module\"}\n")
	return root
}

func fixtureScenario(t *testing.T, root, id string) Scenario {
	t.Helper()
	parsed, err := ParseSpecs(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			if scenario.ID == id {
				return scenario
			}
		}
	}
	t.Fatalf("scenario %s not found", id)
	return Scenario{}
}

func entry(scenarioID, level, rationale string) EvidenceEntry {
	return EvidenceEntry{ID: evidenceID(scenarioID, level), Level: level, Rationale: rationale}
}

// approved returns the entry with an approval for the fixture's current text.
func approved(t *testing.T, root string, value EvidenceEntry) EvidenceEntry {
	t.Helper()
	scenario := fixtureScenario(t, root, value.ID[:len(evidenceScenarioID)])
	value.Approval = &EvidenceApproval{
		Approver: "Reviewer",
		Date:     "2026-01-01",
		Digest:   evidenceDigest(scenario, value),
		Via:      "cli",
	}
	return value
}

func writeEvidencePlan(t *testing.T, root string, scenarios map[string][]EvidenceEntry) {
	t.Helper()
	file := evidencePlanFile{SchemaVersion: 2, ChangeID: "example", Scenarios: map[string]ScenarioEvidence{}}
	for id, entries := range scenarios {
		file.Scenarios[id] = ScenarioEvidence{Evidence: entries}
	}
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/changes/example/linkage-plan.json", string(content)+"\n")
}

func readEvidencePlan(t *testing.T, root, relative string) evidencePlanFile {
	t.Helper()
	var file evidencePlanFile
	if err := json.Unmarshal([]byte(readTestFile(t, root, relative)), &file); err != nil {
		t.Fatal(err)
	}
	return file
}

// writeEvidenceTest writes a named node test carrying the given anchor.
func writeEvidenceTest(t *testing.T, root, relative, anchor, title string) {
	t.Helper()
	writeFixture(t, root, relative, "import test from \"node:test\";\n"+
		"// @verifies "+anchor+"\n"+
		"void test(\""+title+"\", () => {});\n")
}

func diagnosticsWithCode(diagnostics []Diagnostic, code string) []Diagnostic {
	found := make([]Diagnostic, 0)
	for _, item := range diagnostics {
		if item.Code == code {
			found = append(found, item)
		}
	}
	return found
}

func identityOf(item Diagnostic) string {
	if item.IdentityID == nil {
		return ""
	}
	return *item.IdentityID
}

func verifyFixture(t *testing.T, root, stage string) Report {
	t.Helper()
	report, err := RunVerification(root, "example", stage, "")
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// @verifies scn.verificationstrategy.2779f7f381d3.unit
func TestProposalAcceptsApprovedEvidencePlan(t *testing.T) {
	root := evidenceFixture(t)
	missing := "scn.demo.cccccccccccc"
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {
			approved(t, root, entry(evidenceScenarioID, "unit", "Pure logic.")),
			approved(t, root, EvidenceEntry{
				ID:        evidenceScenarioID + ".e2e",
				Level:     "e2e",
				Rationale: "The shipped binary composes the steps.",
				Placement: "tests/cli.test.mts",
			}),
		},
		missing: {approved(t, root, entry(missing, "integration", "Needs the real store."))},
	})
	report := verifyFixture(t, root, "proposal")
	if report.Verdicts.Linkage != "pass" || len(report.Diagnostics) != 0 || report.Stages.Proposal.Status != "pass" {
		t.Fatalf("approved plan did not pass: %#v", report.Diagnostics)
	}
	requirement := report.Requirements[0]
	if requirement.Linkage != "planned" {
		t.Fatalf("requirement linkage = %s", requirement.Linkage)
	}
	scenario := requirement.Scenarios[0]
	if scenario.Linkage != "planned" || len(scenario.Evidence) != 2 || len(scenario.TestLinks) != 2 {
		t.Fatalf("scenario report = %#v", scenario)
	}
	want := []PlannedEvidence{
		{ID: evidenceScenarioID + ".unit", Level: "unit", Approval: "approved"},
		{ID: evidenceScenarioID + ".e2e", Level: "e2e", Approval: "approved", Placement: "tests/cli.test.mts"},
	}
	for index, planned := range want {
		if scenario.Evidence[index] != planned {
			t.Fatalf("evidence %d = %#v, want %#v", index, scenario.Evidence[index], planned)
		}
		link := scenario.TestLinks[index]
		if link.EvidenceID != planned.ID || link.Level != planned.Level || link.State != "planned" {
			t.Fatalf("link %d = %#v", index, link)
		}
	}
	content, err := MarshalDeterministic(report)
	if err != nil || !strings.Contains(string(content), `"evidenceId": "scn.demo.bbbbbbbbbbbb.e2e"`) {
		t.Fatalf("report JSON lacks evidence IDs: %v", err)
	}
}

func TestParseLinkagePlanRejectsMalformedFiles(t *testing.T) {
	for name, content := range map[string]string{
		"syntax":      "{",
		"v2 scenario": `{"schemaVersion":2,"scenarios":{"scn.demo.bbbbbbbbbbbb":"tests/a.test.mts#a"}}`,
		"v1 scenario": `{"schemaVersion":1,"scenarios":{"scn.demo.bbbbbbbbbbbb":{"evidence":[]}}}`,
	} {
		if _, err := parseLinkagePlan([]byte(content)); !errors.Is(err, errInvalidPlan) {
			t.Fatalf("%s: expected an invalid plan error, got %v", name, err)
		}
	}
	if _, _, valid := splitEvidenceID("scn.demo.bbbbbbbbbbbb", "scn.demo.cccccccccccc.unit"); valid {
		t.Fatal("an evidence ID of another scenario was accepted")
	}
	if _, err := readLinkagePlan(filepath.Join(fixtureRoot(t), "missing.json")); err == nil {
		t.Fatal("expected a read error")
	}
	plan, err := parseLinkagePlan([]byte(`{"requirements":{"req.demo.aaaaaaaaaaaa":"src/a.mts#a"}}`))
	if err != nil || plan.Requirements["req.demo.aaaaaaaaaaaa"] != "src/a.mts#a" || plan.EvidenceOnly {
		t.Fatalf("v1 plan = %#v, %v", plan, err)
	}
}

// @verifies scn.verificationstrategy.45a816cfbf7f.unit
func TestProposalRejectsMalformedEvidence(t *testing.T) {
	valid := func(level string) EvidenceEntry { return entry(evidenceScenarioID, level, "A reason.") }
	withApproval := func(approval EvidenceApproval) EvidenceEntry {
		value := valid("unit")
		value.Approval = &approval
		return value
	}
	cases := map[string]EvidenceEntry{
		"unknown level":       {ID: evidenceScenarioID + ".smoke", Level: "smoke", Rationale: "A reason."},
		"level mismatch":      {ID: evidenceScenarioID + ".e2e", Level: "unit", Rationale: "A reason."},
		"first ordinal":       {ID: evidenceScenarioID + ".unit.1", Level: "unit", Rationale: "A reason."},
		"malformed ordinal":   {ID: evidenceScenarioID + ".unit.x", Level: "unit", Rationale: "A reason."},
		"missing rationale":   {ID: evidenceScenarioID + ".unit", Level: "unit", Rationale: "  "},
		"approval via editor": withApproval(EvidenceApproval{Approver: "A", Date: "2026-01-01", Via: "editor"}),
		"approval approver":   withApproval(EvidenceApproval{Date: "2026-01-01", Via: "cli"}),
		"approval date":       withApproval(EvidenceApproval{Approver: "A", Via: "cli"}),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			root := evidenceFixture(t)
			writeEvidencePlan(t, root, map[string][]EvidenceEntry{
				evidenceScenarioID:      {value},
				"scn.demo.cccccccccccc": {entry("scn.demo.cccccccccccc", "unit", "A reason.")},
			})
			invalid := diagnosticsWithCode(verifyFixture(t, root, "proposal").Diagnostics, "PLAN_EVIDENCE_INVALID")
			if len(invalid) != 1 || identityOf(invalid[0]) != value.ID {
				t.Fatalf("PLAN_EVIDENCE_INVALID = %#v", invalid)
			}
		})
	}

	root := evidenceFixture(t)
	second := valid("unit")
	second.ID += ".2"
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {valid("unit"), valid("unit"), second},
	})
	invalid := diagnosticsWithCode(verifyFixture(t, root, "proposal").Diagnostics, "PLAN_EVIDENCE_INVALID")
	if len(invalid) != 1 || !strings.Contains(invalid[0].Message, "duplicate") {
		t.Fatalf("duplicate entries = %#v", invalid)
	}
}

// @verifies scn.verificationstrategy.aef528d23484.unit
func TestProposalRequiresEvidencePerScenario(t *testing.T) {
	root := evidenceFixture(t)
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {},
	})
	for _, stage := range []string{"proposal", "implementation"} {
		report := verifyFixture(t, root, stage)
		missing := diagnosticsWithCode(report.Diagnostics, "PLAN_EVIDENCE_MISSING")
		if report.Verdicts.Linkage != "fail" || len(missing) != 2 {
			t.Fatalf("%s: PLAN_EVIDENCE_MISSING = %#v", stage, missing)
		}
		for _, scenario := range report.Requirements[0].Scenarios {
			if scenario.Linkage != "missing" {
				t.Fatalf("%s: scenario %s linkage = %s", stage, scenario.ID, scenario.Linkage)
			}
		}
	}
}

// @verifies scn.verificationstrategy.509b8e9fab98.unit
func TestProposalRejectsUnknownPlanIDs(t *testing.T) {
	root := evidenceFixture(t)
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID:      {entry("scn.demo.cccccccccccc", "unit", "Listed under the wrong scenario.")},
		"scn.demo.cccccccccccc": {entry("scn.demo.cccccccccccc", "unit", "A reason.")},
		"scn.demo.ffffffffffff": {entry("scn.demo.ffffffffffff", "unit", "Not declared.")},
		"req.demo.aaaaaaaaaaaa": {entry("req.demo.aaaaaaaaaaaa", "unit", "Not a scenario.")},
	})
	unknown := diagnosticsWithCode(verifyFixture(t, root, "proposal").Diagnostics, "PLAN_UNKNOWN_ID")
	identities := make([]string, 0, len(unknown))
	for _, item := range unknown {
		identities = append(identities, identityOf(item))
	}
	sort.Strings(identities)
	want := "req.demo.aaaaaaaaaaaa,scn.demo.cccccccccccc.unit,scn.demo.ffffffffffff"
	if strings.Join(identities, ",") != want {
		t.Fatalf("PLAN_UNKNOWN_ID for %v, want %s", identities, want)
	}
}

// @verifies scn.verificationstrategy.6eea05b9b024.unit
func TestV1PlanStillVerifiesWithWarning(t *testing.T) {
	root := completeFixture(t, false)
	report := verifyFixture(t, root, "implementation")
	warnings := diagnosticsWithCode(report.Diagnostics, "PLAN_V1_DEPRECATED")
	if report.Verdicts.Linkage != "pass" || len(warnings) != 1 || warnings[0].Severity != "warning" ||
		!strings.Contains(warnings[0].Message, "0.2.0") || report.Summary.Warnings != 1 {
		t.Fatalf("v1 plan report = %s, %#v", report.Verdicts.Linkage, report.Diagnostics)
	}
	proposal := verifyFixture(t, root, "proposal")
	if proposal.Verdicts.Linkage != "pass" || proposal.Stages.Proposal.Status != "pass" ||
		proposal.Requirements[0].Scenarios[0].Evidence != nil {
		t.Fatalf("v1 proposal = %#v", proposal)
	}
	writeFixture(t, root, "artifacts/linkage-plan.json",
		`{"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#other"},"scenarios":{}}`)
	mismatch := verifyFixture(t, root, "implementation")
	if !hasDiagnostic(mismatch.Diagnostics, "LINK_TARGET_MISMATCH") {
		t.Fatalf("v1 target rules no longer apply: %#v", mismatch.Diagnostics)
	}
}

func TestArchivedPlansMixSchemaVersions(t *testing.T) {
	root := archivedFixture(t)
	scenario := "scn.demo.bbbbbbbbbbbb"
	v2 := `{"schemaVersion":2,"scenarios":{"` + scenario + `":{"evidence":[{"id":"` + scenario +
		`.unit","level":"unit","rationale":"r"}]}}}`
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-example/linkage-plan.json", scopePlanForExample)
	writeFixture(t, root, "openspec/changes/archive/2026-02-01-rework/linkage-plan.json", v2)
	plan, diagnostics := loadArchivedPlans(root, verificationScope{currentSpecs: true})
	if !plan.usesEvidence(scenario) || plan.Scenarios[scenario] != "" || len(diagnostics) != 1 ||
		diagnostics[0].Source.Path != "openspec/changes/archive/2026-01-01-example/linkage-plan.json" {
		t.Fatalf("later v2 plan did not win: %#v, %#v", plan, diagnostics)
	}
	requirement := Requirement{ID: "req.demo.aaaaaaaaaaaa", Scenarios: []Scenario{{ID: scenario}}}
	if plan.requirementUsesEvidence(requirement) {
		t.Fatal("a requirement with a v1 target must keep the v1 rules")
	}
	if !plan.requirementUsesEvidence(Requirement{ID: "req.demo.dddddddddddd", Scenarios: []Scenario{{ID: scenario}}}) ||
		plan.requirementUsesEvidence(Requirement{ID: "req.demo.dddddddddddd"}) {
		t.Fatal("requirement rules do not follow their scenarios")
	}

	writeFixture(t, root, "openspec/changes/archive/2026-03-01-revert/linkage-plan.json", scopePlanForExample)
	plan, diagnostics = loadArchivedPlans(root, verificationScope{currentSpecs: true})
	if plan.usesEvidence(scenario) || len(diagnostics) != 2 {
		t.Fatalf("later v1 plan did not win: %#v", plan)
	}
	revert := filepath.Join(root, "openspec/changes/archive/2026-03-01-revert/linkage-plan.json")
	if err := os.Remove(revert); err != nil {
		t.Fatal(err)
	}
	report, err := verifyScope(verifyRequest{
		root: root, scope: verificationScope{currentSpecs: true}, mode: "proposal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report.Diagnostics, "PLAN_UNAPPROVED") || hasDiagnostic(report.Diagnostics, "PLAN_UNKNOWN_ID") {
		t.Fatalf("mixed archive diagnostics = %#v", report.Diagnostics)
	}
}
