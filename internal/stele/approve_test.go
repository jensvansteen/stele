package stele

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const changePlanPath = "openspec/changes/example/linkage-plan.json"

// stubApprovalEnvironment fixes the terminal state, input, approver, and date.
func stubApprovalEnvironment(t *testing.T, terminal bool, input string) {
	t.Helper()
	originalTerminal, originalInput := inputIsTerminal, standardInput
	originalUser, originalDate := gitUserName, approvalDate
	t.Cleanup(func() {
		inputIsTerminal, standardInput = originalTerminal, originalInput
		gitUserName, approvalDate = originalUser, originalDate
	})
	inputIsTerminal = func() bool { return terminal }
	standardInput = strings.NewReader(input)
	gitUserName = func(string) string { return "Jens" }
	approvalDate = func() string { return "2026-09-17" }
}

func runApprove(t *testing.T, root string, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"approve", "--root", root}, arguments...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func planEntries(t *testing.T, root, scenario string) []EvidenceEntry {
	t.Helper()
	return readEvidencePlan(t, root, changePlanPath).Scenarios[scenario].Evidence
}

// @verifies scn.verificationstrategy.7efc234ae03e.unit
func TestProposalReportsUnapprovedEntries(t *testing.T) {
	root := evidenceFixture(t)
	other := "scn.demo.cccccccccccc"
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {entry(evidenceScenarioID, "unit", "Pure logic.")},
		other:              {approved(t, root, entry(other, "unit", "Pure logic."))},
	})
	for _, stage := range []string{"proposal", "implementation"} {
		report := verifyFixture(t, root, stage)
		unapproved := diagnosticsWithCode(report.Diagnostics, "PLAN_UNAPPROVED")
		if report.Verdicts.Linkage != "fail" || len(unapproved) != 1 ||
			identityOf(unapproved[0]) != evidenceScenarioID+".unit" ||
			!strings.Contains(unapproved[0].Message, evidenceScenarioID+".unit") {
			t.Fatalf("%s: PLAN_UNAPPROVED = %#v", stage, unapproved)
		}
		if evidence := report.Requirements[0].Scenarios[0].Evidence; evidence[0].Approval != "unapproved" {
			t.Fatalf("%s: evidence = %#v", stage, evidence)
		}
	}
}

// @verifies scn.verificationstrategy.9f94fdeabd16.unit
func TestProposalReportsStaleApprovals(t *testing.T) {
	root := evidenceFixture(t)
	other := "scn.demo.cccccccccccc"
	unit := approved(t, root, entry(evidenceScenarioID, "unit", "Pure logic."))
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {unit},
		other:              {approved(t, root, entry(other, "unit", "Pure logic."))},
	})
	// Whitespace-only edits to the other scenario keep its approval.
	spacious := strings.Replace(evidenceSpec,
		"- **WHEN** no value is stored", "-   **WHEN**   no value\n  is stored", 1)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", spacious)
	if report := verifyFixture(t, root, "proposal"); report.Verdicts.Linkage != "pass" {
		t.Fatalf("whitespace made an approval stale: %#v", report.Diagnostics)
	}

	changes := map[string]func() (string, []EvidenceEntry){
		"rationale": func() (string, []EvidenceEntry) {
			changed := unit
			changed.Rationale = "Pure logic, reviewed again."
			return spacious, []EvidenceEntry{changed}
		},
		"level": func() (string, []EvidenceEntry) {
			changed := unit
			changed.ID, changed.Level = evidenceScenarioID+".e2e", "e2e"
			return spacious, []EvidenceEntry{changed}
		},
		"wording": func() (string, []EvidenceEntry) {
			return strings.Replace(spacious, "THEN** it is returned", "THEN** it is returned unchanged", 1),
				[]EvidenceEntry{unit}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			spec, entries := change()
			writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", spec)
			plan := readEvidencePlan(t, root, changePlanPath)
			writeEvidencePlan(t, root, map[string][]EvidenceEntry{
				evidenceScenarioID: entries,
				other:              plan.Scenarios[other].Evidence,
			})
			for _, stage := range []string{"proposal", "implementation"} {
				report := verifyFixture(t, root, stage)
				stale := diagnosticsWithCode(report.Diagnostics, "PLAN_APPROVAL_STALE")
				if len(stale) != 1 || identityOf(stale[0]) != entries[0].ID {
					t.Fatalf("%s: PLAN_APPROVAL_STALE = %#v", stage, report.Diagnostics)
				}
				if report.Requirements[0].Scenarios[0].Evidence[0].Approval != "stale" {
					t.Fatalf("%s: evidence state = %#v", stage, report.Requirements[0].Scenarios[0].Evidence)
				}
			}
		})
	}
}

// @verifies scn.verificationstrategy.122427afc2f9.unit
func TestApproveReviewsEntriesInteractively(t *testing.T) {
	root := evidenceFixture(t)
	integration := entry(evidenceScenarioID, "integration", "Needs the real store.")
	integration.Placement = "tests/store.test.mts"
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {
			entry(evidenceScenarioID, "unit", "Pure logic."),
			integration,
			entry(evidenceScenarioID, "e2e", "The shipped binary."),
		},
	})
	stubApprovalEnvironment(t, true, "maybe\na\nr\ns\n")
	code, stdout, stderr := runApprove(t, root, "--change", "example")
	if code != 0 {
		t.Fatalf("approve = %d, %q", code, stderr)
	}
	for _, shown := range []string{
		"Value is returned (unapproved)",
		"- **WHEN** a value is stored",
		"Level:     integration",
		"Rationale: Needs the real store.",
		"Placement: tests/store.test.mts (advisory)",
		"✓ approved " + evidenceScenarioID + ".unit by Jens (via cli)",
		"Rejected, revise these entries and ask again: " + evidenceScenarioID + ".integration",
	} {
		if !strings.Contains(stdout, shown) {
			t.Fatalf("output lacks %q:\n%s", shown, stdout)
		}
	}
	if strings.Count(stdout, "Approve, reject, or skip?") != 4 {
		t.Fatalf("expected a repeated question for the unclear answer:\n%s", stdout)
	}
	entries := planEntries(t, root, evidenceScenarioID)
	approval := entries[0].Approval
	if approval == nil || approval.Approver != "Jens" || approval.Date != "2026-09-17" || approval.Via != "cli" ||
		approval.Digest != evidenceDigest(fixtureScenario(t, root, evidenceScenarioID), entries[0]) ||
		entries[1].Approval != nil || entries[2].Approval != nil {
		t.Fatalf("recorded approvals = %#v", entries)
	}

	standardInput = strings.NewReader("")
	code, stdout, _ = runApprove(t, root, "--change", "example")
	if code != 0 || strings.Count(stdout, "Approve, reject, or skip?") != 2 ||
		strings.Contains(stdout, evidenceScenarioID+".unit\n") ||
		!strings.Contains(stdout, "No pending or stale evidence entries were decided.") {
		t.Fatalf("second review = %d:\n%s", code, stdout)
	}
}

// @verifies scn.verificationstrategy.1eaf664142c0.unit
func TestApproveRecordsConversationConfirmation(t *testing.T) {
	root := evidenceFixture(t)
	other := "scn.demo.cccccccccccc"
	stale := approved(t, root, entry(evidenceScenarioID, "e2e", "Old rationale."))
	stale.Rationale = "New rationale."
	current := approved(t, root, entry(other, "unit", "Pure logic."))
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {entry(evidenceScenarioID, "unit", "Pure logic."), stale},
		other:              {current, entry(other, "e2e", "Shipped binary.")},
	})
	stubApprovalEnvironment(t, false, "")

	code, stdout, stderr := runApprove(t, root, "--change", "example", "--confirmed-in-chat",
		"--scenario", evidenceScenarioID, "--by", "Maintainer")
	if code != 0 {
		t.Fatalf("approve = %d, %q", code, stderr)
	}
	for _, id := range []string{evidenceScenarioID + ".unit", evidenceScenarioID + ".e2e"} {
		if !strings.Contains(stdout, "✓ approved "+id+" by Maintainer (via agent-confirmed)") {
			t.Fatalf("output does not name %s:\n%s", id, stdout)
		}
	}
	for _, value := range planEntries(t, root, evidenceScenarioID) {
		if value.Approval == nil || value.Approval.Via != "agent-confirmed" || value.Approval.Approver != "Maintainer" {
			t.Fatalf("entry %s = %#v", value.ID, value.Approval)
		}
	}
	untouched := planEntries(t, root, other)
	if *untouched[0].Approval != *current.Approval || untouched[1].Approval != nil {
		t.Fatalf("unselected entries changed: %#v", untouched)
	}

	code, stdout, _ = runApprove(t, root, "--change", "example", "--confirmed-in-chat",
		"--evidence", other+".e2e,"+other+".unit")
	if code != 0 || !strings.Contains(stdout, "✓ approved "+other+".e2e by Jens") ||
		strings.Contains(stdout, other+".unit") {
		t.Fatalf("evidence selection = %d:\n%s", code, stdout)
	}
	if report := verifyFixture(t, root, "proposal"); report.Verdicts.Linkage != "pass" {
		t.Fatalf("approved plan does not pass: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationstrategy.4e5b236cc0c8.unit
func TestApproveRefusesWithoutConfirmation(t *testing.T) {
	root := evidenceFixture(t)
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {entry(evidenceScenarioID, "unit", "Pure logic.")},
	})
	original := readTestFile(t, root, changePlanPath)
	stubApprovalEnvironment(t, false, "a\n")
	for name, arguments := range map[string][]string{
		"no confirmation":         {"--change", "example"},
		"all without yes":         {"--change", "example", "--all"},
		"yes without all":         {"--change", "example", "--yes"},
		"chat combined with all":  {"--change", "example", "--all", "--yes", "--confirmed-in-chat"},
		"unknown option":          {"--change", "example", "--force"},
		"specs combined w change": {"--change", "example", "--specs", "--confirmed-in-chat"},
	} {
		code, stdout, stderr := runApprove(t, root, arguments...)
		if code != 2 || stdout != "" {
			t.Fatalf("%s: approve = %d, %q, %q", name, code, stdout, stderr)
		}
		if name == "no confirmation" && (!strings.Contains(stderr, "--confirmed-in-chat") ||
			!strings.Contains(stderr, "--all --yes") || !strings.Contains(stderr, "interactive terminal")) {
			t.Fatalf("refusal does not explain how to confirm: %q", stderr)
		}
	}
	if readTestFile(t, root, changePlanPath) != original {
		t.Fatal("a refused approval changed the plan")
	}

	gitUserName = func(string) string { return "" }
	if code, _, stderr := runApprove(t, root, "--change", "example", "--all", "--yes"); code != 2 ||
		!strings.Contains(stderr, "--by") {
		t.Fatalf("missing approver = %d, %q", code, stderr)
	}
	gitUserName = func(string) string { return "Jens" }
	code, stdout, _ := runApprove(t, root, "--change", "example", "--all", "--yes")
	if code != 0 || !strings.Contains(stdout, "(via cli)") ||
		planEntries(t, root, evidenceScenarioID)[0].Approval == nil {
		t.Fatalf("--all --yes = %d, %q", code, stdout)
	}
}

func TestApproveHandlesScopesAndFailures(t *testing.T) {
	stubApprovalEnvironment(t, false, "")

	root := evidenceFixture(t)
	if code, _, stderr := runApprove(t, root, "--change", "example", "--confirmed-in-chat"); code != 2 ||
		!strings.Contains(stderr, "no v2 linkage plan") {
		t.Fatalf("missing plan = %d, %q", code, stderr)
	}
	writeFixture(t, root, changePlanPath, `{"schemaVersion":1,"scenarios":{}}`)
	if code, _, stderr := runApprove(t, root, "--change", "example", "--confirmed-in-chat"); code != 2 ||
		!strings.Contains(stderr, "plan migrate") {
		t.Fatalf("v1 plan = %d, %q", code, stderr)
	}
	writeFixture(t, root, changePlanPath, "{")
	if code, _, _ := runApprove(t, root, "--change", "example", "--confirmed-in-chat"); code != 2 {
		t.Fatalf("broken plan = %d", code)
	}
	if code, _, _ := runApprove(t, root, "--change", "absent", "--confirmed-in-chat"); code != 2 {
		t.Fatalf("missing change = %d", code)
	}

	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {
			entry(evidenceScenarioID, "unit", "Pure logic."),
			entry(evidenceScenarioID, "smoke", "Bad."),
		},
		"scn.demo.ffffffffffff": {entry("scn.demo.ffffffffffff", "unit", "Undeclared.")},
	})
	failure := errors.New("disk full")
	originalWrite := writePlanFile
	writePlanFile = func(string, any) error { return failure }
	code, _, stderr := runApprove(t, root, "--change", "example", "--confirmed-in-chat")
	writePlanFile = originalWrite
	if code != 2 || !strings.Contains(stderr, "disk full") {
		t.Fatalf("write failure = %d, %q", code, stderr)
	}
	code, stdout, _ := runApprove(t, root, "--change", "example", "--confirmed-in-chat")
	if code != 0 || strings.Count(stdout, "✓ approved") != 1 {
		t.Fatalf("invalid and undeclared entries were approved:\n%s", stdout)
	}

	archived := archivedFixture(t)
	v2 := `{"schemaVersion":2,"changeId":"example","scenarios":{"` + evidenceScenarioID +
		`":{"evidence":[{"id":"` + evidenceScenarioID + `.unit","level":"unit","rationale":"r"}]}}}`
	writeFixture(t, archived, "openspec/changes/archive/2026-01-01-old/linkage-plan.json", scopePlanForExample)
	writeFixture(t, archived, "openspec/changes/archive/2026-02-01-new/linkage-plan.json", v2)
	code, stdout, stderr = runApprove(t, archived, "--specs", "--confirmed-in-chat")
	if code != 0 || !strings.Contains(stdout, "✓ approved "+evidenceScenarioID+".unit") {
		t.Fatalf("specs approval = %d, %q, %q", code, stdout, stderr)
	}
	newPlan := readEvidencePlan(t, archived, "openspec/changes/archive/2026-02-01-new/linkage-plan.json")
	if newPlan.Scenarios[evidenceScenarioID].Evidence[0].Approval == nil {
		t.Fatal("archived plan was not approved")
	}
	if !strings.Contains(readTestFile(t, archived, "openspec/changes/archive/2026-01-01-old/linkage-plan.json"),
		"src/demo.mts#value") {
		t.Fatal("archived v1 plan changed")
	}
	if err := os.Remove(filepath.Join(archived, "openspec/specs/demo/spec.md")); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runApprove(t, archived, "--specs", "--confirmed-in-chat"); code != 2 {
		t.Fatalf("specs without specifications = %d", code)
	}
}

func TestApproveParsesSpecFailures(t *testing.T) {
	root := evidenceFixture(t)
	original := relativePath
	t.Cleanup(func() { relativePath = original })
	relativePath = func(string, string) (string, error) { return "", errors.New("relative failed") }
	if _, _, err := loadApprovalDocuments(root, changeScope("example")); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestApprovalEnvironmentDefaults(t *testing.T) {
	_ = stdinIsTerminal()
	root := fixtureRoot(t)
	if name := configuredGitUserName(root); strings.Contains(name, "\n") {
		t.Fatalf("git user name = %q", name)
	}
	if date := approvalDate(); len(date) != len("2006-01-02") {
		t.Fatalf("approval date = %q", date)
	}
	var output bytes.Buffer
	renderApprovals(&output, approvalResult{Rejected: []string{"x"}}, approveAll, "A")
	if !strings.Contains(output.String(), "Rejected") {
		t.Fatalf("render = %q", output.String())
	}
	_ = io.Discard
}
