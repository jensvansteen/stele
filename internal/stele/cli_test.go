package stele

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
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

func TestInitializeWritesConfigAndSkills(t *testing.T) {
	root := fixtureRoot(t)
	created, err := Initialize(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("expected config plus two skills, got %#v", created)
	}
	if !fileExists(root+"/.agents/skills/stele-plan/SKILL.md") || !fileExists(root+"/.agents/skills/stele-verify/SKILL.md") {
		t.Fatalf("skills were not installed")
	}
}

func TestRunHandlesHelpVersionAndInitialization(t *testing.T) {
	for _, argument := range []string{"", "help", "--help", "-h"} {
		var stdout, stderr bytes.Buffer
		var args []string
		if argument != "" {
			args = []string{argument}
		}
		if code := Run(args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "Usage:") {
			t.Fatalf("Run(%q) = %d, %q, %q", argument, code, stdout.String(), stderr.String())
		}
	}
	for _, argument := range []string{"version", "--version", "-v"} {
		var stdout bytes.Buffer
		if code := Run([]string{argument}, &stdout, io.Discard); code != 0 || strings.TrimSpace(stdout.String()) != Version {
			t.Fatalf("Run(%q) = %d, %q", argument, code, stdout.String())
		}
	}

	root := fixtureRoot(t)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", "--root", root}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--change is required") {
		t.Fatalf("missing-change init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	if code := Run([]string{"init", "--root", root, "--change", "example"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "Initialized Stele") {
		t.Fatalf("first init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := Run([]string{"init", "--root", root, "--change", "example"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "already initialized") {
		t.Fatalf("second init = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	if code := Run([]string{"init", "--root", filepath.Join(root, "missing"), "--change", "example"}, io.Discard, &stderr); code != 2 {
		t.Fatalf("failing init exit = %d", code)
	}
}

func TestParseOptions(t *testing.T) {
	parsed, err := parseOptions("verify", []string{"--root", ".", "--change", "demo", "--stage", "proposal", "--report", "report.json", "--evidence", "evidence.json", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.changeID != "demo" || parsed.stage != "proposal" || parsed.reportPath != "report.json" || parsed.evidencePath != "evidence.json" || !parsed.json || !filepath.IsAbs(parsed.root) {
		t.Fatalf("unexpected options: %#v", parsed)
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
	passReport := Report{Mode: "implementation", Verdict: "pass"}
	passReport.Summary.Requirements, passReport.Summary.Scenarios = 1, 1
	failReport := passReport
	failReport.Verdict = "fail"
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
			verifyProject = func(string, string, string, string) (Report, error) { return test.report, nil }
			var stdout bytes.Buffer
			if code := verifyCommand(options{json: test.json}, &stdout, io.Discard); code != test.code || stdout.Len() == 0 {
				t.Fatalf("verifyCommand = %d, %q", code, stdout.String())
			}
		})
	}
	verifyProject = func(string, string, string, string) (Report, error) { return Report{}, errors.New("verify failed") }
	if code := verifyCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("verify error exit = %d", code)
	}

	passEvidence := Evidence{Outcome: "passed", Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "passed"}}}
	failEvidence := Evidence{Outcome: "failed", Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "failed"}, {ID: "b", Outcome: "passed"}}}
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
			runProjectScenarios = func(string, string, string) (Evidence, error) { return test.evidence, nil }
			var stdout bytes.Buffer
			if code := testCommand(options{json: test.json}, &stdout, io.Discard); code != test.code || stdout.Len() == 0 {
				t.Fatalf("testCommand = %d, %q", code, stdout.String())
			}
		})
	}
	runProjectScenarios = func(string, string, string) (Evidence, error) { return Evidence{}, errors.New("test failed") }
	if code := testCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("test error exit = %d", code)
	}
}

func TestValidateCommand(t *testing.T) {
	originalVerify, originalScenarios, originalOpenSpec := verifyProject, runProjectScenarios, validateProjectOpenSpec
	t.Cleanup(func() {
		verifyProject, runProjectScenarios, validateProjectOpenSpec = originalVerify, originalScenarios, originalOpenSpec
	})
	passReport := Report{Verdict: "pass"}
	passReport.Summary.Requirements, passReport.Summary.Scenarios = 1, 1
	passEvidence := Evidence{Outcome: "passed", Scenarios: []ScenarioOutcome{{ID: "a", Outcome: "passed"}}}
	runProjectScenarios = func(string, string, string) (Evidence, error) { return passEvidence, nil }
	verifyProject = func(string, string, string, string) (Report, error) { return passReport, nil }
	validateProjectOpenSpec = func(string, string) (bool, error) { return true, nil }
	for _, jsonOutput := range []bool{false, true} {
		var stdout bytes.Buffer
		if code := validateCommand(options{json: jsonOutput}, &stdout, io.Discard); code != 0 || stdout.Len() == 0 {
			t.Fatalf("validate pass = %d, %q", code, stdout.String())
		}
	}
	validateProjectOpenSpec = func(string, string) (bool, error) { return false, errors.New("openspec failed") }
	if code := validateCommand(options{evidencePath: "e.json", reportPath: "r.json"}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("validate OpenSpec failure = %d", code)
	}
	runProjectScenarios = func(string, string, string) (Evidence, error) { return Evidence{}, errors.New("scenario failed") }
	if code := validateCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("validate scenario error = %d", code)
	}
	runProjectScenarios = func(string, string, string) (Evidence, error) { return passEvidence, nil }
	verifyProject = func(string, string, string, string) (Report, error) { return Report{}, errors.New("verify failed") }
	if code := validateCommand(options{}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("validate verify error = %d", code)
	}
}

func TestRunOpenSpec(t *testing.T) {
	root := fixtureRoot(t)
	originalExecutable := executablePath
	t.Cleanup(func() { executablePath = originalExecutable })
	executablePath = func() (string, error) { return "", errors.New("executable failed") }
	if _, err := runOpenSpec(root, "example"); err == nil {
		t.Fatal("expected executable error")
	}
	executablePath = originalExecutable
	if _, err := runOpenSpec(root, "example"); err == nil {
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
	passed, err := runOpenSpec(root, "example")
	if err != nil || !passed {
		t.Fatalf("runOpenSpec success = %v, %v", passed, err)
	}
	t.Setenv("STELE_FAKE_EXIT", "1")
	if _, err := runOpenSpec(root, "example"); err == nil || !strings.Contains(err.Error(), "validated") {
		t.Fatalf("expected validation output, got %v", err)
	}
}

func TestRunRoutesCommandsAndConfigurationErrors(t *testing.T) {
	originalVerify, originalScenarios, originalOpenSpec := verifyProject, runProjectScenarios, validateProjectOpenSpec
	t.Cleanup(func() {
		verifyProject, runProjectScenarios, validateProjectOpenSpec = originalVerify, originalScenarios, originalOpenSpec
	})
	root := completeFixture(t, false)
	verifyProject = func(string, string, string, string) (Report, error) { return Report{Verdict: "pass"}, nil }
	runProjectScenarios = func(string, string, string) (Evidence, error) { return Evidence{Outcome: "passed"}, nil }
	validateProjectOpenSpec = func(string, string) (bool, error) { return true, nil }
	for _, command := range []string{"verify", "test", "validate"} {
		if code := Run([]string{command, "--root", root, "--change", "example", "--json"}, io.Discard, io.Discard); code != 0 {
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
