package stele

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDiscoverScenarioTestsFiltersAndSorts(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Zed
Verification-ID: scn.demo.cccccccccccc
#### Scenario: Alpha
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(t, root, "tests/z.test.ts", "// @verifies scn.demo.cccccccccccc\ntest(\"zed\", () => {})\n")
	writeFixture(t, root, "tests/a.test.ts", `// @verifies scn.demo.bbbbbbbbbbbb
test("alpha", () => {})
// @verifies scn.demo.dddddddddddd
test("unknown", () => {})
`)
	writeFixture(t, root, "src/demo.ts", "// @implements req.demo.aaaaaaaaaaaa\nfunction demo() {}\n")
	anchors, err := DiscoverScenarioTests(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 2 || anchors[0].Path != "tests/a.test.ts" || anchors[1].Path != "tests/z.test.ts" {
		t.Fatalf("unexpected discovered tests: %#v", anchors)
	}

	selected := selectScenarioTests(
		ParsedSpecs{},
		[]Anchor{{Kind: "test", ID: "unknown"}, {Kind: "code", ID: "unknown"}},
	)
	if len(selected) != 0 {
		t.Fatalf("unexpected selected tests: %#v", selected)
	}
}

func TestGroupScenarioTestsUsesTypedTargetsAndSortsIDs(t *testing.T) {
	alpha, beta, empty := "alpha", "beta", ""
	groups := groupScenarioTests([]Anchor{
		{ID: "scn.demo.cccccccccccc", Path: "b.test.ts", Selector: &alpha},
		{ID: "scn.demo.bbbbbbbbbbbb", Path: "a.test.ts", Selector: &alpha},
		{ID: "scn.demo.aaaaaaaaaaaa", Path: "a.test.ts", Selector: &alpha},
		{ID: "scn.demo.dddddddddddd", Path: "a.test.ts", Selector: &beta},
		{ID: "scn.demo.eeeeeeeeeeee", Path: "a.test.ts"},
		{ID: "scn.demo.ffffffffffff", Path: "a.test.ts", Selector: &empty},
	})
	if len(groups) != 4 {
		t.Fatalf("groupScenarioTests returned %d groups: %#v", len(groups), groups)
	}
	if groups[0].Selector != nil {
		t.Fatalf("group did not retain the first anchor's unresolved selector: %#v", groups[0])
	}
	if groups[1].Key.Selector != "alpha" || groups[2].Key.Selector != "beta" || groups[3].Key.Path != "b.test.ts" {
		t.Fatalf("groups were not sorted by path and selector: %#v", groups)
	}
	if got := groups[1].IDs; len(got) != 2 || got[0] != "scn.demo.aaaaaaaaaaaa" || got[1] != "scn.demo.bbbbbbbbbbbb" {
		t.Fatalf("shared target IDs were not sorted: %#v", got)
	}
	if supportedSource("README.md") {
		t.Fatal("Markdown must not be scanned for anchors")
	}
}

func TestDiscoverScenarioTestsReturnsDependencyErrors(t *testing.T) {
	t.Run("spec", func(t *testing.T) {
		root := fixtureRoot(t)
		directory := filepath.Join(root, "openspec", "changes", "example", "specs")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.md"), filepath.Join(directory, "broken.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := DiscoverScenarioTests(root, "example"); err == nil {
			t.Fatal("expected spec error")
		}
	})
	t.Run("anchor", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(
			t,
			root,
			"openspec/changes/example/specs/demo/spec.md",
			"### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n",
		)
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.ts"), filepath.Join(root, "src", "broken.ts")); err != nil {
			t.Fatal(err)
		}
		if _, err := DiscoverScenarioTests(root, "example"); err == nil {
			t.Fatal("expected anchor error")
		}
	})
}

func TestRunScenarioTestsHandlesMissingAndUnresolvedTests(t *testing.T) {
	t.Run("not run", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Missing
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
		evidence, err := RunScenarioTests(root, "example", "")
		if err != nil {
			t.Fatal(err)
		}
		if evidence.Outcome != "failed" || len(evidence.Executions) != 0 || evidence.Scenarios[0].Outcome != "not-run" {
			t.Fatalf("unexpected evidence: %#v", evidence)
		}
	})

	t.Run("target not resolved", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Unresolved
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
		writeFixture(t, root, "tests/demo.test.ts", "// @verifies scn.demo.bbbbbbbbbbbb\nconst value = 1;\n")
		evidence, err := RunScenarioTests(root, "example", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(evidence.Executions) != 1 || pointerValue(evidence.Executions[0].Reason) != "target-not-resolved" {
			t.Fatalf("unexpected unresolved evidence: %#v", evidence)
		}
	})

	t.Run("selected but not executed", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Selected
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
		writeFixture(t, root, "tests/demo.test.ts", `// @verifies scn.demo.bbbbbbbbbbbb
test("selected", () => {});
`)
		original := runExactTest
		t.Cleanup(func() { runExactTest = original })
		runOneByOne(t)
		runExactTest = func(string, string, string) (bool, bool, error) { return false, false, nil }
		evidence, err := RunScenarioTests(root, "example", "")
		if err != nil {
			t.Fatal(err)
		}
		if pointerValue(evidence.Executions[0].Reason) != "test-not-executed" {
			t.Fatalf("unexpected evidence: %#v", evidence)
		}
	})

	t.Run("executed failure without process error", func(t *testing.T) {
		selector := "selected"
		original := runExactTest
		t.Cleanup(func() { runExactTest = original })
		runExactTest = func(string, string, string) (bool, bool, error) { return false, true, nil }
		execution := executeTestGroup("unused", testGroup{
			Key:      testGroupKey{Path: "tests/demo.test.ts", Selector: selector},
			Selector: &selector,
			IDs:      []string{"scn.demo.bbbbbbbbbbbb"},
		})
		if execution.Outcome != "failed" || pointerValue(execution.Reason) != "test-process-failed" {
			t.Fatalf("unexpected execution: %#v", execution)
		}
	})
}

// @verifies scn.execution.bce9246e1444.integration
func TestExecuteExactTypeScriptTest(t *testing.T) {
	root := fixtureRoot(t)
	testSource := `import test from "node:test";
const value: number = 1;
test("passes", () => { if (value !== 1) throw new Error("failed"); });
`
	for _, test := range []struct {
		path string
	}{
		{"tests/demo.test.ts"},
		{"tests/demo.test.mts"},
	} {
		t.Run(test.path, func(t *testing.T) {
			writeFixture(t, root, test.path, testSource)
			passed, ran, err := executeExactTest(root, test.path, "passes")
			if err != nil || !passed || !ran {
				t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
			}
			if execution := runBatchedGroup(root, test.path, "passes"); execution.Outcome != "passed" {
				t.Fatalf("batched execution = %#v", execution)
			}
		})
	}
}

// @verifies scn.execution.8653331d296a.integration
func TestBatchRunsOneNodeFile(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/demo.test.mts", "import test from \"node:test\";\n"+
		"void test(\"passes\", () => {});\n"+
		"void test(\"fails\", () => { throw new Error(\"no\"); });\n"+
		"void test(\"skips\", (context) => { context.skip(\"not now\"); });\n")
	processes := countBatchProcesses(t)
	executions := runBatched(root,
		selectedGroup("tests/demo.test.mts", "passes"),
		selectedGroup("tests/demo.test.mts", "fails"),
		selectedGroup("tests/demo.test.mts", "skips"),
		selectedGroup("tests/demo.test.mts", "missing"),
	)
	if *processes != 1 {
		t.Fatalf("the file's tests started %d processes, want 1", *processes)
	}
	want := []string{"passed:", "failed:test-process-failed", "failed:test-skipped", "failed:test-not-executed"}
	if got := reasonsOf(executions); !slices.Equal(got, want) {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}
}

// @verifies scn.execution.a9dde959cf57.unit
func TestExecuteExactTestRejectsUnsupportedExtensions(t *testing.T) {
	for _, test := range []struct {
		path    string
		message string
	}{
		{
			"demo/demo.go",
			`unsupported test file extension ".go"; supported extensions: .mts, .ts, _test.go`,
		},
		{
			"tests/demo.test.mjs",
			`unsupported test file extension ".mjs"; supported extensions: .mts, .ts, _test.go`,
		},
		{
			"tests/demo.test.tsx",
			`unsupported test file extension ".tsx"; supported extensions: .mts, .ts, _test.go`,
		},
		{
			"tests/demo",
			`unsupported test file extension ""; supported extensions: .mts, .ts, _test.go`,
		},
	} {
		t.Run(test.path, func(t *testing.T) {
			passed, ran, err := executeExactTest("unused", test.path, "passes")
			if passed || ran || !errors.Is(err, errUnsupportedTestExtension) {
				t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
			}
			if err.Error() != test.message {
				t.Fatalf("error = %q, want %q", err, test.message)
			}
		})
	}

	processes := countBatchProcesses(t)
	group := selectedGroup("demo/demo.go", "passes")
	if plan := planTestBatches("unused", []testGroup{group}); len(plan.batches) != 0 || len(plan.settled) != 1 {
		t.Fatalf("a file without a runner was batched: %#v", plan)
	}
	execution := runBatched("unused", group)[0]
	if pointerValue(execution.Reason) != "unsupported-test-extension" || *processes != 0 {
		t.Fatalf("unsupported execution = %#v, %d processes", execution, *processes)
	}
}

// @verifies scn.execution.8371d74b5134.integration
func TestExecuteNodeTestDetectsNoMatchingExecution(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/demo.test.ts", "import test from 'node:test'; test('different', () => {});\n")
	passed, ran, err := executeExactTest(root, "tests/demo.test.ts", "missing")
	if err != nil || passed || ran {
		t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
	}
	execution := runBatchedGroup(root, "tests/demo.test.ts", "missing")
	if execution.Outcome != "failed" || pointerValue(execution.Reason) != "test-not-executed" {
		t.Fatalf("batched execution = %#v", execution)
	}
	if pointerValue(nil) != "" {
		t.Fatal("nil pointer value should be empty")
	}
}

// @verifies scn.execution.9cbf5cc6d03d.integration
func TestExecuteNodeTestIgnoresEnclosingTestRunner(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/demo.test.mts", "import test from 'node:test';\nvoid test('passes', () => {});\n")
	t.Setenv("NODE_TEST_CONTEXT", "child-v8")
	passed, ran, err := executeExactTest(root, "tests/demo.test.mts", "passes")
	if err != nil || !passed || !ran {
		t.Fatalf("executeExactTest inside a node test context = %v, %v, %v", passed, ran, err)
	}
	for _, entry := range nodeTestEnvironment() {
		if strings.HasPrefix(entry, "NODE_TEST_CONTEXT=") {
			t.Fatalf("child environment still contains %s", entry)
		}
	}
	if !slices.Contains(nodeTestEnvironment(), "STELE_CHILD_TEST=1") {
		t.Fatal("child environment does not mark Stele child tests")
	}
}

func TestRunScenarioTestsReturnsInputAndWriteErrors(t *testing.T) {
	t.Run("input", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.ts"), filepath.Join(root, "src", "broken.ts")); err != nil {
			t.Fatal(err)
		}
		if _, err := RunScenarioTests(root, "example", ""); err == nil {
			t.Fatal("expected digest error")
		}
	})

	t.Run("spec", func(t *testing.T) {
		root := fixtureRoot(t)
		directory := filepath.Join(root, "openspec", "changes", "example", "specs")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, directory, "bad.md", string(make([]byte, 70_000)))
		if _, err := RunScenarioTests(root, "example", ""); err == nil {
			t.Fatal("expected spec error")
		}
	})

	t.Run("anchor", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.ts"), filepath.Join(root, "src", "broken.ts")); err != nil {
			t.Fatal(err)
		}
		if _, err := RunScenarioTests(root, "example", ""); err == nil {
			t.Fatal("expected anchor error")
		}
	})

	t.Run("evidence", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		writeFixture(t, root, "artifacts", "blocking file")
		if _, err := RunScenarioTests(root, "example", "artifacts/evidence.json"); err == nil {
			t.Fatal("expected evidence write error")
		}
	})

	t.Run("no scenarios", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		evidence, err := RunScenarioTests(root, "example", "")
		if err != nil {
			t.Fatal(err)
		}
		if evidence.Outcome != "failed" || len(evidence.Scenarios) != 0 {
			t.Fatalf("a scope without scenarios must not pass: %#v", evidence)
		}
	})
}

func TestRunScenarioTestsDependencyErrorsAndOrdering(t *testing.T) {
	originalDigest := computeScenarioDigest
	originalParse := parseScenarioSpecs
	originalScan := scanScenarioAnchors
	originalRun := runExactTest
	t.Cleanup(func() {
		computeScenarioDigest = originalDigest
		parseScenarioSpecs = originalParse
		scanScenarioAnchors = originalScan
		runExactTest = originalRun
	})
	runOneByOne(t)
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
	computeScenarioDigest = func(string) (string, error) { return "digest", nil }
	parseScenarioSpecs = func(string, verificationScope) (ParsedSpecs, error) {
		return ParsedSpecs{}, errors.New("parse failed")
	}
	if _, err := RunScenarioTests(root, "example", ""); err == nil {
		t.Fatal("expected injected parse error")
	}
	parseScenarioSpecs = func(string, verificationScope) (ParsedSpecs, error) { return ParsedSpecs{}, nil }
	scanScenarioAnchors = func(string) ([]Anchor, error) { return nil, errors.New("scan failed") }
	if _, err := RunScenarioTests(root, "example", ""); err == nil {
		t.Fatal("expected injected scan error")
	}

	firstSelector, secondSelector := "first", "second"
	parseScenarioSpecs = func(string, verificationScope) (ParsedSpecs, error) {
		return ParsedSpecs{
			Requirements: []Requirement{{
				Scenarios: []Scenario{
					{ID: "scn.demo.cccccccccccc"},
					{ID: "scn.demo.bbbbbbbbbbbb"},
				},
			}},
		}, nil
	}
	scanScenarioAnchors = func(string) ([]Anchor, error) {
		return []Anchor{
			{ID: "scn.demo.cccccccccccc", Kind: "test", Path: "z.test.ts", Selector: &secondSelector},
			{ID: "scn.demo.bbbbbbbbbbbb", Kind: "test", Path: "a.test.ts", Selector: &firstSelector},
		}, nil
	}
	runExactTest = func(string, string, string) (bool, bool, error) { return true, true, nil }
	evidence, err := RunScenarioTests(root, "example", "")
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Executions[0].Path != "a.test.ts" || evidence.Scenarios[0].ID != "scn.demo.bbbbbbbbbbbb" {
		t.Fatalf("results were not sorted: %#v", evidence)
	}
}

// @verifies scn.verificationstrategy.2f332f9ca4e2.unit
func TestScenarioFailsWhenOneEvidenceFails(t *testing.T) {
	root := evidenceFixture(t)
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeFixture(t, root, "tests/journey.test.mts", "import test from \"node:test\";\n"+
		"// @verifies "+evidenceScenarioID+".e2e\n"+
		"// @verifies "+otherScenarioID+".e2e\n"+
		"void test(\"runs the journey\", () => {});\n")
	writeEvidenceTest(t, root, "tests/other.test.mts", otherScenarioID+".unit", "reports a missing value")
	original := runExactTest
	t.Cleanup(func() { runExactTest = original })
	runOneByOne(t)
	runExactTest = func(_ string, path, _ string) (bool, bool, error) {
		return path != "tests/journey.test.mts", true, nil
	}
	evidence, err := RunScenarioTests(root, "example", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []ScenarioOutcome{
		{ID: evidenceScenarioID, Outcome: "failed", FailedEvidence: []string{evidenceScenarioID + ".e2e"}},
		{ID: otherScenarioID, Outcome: "failed", FailedEvidence: []string{otherScenarioID + ".e2e"}},
	}
	if evidence.Outcome != "failed" || len(evidence.Scenarios) != 2 {
		t.Fatalf("evidence = %#v", evidence)
	}
	for index, scenario := range want {
		got := evidence.Scenarios[index]
		if got.ID != scenario.ID || got.Outcome != scenario.Outcome ||
			!slices.Equal(got.FailedEvidence, scenario.FailedEvidence) {
			t.Fatalf("scenario %d = %#v, want %#v", index, got, scenario)
		}
	}
	for _, execution := range evidence.Executions {
		if execution.Path == "tests/journey.test.mts" &&
			!slices.Equal(execution.EvidenceIDs, []string{evidenceScenarioID + ".e2e", otherScenarioID + ".e2e"}) {
			t.Fatalf("journey execution = %#v", execution)
		}
		if execution.Path == "tests/value.test.mts" && execution.Outcome != "passed" {
			t.Fatalf("unit evidence did not pass: %#v", execution)
		}
	}

	runExactTest = func(string, string, string) (bool, bool, error) { return true, true, nil }
	passed, err := RunScenarioTests(root, "example", "")
	if err != nil || passed.Outcome != "passed" || passed.Scenarios[0].FailedEvidence != nil {
		t.Fatalf("passing evidence = %#v, %v", passed, err)
	}
}

// behaviorFixture returns a change "example" with two spec files. Requirement
// aaaa has scenario bbbb (unit and e2e tests) and cccc; requirement dddd has
// eeee and ffff; the other file declares requirement 1111 with scenario 2222.
// cccc and eeee share one test.
func behaviorFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `## ADDED Requirements
### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
#### Scenario: Missing value is reported
Verification-ID: scn.demo.cccccccccccc
### Requirement: Store value
Verification-ID: req.demo.dddddddddddd
#### Scenario: Value is stored
Verification-ID: scn.demo.eeeeeeeeeeee
#### Scenario: Value is replaced
Verification-ID: scn.demo.ffffffffffff
`)
	writeFixture(t, root, "openspec/changes/example/specs/other/spec.md", `## ADDED Requirements
### Requirement: Other
Verification-ID: req.other.111111111111
#### Scenario: Other behavior
Verification-ID: scn.other.222222222222
`)
	writeEvidenceTest(t, root, "tests/b.test.mts", "scn.demo.bbbbbbbbbbbb.unit", "b unit")
	writeEvidenceTest(t, root, "tests/e2e/b.test.mts", "scn.demo.bbbbbbbbbbbb.e2e", "b e2e")
	writeEvidenceTest(t, root, "tests/c.test.mts", "scn.demo.cccccccccccc.unit", "c unit")
	writeEvidenceTest(t, root, "tests/e.test.mts", "scn.demo.eeeeeeeeeeee.unit", "e unit")
	writeEvidenceTest(t, root, "tests/f.test.mts", "scn.demo.ffffffffffff.unit", "f unit")
	writeEvidenceTest(t, root, "tests/other.test.mts", "scn.other.222222222222.unit", "other unit")
	writeFixture(t, root, "tests/shared.test.mts", "import test from \"node:test\";\n"+
		"// @verifies scn.demo.cccccccccccc\n"+
		"// @verifies scn.demo.eeeeeeeeeeee\n"+
		"void test(\"shared\", () => {});\n")
	return root
}

// recordTests replaces the test runner with a stub that records every test it
// runs and fails the named selectors.
func recordTests(t *testing.T, failing ...string) *[]string {
	t.Helper()
	original := runExactTest
	t.Cleanup(func() { runExactTest = original })
	runOneByOne(t)
	ran := make([]string, 0)
	runExactTest = func(_, _, selector string) (bool, bool, error) {
		ran = append(ran, selector)
		return !slices.Contains(failing, selector), true, nil
	}
	return &ran
}

func runTargets(t *testing.T, root string, targets ...string) testRun {
	t.Helper()
	run, err := runScopeTests(testRequest{
		root:         root,
		scope:        changeScope("example"),
		evidencePath: defaultEvidencePath,
		targets:      targets,
	})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func storedEvidence(t *testing.T, root string) Evidence {
	t.Helper()
	var evidence Evidence
	if !readJSON(filepath.Join(root, defaultEvidencePath), &evidence) {
		t.Fatal("no stored evidence")
	}
	return evidence
}

func scenarioOutcome(evidence Evidence, id string) string {
	for _, scenario := range evidence.Scenarios {
		if scenario.ID == id {
			return scenario.Outcome
		}
	}
	return ""
}

func executionOutcome(evidence Evidence, selector string) string {
	for _, execution := range evidence.Executions {
		if pointerValue(execution.Selector) == selector {
			return execution.Outcome
		}
	}
	return ""
}

// @verifies scn.linkindex.352d7120ec6c.unit
func TestRunSelectedScenarioMergesEvidence(t *testing.T) {
	root := behaviorFixture(t)
	recordTests(t)
	runTargets(t, root)
	if stored := storedEvidence(t, root); stored.Outcome != "passed" || stored.SchemaVersion != 3 {
		t.Fatalf("full run = %#v", stored)
	}

	ran := recordTests(t, "b e2e")
	run := runTargets(t, root, "scn.demo.bbbbbbbbbbbb")
	if slices.Sort(*ran); !slices.Equal(*ran, []string{"b e2e", "b unit"}) {
		t.Fatalf("ran %v", *ran)
	}
	if len(run.selected) != 2 || !run.targeted {
		t.Fatalf("selected executions = %#v", run.selected)
	}
	stored := storedEvidence(t, root)
	if scenarioOutcome(stored, "scn.demo.bbbbbbbbbbbb") != "failed" ||
		executionOutcome(stored, "b e2e") != "failed" || executionOutcome(stored, "b unit") != "passed" {
		t.Fatalf("the selected outcomes were not recorded: %#v", stored)
	}
	for _, id := range []string{
		"scn.demo.cccccccccccc", "scn.demo.eeeeeeeeeeee", "scn.demo.ffffffffffff", "scn.other.222222222222",
	} {
		if scenarioOutcome(stored, id) != "passed" {
			t.Fatalf("the outcome of %s changed: %#v", id, stored)
		}
	}
	if len(stored.Executions) != 7 || stored.Outcome != "failed" {
		t.Fatalf("other executions were discarded: %#v", stored)
	}
}

// @verifies scn.linkindex.1f063f3c5e49.unit
func TestRunSelectedEvidenceOnly(t *testing.T) {
	root := behaviorFixture(t)
	recordTests(t)
	runTargets(t, root)

	ran := recordTests(t, "b e2e", "b unit")
	runTargets(t, root, "scn.demo.bbbbbbbbbbbb.e2e")
	if !slices.Equal(*ran, []string{"b e2e"}) {
		t.Fatalf("ran %v", *ran)
	}
	stored := storedEvidence(t, root)
	if executionOutcome(stored, "b e2e") != "failed" || executionOutcome(stored, "b unit") != "passed" {
		t.Fatalf("outcomes other than the e2e test changed: %#v", stored)
	}
}

// @verifies scn.linkindex.49a524bcf0ac.unit
func TestRunSelectedRequirementScenarios(t *testing.T) {
	root := behaviorFixture(t)
	ran := recordTests(t)
	run := runTargets(t, root, "req.demo.dddddddddddd")
	if slices.Sort(*ran); !slices.Equal(*ran, []string{"e unit", "f unit", "shared"}) {
		t.Fatalf("ran %v", *ran)
	}
	// Without earlier evidence, scenarios whose tests did not run are not run.
	if scenarioOutcome(run.evidence, "scn.demo.eeeeeeeeeeee") != "passed" ||
		scenarioOutcome(run.evidence, "scn.demo.ffffffffffff") != "passed" ||
		scenarioOutcome(run.evidence, "scn.demo.cccccccccccc") != "not-run" ||
		run.evidence.Outcome != "failed" {
		t.Fatalf("evidence = %#v", run.evidence)
	}
}

// @verifies scn.linkindex.e0b21aa95623.unit
func TestRunSelectedSpecFileScenarios(t *testing.T) {
	root := behaviorFixture(t)
	ran := recordTests(t)
	runTargets(t, root, "openspec/changes/example/specs/other/spec.md")
	if !slices.Equal(*ran, []string{"other unit"}) {
		t.Fatalf("ran %v", *ran)
	}
	*ran = (*ran)[:0]
	runTargets(t, root, filepath.Join(root, "openspec", "changes", "example", "specs", "demo", "spec.md"))
	if slices.Sort(*ran); !slices.Equal(*ran, []string{"b e2e", "b unit", "c unit", "e unit", "f unit", "shared"}) {
		t.Fatalf("ran %v", *ran)
	}
}

// @verifies scn.linkindex.6265c70bed70.unit
func TestRunSelectedTargetsCombine(t *testing.T) {
	root := behaviorFixture(t)
	ran := recordTests(t)
	runTargets(t, root, "scn.demo.cccccccccccc", "req.demo.dddddddddddd", "scn.demo.eeeeeeeeeeee")
	if slices.Sort(*ran); !slices.Equal(*ran, []string{"c unit", "e unit", "f unit", "shared"}) {
		t.Fatalf("ran %v", *ran)
	}
}

func TestRunSelectedMergesOlderEvidence(t *testing.T) {
	root := behaviorFixture(t)
	unit, other := "b unit", "retired"
	older := Evidence{
		SchemaVersion: 2,
		InputDigest:   "older",
		Scenarios: []ScenarioOutcome{
			{ID: "scn.demo.cccccccccccc", Outcome: "passed"},
			{ID: "scn.retired.aaaaaaaaaaaa", Outcome: "passed"},
		},
		Executions: []TestExecution{
			{Path: "tests/b.test.mts", Selector: &unit, Outcome: "passed"},
			{Path: "tests/retired.test.mts", Selector: &other, Outcome: "passed"},
		},
	}
	if err := writeJSON(filepath.Join(root, defaultEvidencePath), older); err != nil {
		t.Fatal(err)
	}
	recordTests(t)
	run := runTargets(t, root, "scn.demo.bbbbbbbbbbbb.e2e")
	// The unit test ran with older inputs, so the scenario's outcome is stale.
	if scenarioOutcome(run.evidence, "scn.demo.bbbbbbbbbbbb") != "stale" {
		t.Fatalf("scope evidence = %#v", run.evidence)
	}
	stored := storedEvidence(t, root)
	for _, execution := range stored.Executions {
		if execution.InputDigest == "" {
			t.Fatalf("an older execution kept no digest: %#v", execution)
		}
	}
	if len(stored.Executions) != 3 || scenarioOutcome(stored, "scn.retired.aaaaaaaaaaaa") != "stale" ||
		stored.Outcome != "failed" {
		t.Fatalf("stored evidence = %#v", stored)
	}

	// Planned evidence without an anchored test is a known target that runs nothing.
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		"scn.demo.bbbbbbbbbbbb": {entry("scn.demo.bbbbbbbbbbbb", "integration", "Storage.")},
	})
	ran := recordTests(t)
	if run := runTargets(t, root, "scn.demo.bbbbbbbbbbbb.integration"); len(*ran) != 0 || len(run.selected) != 0 {
		t.Fatalf("planned evidence without tests ran %v", *ran)
	}
}
