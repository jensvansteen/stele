package stele

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// @verifies scn.validate.ed99aac59b49.unit
func TestCheckRunsEveryStep(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `<!-- stele: spec v1 -->
### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
#### Scenario: Value is cached
`)
	code, stdout, _ := runCommand(t, "check", "--root", root, "--change", "example")
	if code != 1 {
		t.Fatalf("check = %d\n%s", code, stdout)
	}
	assertOrdered(t, stdout,
		"stele "+Version+" · check · change example",
		"✗ Verification-IDs    1 heading without a Verification-ID; run `stele ids`",
		`openspec/changes/example/specs/demo/spec.md:6 scenario "Value is cached"`,
		"✓ Annotations         1 file annotated",
		"✗ Validation          see the report below",
		"stele "+Version+" · validate · change example",
		"✗ FAILED  check · change example — verification-ids, validation",
	)

	code, stdout, _ = runCommand(t, "check", "--root", root, "--change", "example", "--json")
	var result checkResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || code != 1 || result.Verdict != "fail" ||
		len(result.Steps) != 3 || result.Steps[0].ExitCode != 1 || result.Steps[1].ExitCode != 0 ||
		result.Steps[2].Step != "validate" || string(result.Steps[2].Result) == "null" {
		t.Fatalf("check --json = %d, %v, %q", code, err, stdout)
	}
	code, stdout, _ = runCommand(t, "check", "--root", root, "--change", "example", "--quiet")
	if code != 1 || stdout != "✗ FAILED  check · change example — verification-ids, validation\n" {
		t.Fatalf("check --quiet = %d, %q", code, stdout)
	}
}

// @verifies scn.validate.d768594936b1.unit
func TestCheckExitsWithTheWorstCode(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) {
		return false, errors.New("openspec could not run")
	}
	original := runProjectScenarios
	t.Cleanup(func() { runProjectScenarios = original })
	runProjectScenarios = func(testRequest) (testRun, error) { return testRun{}, errors.New("runner failed") }
	code, stdout, stderr := runCommand(t, "check", "--root", root, "--change", "broken")
	if code != 2 || !strings.Contains(stdout, "✗ Annotations") || !strings.Contains(stderr, "runner failed") {
		t.Fatalf("check with a tool failure = %d, %q, %q", code, stdout, stderr)
	}
	runProjectScenarios = original
	validateProjectOpenSpec = func(string, verificationScope) (bool, error) { return true, nil }
	if code, _, _ := runCommand(t, "check", "--root", root, "--change", "example"); code != 0 {
		t.Fatal("a passing check did not exit with 0")
	}
	failing := checkStepSummary{}
	for _, broken := range []func() (checkStepSummary, checkStep){
		func() (checkStepSummary, checkStep) {
			return checkIdentities(root, []verificationScope{changeScope("none")})
		},
		func() (checkStepSummary, checkStep) {
			return checkAnnotations(root, []verificationScope{changeScope("none")})
		},
	} {
		if failing, _ = broken(); failing.code != 0 && failing.code != 2 {
			t.Fatalf("a failing step = %#v", failing)
		}
	}
	if rawJSON([]byte("not json")) == nil {
		t.Fatal("invalid step output was kept")
	}
}

// @verifies scn.validate.704b1662c309.unit
func TestCheckCoversEveryScope(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	code, stdout, _ := runCommand(t, "check", "--all", "--root", root)
	if code != 1 {
		t.Fatalf("check --all = %d\n%s", code, stdout)
	}
	assertOrdered(t, stdout,
		"stele "+Version+" · check · all scopes",
		"✓ Verification-IDs    3 scopes; every requirement and scenario has an ID",
		"✗ Annotations         2 files without a valid annotation; run `stele annotate`",
		"MISSING openspec/specs/demo/spec.md",
		"MISSING openspec/changes/broken/specs/broken/spec.md",
		"== current specifications ==", "== change broken ==", "== change example ==",
		"✗ FAILED  1 of 3 scopes failed: change broken",
		"✗ FAILED  check · all scopes — annotations, validation",
	)
}

// @verifies scn.validate.0c6103aa40e7.unit
func TestCheckAnnotatesMissingIDsAndAnnotations(t *testing.T) {
	root := allScopesFixture(t)
	recordTests(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `<!-- stele: spec v1 -->
### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
#### Scenario: Value is cached
`)
	stubEnvironment(t, map[string]string{"GITHUB_ACTIONS": "true"})
	code, _, stderr := runCommand(t, "check", "--root", root, "--change", "example")
	if code != 1 || !strings.Contains(stderr,
		"::error file=openspec/changes/example/specs/demo/spec.md,line=6,title=ID_SCENARIO_MISSING::"+
			"A scenario has no Verification-ID. scenario \"Value is cached\"; run `stele ids --change example`") {
		t.Fatalf("check annotations = %d\n%s", code, stderr)
	}
	code, stdout, stderr := runCommand(t, "check", "--all", "--root", root, "--json")
	if code != 1 || !json.Valid([]byte(stdout)) || strings.Count(stderr, "title=ID_SCENARIO_MISSING::") != 2 {
		t.Fatalf("check --all --json = %d\n%s", code, stderr)
	}
	assertOrdered(t, stderr,
		"::error file=openspec/changes/example/specs/demo/spec.md,line=6,title=ID_SCENARIO_MISSING::",
		"::error file=openspec/specs/demo/spec.md,title=SPEC_ANNOTATION_MISSING::"+
			"A specification file has no Stele annotation. Fix: run `stele annotate --specs`",
		"::error file=openspec/changes/broken/specs/broken/spec.md,title=SPEC_ANNOTATION_MISSING::",
		"title=LINK_CODE_MISSING::",
	)
	requirement := identityAnnotation(
		IdentityInsertion{Kind: "requirement", Title: "R", Path: "s.md", Line: 2}, "--specs")
	if requirement.title != "ID_REQUIREMENT_MISSING" {
		t.Fatalf("requirement annotation = %#v", requirement)
	}
}
