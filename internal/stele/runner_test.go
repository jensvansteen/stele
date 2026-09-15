package stele

import (
	"errors"
	"os"
	"path/filepath"
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

func TestExecuteExactTestRejectsUnsupportedExtensions(t *testing.T) {
	for _, test := range []struct {
		path    string
		message string
	}{
		{
			"demo/demo_test.go",
			`unsupported test file extension ".go"; supported extensions: .mts, .ts`,
		},
		{
			"tests/demo.test.mjs",
			`unsupported test file extension ".mjs"; supported extensions: .mts, .ts`,
		},
		{
			"tests/demo.test.tsx",
			`unsupported test file extension ".tsx"; supported extensions: .mts, .ts`,
		},
		{
			"tests/demo",
			`unsupported test file extension ""; supported extensions: .mts, .ts`,
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
		Key:      testGroupKey{Path: "demo/demo_test.go", Selector: selector},
		Selector: &selector,
	})
	if pointerValue(execution.Reason) != "unsupported-test-extension" {
		t.Fatalf("unsupported execution = %#v", execution)
	}
}

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

func TestRunScenarioTestsReturnsInputAndWriteErrors(t *testing.T) {
	t.Run("input", func(t *testing.T) {
		root := fixtureRoot(t)
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
		writeFixture(t, root, "artifacts", "blocking file")
		if _, err := RunScenarioTests(root, "example", "artifacts/evidence.json"); err == nil {
			t.Fatal("expected evidence write error")
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
	computeScenarioDigest = func(string) (string, error) { return "digest", nil }
	parseScenarioSpecs = func(string, string) (ParsedSpecs, error) { return ParsedSpecs{}, errors.New("parse failed") }
	if _, err := RunScenarioTests(root, "example", ""); err == nil {
		t.Fatal("expected injected parse error")
	}
	parseScenarioSpecs = func(string, string) (ParsedSpecs, error) { return ParsedSpecs{}, nil }
	scanScenarioAnchors = func(string) ([]Anchor, error) { return nil, errors.New("scan failed") }
	if _, err := RunScenarioTests(root, "example", ""); err == nil {
		t.Fatal("expected injected scan error")
	}

	firstSelector, secondSelector := "first", "second"
	parseScenarioSpecs = func(string, string) (ParsedSpecs, error) {
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
