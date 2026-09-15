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
	writeFixture(t, root, "tests/z.test.mjs", "// @verifies scn.demo.cccccccccccc\ntest(\"zed\", () => {})\n")
	writeFixture(t, root, "tests/a.test.mjs", "// @verifies scn.demo.bbbbbbbbbbbb\ntest(\"alpha\", () => {})\n// @verifies scn.demo.dddddddddddd\ntest(\"unknown\", () => {})\n")
	writeFixture(t, root, "src/demo.mjs", "// @implements req.demo.aaaaaaaaaaaa\nfunction demo() {}\n")
	anchors, err := DiscoverScenarioTests(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 2 || anchors[0].Path != "tests/a.test.mjs" || anchors[1].Path != "tests/z.test.mjs" {
		t.Fatalf("unexpected discovered tests: %#v", anchors)
	}

	if got := selectScenarioTests(ParsedSpecs{}, []Anchor{{Kind: "test", ID: "unknown"}, {Kind: "code", ID: "unknown"}}); len(got) != 0 {
		t.Fatalf("unexpected selected tests: %#v", got)
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
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n")
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.js"), filepath.Join(root, "src", "broken.js")); err != nil {
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
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n#### Scenario: Missing\nVerification-ID: scn.demo.bbbbbbbbbbbb\n")
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
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n#### Scenario: Unresolved\nVerification-ID: scn.demo.bbbbbbbbbbbb\n")
		writeFixture(t, root, "tests/demo.test.mjs", "// @verifies scn.demo.bbbbbbbbbbbb\nconst value = 1;\n")
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
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\nVerification-ID: req.demo.aaaaaaaaaaaa\n#### Scenario: Selected\nVerification-ID: scn.demo.bbbbbbbbbbbb\n")
		writeFixture(t, root, "tests/demo.test.mjs", "// @verifies scn.demo.bbbbbbbbbbbb\ntest(\"selected\", () => {});\n")
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
}

func TestExecuteExactGoTest(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "go.mod", "module example.test/demo\n\ngo 1.24\n")
	writeFixture(t, root, "demo/demo_test.go", `package demo
import "testing"
func TestPass(t *testing.T) {}
func TestFail(t *testing.T) { t.Fatal("no") }
`)
	for _, test := range []struct {
		selector       string
		passed, ran    bool
		wantProcessErr bool
	}{
		{"TestPass", true, true, false},
		{"TestMissing", false, false, false},
		{"TestFail", false, true, true},
	} {
		t.Run(test.selector, func(t *testing.T) {
			passed, ran, err := executeExactTest(root, "demo/demo_test.go", test.selector)
			if passed != test.passed || ran != test.ran || (err != nil) != test.wantProcessErr {
				t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
			}
		})
	}
}

func TestExecuteNodeTestDetectsNoMatchingExecution(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/demo.test.mjs", "import test from 'node:test'; test('different', () => {});\n")
	passed, ran, err := executeNodeTest(root, "tests/demo.test.mjs", "missing")
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
		if err := os.Symlink(filepath.Join(root, "missing.go"), filepath.Join(root, "src", "broken.go")); err != nil {
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
		if err := os.Symlink(filepath.Join(root, "missing.js"), filepath.Join(root, "src", "broken.js")); err != nil {
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
	originalDigest, originalParse, originalScan, originalRun := computeScenarioDigest, parseScenarioSpecs, scanScenarioAnchors, runExactTest
	t.Cleanup(func() {
		computeScenarioDigest, parseScenarioSpecs, scanScenarioAnchors, runExactTest = originalDigest, originalParse, originalScan, originalRun
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
		return ParsedSpecs{Requirements: []Requirement{{Scenarios: []Scenario{{ID: "scn.demo.cccccccccccc"}, {ID: "scn.demo.bbbbbbbbbbbb"}}}}}, nil
	}
	scanScenarioAnchors = func(string) ([]Anchor, error) {
		return []Anchor{
			{ID: "scn.demo.cccccccccccc", Kind: "test", Path: "z.test.mjs", Selector: &secondSelector},
			{ID: "scn.demo.bbbbbbbbbbbb", Kind: "test", Path: "a.test.mjs", Selector: &firstSelector},
		}, nil
	}
	runExactTest = func(string, string, string) (bool, bool, error) { return true, true, nil }
	evidence, err := RunScenarioTests(root, "example", "")
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Executions[0].Path != "a.test.mjs" || evidence.Scenarios[0].ID != "scn.demo.bbbbbbbbbbbb" {
		t.Fatalf("results were not sorted: %#v", evidence)
	}
}
