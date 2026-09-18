package stele

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func runPlanMigrate(t *testing.T, root string, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"plan", "migrate", "--root", root}, arguments...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// @verifies scn.verificationstrategy.5cb8e9ef14e7.unit
func TestPlanMigrateConvertsV1Plans(t *testing.T) {
	root := evidenceFixture(t)
	writeFixture(t, root, changePlanPath, `{"schemaVersion":1,"changeId":"example",`+
		`"requirements":{"req.demo.aaaaaaaaaaaa":"src/demo.mts#value"},`+
		`"scenarios":{"`+evidenceScenarioID+`":"tests/value.test.mts#returns the value",`+
		`"`+otherScenarioID+`":"tests/other.test.mts#reports a missing value"}}`)
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".e2e.2", "runs the second journey")
	writeEvidenceTest(t, root, "tests/journey.test.mts", evidenceScenarioID+".e2e", "runs the journey")
	writeEvidenceTest(t, root, "tests/unit.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/again.test.mts", evidenceScenarioID+".unit", "returns the value again")
	writeEvidenceTest(t, root, "tests/other.test.mts", otherScenarioID, "reports a missing value")

	code, stdout, stderr := runPlanMigrate(t, root, "--change", "example")
	summary := "✓ migrated " + changePlanPath + ": 2 scenarios, 4 unapproved evidence entries"
	if code != 0 || !strings.Contains(stdout, summary) {
		t.Fatalf("migrate = %d, %q, %q", code, stdout, stderr)
	}
	content := readTestFile(t, root, changePlanPath)
	if strings.Contains(content, "requirements") || strings.Contains(content, "#") ||
		strings.Contains(content, "approval") {
		t.Fatalf("migrated plan keeps targets or invents approvals:\n%s", content)
	}
	plan := readEvidencePlan(t, root, changePlanPath)
	if plan.SchemaVersion != 2 || plan.ChangeID != "example" {
		t.Fatalf("migrated plan header = %#v", plan)
	}
	ids := make([]string, 0)
	for _, value := range plan.Scenarios[evidenceScenarioID].Evidence {
		ids = append(ids, value.ID+"="+value.Level)
		if value.Rationale != migratedRationale {
			t.Fatalf("entry %s rationale = %q", value.ID, value.Rationale)
		}
	}
	want := []string{
		evidenceScenarioID + ".unit=unit",
		evidenceScenarioID + ".e2e=e2e",
		evidenceScenarioID + ".e2e.2=e2e",
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("migrated levels = %v, want %v", ids, want)
	}
	flagged := plan.Scenarios[otherScenarioID].Evidence
	if len(flagged) != 1 || flagged[0].ID != otherScenarioID+".unit" ||
		flagged[0].Rationale != migratedWithoutLevelNote {
		t.Fatalf("scenario without a level = %#v", flagged)
	}
	report := verifyFixture(t, root, "proposal")
	if len(diagnosticsWithCode(report.Diagnostics, "PLAN_UNAPPROVED")) != 4 ||
		hasDiagnostic(report.Diagnostics, "PLAN_V1_DEPRECATED") {
		t.Fatalf("migrated plan diagnostics = %#v", report.Diagnostics)
	}

	code, stdout, _ = runPlanMigrate(t, root, "--change", "example")
	if code != 0 || !strings.Contains(stdout, "No v1 linkage plans to migrate.") {
		t.Fatalf("second migrate = %d, %q", code, stdout)
	}
}

func TestPlanMigrateHandlesArchivesAndFailures(t *testing.T) {
	root := archivedFixture(t)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-old/linkage-plan.json", scopePlanForExample)
	writeFixture(t, root, "openspec/changes/archive/2026-02-01-new/linkage-plan.json",
		`{"schemaVersion":2,"scenarios":{}}`)
	code, stdout, stderr := runPlanMigrate(t, root, "--specs")
	if code != 0 || !strings.Contains(stdout, "2026-01-01-old/linkage-plan.json: 1 scenarios") ||
		strings.Contains(stdout, "2026-02-01-new") {
		t.Fatalf("specs migrate = %d, %q, %q", code, stdout, stderr)
	}

	for name, arguments := range map[string][]string{
		"missing subcommand": {"plan"},
		"unknown subcommand": {"plan", "approve"},
		"unknown option":     {"plan", "migrate", "--stage", "x", "--bad"},
	} {
		var stderr bytes.Buffer
		if code := Run(arguments, &bytes.Buffer{}, &stderr); code != 2 {
			t.Fatalf("%s = %d, %q", name, code, stderr.String())
		}
	}

	fresh := evidenceFixture(t)
	code, _, stderr = runPlanMigrate(t, fresh, "--change", "example")
	if code != 2 || !strings.Contains(stderr, "read ") {
		t.Fatalf("missing plan = %d, %q", code, stderr)
	}
	writeFixture(t, fresh, changePlanPath, `{"scenarios":{"`+evidenceScenarioID+`":"tests/a.test.mts#a"}}`)
	failure := errors.New("write failed")
	originalWrite := writePlanFile
	writePlanFile = func(string, any) error { return failure }
	code, _, stderr = runPlanMigrate(t, fresh, "--change", "example")
	writePlanFile = originalWrite
	if code != 2 || !strings.Contains(stderr, "write failed") {
		t.Fatalf("write failure = %d, %q", code, stderr)
	}
	originalScan := scanMigrationAnchors
	t.Cleanup(func() { scanMigrationAnchors = originalScan })
	scanMigrationAnchors = func(string) ([]Anchor, error) { return nil, failure }
	if code, _, _ := runPlanMigrate(t, fresh, "--change", "example"); code != 2 {
		t.Fatalf("scan failure = %d", code)
	}
}
