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
		})
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

	selector := "passes"
	execution := executeTestGroup("unused", testGroup{
		Key:      testGroupKey{Path: "demo/demo.go", Selector: selector},
		Selector: &selector,
	})
	if pointerValue(execution.Reason) != "unsupported-test-extension" {
		t.Fatalf("unsupported execution = %#v", execution)
	}
}

// @verifies scn.execution.8371d74b5134.integration
func TestExecuteNodeTestDetectsNoMatchingExecution(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/demo.test.ts", "import test from 'node:test'; test('different', () => {});\n")
	passed, ran, err := executeNodeTest(root, "tests/demo.test.ts", "missing")
	if err != nil || passed || ran {
		t.Fatalf("executeNodeTest = %v, %v, %v", passed, ran, err)
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
	passed, ran, err := executeNodeTest(root, "tests/demo.test.mts", "passes")
	if err != nil || !passed || !ran {
		t.Fatalf("executeNodeTest inside a node test context = %v, %v, %v", passed, ran, err)
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
