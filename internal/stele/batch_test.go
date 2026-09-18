package stele

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// runOneByOne makes the runner use the per-test oracle, so stubs of
// runExactTest decide every outcome.
func runOneByOne(t *testing.T) {
	t.Helper()
	original := runTestGroups
	t.Cleanup(func() { runTestGroups = original })
	runTestGroups = executeGroupsOneByOne
}

// countBatchProcesses counts the test processes the runner starts.
func countBatchProcesses(t *testing.T) *int {
	t.Helper()
	original := startBatchProcess
	t.Cleanup(func() { startBatchProcess = original })
	count := 0
	startBatchProcess = func(command *exec.Cmd, stderr io.Writer) (batchProcess, error) {
		count++
		return original(command, stderr)
	}
	return &count
}

func selectedGroup(path, selector string) testGroup {
	return testGroup{
		Key:      testGroupKey{Path: path, Selector: selector},
		Selector: &selector,
		IDs:      []string{"scn.demo.aaaaaaaaaaaa"},
	}
}

// runBatched runs groups through the shipped batch path.
func runBatched(root string, groups ...testGroup) []TestExecution {
	executions, _ := executeTestGroups(root, groups, "", silentProgress{}, ParsedSpecs{})
	return executions
}

func runBatchedGroup(root, path, selector string) TestExecution {
	return runBatched(root, selectedGroup(path, selector))[0]
}

func reasonsOf(executions []TestExecution) []string {
	reasons := make([]string, 0, len(executions))
	for _, execution := range executions {
		reasons = append(reasons, execution.Outcome+":"+pointerValue(execution.Reason))
	}
	return reasons
}

// @verifies scn.execution.0aa78cf92f34.unit
func TestPlanTestBatchesKeepsTagSetsApart(t *testing.T) {
	root := goRunnerModule(t)
	unresolved := testGroup{Key: testGroupKey{Path: "tests/a.test.mts"}}
	groups := []testGroup{
		selectedGroup("tests/a.test.mts", "x"),
		selectedGroup("module/pkg/tagged_test.go", "TestTagged"),
		selectedGroup("module/pkg/demo_test.go", "TestPasses"),
		selectedGroup("tests/a.test.mts", "a.b"),
		selectedGroup("module/pkg/demo_test.go", "TestFails"),
		selectedGroup("module/pkg/never_test.go", "TestNever"),
		selectedGroup("module/pkg/missing_test.go", "TestMissing"),
		selectedGroup("tests/a.test.mjs", "y"),
		unresolved,
	}
	plan := planTestBatches(root, groups)
	labels := make([]string, 0)
	commands := make([]string, 0)
	planned := len(plan.settled)
	for _, batch := range plan.batches {
		labels = append(labels, batch.label())
		commands = append(commands, strings.Join(batch.command().Args, " "))
		planned += len(batch.groups)
	}
	wantLabels := []string{"module/pkg", "module/pkg (-tags=integration)", "tests/a.test.mts"}
	if !slices.Equal(labels, wantLabels) {
		t.Fatalf("batches = %v, want %v", labels, wantLabels)
	}
	wantCommands := []string{
		"go test -json -count=1 -run ^(TestFails|TestPasses)$ ./pkg",
		"go test -json -count=1 -run ^(TestTagged)$ -tags=integration ./pkg",
		`node --test --test-reporter=tap --test-name-pattern=^(a\.b|x)$ tests/a.test.mts`,
	}
	if !slices.Equal(commands, wantCommands) {
		t.Fatalf("commands =\n%s\nwant\n%s", strings.Join(commands, "\n"), strings.Join(wantCommands, "\n"))
	}
	if plan.batches[0].directory != filepath.Join(root, "module") || plan.batches[2].directory != root {
		t.Fatalf("working directories = %q, %q", plan.batches[0].directory, plan.batches[2].directory)
	}
	if planned != len(groups) || len(plan.settled) != 4 {
		t.Fatalf("every test must be planned once: %d of %d, settled %#v", planned, len(groups), plan.settled)
	}
	executions := runBatched(root, groups[1], groups[2], groups[4])
	if got := reasonsOf(executions); !slices.Equal(got, []string{
		"passed:", "passed:", "failed:test-process-failed",
	}) {
		t.Fatalf("outcomes = %v", got)
	}
}

// crashSpec declares one scenario per test of the crash fixtures.
func crashSpec(identities ...string) string {
	var spec strings.Builder
	spec.WriteString("<!-- stele: spec v1 -->\n## ADDED Requirements\n\n### Requirement: Crash\n" +
		"Verification-ID: req.demo.aaaaaaaaaaaa\n\nThe system SHALL survive.\n")
	for index, identity := range identities {
		fmt.Fprintf(&spec, "\n#### Scenario: Case %d\nVerification-ID: %s\n\n- **WHEN** it runs\n"+
			"- **THEN** it reports\n", index, identity)
	}
	return spec.String()
}

func scenarioIdentity(index int) string {
	return fmt.Sprintf("scn.demo.%012x", index)
}

// @verifies scn.execution.aba48bf93edd.integration
func TestCrashedBatchesMarkUnreportedTests(t *testing.T) {
	root := fixtureRoot(t)
	identities := []string{}
	for index := 1; index <= 6; index++ {
		identities = append(identities, scenarioIdentity(index))
	}
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", crashSpec(identities...))
	writeFixture(t, root, "go.mod", "module example.com/crash\n\ngo 1.22\n")
	writeFixture(t, root, "internal/crash/crash_test.go", "package crash\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n"+
		"// @verifies "+identities[0]+"\nfunc TestFirst(t *testing.T) {}\n\n"+
		"// @verifies "+identities[1]+"\nfunc TestExit(t *testing.T) { os.Exit(3) }\n\n"+
		"// @verifies "+identities[2]+"\nfunc TestLast(t *testing.T) {}\n")
	writeFixture(t, root, "tests/crash.test.mts", "import test from \"node:test\";\n"+
		"// @verifies "+identities[3]+"\nvoid test(\"first\", () => {});\n"+
		"// @verifies "+identities[4]+"\nvoid test(\"exits\", () => { process.exit(3); });\n"+
		"// @verifies "+identities[5]+"\nvoid test(\"after\", () => {});\n")

	code, stdout, stderr := runCommand(t, "test", "--root", root, "--change", "example", "--color", "never",
		"--evidence-file", "out/evidence.json")
	if code != 1 {
		t.Fatalf("exit code = %d\n%s\n%s", code, stdout, stderr)
	}
	var evidence Evidence
	if !readJSON(filepath.Join(root, "out", "evidence.json"), &evidence) {
		t.Fatal("no evidence")
	}
	outcomes := map[string]string{}
	for _, execution := range evidence.Executions {
		outcomes[pointerValue(execution.Selector)] = execution.Outcome + ":" + pointerValue(execution.Reason)
	}
	want := map[string]string{
		"TestFirst": "passed:",
		"TestExit":  "failed:test-process-failed",
		"TestLast":  "failed:test-process-failed",
		"first":     "failed:test-process-failed",
		"exits":     "failed:test-process-failed",
		"after":     "failed:test-process-failed",
	}
	for selector, outcome := range want {
		if outcomes[selector] != outcome {
			t.Errorf("%s = %q, want %q", selector, outcomes[selector], outcome)
		}
	}
	for _, text := range []string{
		"Incomplete test processes\n",
		"  ✗ internal/crash  exit status 1, 2 tests without a result\n",
		"  ✗ tests/crash.test.mts  exit status 1, 3 tests without a result\n",
		"exitCode: 3",
	} {
		if !strings.Contains(stdout, text) {
			t.Errorf("the report does not contain %q:\n%s", text, stdout)
		}
	}
	for _, text := range []string{
		"  INCOMPLETE internal/crash did not complete (exit status 1)\n",
		"  INCOMPLETE tests/crash.test.mts did not complete (exit status 1)\n",
	} {
		if !strings.Contains(stderr, text) {
			t.Errorf("progress does not contain %q:\n%s", text, stderr)
		}
	}
	if strings.Contains(readFile(t, filepath.Join(root, "out", "evidence.json")), "exit status") {
		t.Fatal("evidence contains process output")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// @verifies scn.execution.e49b224babb8.integration
func TestBatchedRunsMatchOneProcessPerTest(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	identities := []string{}
	for index := 1; index <= 9; index++ {
		identities = append(identities, scenarioIdentity(index))
	}
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", crashSpec(identities...))
	writeFixture(t, root, "internal/pkg/demo_test.go", "package pkg\n\nimport \"testing\"\n\n"+
		"// @verifies "+identities[0]+"\nfunc TestPasses(t *testing.T) {}\n\n"+
		"// @verifies "+identities[1]+"\nfunc TestFails(t *testing.T) { t.Fatal(\"expected failure\") }\n\n"+
		"// @verifies "+identities[2]+"\nfunc TestSkips(t *testing.T) { t.Skip(\"skipped\") }\n")
	writeFixture(t, root, "internal/pkg/tagged_test.go", "//go:build integration\n\npackage pkg\n\n"+
		"import \"testing\"\n\n// @verifies "+identities[3]+"\nfunc TestTagged(t *testing.T) {}\n")
	// Go ignores files whose name starts with "_", so TestGone never runs.
	writeFixture(t, root, "internal/pkg/_gone_test.go", "package pkg\n\nimport \"testing\"\n\n// @verifies "+
		identities[4]+"\nfunc TestGone(t *testing.T) {}\n")
	writeFixture(t, root, "tests/demo.test.mts", "import test from \"node:test\";\n"+
		"// @verifies "+identities[5]+"\nvoid test(\"passes\", () => {});\n"+
		"// @verifies "+identities[6]+"\nvoid test(\"fails\", () => { throw new Error(\"no\"); });\n"+
		"// @verifies "+identities[7]+"\nvoid test(\"skips\", { skip: true }, () => {});\n"+
		"if (process.env.STELE_NEVER_SET) {\n// @verifies "+identities[8]+"\nvoid test(\"hidden\", () => {});\n}\n")

	run := func(name string) (string, string) {
		code, stdout, stderr := runCommand(t, "test", "--root", root, "--change", "example", "--json",
			"--evidence-file", "out/"+name+".json")
		if code != 1 {
			t.Fatalf("%s exit code = %d\n%s", name, code, stderr)
		}
		return stdout, readFile(t, filepath.Join(root, "out", name+".json"))
	}
	processes := countBatchProcesses(t)
	batchedJSON, batchedFile := run("batched")
	if *processes != 3 {
		t.Fatalf("batched run started %d processes, want 3", *processes)
	}
	runOneByOne(t)
	singleJSON, singleFile := run("single")
	if *processes != 3+9 {
		t.Fatalf("per-test run started %d processes, want 9", *processes-3)
	}
	if batchedJSON != singleJSON || batchedFile != singleFile {
		t.Fatalf("batched and per-test runs differ:\n%s\n%s", batchedJSON, singleJSON)
	}
	var evidence Evidence
	if !readJSON(filepath.Join(root, "out", "batched.json"), &evidence) {
		t.Fatal("no evidence")
	}
	want := []string{
		"failed:test-not-executed", "failed:test-process-failed", "passed:", "failed:test-skipped", "passed:",
		"failed:test-process-failed", "failed:test-not-executed", "passed:", "failed:test-skipped",
	}
	if got := reasonsOf(evidence.Executions); !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}
}

// @verifies scn.execution.93ab1bbe751c.unit
func TestBatchShowsFailuresBeforeItEnds(t *testing.T) {
	fakeTerminal(t, false, map[string]string{})
	root := goRunnerModule(t)
	output, input := io.Pipe()
	release := make(chan struct{})
	original := startBatchProcess
	t.Cleanup(func() { startBatchProcess = original })
	startBatchProcess = func(*exec.Cmd, io.Writer) (batchProcess, error) {
		return batchProcess{output: output, wait: func() error { <-release; return nil }}, nil
	}
	file, err := os.Create(filepath.Join(root, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	progress := newProgress(file, false, "", reportStyle{})
	finished := make(chan []TestExecution)
	go func() {
		executions, _ := executeTestGroups(root, []testGroup{
			selectedGroup("module/pkg/demo_test.go", "TestFails"),
			selectedGroup("module/pkg/demo_test.go", "TestPasses"),
			selectedGroup("module/pkg/demo_test.go", "TestSkips"),
		}, "", progress, ParsedSpecs{})
		finished <- executions
	}()
	write := func(line string) {
		if _, err := io.WriteString(input, line+"\n"); err != nil {
			t.Error(err)
		}
	}
	write(`{"Action":"run","Test":"TestFails"}`)
	write(`{"Action":"fail","Test":"TestFails"}`)
	failure := "  FAILED module/pkg/demo_test.go TestFails\n"
	waitForText(t, filepath.Join(root, "stderr.txt"), failure)
	if strings.Contains(readFile(t, filepath.Join(root, "stderr.txt")), "module/pkg/demo_test.go  ") {
		t.Fatal("the file line appeared before its last test finished")
	}
	write(`{"Action":"pass","Test":"TestPasses"}`)
	write(`{"Action":"skip","Test":"TestSkips"}`)
	write(`{"Action":"output","Output":"FAIL\n"}`)
	_ = input.Close()
	close(release)
	executions := <-finished
	if got := reasonsOf(executions); !slices.Equal(got, []string{
		"failed:test-process-failed", "passed:", "failed:test-skipped",
	}) {
		t.Fatalf("outcomes = %v", got)
	}
	assertOrdered(t, readFile(t, filepath.Join(root, "stderr.txt")),
		"test execution: 3 tests in 1 files\n",
		failure,
		"  FAILED module/pkg/demo_test.go TestSkips\n",
		"  ✗ module/pkg/demo_test.go  1/3",
		"test execution: 1/3 passed",
	)
}

func waitForText(t *testing.T, path, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(readFile(t, path), text) {
		if time.Now().After(deadline) {
			t.Fatalf("%q never appeared:\n%s", text, readFile(t, path))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBatchReaders(t *testing.T) {
	goBatch := testBatch{runner: "go", groups: []testGroup{
		selectedGroup("a_test.go", "TestA"), selectedGroup("a_test.go", ""),
	}}
	reader := newBatchReader(goBatch)
	for _, test := range []struct {
		line  string
		found bool
		shown string
	}{
		{"not json", false, "not json"},
		{`{"Action":"run","Test":"TestA"}`, false, ""},
		{`{"Action":"output","Test":"TestA","Output":"--- PASS: TestA\n"}`, false, "--- PASS: TestA"},
		{`{"Action":"pass","Test":"TestA"}`, true, ""},
		{`{"Action":"fail","Test":"TestA"}`, false, ""},
		{`{"Action":"pass"}`, false, ""},
		{`{"Action":"output","Output":"PASS\n"}`, false, "PASS"},
	} {
		line := reader.read(test.line)
		if line.found != test.found || line.display != test.shown {
			t.Errorf("go read(%q) = %#v", test.line, line)
		}
	}
	if !reader.completed() {
		t.Error("the final PASS line did not complete the Go batch")
	}

	nodeBatch := testBatch{runner: "node", target: "tests/a.test.mts", groups: []testGroup{
		selectedGroup("tests/a.test.mts", "passes"), selectedGroup("tests/a.test.mts", "fails"),
		selectedGroup("tests/a.test.mts", "skips"), selectedGroup("tests/a.test.mts", "todo"),
	}}
	reader = newBatchReader(nodeBatch)
	for _, test := range []struct {
		line   string
		name   string
		result exactResult
	}{
		{"TAP version 13", "", exactResult{}},
		{"    ok 1 - passes", "passes", exactResult{passed: true, executed: true}},
		{"not ok 2 - passes", "", exactResult{}},
		{"not ok 3 - fails", "fails", exactResult{executed: true}},
		{"ok 4 - skips # SKIP not now", "skips", exactResult{executed: true, err: errTestSkipped}},
		{"not ok 5 - todo # TODO later", "todo", exactResult{executed: true}},
		{"ok 6 - unknown", "", exactResult{}},
	} {
		line := reader.read(test.line)
		if line.name != test.name || line.found != (test.name != "") || line.result != test.result ||
			line.display != test.line {
			t.Errorf("node read(%q) = %#v", test.line, line)
		}
	}
	if reader.completed() {
		t.Error("a Node batch completed without its summary")
	}
	reader.read("# tests 5")
	if !reader.completed() {
		t.Error("the summary did not complete the Node batch")
	}
	reader.read("not ok 7 - /abs/tests/a.test.mts")
	if reader.completed() {
		t.Error("a failed file entry did not mark the Node batch incomplete")
	}

	outcome := batchOutcome{results: map[string]exactResult{"a": {passed: true, executed: true}}}
	if outcome.resultOf("a") != (exactResult{passed: true, executed: true}) ||
		!errors.Is(outcome.resultOf("b").err, errBatchIncomplete) {
		t.Fatal("results of an incomplete batch")
	}
	outcome.completed = true
	if outcome.resultOf("b") != (exactResult{}) {
		t.Fatal("a missing test of a completed batch must not have run")
	}
}

func TestRunTestBatchProcesses(t *testing.T) {
	if _, err := startCommand(&exec.Cmd{Stdout: io.Discard}, io.Discard); err == nil {
		t.Fatal("expected a pipe error")
	}
	missing := testBatch{
		runner: "node", target: "a.test.mts", directory: filepath.Join(t.TempDir(), "missing"),
		groups: []testGroup{selectedGroup("a.test.mts", "a")},
	}
	outcome := runTestBatch(missing, func(string, exactResult) {})
	if outcome.completed || !strings.Contains(outcome.status, "missing") ||
		!errors.Is(outcome.resultOf("a").err, errBatchIncomplete) {
		t.Fatalf("a batch that could not start = %#v", outcome)
	}

	original := startBatchProcess
	t.Cleanup(func() { startBatchProcess = original })
	startBatchProcess = func(_ *exec.Cmd, stderr io.Writer) (batchProcess, error) {
		var lines strings.Builder
		for index := range 25 {
			fmt.Fprintf(&lines, "line %d\n", index)
		}
		lines.WriteString("ok 1 - a")
		_, _ = io.WriteString(stderr, "warning\n\n")
		return batchProcess{output: strings.NewReader(lines.String()), wait: func() error {
			return errors.New("signal: killed")
		}}, nil
	}
	reported := []string{}
	outcome = runTestBatch(missing, func(name string, _ exactResult) { reported = append(reported, name) })
	if outcome.status != "signal: killed" || outcome.completed || !slices.Equal(reported, []string{"a"}) {
		t.Fatalf("outcome = %#v, reported %v", outcome, reported)
	}
	if len(outcome.tail) != batchTailLines || outcome.tail[0] != "line 7" ||
		outcome.tail[batchTailLines-2] != "ok 1 - a" || outcome.tail[batchTailLines-1] != "warning" {
		t.Fatalf("tail = %q", outcome.tail)
	}
	if exitStatus(nil) != "exit status 0" {
		t.Fatal("exit status of a successful process")
	}
}

func TestBatchProgressNamesIncompleteBatches(t *testing.T) {
	fakeClock(t)
	var stderr strings.Builder
	complete := batchEvent{label: "pkg", status: "exit status 0", completed: true}
	crashed := batchEvent{label: "pkg (-tags=integration)", status: "exit status 1", unreported: 2}
	terminal := &terminalProgress{state: newProgressState(&stderr, "", reportStyle{}), width: 200}
	terminal.planned(3, map[string]int{"pkg/a_test.go": 3})
	terminal.started(progressEvent{batch: "pkg", path: "pkg/a_test.go", selector: "TestA", title: "A"})
	terminal.started(progressEvent{batch: "pkg", path: "pkg/a_test.go", selector: "TestB", title: "B"})
	terminal.batchEnded(complete)
	terminal.batchEnded(crashed)
	terminal.started(progressEvent{path: "pkg/b_test.go", selector: "TestC"})
	assertOrdered(t, stderr.String(), "tests 0/3 · pkg · 2 running",
		clearLine+"✗ pkg (-tags=integration) did not complete (exit status 1)\n",
		"tests 0/3 · pkg/b_test.go · 3 running")

	stderr.Reset()
	line := &lineProgress{state: newProgressState(&stderr, "[x] ", reportStyle{})}
	line.batchEnded(complete)
	line.batchEnded(crashed)
	if stderr.String() != "[x]   INCOMPLETE pkg (-tags=integration) did not complete (exit status 1)\n" {
		t.Fatalf("plain progress = %q", stderr.String())
	}

	var report strings.Builder
	renderIncomplete(&strings.Builder{}, nil, reportStyle{})
	tail := []string{"panic: boom", ""}
	crashedPackage := batchEvent{label: "pkg", status: "exit status 2", unreported: 1, tail: tail}
	renderIncomplete(&report, []batchEvent{crashedPackage}, reportStyle{color: true})
	want := "\nIncomplete test processes\n" +
		"  \x1b[31m✗\x1b[0m pkg  exit status 2, 1 test without a result\n" +
		"      \x1b[2mpanic: boom\x1b[0m\n\n"
	if report.String() != want {
		t.Fatalf("report = %q", report.String())
	}
}
