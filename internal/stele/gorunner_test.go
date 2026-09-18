package stele

import (
	"errors"
	"go/build/constraint"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

const goRunnerTests = `package pkg

import "testing"

func TestPasses(t *testing.T) {}

func TestPassesExtra(t *testing.T) { t.Fatal("selected by a prefix match") }

func TestFails(t *testing.T) { t.Fatal("expected failure") }

func TestSkips(t *testing.T) { t.Skip("skipped on purpose") }
`

// goRunnerModule writes a dependency-free module below a repository root, so
// the runner has to find the nearest go.mod.
func goRunnerModule(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "module/go.mod", "module example.com/fixture\n\ngo 1.22\n")
	writeFixture(t, root, "module/pkg/demo_test.go", goRunnerTests)
	writeFixture(t, root, "module/pkg/tagged_test.go", "//go:build integration && !windows\n\n"+
		"package pkg\n\nimport \"testing\"\n\nfunc TestTagged(t *testing.T) {}\n")
	writeFixture(t, root, "module/pkg/never_test.go", "//go:build linux && darwin\n\n"+
		"package pkg\n\nimport \"testing\"\n\nfunc TestNever(t *testing.T) {}\n")
	writeFixture(t, root, "module/badtags/bad_test.go", "//go:build (\n\npackage badtags\n")
	return root
}

func runGoGroup(root, path, selector string) TestExecution {
	return executeTestGroup(root, testGroup{
		Key:      testGroupKey{Path: path, Selector: selector},
		Selector: &selector,
		IDs:      []string{"scn.demo.aaaaaaaaaaaa"},
	})
}

// @verifies scn.gosupport.d195095fc292.integration
func TestExecuteExactGoTestPasses(t *testing.T) {
	root := goRunnerModule(t)
	passed, ran, err := executeExactTest(root, "module/pkg/demo_test.go", "TestPasses")
	if err != nil || !passed || !ran {
		t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
	}
	execution := runGoGroup(root, "module/pkg/demo_test.go", "TestPasses")
	if execution.Outcome != "passed" || execution.Reason != nil {
		t.Fatalf("passing Go test was not recorded as passed: %#v", execution)
	}
}

// @verifies scn.gosupport.208b6a95ea3f.integration
func TestExecuteExactGoTestFails(t *testing.T) {
	root := goRunnerModule(t)
	execution := runGoGroup(root, "module/pkg/demo_test.go", "TestFails")
	if execution.Outcome != "failed" || pointerValue(execution.Reason) != "test-process-failed" {
		t.Fatalf("failing Go test = %#v", execution)
	}
}

// @verifies scn.gosupport.75d202e0ed36.integration
func TestExecuteExactGoTestReportsSkip(t *testing.T) {
	root := goRunnerModule(t)
	passed, ran, err := executeExactTest(root, "module/pkg/demo_test.go", "TestSkips")
	if passed || !ran || !errors.Is(err, errTestSkipped) {
		t.Fatalf("executeExactTest = %v, %v, %v", passed, ran, err)
	}
	execution := runGoGroup(root, "module/pkg/demo_test.go", "TestSkips")
	if execution.Outcome != "failed" || pointerValue(execution.Reason) != "test-skipped" {
		t.Fatalf("skipped Go test = %#v", execution)
	}
}

// @verifies scn.gosupport.203331e4d990.integration
func TestExecuteExactGoTestDetectsNoMatchingExecution(t *testing.T) {
	root := goRunnerModule(t)
	for _, test := range []struct {
		path     string
		selector string
	}{
		{"module/pkg/demo_test.go", "TestMissing"},
		{"module/pkg/never_test.go", "TestNever"},
		{"module/badtags/bad_test.go", "TestBad"},
	} {
		execution := runGoGroup(root, test.path, test.selector)
		if execution.Outcome != "failed" || pointerValue(execution.Reason) != "test-not-executed" {
			t.Fatalf("%s#%s = %#v", test.path, test.selector, execution)
		}
	}

	execution := runGoGroup(root, "module/pkg/missing_test.go", "TestMissing")
	if pointerValue(execution.Reason) != "test-process-failed" {
		t.Fatalf("missing Go test file = %#v", execution)
	}

	original := relativePath
	t.Cleanup(func() { relativePath = original })
	relativePath = func(string, string) (string, error) { return "", errors.New("relative failed") }
	if _, _, err := executeGoTest(root, "module/pkg/demo_test.go", "TestPasses"); err == nil {
		t.Fatal("expected relative path error")
	}
}

// @verifies scn.gosupport.4a0637d44b7a.integration
func TestExecuteExactGoTestAppliesBuildConstraints(t *testing.T) {
	root := goRunnerModule(t)
	execution := runGoGroup(root, "module/pkg/tagged_test.go", "TestTagged")
	if execution.Outcome != "passed" {
		t.Fatalf("tagged Go test = %#v", execution)
	}

	// A test file directly in the module root runs as package ".".
	writeFixture(t, root, "go.mod", "module example.com/root\n\ngo 1.22\n")
	writeFixture(t, root, "root_test.go", "package root\n\nimport \"testing\"\n\nfunc TestRoot(t *testing.T) {}\n")
	if execution := runGoGroup(root, "root_test.go", "TestRoot"); execution.Outcome != "passed" {
		t.Fatalf("root package Go test = %#v", execution)
	}
}

func TestGoBuildTags(t *testing.T) {
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}
	for _, test := range []struct {
		source    string
		tags      []string
		satisfied bool
	}{
		{"package demo\n", nil, true},
		{"// Copyright\n\n//go:build integration || e2e\n\npackage demo\n", []string{"e2e", "integration"}, true},
		{"//go:build " + runtime.GOOS + " && " + runtime.GOARCH + " && gc && go1.1\npackage demo\n", []string{}, true},
		{"//go:build !" + other + " && !gccgo\npackage demo\n", []string{}, true},
		{"//go:build " + other + "\npackage demo\n", []string{}, false},
		{"//go:build integration && !integration\npackage demo\n", []string{"integration"}, false},
		{"//go:build (\npackage demo\n", nil, false},
		{"package demo\n//go:build ignored\n", nil, true},
	} {
		tags, satisfied := goBuildTags([]byte(test.source))
		if satisfied != test.satisfied || !slices.Equal(tags, test.tags) {
			t.Errorf("goBuildTags(%q) = %v, %v; want %v, %v", test.source, tags, satisfied, test.tags, test.satisfied)
		}
	}

	unix := platformTagHolds("unix")
	if unix != slices.Contains(goUnixSystems, runtime.GOOS) {
		t.Errorf("platformTagHolds(unix) = %v", unix)
	}
	_ = platformTagHolds("cgo")
	if platformTagHolds("go1.999") {
		t.Error("an unreleased Go version tag holds")
	}
	expression, err := constraint.Parse("//go:build cgo || unix")
	if err != nil {
		t.Fatal(err)
	}
	if tags := positiveCustomTags(expression); len(tags) != 0 {
		t.Errorf("platform tags were passed as custom tags: %v", tags)
	}
}

func TestNearestGoModule(t *testing.T) {
	root := fixtureRoot(t)
	nested := filepath.Join(root, "a", "b")
	if got := nearestGoModule(root, nested); got != root {
		t.Fatalf("nearestGoModule without go.mod = %q, want %q", got, root)
	}
	separator := string(filepath.Separator)
	if got := nearestGoModule(filepath.Join(root, "unrelated"), separator); got != filepath.Join(root, "unrelated") {
		t.Fatalf("nearestGoModule at the filesystem root = %q", got)
	}
}

func TestScenarioOutcomesRequireEveryLinkedTest(t *testing.T) {
	parsed := ParsedSpecs{Requirements: []Requirement{{Scenarios: []Scenario{{ID: "scn.demo.aaaaaaaaaaaa"}}}}}
	groups := []testGroup{
		{Key: testGroupKey{Path: "a.test.ts", Selector: "a"}, IDs: []string{"scn.demo.aaaaaaaaaaaa"}},
		{Key: testGroupKey{Path: "b.test.ts", Selector: "b"}, IDs: []string{"scn.demo.aaaaaaaaaaaa"}},
	}
	first, second := "a", "b"
	executions := []TestExecution{
		{Path: "a.test.ts", Selector: &first, Outcome: "failed", InputDigest: "digest"},
		{Path: "b.test.ts", Selector: &second, Outcome: "passed", InputDigest: "digest"},
	}
	outcomes := scenarioOutcomes(parsed, groups, executions, "digest")
	if len(outcomes) != 1 || outcomes[0].Outcome != "failed" {
		t.Fatalf("a failing linked test was masked: %#v", outcomes)
	}
	if outcomes := scenarioOutcomes(parsed, groups, executions[1:], "digest"); outcomes[0].Outcome != "not-run" {
		t.Fatalf("a test without an execution did not count as not run: %#v", outcomes)
	}
	if outcomes := scenarioOutcomes(parsed, groups[1:], executions[1:], "other"); outcomes[0].Outcome != "stale" {
		t.Fatalf("an execution for other inputs did not count as stale: %#v", outcomes)
	}
}
