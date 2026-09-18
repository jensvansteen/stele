package stele

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// reportFixture builds a verification report with two capabilities: todo with
// two scenarios and export with one.
func reportFixture() Report {
	report := Report{Mode: "implementation", Verdicts: ReportVerdicts{"pass", "passed", "pass"}}
	evidence := func(id, approval string) []PlannedEvidence {
		return []PlannedEvidence{{ID: id + ".unit", Level: "unit", Approval: approval}}
	}
	report.Requirements = []RequirementReport{
		{
			ID: "req.todo.aaaaaaaaaaaa", Title: "Add todos", Source: Source{"openspec/specs/todo/spec.md", 3},
			Scenarios: []ScenarioReport{
				{
					ID: "scn.todo.bbbbbbbbbbbb", Title: "Add a todo", Source: Source{"openspec/specs/todo/spec.md", 8},
					Evidence:  evidence("scn.todo.bbbbbbbbbbbb", "approved"),
					TestLinks: []Link{{Path: "tests/todo.test.mts", Line: 4, Selector: stringPointer("adds")}},
				},
				{
					ID: "scn.todo.cccccccccccc", Title: "Reject empty text",
					Source:   Source{"openspec/specs/todo/spec.md", 14},
					Evidence: evidence("scn.todo.cccccccccccc", "unapproved"),
				},
			},
		},
		{
			ID: "req.export.dddddddddddd", Title: "Export todos", Source: Source{"openspec/specs/export/spec.md", 3},
			Scenarios: []ScenarioReport{
				{
					ID: "scn.export.eeeeeeeeeeee", Title: "Export as CSV",
					Source:   Source{"openspec/specs/export/spec.md", 8},
					Evidence: evidence("scn.export.eeeeeeeeeeee", "approved"),
				},
			},
		},
	}
	report.Summary = ReportSummary{Requirements: 2, Scenarios: 3, LinkedRequirements: 2, LinkedScenarios: 3}
	return report
}

func execution(path, selector, evidence, outcome, digest string) TestExecution {
	scenario := baseIdentityOf(evidence)
	return TestExecution{
		Path: path, Selector: stringPointer(selector), ScenarioIDs: []string{scenario},
		EvidenceIDs: []string{evidence}, Outcome: outcome, InputDigest: digest,
	}
}

func renderedReport(input humanReportInput, style reportStyle) string {
	var output bytes.Buffer
	renderHumanReport(&output, buildHumanReport(input), style)
	return output.String()
}

func validateInput(report Report, executions ...TestExecution) humanReportInput {
	openSpec := true
	return humanReportInput{
		command: "validate", scope: verificationScope{currentSpecs: true}, report: &report,
		executions: executions, currentDigest: "now", testsRelevant: true, openSpec: &openSpec,
		passed: report.Verdicts.Linkage == "pass",
	}
}

// @verifies scn.terminalreport.00fe379ba4b4.unit
func TestReportNamesTheFailingStage(t *testing.T) {
	report := reportFixture()
	report.Verdicts.Linkage = "fail"
	identity := "scn.todo.cccccccccccc.unit"
	report.Diagnostics = []Diagnostic{
		diagnostic("PLAN_UNAPPROVED", "error", "Evidence is not approved.", "", 0, identity),
	}
	output := renderedReport(validateInput(report,
		execution("tests/todo.test.mts", "adds", "scn.todo.bbbbbbbbbbbb.unit", "passed", "now")), reportStyle{})
	assertOrdered(t, output,
		"stele "+Version+" · validate · current specifications",
		"✓ OpenSpec strict validation  passed",
		"✓ Specifications              2 requirements, 3 scenarios",
		"✗ Plan approval               2/3 evidence entries approved; 1 unapproved",
		"✓ Linkage (anchors)",
		"✓ Test execution              1/1 passed",
		"scn.todo.cccccccccccc.unit  Reject empty text  openspec/specs/todo/spec.md:14",
		"✗ FAILED  current specifications — plan approval (1 unapproved)",
	)
	if strings.Contains(output, "implementation verification") {
		t.Fatalf("the report still names implementation verification:\n%s", output)
	}
}

// @verifies scn.terminalreport.99cec4c2f435.unit
func TestReportOverviewPerCapability(t *testing.T) {
	report := reportFixture()
	output := renderedReport(validateInput(report,
		execution("tests/todo.test.mts", "adds", "scn.todo.bbbbbbbbbbbb.unit", "passed", "now"),
		execution("tests/todo.test.mts", "rejects", "scn.todo.cccccccccccc.unit", "passed", "now"),
		execution("tests/export.test.mts", "exports", "scn.export.eeeeeeeeeeee.unit", "failed", "now"),
	), reportStyle{})
	assertOrdered(t, output,
		"Capability  Scenarios  Tests               Plan          Status",
		"todo                2  2 passed            1/2 approved  ✓",
		"export              1  0 passed, 1 failed  1/1 approved  ✗",
	)
	if capabilityOf("openspec/changes/x/specs/platform/auth/spec.md") != "platform/auth" ||
		capabilityOf("openspec/specs/verify/spec.md") != "verify" || capabilityOf("openspec/specs/todo.md") != "todo" ||
		capabilityOf("a/specs/b/specs/c/spec.md") != "c" ||
		capabilityOf("notes.md") != "" {
		t.Fatal("capabilities are not read from specification paths")
	}
}

// @verifies scn.terminalreport.5b46740a15da.unit
func TestReportCountsLevelsAndDuration(t *testing.T) {
	input := validateInput(reportFixture(),
		execution("tests/todo.test.mts", "adds", "scn.todo.bbbbbbbbbbbb.unit", "passed", "now"),
		execution("tests/todo.test.mts", "rejects", "scn.todo.cccccccccccc.unit", "passed", "now"),
		execution("tests/cli.test.mts", "exports", "scn.export.eeeeeeeeeeee.e2e", "passed", "now"),
	)
	duration := 42 * time.Second
	input.duration = &duration
	output := renderedReport(input, reportStyle{})
	if !strings.Contains(output, "✓ Test execution              3/3 passed · unit 2 · e2e 1 · 42.0s") {
		t.Fatalf("the test line lacks counts and duration:\n%s", output)
	}
}

func manyUnapproved(count int) Report {
	report := reportFixture()
	report.Verdicts.Linkage = "fail"
	for index := range count {
		report.Diagnostics = append(report.Diagnostics, diagnostic("PLAN_UNAPPROVED", "error", "Not approved.",
			"openspec/specs/todo/spec.md", 100+index, fmt.Sprintf("scn.todo.cccccccccccc.unit.%d", index+2)))
	}
	return report
}

// @verifies scn.terminalreport.37651d67c39c.unit
func TestReportGroupsAndTruncatesFindings(t *testing.T) {
	output := renderedReport(validateInput(manyUnapproved(123)), reportStyle{})
	assertOrdered(t, output,
		"Errors",
		"PLAN_UNAPPROVED ×123  Planned evidence has no human approval yet.",
		"→ Fix: review the levels in design.md, then run `stele approve --specs`",
		"scn.todo.cccccccccccc.unit.2  Reject empty text  openspec/specs/todo/spec.md:100",
		"scn.todo.cccccccccccc.unit.6  Reject empty text  openspec/specs/todo/spec.md:104",
		"… 118 more (--details)",
	)
	if strings.Count(output, "Reject empty text  openspec") != reportItemLimit {
		t.Fatalf("expected %d findings:\n%s", reportItemLimit, output)
	}
}

// @verifies scn.terminalreport.3c53b931fbf3.unit
func TestReportDetailsListsEveryFinding(t *testing.T) {
	output := renderedReport(validateInput(manyUnapproved(123)), reportStyle{details: true})
	if strings.Count(output, "Reject empty text  openspec") != 123 || strings.Contains(output, "more (--details)") {
		t.Fatalf("--details truncated the findings:\n%s", output)
	}
}

// @verifies scn.terminalreport.4bf69bd755ef.unit
func TestReportListsFailedAndStaleTests(t *testing.T) {
	failed := execution("tests/todo.test.mts", "adds", "scn.todo.bbbbbbbbbbbb.unit", "failed", "now")
	failed.Reason = stringPointer("test-process-failed")
	input := validateInput(reportFixture(), failed,
		execution("tests/export.test.mts", "exports", "scn.export.eeeeeeeeeeee.unit", "passed", "old"))
	input.passed = false
	output := renderedReport(input, reportStyle{})
	assertOrdered(t, output,
		"✗ Test execution              0/2 passed · 1 failed · 1 stale",
		"Failed tests",
		"✗ tests/todo.test.mts:4  adds",
		"scn.todo.bbbbbbbbbbbb.unit · Add a todo · test-process-failed",
		"Stale tests",
		"! tests/export.test.mts  exports",
		"scn.export.eeeeeeeeeeee.unit · Export as CSV · run again: stele test scn.export.eeeeeeeeeeee.unit",
		"test execution (1 failed test)",
	)
	unnamed := TestExecution{Path: "tests/x.test.mts", Outcome: "failed"}
	lines := classifyTests(humanReportInput{executions: []TestExecution{unnamed}}, reportIndex{}).failedLines(
		reportIndex{})
	if lines[0].name != "(no exact test name)" || lines[0].detail != "" {
		t.Fatalf("an unresolved test line = %#v", lines[0])
	}
}

// @verifies scn.terminalreport.23e3fa620ea0.unit
func TestReportIsDeterministic(t *testing.T) {
	first := manyUnapproved(7)
	first.Diagnostics = append(first.Diagnostics,
		diagnostic("PLAN_V1_DEPRECATED", "warning", "Version 1 plan.", "plan.json", 1, ""),
		diagnostic("LINK_CODE_MISSING", "error", "No code.", "openspec/specs/export/spec.md", 3,
			"req.export.dddddddddddd"))
	second := first
	second.Diagnostics = slices.Clone(first.Diagnostics)
	slices.Reverse(second.Diagnostics)
	tests := []TestExecution{
		execution("tests/a.test.mts", "a", "scn.todo.bbbbbbbbbbbb.unit", "failed", "now"),
		execution("tests/b.test.mts", "b", "scn.export.eeeeeeeeeeee.unit", "failed", "now"),
	}
	duration := 1500 * time.Millisecond
	left, right := validateInput(first, tests...), validateInput(second, tests[1], tests[0])
	left.duration, right.duration = &duration, &duration
	if a, b := renderedReport(left, reportStyle{}), renderedReport(right, reportStyle{}); a != b {
		t.Fatalf("collection order changed the report:\n%s\n---\n%s", a, b)
	}
}

// @verifies scn.terminalreport.cf302ba34005.unit
func TestColorRespectsNoColor(t *testing.T) {
	originalTerminal, originalEnv := isTerminal, lookupEnv
	t.Cleanup(func() { isTerminal, lookupEnv = originalTerminal, originalEnv })
	isTerminal = func(io.Writer) bool { return true }
	environment := map[string]string{"NO_COLOR": "1"}
	lookupEnv = func(name string) (string, bool) {
		value, found := environment[name]
		return value, found
	}
	if colorEnabled("auto", nil) || !colorEnabled("always", nil) || colorEnabled("never", nil) {
		t.Fatal("NO_COLOR or an explicit mode was ignored")
	}
	environment = map[string]string{"TERM": "dumb"}
	if colorEnabled("auto", nil) {
		t.Fatal("a dumb terminal was colored")
	}
	environment = map[string]string{}
	if !colorEnabled("auto", nil) {
		t.Fatal("a terminal was not colored")
	}
	colored := renderedReport(validateInput(manyUnapproved(1)), reportStyle{color: true})
	plain := renderedReport(validateInput(manyUnapproved(1)), reportStyle{})
	if !strings.Contains(colored, "\x1b[31m✗\x1b[0m") || !strings.Contains(colored, "\x1b[2mopenspec/specs") ||
		strings.Contains(plain, "\x1b[") {
		t.Fatalf("color output:\n%q\nplain output:\n%q", colored, plain)
	}
	if (reportStyle{color: true}).paint(markWarn) != "\x1b[33m!\x1b[0m" || (reportStyle{color: true}).dim("") != "" {
		t.Fatal("marks are not colored")
	}
}

func TestRealTerminalDetection(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is a terminal")
	}
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if !isTerminal(device) {
		t.Fatalf("%s is not a character device", os.DevNull)
	}
	_ = device.Close()
	if isTerminal(device) {
		t.Fatal("a closed file is a terminal")
	}
}

func TestReportWithoutTestsOrPlans(t *testing.T) {
	report := reportFixture()
	for index := range report.Requirements {
		for scenario := range report.Requirements[index].Scenarios {
			report.Requirements[index].Scenarios[scenario].Evidence = nil
		}
	}
	report.Verdicts = ReportVerdicts{"pass", "stale", "incomplete"}
	report.Summary.PassedScenarios = 0
	input := humanReportInput{
		command: "verify", scope: changeScope("example"), report: &report,
		testsRelevant: true, passed: true, notes: []string{"overall incomplete"},
	}
	output := renderedReport(input, reportStyle{})
	assertOrdered(t, output,
		"stele "+Version+" · verify · change example",
		"todo                2  not run  –     !",
		"✓ Plan approval               no evidence entries",
		"! Test execution              0/3 scenarios passed; stale",
		"✓ PASSED  change example · overall incomplete",
	)
	report.Verdicts.Execution = "passed"
	if line := scenarioExecutionLine(&report); line.mark != markPass {
		t.Fatalf("passed scenarios = %#v", line)
	}
	if line := scenarioExecutionLine(nil); line.detail != "not run" {
		t.Fatalf("no report = %#v", line)
	}
	failing := humanReportInput{command: "test", scope: changeScope("example"), passed: false}
	if output := renderedReport(failing, reportStyle{}); !strings.Contains(output, "✗ FAILED  change example\n") {
		t.Fatalf("a failure without a failed check:\n%s", output)
	}
	if baseIdentityOf("scn.todo.bbbbbbbbbbbb") != "scn.todo.bbbbbbbbbbbb" || levelOf("x") != "" {
		t.Fatal("plain identities changed")
	}
	if truncate("abcdef", 4) != "abc…" || plural(1, "file") != "1 file" {
		t.Fatal("text helpers")
	}
	unknown := guideFor("SOMETHING_NEW")
	if unknown.stage != stageLinkage || unknown.fixFor("--specs") == "" {
		t.Fatalf("unknown code guide = %#v", unknown)
	}
}
