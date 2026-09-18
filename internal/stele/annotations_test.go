package stele

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubEnvironment replaces the environment seam with fixed values.
func stubEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	original := lookupEnv
	t.Cleanup(func() { lookupEnv = original })
	lookupEnv = func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// annotationLines renders a report's annotations as they are written.
func annotationLines(t *testing.T, report humanReport, details bool) []string {
	t.Helper()
	sink := newAnnotationSink("github", t.TempDir())
	sink.addReport(report, details)
	var output bytes.Buffer
	sink.flush(&output)
	return strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
}

func linesWithPrefix(lines []string, prefix string) []string {
	matched := make([]string, 0)
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			matched = append(matched, line)
		}
	}
	return matched
}

// @verifies scn.terminalreport.447957e12de7.unit
func TestAnnotationsNameFailedTests(t *testing.T) {
	stubEnvironment(t, nil)
	report := reportFixture()
	report.Requirements[0].Scenarios[0].TestLinks[0].Line = 20
	input := validateInput(report,
		execution("tests/todo.test.mts", "adds", "scn.todo.bbbbbbbbbbbb.unit", "failed", "now"),
		execution("tests/todo.test.mts", "rejects", "scn.todo.cccccccccccc.unit", "passed", "now"))
	input.passed = false
	lines := annotationLines(t, buildHumanReport(input), false)
	failed := linesWithPrefix(lines, "::error file=tests/todo.test.mts,line=20,title=Test failed::")
	if len(failed) != 1 || !strings.Contains(failed[0], "scn.todo.bbbbbbbbbbbb.unit") ||
		!strings.Contains(failed[0], "Add a todo") || len(linesWithPrefix(lines, "::")) != len(lines) {
		t.Fatalf("failed test annotations = %q", lines)
	}

	scenarioOnly := humanReport{failed: failedScenarios(Report{Requirements: []RequirementReport{{
		Scenarios: []ScenarioReport{{
			ID: "scn.todo.bbbbbbbbbbbb", Title: "Add a todo", Source: Source{"openspec/specs/todo/spec.md", 8},
			Execution: ExecutionState{Outcome: "failed"},
		}},
	}}})}
	lines = annotationLines(t, scenarioOnly, false)
	if len(lines) != 1 || lines[0] != "::error file=openspec/specs/todo/spec.md,line=8,title=Test failed::"+
		"Add a todo · scn.todo.bbbbbbbbbbbb" {
		t.Fatalf("failed scenario annotation = %q", lines)
	}
}

// @verifies scn.terminalreport.be5c922ef009.unit
func TestAnnotationsFollowTheTruncationOfTheReport(t *testing.T) {
	stubEnvironment(t, nil)
	report := reportFixture()
	for index := range 123 {
		report.Diagnostics = append(report.Diagnostics, diagnostic("PLAN_UNAPPROVED", "error", "unapproved",
			"openspec/specs/todo/linkage-plan.json", index+1, fmt.Sprintf("scn.todo.%012x.unit", index)))
	}
	built := buildHumanReport(validateInput(report))
	lines := annotationLines(t, built, false)
	located := linesWithPrefix(lines, "::error file=openspec/specs/todo/linkage-plan.json,line=")
	remainder := linesWithPrefix(lines, "::error title=PLAN_UNAPPROVED::")
	if len(located) != 5 || len(remainder) != 1 || len(lines) != 6 ||
		!strings.Contains(remainder[0], "118 more findings") || !strings.Contains(remainder[0], "--details") {
		t.Fatalf("truncated annotations = %q", lines)
	}
	if !strings.Contains(located[0], "title=PLAN_UNAPPROVED::Planned evidence has no human approval yet. "+
		"scn.todo.000000000000.unit") {
		t.Fatalf("finding annotation = %q", located[0])
	}
	lines = annotationLines(t, built, true)
	if len(lines) != 123 || len(linesWithPrefix(lines, "::error title=")) != 0 {
		t.Fatalf("--details annotations = %d lines, %q", len(lines), linesWithPrefix(lines, "::error title="))
	}

	failing := make([]TestExecution, 0, 7)
	for index := range 7 {
		failing = append(failing, execution(fmt.Sprintf("tests/t%d.test.mts", index), "fails",
			"scn.todo.bbbbbbbbbbbb.unit", "failed", "now"))
	}
	warned := reportFixture()
	warned.Diagnostics = []Diagnostic{
		deprecatedPlanDiagnostic("openspec/changes/archive/2026-01-01-a/linkage-plan.json"),
	}
	lines = annotationLines(t, buildHumanReport(validateInput(warned, failing...)), false)
	if len(linesWithPrefix(lines, "::warning file=openspec/changes/archive/2026-01-01-a/linkage-plan.json,")) != 1 ||
		len(linesWithPrefix(lines, "::error file=tests/")) != 5 ||
		len(linesWithPrefix(lines, "::error title=Test failed::2 more failed tests")) != 1 {
		t.Fatalf("warning and failed test annotations = %q", lines)
	}
}

// @verifies scn.terminalreport.fbadf694cd04.unit
func TestAnnotationsLocateFilesFromTheWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "app")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	located := func(workspace, root string) string {
		stubEnvironment(t, map[string]string{"GITHUB_WORKSPACE": workspace})
		sink := newAnnotationSink("github", root)
		sink.add(annotation{level: "error", path: "src/todo.ts", line: 3, title: "ANCHOR_DANGLING", message: "m"})
		return sink.lines[0]
	}
	if got := located(workspace, root); !strings.HasPrefix(got, "::error file=app/src/todo.ts,line=3,") {
		t.Fatalf("workspace-relative annotation = %q", got)
	}
	for name, paths := range map[string][2]string{
		"root is the workspace": {workspace, workspace},
		"root outside":          {root, workspace},
		"relative workspace":    {"workspace", root},
		"no workspace":          {"", root},
	} {
		if got := located(paths[0], paths[1]); !strings.HasPrefix(got, "::error file=src/todo.ts,line=3,") {
			t.Fatalf("%s: annotation = %q", name, got)
		}
	}
}

// @verifies scn.terminalreport.f466c27970e9.unit
func TestAnnotationsEscapeWorkflowCommandValues(t *testing.T) {
	got := workflowCommand("error", "src/a,b.ts", 2, "T:1", "100% done\r\nnext")
	if got != "::error file=src/a%2Cb.ts,line=2,title=T%3A1::100%25 done%0D%0Anext" ||
		strings.ContainsAny(got, "\r\n") {
		t.Fatalf("escaped command = %q", got)
	}
	if got := workflowCommand("warning", "", 0, "Test failed", "m"); got != "::warning title=Test failed::m" {
		t.Fatalf("command without a location = %q", got)
	}
	if got := workflowCommand("error", "spec.md", 0, "T", "m"); got != "::error file=spec.md,title=T::m" {
		t.Fatalf("command without a line = %q", got)
	}
}

func TestAnnotationSinkIsOffAndDeduplicates(t *testing.T) {
	stubEnvironment(t, map[string]string{"GITHUB_ACTIONS": "true"})
	var sink *annotationSink
	sink.addAll([]annotation{{level: "error", title: "T", message: "m"}})
	var output bytes.Buffer
	sink.flush(&output)
	for _, mode := range []string{"never", ""} {
		if newAnnotationSink(mode, t.TempDir()) != nil {
			t.Fatalf("mode %q is on", mode)
		}
	}
	sink = newAnnotationSink("auto", t.TempDir())
	sink.flush(&output)
	sink.addAll([]annotation{{level: "error", title: "T", message: "m"}, {level: "error", title: "T", message: "m"}})
	sink.flush(&output)
	if output.String() != "::error title=T::m\n" {
		t.Fatalf("annotations = %q", output.String())
	}
}
