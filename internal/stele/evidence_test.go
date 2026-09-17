package stele

import (
	"strings"
	"testing"
)

const otherScenarioID = "scn.demo.cccccccccccc"

// approvedEvidenceFixture plans and approves a unit and an e2e entry for the
// first scenario and a unit entry for the second.
func approvedEvidenceFixture(t *testing.T) string {
	t.Helper()
	root := evidenceFixture(t)
	e2e := entry(evidenceScenarioID, "e2e", "The shipped binary composes the steps.")
	e2e.Placement = "tests/cli.test.mts, with the other executable tests"
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {
			approved(t, root, entry(evidenceScenarioID, "unit", "Pure logic.")),
			approved(t, root, e2e),
		},
		otherScenarioID: {approved(t, root, entry(otherScenarioID, "unit", "Pure logic."))},
	})
	return root
}

// @verifies scn.verificationstrategy.1b1070d81978.unit
func TestImplementationAcceptsEvidenceAnywhere(t *testing.T) {
	root := approvedEvidenceFixture(t)
	writeEvidenceTest(t, root, "scripts/checks/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/deep/journey/value.spec.ts", evidenceScenarioID+".e2e", "runs the journey")
	writeEvidenceTest(t, root, "src/demo.test.mts", otherScenarioID+".unit", "reports a missing value")
	report := verifyFixture(t, root, "implementation")
	if report.Verdict != "pass" || len(report.Diagnostics) != 0 {
		t.Fatalf("anchored evidence did not pass: %#v", report.Diagnostics)
	}
	scenario := report.Requirements[0].Scenarios[0]
	if scenario.Linkage != "linked" || len(scenario.TestLinks) != 2 {
		t.Fatalf("scenario report = %#v", scenario)
	}
	locations := map[string]string{}
	for _, link := range scenario.TestLinks {
		locations[link.EvidenceID] = link.Path + "#" + pointerValue(link.Selector) + "@" + link.Level
	}
	if locations[evidenceScenarioID+".unit"] != "scripts/checks/value.test.mts#returns the value@unit" ||
		locations[evidenceScenarioID+".e2e"] != "tests/deep/journey/value.spec.ts#runs the journey@e2e" {
		t.Fatalf("evidence locations = %#v", locations)
	}
	if report.Summary.LinkedScenarios != 2 || report.Summary.LinkedRequirements != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

// @verifies scn.verificationstrategy.2bcfb4ad1dbd.unit
func TestImplementationReportsMissingEvidence(t *testing.T) {
	root := approvedEvidenceFixture(t)
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	// An anchor without a named test below it does not count.
	writeFixture(t, root, "tests/journey.test.mts", "// @verifies "+evidenceScenarioID+".e2e\nconst journey = 1;\n")
	plan := readEvidencePlan(t, root, changePlanPath)
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: plan.Scenarios[evidenceScenarioID].Evidence,
		otherScenarioID: {
			plan.Scenarios[otherScenarioID].Evidence[0],
			entry(otherScenarioID, "e2e", "Not approved yet."),
		},
	})
	report := verifyFixture(t, root, "implementation")
	missing := diagnosticsWithCode(report.Diagnostics, "LINK_EVIDENCE_MISSING")
	identities := make([]string, 0, len(missing))
	for _, item := range missing {
		identities = append(identities, identityOf(item))
	}
	if strings.Join(identities, ",") != evidenceScenarioID+".e2e,"+otherScenarioID+".unit" {
		t.Fatalf("LINK_EVIDENCE_MISSING for %v", identities)
	}
	if report.Verdict != "fail" || report.Requirements[0].Scenarios[0].Linkage != "missing" {
		t.Fatalf("report = %s, %s", report.Verdict, report.Requirements[0].Scenarios[0].Linkage)
	}
}

// @verifies scn.verificationstrategy.6d8dbaecdc2f.unit
func TestImplementationRejectsUnplannedEvidence(t *testing.T) {
	root := approvedEvidenceFixture(t)
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/journey.test.mts", evidenceScenarioID+".e2e", "runs the journey")
	writeEvidenceTest(t, root, "tests/other.test.mts", otherScenarioID+".unit", "reports a missing value")
	writeEvidenceTest(t, root, "tests/store.test.mts", evidenceScenarioID+".integration", "uses the store")
	writeEvidenceTest(t, root, "tests/bare.test.mts", evidenceScenarioID, "has no level")
	writeFixture(t, root, "src/demo.mts", "export function value() { return 1; }\n")
	report := verifyFixture(t, root, "implementation")
	unplanned := diagnosticsWithCode(report.Diagnostics, "ANCHOR_EVIDENCE_UNPLANNED")
	if len(unplanned) != 2 || identityOf(unplanned[0]) != evidenceScenarioID ||
		identityOf(unplanned[1]) != evidenceScenarioID+".integration" ||
		unplanned[1].Source.Path != "tests/store.test.mts" {
		t.Fatalf("ANCHOR_EVIDENCE_UNPLANNED = %#v", unplanned)
	}
	if !hasDiagnostic(report.Diagnostics, "LINK_CODE_MISSING") {
		t.Fatalf("a requirement without @implements passed: %#v", report.Diagnostics)
	}
	if proposal := verifyFixture(t, root, "proposal"); proposal.Verdict != "pass" {
		t.Fatalf("the proposal stage judged anchors: %#v", proposal.Diagnostics)
	}
}

// @verifies scn.verify.1d0f8685d8c6.unit
func TestV2VerificationIgnoresLocation(t *testing.T) {
	root := approvedEvidenceFixture(t)
	writeEvidenceTest(t, root, "tests/elsewhere/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/elsewhere/journey.test.mts", evidenceScenarioID+".e2e", "runs the journey")
	writeEvidenceTest(t, root, "tests/elsewhere/other.test.mts", otherScenarioID+".unit", "reports a missing value")
	writeFixture(t, root, "src/moved/demo.mts",
		"// @implements "+"req.demo.aaaaaaaaaaaa\nexport function renamed() { return 1; }\n")
	writeFixture(t, root, "src/demo.mts", "export const nothing = 0;\n")
	for _, stage := range []string{"proposal", "implementation"} {
		report := verifyFixture(t, root, stage)
		if report.Verdict != "pass" || len(report.Diagnostics) != 0 {
			t.Fatalf("%s: location affected v2 verification: %#v", stage, report.Diagnostics)
		}
	}
}
