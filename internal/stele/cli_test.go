package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCLIHasStableExitCodes(t *testing.T) {
	var output bytes.Buffer
	var errors bytes.Buffer
	if code := Run([]string{"unknown"}, &output, &errors); code != 2 {
		t.Fatalf("unknown command exit code = %d", code)
	}
	if !strings.Contains(errors.String(), "unknown command") {
		t.Fatalf("missing invocation error: %s", errors.String())
	}
}

// @verifies scn.init.cbf5781012fa.unit
func TestInitializeWritesConfigAndSkills(t *testing.T) {
	root := fixtureRoot(t)
	created, err := Initialize(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1+len(steleSkills) {
		t.Fatalf("expected config plus five skills, got %#v", created)
	}
	for _, skill := range steleSkills {
		if !fileExists(filepath.Join(root, ".agents", "skills", skill, "SKILL.md")) {
			t.Fatalf("skill %s was not installed", skill)
		}
	}
	if !directoryExists(filepath.Join(root, "artifacts")) {
		t.Fatal("artifacts directory was not created")
	}
}

// stubOpenSpecSetup replaces the OpenSpec backend setup for unit tests and
// records the options it received.
func stubOpenSpecSetup(t *testing.T, notes []string, err error) *[]openSpecSetup {
	t.Helper()
	original := setUpOpenSpec
	t.Cleanup(func() { setUpOpenSpec = original })
	calls := make([]openSpecSetup, 0)
	setUpOpenSpec = func(_ string, setup openSpecSetup) ([]string, error) {
		calls = append(calls, setup)
		return notes, err
	}
	return &calls
}

// @verifies scn.init.e841b29256e0.unit.2
// @verifies scn.init.754fd262e114.unit
func TestRunHandlesHelpVersionAndInitialization(t *testing.T) {
	for _, argument := range []string{"", "help", "--help", "-h"} {
		var stdout, stderr bytes.Buffer
		var args []string
		if argument != "" {
			args = []string{argument}
		}
		code := Run(args, &stdout, &stderr)
		if code != 0 || !strings.Contains(stdout.String(), "Usage:") {
			t.Fatalf("Run(%q) = %d, %q, %q", argument, code, stdout.String(), stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), "stele "+Version+" —") {
			t.Fatalf("help does not use Version: %q", stdout.String())
		}
	}
	for _, argument := range []string{"version", "--version", "-v"} {
		var stdout bytes.Buffer
		code := Run([]string{argument}, &stdout, io.Discard)
		if code != 0 || strings.TrimSpace(stdout.String()) != Version {
			t.Fatalf("Run(%q) = %d, %q", argument, code, stdout.String())
		}
	}

	stubOpenSpecSetup(t, nil, nil)
	bare := fixtureRoot(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--root", bare}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Initialized Stele") {
		t.Fatalf("init without a change = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	code = Run([]string{"verify", "--root", bare}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "no OpenSpec change selected") {
		t.Fatalf("bare verify after init = %d, %q, %q", code, stdout.String(), stderr.String())
	}

	root := fixtureRoot(t)
	stderr.Reset()
	code = Run([]string{"init", "--root", root, "--change", "example"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Initialized Stele") {
		t.Fatalf("first init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	code = Run([]string{"init", "--root", root, "--change", "example"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "already initialized") {
		t.Fatalf("second init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	code = Run(
		[]string{"init", "--root", filepath.Join(root, "missing"), "--change", "example"},
		io.Discard,
		&stderr,
	)
	if code != 2 {
		t.Fatalf("failing init exit = %d", code)
	}
}

// @verifies scn.init.c8813c799047.unit
func TestInitPrintsDefaultWorkflow(t *testing.T) {
	calls := stubOpenSpecSetup(t, []string{"Installed the stele workflow schema in openspec/schemas/stele."}, nil)
	var stdout bytes.Buffer
	root := fixtureRoot(t)
	if code := Run([]string{"init", "--root", root, "--tools", "claude,cursor", "--refresh-schema"},
		&stdout, io.Discard); code != 0 {
		t.Fatalf("init = %d", code)
	}
	if len(*calls) != 1 || (*calls)[0] != (openSpecSetup{tools: "claude,cursor", refreshSchema: true}) {
		t.Fatalf("setup options = %#v", *calls)
	}
	assertOrdered(t, stdout.String(),
		"Initialized Stele:",
		"Installed the stele workflow schema",
		"stele-propose, stele-apply, and stele-archive skills",
		"Using OpenSpec skills directly?",
		"stele ids, then stele verify --stage proposal",
		"stele validate --change <change>",
		"stele validate --specs",
		"openspec config profile",
	)

	stdout.Reset()
	if code := Run([]string{"init", "--root", root}, &stdout, io.Discard); code != 0 {
		t.Fatalf("second init = %d", code)
	}
	if (*calls)[1] != (openSpecSetup{tools: defaultOpenSpecTools}) ||
		!strings.Contains(stdout.String(), "already initialized") ||
		!strings.Contains(stdout.String(), "stele-propose") {
		t.Fatalf("second init = %#v, %q", (*calls)[1], stdout.String())
	}
}

func TestParseOptions(t *testing.T) {
	parsed, err := parseOptions("validate", []string{
		"--root", ".",
		"--change", "demo",
		"--stage", "proposal",
		"--report-file", "report.json",
		"--evidence-file", "evidence.json",
		"--json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.changeID != "demo" || parsed.stage != "proposal" ||
		parsed.reportPath != "report.json" || parsed.evidencePath != "evidence.json" ||
		!parsed.json || !filepath.IsAbs(parsed.root) || len(parsed.deprecated) != 0 {
		t.Fatalf("unexpected options: %#v", parsed)
	}
	parsed, err = parseOptions("test", []string{"scn.demo.aaaaaaaaaaaa", "--change", "demo", "req.demo.bbbbbbbbbbbb"})
	if err != nil || !slices.Equal(parsed.targets, []string{"scn.demo.aaaaaaaaaaaa", "req.demo.bbbbbbbbbbbb"}) ||
		parsed.changeID != "demo" {
		t.Fatalf("test targets = %#v, %v", parsed, err)
	}
	if value := (deprecatedPathFlag{}).String(); value != "" {
		t.Fatalf("empty deprecated flag = %q", value)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{"missing value", []string{"--root"}},
		{"unknown option", []string{"--wat"}},
		{"positional", []string{"extra"}},
		{"invalid stage", []string{"--stage", "acceptance"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseOptions("verify", test.args); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}

	originalGetwd, originalAbs := currentWorkingDirectory, absolutePath
	t.Cleanup(func() { currentWorkingDirectory, absolutePath = originalGetwd, originalAbs })
	currentWorkingDirectory = func() (string, error) { return "", errors.New("cwd failed") }
	if _, err := parseOptions("verify", nil); err == nil {
		t.Fatal("expected cwd error")
	}
	currentWorkingDirectory = originalGetwd
	absolutePath = func(string) (string, error) { return "", errors.New("absolute failed") }
	if _, err := parseOptions("verify", nil); err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestWithConfig(t *testing.T) {
	root := fixtureRoot(t)
	if _, err := withConfig(options{root: root}); err == nil {
		t.Fatal("expected missing change error")
	}
	writeFixture(t, root, "stele.config.json", `{"change":"configured"}`)
	parsed, err := withConfig(options{root: root})
	if err != nil || parsed.changeID != "configured" {
		t.Fatalf("configured = %#v, %v", parsed, err)
	}
	parsed, err = withConfig(options{root: root, changeID: "explicit"})
	if err != nil || parsed.changeID != "explicit" {
		t.Fatalf("explicit = %#v, %v", parsed, err)
	}
	writeFixture(t, root, "stele.config.json", "{")
	if _, err := withConfig(options{root: root}); err == nil {
		t.Fatal("expected invalid config error")
	}
	if err := os.Remove(filepath.Join(root, "stele.config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "stele.config.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := withConfig(options{root: root}); err == nil {
		t.Fatal("expected config read error")
	}
}

func TestVerificationAndTestCommands(t *testing.T) {
	originalVerify, originalScenarios := verifyProject, runProjectScenarios
	t.Cleanup(func() { verifyProject, runProjectScenarios = originalVerify, originalScenarios })
	passReport := Report{Mode: "implementation", Verdict: "pass", Verdicts: ReportVerdicts{"pass", "passed", "pass"}}
	passReport.Summary.Requirements, passReport.Summary.Scenarios = 1, 1
	failReport := passReport
	failReport.Verdict = "fail"
	failReport.Verdicts = ReportVerdicts{"fail", "failed", "fail"}
	failReport.Requirements = []RequirementReport{{Scenarios: []ScenarioReport{
		{ID: "scn.demo.aaaaaaaaaaaa", Execution: ExecutionState{State: "executed", Outcome: "failed"}},
		{ID: "scn.demo.bbbbbbbbbbbb", Execution: ExecutionState{State: "executed", Outcome: "passed"}},
	}}}
	failReport.Summary.Errors = 1
	failReport.Diagnostics = []Diagnostic{{Code: "FAIL", Severity: "error", Message: "failed"}}

	for _, test := range []struct {
		name   string
		report Report
		json   bool
		code   int
	}{
		{"human pass", passReport, false, 0},
		{"human fail", failReport, false, 1},
		{"json pass", passReport, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			verifyProject = func(verifyRequest) (Report, error) { return test.report, nil }
			var stdout bytes.Buffer
			code := verifyCommand(options{json: test.json}, &stdout, io.Discard)
			if code != test.code || stdout.Len() == 0 {
				t.Fatalf("verifyCommand = %d, %q", code, stdout.String())
			}
		})
	}
	verifyProject = func(verifyRequest) (Report, error) {
		return Report{}, errors.New("verify failed")
	}
	if code := verifyCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("verify error exit = %d", code)
	}

	passEvidence := Evidence{Outcome: "passed", Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "passed"}}}
	failEvidence := Evidence{
		Outcome: "failed",
		Scenarios: []ScenarioOutcome{
			{ID: "a", Outcome: "failed"},
			{ID: "b", Outcome: "passed"},
		},
	}
	for _, test := range []struct {
		name     string
		evidence Evidence
		json     bool
		code     int
	}{
		{"human pass", passEvidence, false, 0},
		{"human fail", failEvidence, false, 1},
		{"json pass", passEvidence, true, 0},
	} {
		t.Run("test "+test.name, func(t *testing.T) {
			runProjectScenarios = func(testRequest) (testRun, error) {
				return testRun{evidence: test.evidence}, nil
			}
			var stdout bytes.Buffer
			code := testCommand(options{json: test.json}, &stdout, io.Discard)
			if code != test.code || stdout.Len() == 0 {
				t.Fatalf("testCommand = %d, %q", code, stdout.String())
			}
		})
	}
	runProjectScenarios = func(testRequest) (testRun, error) {
		return testRun{}, errors.New("test failed")
	}
	if code := testCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("test error exit = %d", code)
	}
}

// @verifies scn.validate.d9553f1a4c1c.unit
func TestValidateCommand(t *testing.T) {
	originalVerify, originalScenarios, originalOpenSpec := verifyProject, runProjectScenarios, validateProjectOpenSpec
	t.Cleanup(func() {
		verifyProject = originalVerify
		runProjectScenarios = originalScenarios
		validateProjectOpenSpec = originalOpenSpec
	})
	passReport := Report{Verdict: "pass", Verdicts: ReportVerdicts{"pass", "passed", "pass"}}
	passReport.Summary.Requirements, passReport.Summary.Scenarios = 1, 1
	passEvidence := Evidence{Outcome: "passed", Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "passed"}}}
	runProjectScenarios = func(testRequest) (testRun, error) { return testRun{evidence: passEvidence}, nil }
	verifyProject = func(verifyRequest) (Report, error) { return passReport, nil }
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	originalCheck := checkScopeSpecs
	t.Cleanup(func() { checkScopeSpecs = originalCheck })
	checkScopeSpecs = func(string, verificationScope) error { return nil }
	for _, jsonOutput := range []bool{false, true} {
		var stdout bytes.Buffer
		if code := validateCommand(options{json: jsonOutput}, &stdout, io.Discard); code != 0 || stdout.Len() == 0 {
			t.Fatalf("validate pass = %d, %q", code, stdout.String())
		}
	}
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) {
		return false, errors.New("openspec failed")
	}
	var failure bytes.Buffer
	code := validateCommand(
		options{evidencePath: "e.json", reportPath: "r.json"},
		&failure,
		io.Discard,
	)
	if code != 1 || !strings.Contains(failure.String(), "✗ OpenSpec strict validation  failed") ||
		!strings.Contains(failure.String(), "✗ FAILED  change  — OpenSpec strict validation") {
		t.Fatalf("validate OpenSpec failure = %d, %q", code, failure.String())
	}
	runProjectScenarios = func(testRequest) (testRun, error) {
		return testRun{}, errors.New("scenario failed")
	}
	if code := validateCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("validate scenario error = %d", code)
	}
	runProjectScenarios = func(testRequest) (testRun, error) { return testRun{evidence: passEvidence}, nil }
	verifyProject = func(verifyRequest) (Report, error) {
		return Report{}, errors.New("verify failed")
	}
	if code := validateCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("validate verify error = %d", code)
	}
}

func TestRunOpenSpec(t *testing.T) {
	root := fixtureRoot(t)
	originalExecutable := executablePath
	t.Cleanup(func() { executablePath = originalExecutable })
	executablePath = func() (string, error) { return "", errors.New("executable failed") }
	if _, err := runOpenSpec(root, changeScope("example")); err == nil {
		t.Fatal("expected executable error")
	}
	executablePath = originalExecutable
	if _, err := runOpenSpec(root, changeScope("example")); err == nil {
		t.Fatal("expected missing OpenSpec error")
	}
	writeFixture(t, root, "node_modules/@fission-ai/openspec/bin/openspec.js", "placeholder")
	bin := filepath.Join(root, "fake-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	node := filepath.Join(bin, "node")
	writeFixture(t, bin, "node", "#!/bin/sh\nprintf 'validated'\nexit \"${STELE_FAKE_EXIT:-0}\"\n")
	if err := os.Chmod(node, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	passed, err := runOpenSpec(root, changeScope("example"))
	if err != nil || !passed {
		t.Fatalf("runOpenSpec success = %v, %v", passed, err)
	}
	t.Setenv("STELE_FAKE_EXIT", "1")
	if _, err := runOpenSpec(root, changeScope("example")); err == nil || !strings.Contains(err.Error(), "validated") {
		t.Fatalf("expected validation output, got %v", err)
	}
}

func TestRunRoutesCommandsAndConfigurationErrors(t *testing.T) {
	originalVerify, originalScenarios, originalOpenSpec := verifyProject, runProjectScenarios, validateProjectOpenSpec
	t.Cleanup(func() {
		verifyProject = originalVerify
		runProjectScenarios = originalScenarios
		validateProjectOpenSpec = originalOpenSpec
	})
	root := completeFixture(t, false)
	verifyProject = func(verifyRequest) (Report, error) {
		return Report{Verdict: "pass", Verdicts: ReportVerdicts{"pass", "passed", "pass"}}, nil
	}
	runProjectScenarios = func(testRequest) (testRun, error) {
		return testRun{evidence: Evidence{Outcome: "passed"}}, nil
	}
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	for _, command := range []string{"verify", "test", "validate"} {
		code := Run(
			[]string{command, "--root", root, "--change", "example", "--json"},
			io.Discard,
			io.Discard,
		)
		if code != 0 {
			t.Fatalf("Run(%s) = %d", command, code)
		}
	}
	if code := Run([]string{"verify", "--stage", "bad"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("parse error exit = %d", code)
	}
	writeFixture(t, root, "stele.config.json", "{")
	if code := Run([]string{"verify", "--root", root}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("config error exit = %d", code)
	}
}

func TestJSONHelpers(t *testing.T) {
	if choose(true, "yes", "no") != "yes" || choose(false, "yes", "no") != "no" {
		t.Fatal("choose returned an unexpected value")
	}
	if got := countPassed(Evidence{Scenarios: []ScenarioOutcome{{Outcome: "passed"}, {Outcome: "failed"}}}); got != 1 {
		t.Fatalf("countPassed = %d", got)
	}
	original := marshalJSON
	t.Cleanup(func() { marshalJSON = original })
	marshalJSON = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal failed") }
	writeMachineJSON(io.Discard, make(chan int))
}

// @verifies scn.linkindex.5c91d0c168f0.unit
func TestTestRejectsUnknownTargets(t *testing.T) {
	root := behaviorFixture(t)
	writeFixture(t, root, "openspec/changes/other/specs/other/spec.md", "### Requirement: Other\n")
	ran := recordTests(t)
	for _, target := range []string{
		"req.demo.999999999999",
		"scn.demo.999999999999",
		"scn.demo.bbbbbbbbbbbb.integration",
		"openspec/changes/example/specs/missing/spec.md",
		"README.md",
		"openspec/changes/other/specs/other/spec.md",
	} {
		code, stdout, stderr := runCommand(t, "test", "scn.demo.bbbbbbbbbbbb", target,
			"--root", root, "--change", "example")
		if code != 2 || !strings.Contains(stderr, "unknown test target "+target) || stdout != "" {
			t.Fatalf("test %s = %d, %q, %q", target, code, stdout, stderr)
		}
	}
	if len(*ran) != 0 {
		t.Fatalf("tests ran for an invalid target: %v", *ran)
	}
	if fileExists(filepath.Join(root, defaultEvidencePath)) {
		t.Fatal("an invalid target wrote evidence")
	}

	code, stdout, _ := runCommand(t, "test", "scn.demo.bbbbbbbbbbbb.unit", "--root", root, "--change", "example")
	if code != 0 || !strings.Contains(stdout, "✓ Test execution              1/1 passed") ||
		!fileExists(filepath.Join(root, defaultEvidencePath)) {
		t.Fatalf("a valid target = %d, %q", code, stdout)
	}
	recordTests(t, "b e2e")
	code, stdout, _ = runCommand(t, "test", "scn.demo.bbbbbbbbbbbb", "--root", root, "--change", "example")
	if code != 1 || !strings.Contains(stdout, "✗ tests/e2e/b.test.mts  b e2e") ||
		!strings.Contains(stdout, "test-process-failed") ||
		!strings.Contains(stdout, "✗ Test execution              1/2 passed · 1 failed") {
		t.Fatalf("a failing target = %d, %q", code, stdout)
	}
	if selectedPassed(nil) {
		t.Fatal("a target without tests passed")
	}
}

// allScopesFixture returns current specifications and change "example" that
// pass, and change "broken" whose requirement has no anchors.
func allScopesFixture(t *testing.T) string {
	t.Helper()
	root := completeFixture(t, false)
	writeFixture(t, root, "openspec/specs/demo/spec.md", `### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(t, root, "openspec/changes/broken/specs/broken/spec.md", `### Requirement: Broken
Verification-ID: req.broken.111111111111
#### Scenario: Broken
Verification-ID: scn.broken.222222222222
`)
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	withoutOpenSpecOnPath(t)
	original := validateProjectOpenSpec
	t.Cleanup(func() { validateProjectOpenSpec = original })
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	return root
}

// @verifies scn.linkindex.7c1d78728036.unit
func TestAllScopesReportSeparately(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, stdout, stderr := runCommand(t, "verify", "--all", "--root", root, "--report-file", "out/reports.json")
	if code != 1 {
		t.Fatalf("verify --all = %d, %q, %q", code, stdout, stderr)
	}
	assertOrdered(t, stdout,
		"== current specifications ==", "✓ PASSED  current specifications",
		"== change broken ==", "✗ Linkage (anchors)", "LINK_CODE_MISSING", "✗ FAILED  change broken",
		"== change example ==", "✓ PASSED  change example",
		"Scopes", "✓ PASSED  current specifications", "✗ FAILED  change broken", "✓ PASSED  change example",
		"✗ FAILED  1 of 3 scopes failed: change broken",
	)
	var reports []Report
	if !readJSON(filepath.Join(root, "out", "reports.json"), &reports) || len(reports) != 3 ||
		reports[1].OpenSpec.ChangeID != "broken" {
		t.Fatalf("the report file does not hold every scope: %#v", reports)
	}

	code, stdout, _ = runCommand(t, "verify", "--all", "--root", root, "--json")
	var results []scopeResult
	if err := json.Unmarshal([]byte(stdout), &results); err != nil || code != 1 || len(results) != 3 {
		t.Fatalf("verify --all --json = %d, %v, %q", code, err, stdout)
	}
	for index, want := range []struct {
		scope, kind string
		code        int
	}{{"specs", "specs", 0}, {"broken", "change", 1}, {"example", "change", 0}} {
		got := results[index]
		if got.Scope != want.scope || got.Kind != want.kind || got.ExitCode != want.code {
			t.Fatalf("result %d = %#v", index, results[index])
		}
	}

	code, stdout, _ = runCommand(t, "validate", "--all", "--root", root)
	if code != 1 || !strings.Contains(stdout, "✗ FAILED  1 of 3 scopes failed: change broken") {
		t.Fatalf("validate --all = %d, %q", code, stdout)
	}
	var evidence Evidence
	if !readJSON(filepath.Join(root, defaultEvidencePath), &evidence) ||
		!slices.Equal([]string{evidence.Scenarios[0].ID, evidence.Scenarios[1].ID},
			[]string{"scn.broken.222222222222", "scn.demo.bbbbbbbbbbbb"}) {
		t.Fatalf("the evidence file does not merge every scope: %#v", evidence)
	}
	if !readJSON(filepath.Join(root, "artifacts", "verification-report.json"), &reports) || len(reports) != 3 {
		t.Fatalf("the default report file does not hold every scope: %#v", reports)
	}

	if err := os.RemoveAll(filepath.Join(root, "openspec", "changes", "broken")); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runCommand(t, "test", "--all", "--root", root, "--evidence-file", "out/evidence.json")
	if code != 0 ||
		!strings.Contains(stdout, "✓ PASSED  all 2 scopes passed") {
		t.Fatalf("test --all = %d, %q", code, stdout)
	}
	if err := os.RemoveAll(filepath.Join(root, "openspec", "specs")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/changes/draft/proposal.md", "## Why\n")
	code, stdout, stderr = runCommand(t, "verify", "--all", "--root", root)
	if code != 2 || strings.Contains(stdout, "current specifications") ||
		!strings.Contains(stderr, "change draft has no delta specs") {
		t.Fatalf("verify --all with an unverifiable change = %d, %q, %q", code, stdout, stderr)
	}
	if code, _, stderr = runCommand(t, "verify", "--all", "--root", fixtureRoot(t)); code != 2 ||
		!strings.Contains(stderr, "no current specifications and no active changes") {
		t.Fatalf("verify --all without scopes = %d, %q", code, stderr)
	}
	if err := os.RemoveAll(filepath.Join(root, "openspec", "changes", "draft")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "blocked", "blocking file")
	code, _, _ = runCommand(t, "verify", "--all", "--root", root, "--report-file", "blocked/reports.json")
	if code != 2 {
		t.Fatalf("verify --all with an unwritable report = %d", code)
	}
}

// @verifies scn.linkindex.4d6ba51a47bc.unit
func TestAllRejectsConflictingScopes(t *testing.T) {
	root := allScopesFixture(t)
	ran := recordTests(t)
	for _, arguments := range [][]string{
		{"verify", "--all", "--change", "example"},
		{"validate", "--all", "--specs"},
		{"test", "--all", "scn.demo.bbbbbbbbbbbb"},
		{"index", "--all", "--change", "example"},
	} {
		code, stdout, stderr := runCommand(t, append(arguments, "--root", root)...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "--all cannot be combined") {
			t.Fatalf("%v = %d, %q, %q", arguments, code, stdout, stderr)
		}
	}
	if len(*ran) != 0 || fileExists(filepath.Join(root, "artifacts", "verification-report.json")) {
		t.Fatalf("a conflicting --all checked something: %v", *ran)
	}
}

// @verifies scn.verify.06f2be2af1e7.unit
func TestOutputFileFlags(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, _, stderr := runCommand(t, "validate", "--root", root, "--change", "example",
		"--evidence-file", "out/evidence.json", "--report-file", "out/report.json")
	if code != 0 || strings.Contains(stderr, "deprecated") {
		t.Fatalf("validate = %d, %q", code, stderr)
	}
	var evidence Evidence
	var report Report
	if !readJSON(filepath.Join(root, "out", "evidence.json"), &evidence) || evidence.Outcome != "passed" ||
		!readJSON(filepath.Join(root, "out", "report.json"), &report) || report.Verdicts.Overall != "pass" {
		t.Fatalf("output files = %#v, %#v", evidence, report)
	}
	if fileExists(filepath.Join(root, defaultEvidencePath)) {
		t.Fatal("validate wrote the default evidence file")
	}
	code, _, _ = runCommand(t, "verify", "--root", root, "--change", "example", "--report-file", "out/verify.json")
	if code != 0 || !fileExists(filepath.Join(root, "out", "verify.json")) {
		t.Fatalf("verify --report-file = %d", code)
	}
	code, _, _ = runCommand(t, "test", "--root", root, "--change", "example", "--evidence-file", "out/test.json")
	if code != 0 || !fileExists(filepath.Join(root, "out", "test.json")) {
		t.Fatalf("test --evidence-file = %d", code)
	}
}

// @verifies scn.verify.70e950bf161c.unit
func TestDeprecatedOutputFlagsWarn(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, _, stderr := runCommand(t, "verify", "--root", root, "--change", "example", "--report", "out/report.json")
	if code != 0 || !fileExists(filepath.Join(root, "out", "report.json")) ||
		stderr != "stele: warning: --report is deprecated and will be removed in 0.2.0; use --report-file\n" {
		t.Fatalf("verify --report = %d, %q", code, stderr)
	}
	code, _, stderr = runCommand(t, "verify", "--root", root, "--change", "broken", "--report", "out/broken.json")
	if code != 1 || !fileExists(filepath.Join(root, "out", "broken.json")) ||
		!strings.Contains(stderr, "--report is deprecated") {
		t.Fatalf("failing verify --report = %d, %q", code, stderr)
	}
	code, _, stderr = runCommand(t, "test", "--root", root, "--change", "example", "--evidence", "out/evidence.json")
	if code != 0 || !fileExists(filepath.Join(root, "out", "evidence.json")) ||
		!strings.Contains(stderr, "--evidence is deprecated and will be removed in 0.2.0; use --evidence-file") {
		t.Fatalf("test --evidence = %d, %q", code, stderr)
	}
	if code, _, _ := runCommand(t, "ids", "--root", root, "--change", "example", "--report", "x.json"); code != 2 {
		t.Fatalf("ids accepted --report: %d", code)
	}
}

// unannotatedFixture returns a project whose current specification and whose
// change "example" pass linkage but have no annotation.
func unannotatedFixture(t *testing.T, config string) string {
	t.Helper()
	root := completeFixture(t, false)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(t, root, "openspec/specs/demo/spec.md", `### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	if config != "" {
		writeFixture(t, root, "stele.config.json", config)
	}
	return root
}

// @verifies scn.specannotation.0f1fab2a02b8.unit
func TestUnannotatedSpecificationWarnsByDefault(t *testing.T) {
	root := unannotatedFixture(t, "")
	originalScenarios, originalOpenSpec := runProjectScenarios, validateProjectOpenSpec
	t.Cleanup(func() { runProjectScenarios, validateProjectOpenSpec = originalScenarios, originalOpenSpec })
	runProjectScenarios = func(testRequest) (testRun, error) {
		return testRun{evidence: Evidence{Outcome: "passed", Scenarios: []ScenarioOutcome{
			{ID: "scn.demo.bbbbbbbbbbbb", Outcome: "passed"},
		}}}, nil
	}
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }

	code, stdout, stderr := runCommand(t, "validate", "--root", root, "--specs")
	if code != 0 || !strings.Contains(stdout, "✓ PASSED  current specifications") {
		t.Fatalf("validate --specs = %d, %q, %q", code, stdout, stderr)
	}
	var report Report
	if !readJSON(filepath.Join(root, "artifacts", "verification-report.json"), &report) {
		t.Fatal("validate wrote no report")
	}
	warnings := diagnosticsWithCode(report.Diagnostics, "SPEC_ANNOTATION_MISSING")
	if len(warnings) != 1 || warnings[0].Severity != "warning" ||
		!strings.Contains(warnings[0].Message, "openspec/specs/demo/spec.md") ||
		!strings.Contains(warnings[0].Message, "stele annotate --specs") {
		t.Fatalf("missing annotation warning = %#v", report.Diagnostics)
	}
	if report.Summary.Requirements != 1 || report.Summary.LinkedRequirements != 1 ||
		report.Requirements[0].Linkage != "linked" {
		t.Fatalf("the unannotated requirement was not verified: %#v", report.Requirements)
	}
}

// @verifies scn.specannotation.3c8b5da99a7e.unit
func TestUnannotatedSpecificationFailsUnderTheErrorPolicy(t *testing.T) {
	root := unannotatedFixture(t, `{"change":"example","unannotatedSpecs":"error"}`)
	code, stdout, stderr := runCommand(t, "verify", "--root", root, "--change", "example", "--json")
	var report Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("verify output: %v, %q", err, stderr)
	}
	errors := diagnosticsWithCode(report.Diagnostics, "SPEC_ANNOTATION_MISSING")
	if code != 1 || len(errors) != 1 || errors[0].Severity != "error" ||
		!strings.Contains(errors[0].Message, "openspec/changes/example/specs/demo/spec.md") ||
		!strings.Contains(errors[0].Message, "stele annotate --change example") {
		t.Fatalf("verify under the error policy = %d, %#v", code, report.Diagnostics)
	}

	writeFixture(t, root, "stele.config.json", `{"change":"example","unannotatedSpecs":"warn"}`)
	if code, stdout, _ := runCommand(t, "verify", "--root", root); code != 0 {
		t.Fatalf("verify under the warn policy = %d, %q", code, stdout)
	}
}

// @verifies scn.specannotation.ee994e1a42b9.unit
func TestUnknownAnnotationPolicyIsRejected(t *testing.T) {
	root := unannotatedFixture(t, `{"change":"example","unannotatedSpecs":"ignore"}`)
	originalVerify := verifyProject
	t.Cleanup(func() { verifyProject = originalVerify })
	verified := false
	verifyProject = func(request verifyRequest) (Report, error) {
		verified = true
		return originalVerify(request)
	}
	for _, command := range []string{"verify", "ids", "annotate", "index", "init"} {
		code, stdout, stderr := runCommand(t, command, "--root", root)
		if code != 2 || !strings.Contains(stderr, `unannotatedSpecs "ignore"`) ||
			!strings.Contains(stderr, "warn, error") {
			t.Fatalf("%s with an unknown policy = %d, %q, %q", command, code, stdout, stderr)
		}
	}
	if verified {
		t.Fatal("verify ran with an unknown policy")
	}
}

// @verifies scn.specannotation.a518efc1ddd8.unit
func TestAnnotateCheckWritesNothing(t *testing.T) {
	root := annotateFixture(t)
	before := readTestFile(t, root, "openspec/specs/plain/spec.md")
	code, stdout, stderr := runAnnotate(t, "--root", root, "--specs", "--check", "--json")
	var result AnnotationResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("check output: %v, %q, %q", err, stdout, stderr)
	}
	if code != 1 || result.Mode != "check" || result.Verdict != "fail" || len(result.Files) != 2 {
		t.Fatalf("check = %d, %#v", code, result)
	}
	marked, plain := result.Files[0], result.Files[1]
	if marked.Path != "openspec/specs/marked/spec.md" || marked.State != annotationAnnotated ||
		marked.Scope != "specs" || *marked.Version != "v1" ||
		plain.Path != "openspec/specs/plain/spec.md" || plain.State != annotationMissing ||
		plain.Version != nil || plain.Changed {
		t.Fatalf("check files = %#v", result.Files)
	}
	if readTestFile(t, root, "openspec/specs/plain/spec.md") != before {
		t.Fatal("check wrote a file")
	}
	if code, stdout, _ := runAnnotate(t, "--root", root, "--specs", "--check"); code != 1 ||
		!strings.Contains(stdout, "MISSING openspec/specs/plain/spec.md") {
		t.Fatalf("human check = %d, %q", code, stdout)
	}

	if code, _, _ := runAnnotate(t, "--root", root, "--specs"); code != 0 {
		t.Fatalf("annotate = %d", code)
	}
	if code, stdout, _ := runAnnotate(t, "--root", root, "--specs", "--check", "--json"); code != 0 ||
		!strings.Contains(stdout, `"verdict": "pass"`) {
		t.Fatalf("check after annotating = %d, %q", code, stdout)
	}
}

// stubValidation replaces the stages of validate with passing stubs that
// return the given report.
func stubValidation(t *testing.T, report Report) {
	t.Helper()
	originalVerify, originalScenarios := verifyProject, runProjectScenarios
	originalOpenSpec, originalCheck := validateProjectOpenSpec, checkScopeSpecs
	t.Cleanup(func() {
		verifyProject, runProjectScenarios = originalVerify, originalScenarios
		validateProjectOpenSpec, checkScopeSpecs = originalOpenSpec, originalCheck
	})
	passEvidence := Evidence{
		Outcome: "passed", InputDigest: "now",
		Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "passed"}},
	}
	runProjectScenarios = func(testRequest) (testRun, error) { return testRun{evidence: passEvidence}, nil }
	verifyProject = func(verifyRequest) (Report, error) { return report, nil }
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	checkScopeSpecs = func(string, verificationScope) error { return nil }
}

// @verifies scn.terminalreport.a76da19c3313.unit
func TestValidateShowsWarnings(t *testing.T) {
	report := reportFixture()
	report.Diagnostics = []Diagnostic{
		deprecatedPlanDiagnostic("openspec/changes/archive/2026-01-01-a/linkage-plan.json"),
	}
	stubValidation(t, report)
	var stdout bytes.Buffer
	code := validateCommand(options{specs: true}, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("warnings changed the exit code: %d\n%s", code, stdout.String())
	}
	assertOrdered(t, stdout.String(),
		"! Plan approval",
		"Warnings",
		"PLAN_V1_DEPRECATED ×1",
		"→ Fix: run `stele plan migrate --specs`",
		"openspec/changes/archive/2026-01-01-a/linkage-plan.json:1",
		"✓ PASSED  current specifications · 1 warning",
	)
}

// @verifies scn.terminalreport.9f3cb9c00f42.unit
func TestJSONOutputStaysCleanWithProgress(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, stdout, stderr := runCommand(t, "validate", "--root", root, "--change", "example", "--json")
	var result validationResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || code != 0 || result.Verdict != "pass" {
		t.Fatalf("validate --json = %d, %v, %q", code, err, stdout)
	}
	encoded, _ := MarshalDeterministic(result)
	if string(encoded) != stdout {
		t.Fatalf("standard output is not exactly the JSON document: %q", stdout)
	}
	if !strings.Contains(stderr, "test execution: 1 tests in 1 files") || strings.Contains(stdout, "test execution") {
		t.Fatalf("progress is not on standard error only: %q", stderr)
	}
	for _, flags := range [][]string{{"--details"}, {"--color=always"}, {"--quiet"}} {
		_, again, quietErrors := runCommand(t, append([]string{
			"validate", "--root", root, "--change", "example",
			"--json",
		}, flags...)...)
		if again != stdout {
			t.Fatalf("%v changed the JSON output: %q", flags, again)
		}
		if flags[0] == "--quiet" && quietErrors != "" {
			t.Fatalf("--quiet printed progress: %q", quietErrors)
		}
	}
}

// @verifies scn.terminalreport.6678ddf5a27f.unit
func TestQuietPrintsOnlyTheVerdict(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, stdout, stderr := runCommand(t, "validate", "--root", root, "--change", "broken", "--quiet")
	if code != 1 || stderr != "" || strings.Count(stdout, "\n") != 1 ||
		!strings.HasPrefix(stdout, "✗ FAILED  change broken — linkage (anchors) (") {
		t.Fatalf("validate --quiet = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, _ = runCommand(t, "validate", "--root", root, "--all", "--quiet")
	if code != 1 || stdout != "✗ FAILED  1 of 3 scopes failed: change broken\n" {
		t.Fatalf("validate --all --quiet = %d, %q", code, stdout)
	}
}

// @verifies scn.terminalreport.14b3f3d729fb.unit
func TestUnknownColorModeIsRejected(t *testing.T) {
	root := allScopesFixture(t)
	ran := recordTests(t)
	code, stdout, stderr := runCommand(t, "validate", "--root", root, "--change", "example", "--color=sometimes")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "accepted values: auto, always, never") ||
		len(*ran) != 0 {
		t.Fatalf("validate --color=sometimes = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, _ = runCommand(t, "validate", "--root", root, "--change", "example", "--color=always")
	if code != 0 || !strings.Contains(stdout, "\x1b[32m✓\x1b[0m") {
		t.Fatalf("validate --color=always = %d, %q", code, stdout)
	}
}

// @verifies scn.terminalreport.50fd0b8c9244.unit
func TestUnknownAnnotationModeIsRejected(t *testing.T) {
	root := allScopesFixture(t)
	ran := recordTests(t)
	code, stdout, stderr := runCommand(t, "validate", "--root", root, "--change", "example", "--annotations=gitlab")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "accepted values: auto, github, never") ||
		len(*ran) != 0 {
		t.Fatalf("validate --annotations=gitlab = %d, %q, %q", code, stdout, stderr)
	}
}

// danglingAnchorFixture adds an anchor at tests/todo.test.mts:14 for an
// identity that no specification declares.
func danglingAnchorFixture(t *testing.T) string {
	t.Helper()
	root := allScopesFixture(t)
	writeFixture(t, root, "tests/todo.test.mts", strings.Repeat("\n", 13)+
		"// @verifies "+"scn.todo.abcdef012345\n")
	return root
}

// @verifies scn.terminalreport.d9386a6ce84d.unit
func TestAnnotationsLeaveOutputAndExitCodeUnchanged(t *testing.T) {
	root := danglingAnchorFixture(t)
	recordTests(t)
	arguments := []string{"validate", "--change", "example", "--root", root}
	stubEnvironment(t, nil)
	plainCode, plainStdout, plainStderr := runCommand(t, arguments...)
	stubEnvironment(t, map[string]string{"GITHUB_ACTIONS": "true"})
	code, stdout, stderr := runCommand(t, arguments...)
	if code != 1 || code != plainCode || stdout != plainStdout || strings.Contains(plainStderr, "::") {
		t.Fatalf("annotations changed the run: %d/%d\n%s\n---\n%s", plainCode, code, plainStdout, stdout)
	}
	if !strings.Contains(stderr, "\n::error file=tests/todo.test.mts,line=14,title=ANCHOR_DANGLING::") ||
		!strings.HasPrefix(stderr, plainStderr) {
		t.Fatalf("annotations on standard error:\n%s", stderr)
	}
	for _, command := range []string{"verify", "test"} {
		_, jsonStdout, jsonStderr := runCommand(t, command, "--change", "example", "--root", root, "--json")
		if !json.Valid([]byte(jsonStdout)) || strings.Contains(jsonStdout, "::") ||
			(command == "verify" && !strings.Contains(jsonStderr, "title=ANCHOR_DANGLING::")) {
			t.Fatalf("%s --json = %q, %q", command, jsonStdout, jsonStderr)
		}
	}
	_, quietStdout, quietStderr := runCommand(t, append(arguments, "--quiet")...)
	if strings.Contains(quietStdout, "::") || !strings.HasPrefix(quietStderr, "::error file=tests/todo.test.mts") {
		t.Fatalf("--quiet = %q, %q", quietStdout, quietStderr)
	}
}

// @verifies scn.terminalreport.e045d1b2d55c.unit
func TestAnnotationsOnlyInGitHubActionsUnlessAsked(t *testing.T) {
	root := danglingAnchorFixture(t)
	recordTests(t)
	arguments := []string{"validate", "--change", "example", "--root", root}
	annotated := func(environment map[string]string, extra ...string) bool {
		stubEnvironment(t, environment)
		code, _, stderr := runCommand(t, append(append([]string{}, arguments...), extra...)...)
		if code != 1 {
			t.Fatalf("validate %v = %d", extra, code)
		}
		return strings.Contains("\n"+stderr, "\n::")
	}
	if annotated(nil) || !annotated(nil, "--annotations=github") ||
		annotated(map[string]string{"GITHUB_ACTIONS": "true"}, "--annotations=never") ||
		annotated(map[string]string{"GITHUB_ACTIONS": "false"}) {
		t.Fatal("annotations do not follow --annotations and GITHUB_ACTIONS")
	}
}

// @verifies scn.terminalreport.42e58de06dfa.unit
func TestAllScopesSummary(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, stdout, stderr := runCommand(t, "validate", "--all", "--root", root)
	if code != 1 {
		t.Fatalf("validate --all = %d, %q", code, stdout)
	}
	assertOrdered(t, stdout,
		"== current specifications ==", "stele "+Version+" · validate · current specifications",
		"== change broken ==", "stele "+Version+" · validate · change broken",
		"== change example ==", "stele "+Version+" · validate · change example",
		"Scopes\n",
		"  ✓ PASSED  current specifications · 1 warning\n",
		"  ✗ FAILED  change broken — linkage (anchors) (1 unimplemented requirement, 1 untested scenario) · 2 warnings",
		"  ✓ PASSED  change example · 1 warning\n",
		"✗ FAILED  1 of 3 scopes failed: change broken\n",
	)
	assertOrdered(t, stderr, "[current specifications] OpenSpec strict validation: passed",
		"[change broken] OpenSpec strict validation: passed", "[change example] test execution:")

	writeFixture(t, root, "openspec/changes/draft/proposal.md", "## Why\n")
	_, stdout, _ = runCommand(t, "validate", "--all", "--root", root)
	if !strings.Contains(stdout, "  ✗ FAILED  change draft — could not be checked\n") {
		t.Fatalf("an unverifiable scope is missing from the summary:\n%s", stdout)
	}
}

// @verifies scn.terminalreport.026c6192674a.unit
func TestVersionIsUsedEverywhere(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "9.9.9-test"
	if _, stdout, _ := runCommand(t, "--version"); stdout != "9.9.9-test\n" {
		t.Fatalf("--version = %q", stdout)
	}
	if _, stdout, _ := runCommand(t, "--help"); !strings.HasPrefix(stdout, "stele 9.9.9-test —") {
		t.Fatalf("help = %q", stdout)
	}
	root := allScopesFixture(t)
	_, stdout, _ := runCommand(t, "verify", "--root", root, "--change", "example", "--report-file", "out/r.json")
	var report Report
	if !strings.HasPrefix(stdout, "stele 9.9.9-test · verify · change example\n") ||
		!readJSON(filepath.Join(root, "out", "r.json"), &report) || report.Verifier.Version != "9.9.9-test" {
		t.Fatalf("report header or verifier version = %q, %#v", stdout, report.Verifier)
	}
}

func TestStoredAndStaleExecutions(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, defaultEvidencePath, `{"inputDigest":"stored","executions":[
{"path":"a.test.mts","scenarioIds":["scn.demo.bbbbbbbbbbbb"],"outcome":"passed"},
{"path":"b.test.mts","scenarioIds":["scn.other.cccccccccccc"],"outcome":"passed","inputDigest":"x"}]}`)
	report := Report{Requirements: []RequirementReport{{Scenarios: []ScenarioReport{{ID: "scn.demo.bbbbbbbbbbbb"}}}}}
	executions := storedExecutions(root, report)
	if len(executions) != 1 || executions[0].InputDigest != "stored" {
		t.Fatalf("stored executions = %#v", executions)
	}
	if storedExecutions(fixtureRoot(t), report) != nil {
		t.Fatal("a missing evidence file has executions")
	}
	run := testRun{evidence: Evidence{InputDigest: "now", Executions: []TestExecution{
		{Path: "a", InputDigest: "now"}, {Path: "b", InputDigest: "old"},
	}}}
	if stale := staleExecutions(run); len(stale) != 1 || stale[0].Path != "b" {
		t.Fatalf("stale executions = %#v", stale)
	}
	if err := os.MkdirAll(filepath.Join(root, "openspec", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing.md"), filepath.Join(root, "openspec", "specs", "demo",
		"spec.md")); err != nil {
		t.Fatal(err)
	}
	if summary, step := checkAnnotations(root, []verificationScope{{currentSpecs: true}}); summary.code != 2 ||
		step.Error == "" {
		t.Fatalf("an unreadable specification = %#v", summary)
	}
}
