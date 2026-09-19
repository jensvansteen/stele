package stele

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The targeted fixture: one change whose share capability applies to ios and
// android, with one scenario narrowed to ios, and an e2e suite as system.
const (
	shareRequirementID  = "req.share.aaaaaaaaaaaa"
	shareTextID         = "scn.share.bbbbbbbbbbbb"
	shareSheetID        = "scn.share.cccccccccccc"
	mobileTargetsConfig = `{"targets":{"ios":{"paths":["ios/**"]},"android":{"paths":["android/**"]}}}`
	targetsConfig       = `{"schemaVersion":1,"change":"example","targets":{` +
		`"android":{"paths":["android/**","shared/**"]},` +
		`"ios":{"paths":["ios/**","shared/**"]},` +
		`"system":{"paths":["e2e/**"],"evidenceOnly":true}}}`
	shareSpec = `<!-- stele: spec v1; targets: ios, android -->
## ADDED Requirements

### Requirement: Share a list
Verification-ID: req.share.aaaaaaaaaaaa

The app SHALL share a list as plain text.

#### Scenario: Share text contains every item
Verification-ID: scn.share.bbbbbbbbbbbb

- **WHEN** the user shares a list with three items
- **THEN** the shared text lists the three items in order

#### Scenario: Share through the iOS share sheet
Verification-ID: scn.share.cccccccccccc
Targets: ios

- **WHEN** the user taps Share
- **THEN** the system share sheet opens
`
)

// testTargets builds a target registry from one path pattern per target.
func testTargets(t *testing.T, paths map[string]string) *projectTargets {
	t.Helper()
	raw := make(map[string]json.RawMessage, len(paths))
	for name, pattern := range paths {
		raw[name] = json.RawMessage(`{"paths":["` + pattern + `"]}`)
	}
	targets, err := parseProjectTargets(raw)
	if err != nil {
		t.Fatal(err)
	}
	return targets
}

// parseTargetedScope parses a scope with a target registry.
func parseTargetedScope(t *testing.T, root string, scope verificationScope, paths map[string]string) ParsedSpecs {
	t.Helper()
	scope.targets = testTargets(t, paths)
	parsed, err := parseScopeSpecs(root, scope)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// configuredScope returns a scope with the fixture's configured targets.
func configuredScope(t *testing.T, root string, scope verificationScope, selected ...string) verificationScope {
	t.Helper()
	config, err := readConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if scope.targets, err = parseProjectTargets(config.Targets); err != nil {
		t.Fatal(err)
	}
	scope.selected = selected
	return scope
}

// targetedFixture writes the targeted project with ios and android code.
func targetedFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", targetsConfig)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", shareSpec)
	writeFixture(t, root, "ios/share.mts", "// @implements "+shareRequirementID+"\nexport function share() {}\n")
	writeFixture(t, root, "android/share.mts", "// @implements "+shareRequirementID+"\nexport function share() {}\n")
	return root
}

// targetedEntry returns an unapproved entry of a target.
func targetedEntry(scenarioID, target, level string) EvidenceEntry {
	return EvidenceEntry{
		ID: evidenceID(scenarioID, target, level), Target: target, Level: level, Rationale: "Pure logic.",
	}
}

// writeTargetedPlan writes a change plan, approving every entry for the
// scenario wording the scope parses.
func writeTargetedPlan(t *testing.T, root string, approve bool, entries ...EvidenceEntry) {
	t.Helper()
	scope := configuredScope(t, root, changeScope("example"))
	parsed, err := parseScopeSpecs(root, scope)
	if err != nil {
		t.Fatal(err)
	}
	scenarios := make(map[string]Scenario)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			scenarios[scenario.ID] = scenario
		}
	}
	plan := evidencePlanFile{SchemaVersion: 2, ChangeID: "example", Scenarios: map[string]ScenarioEvidence{}}
	for _, entry := range entries {
		scenarioID := entry.ID[:len("scn.share.bbbbbbbbbbbb")]
		for known := range scenarios {
			if strings.HasPrefix(entry.ID, known+".") {
				scenarioID = known
			}
		}
		if approve {
			entry.Approval = &EvidenceApproval{
				Approver: "reviewer", Date: "2026-09-19", Via: "cli",
				Digest: evidenceDigest(scenarios[scenarioID], entry),
			}
		}
		evidence := plan.Scenarios[scenarioID]
		evidence.Evidence = append(evidence.Evidence, entry)
		plan.Scenarios[scenarioID] = evidence
	}
	content, err := MarshalDeterministic(plan)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "openspec/changes/example/linkage-plan.json", string(content))
}

// writeTargetedTest writes a TypeScript test with a @verifies anchor.
func writeTargetedTest(t *testing.T, root, relative, evidence, name string) {
	t.Helper()
	writeFixture(t, root, relative, "import { test } from \"node:test\";\n\n// @verifies "+evidence+
		"\ntest(\""+name+"\", () => {});\n")
}

// completeTargetedFixture plans, approves, and anchors every applicable
// entry of the targeted fixture.
func completeTargetedFixture(t *testing.T) string {
	t.Helper()
	root := targetedFixture(t)
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "android", "unit"),
		targetedEntry(shareSheetID, "ios", "unit"))
	writeTargetedTest(t, root, "ios/share.test.mts", shareTextID+".ios.unit", "ios share text")
	writeTargetedTest(t, root, "android/share.test.mts", shareTextID+".android.unit", "android share text")
	writeTargetedTest(t, root, "ios/sheet.test.mts", shareSheetID+".ios.unit", "ios share sheet")
	return root
}

func verifyTargeted(t *testing.T, root, mode string, selected ...string) Report {
	t.Helper()
	report, err := verifyScope(verifyRequest{
		root: root, scope: configuredScope(t, root, changeScope("example"), selected...), mode: mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func diagnosticCodes(diagnostics []Diagnostic) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, item := range diagnostics {
		codes = append(codes, item.Code)
	}
	return codes
}

func targetDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	found := make([]Diagnostic, 0)
	for _, item := range diagnostics {
		if strings.Contains(item.Code, "TARGET") {
			found = append(found, item)
		}
	}
	return found
}

// @verifies scn.verificationtargets.eb1c6810d349.unit
func TestTargetConfigurationIsAccepted(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"targets":{`+
		`"zed":{"paths":["zed/**"]},"vscode":{"paths":["vscode/**"]},"jetbrains":{"paths":["jetbrains/**"]},`+
		`"system":{"paths":["e2e/**"],"evidenceOnly":true}}}`)
	writeFixture(t, root, "openspec/specs/lens/spec.md", `<!-- stele: spec v1; targets: vscode, zed, jetbrains -->
### Requirement: Show a lens
Verification-ID: req.lens.aaaaaaaaaaaa
#### Scenario: Lens on a function
Verification-ID: scn.lens.bbbbbbbbbbbb
- **WHEN** a function is anchored
- **THEN** a lens names the requirement
`)
	index := runIndex(t, "--specs", "--root", root)
	want := []IndexTarget{
		{Name: "jetbrains", Paths: []string{"jetbrains/**"}},
		{Name: "system", Paths: []string{"e2e/**"}, EvidenceOnly: true},
		{Name: "vscode", Paths: []string{"vscode/**"}},
		{Name: "zed", Paths: []string{"zed/**"}},
	}
	if content, _ := json.Marshal(index.Targets); string(content) != mustJSON(t, want) {
		t.Fatalf("index targets = %s", content)
	}
	for pattern, file := range map[string]string{
		"ios/**": "ios/App/Share.swift", "shared/*.go": "shared/share.go", "**": "anything/at/all.ts",
		"apps/*/src/**": "apps/web/src/deep/page.tsx", "e2e/checkout.spec.ts": "e2e/checkout.spec.ts",
	} {
		if !testTargets(t, map[string]string{"one": pattern}).matches("one", file) {
			t.Fatalf("%s does not match %s", pattern, file)
		}
	}
	for pattern, file := range map[string]string{
		"ios/**": "android/Share.kt", "shared/*.go": "shared/deep/share.go", "ios/*": "ios",
	} {
		if testTargets(t, map[string]string{"one": pattern}).matches("one", file) {
			t.Fatalf("%s matches %s", pattern, file)
		}
	}
	targets := testTargets(t, map[string]string{"ios": "ios/**", "shared": "shared/*.go", "all": "**"})
	if roots := targets.scanRoots(); !slices.Equal(roots, []string{".", "ios", "shared"}) {
		t.Fatalf("scan roots = %v", roots)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// @verifies scn.verificationtargets.36d207a647d0.unit
func TestInvalidTargetDefinitionsStopTheCommand(t *testing.T) {
	for definition, want := range map[string]string{
		`"e2e":{"paths":["e2e/**"]}`:                  `invalid target "e2e"`,
		`"iOS":{"paths":["ios/**"]}`:                  `invalid target "iOS"`,
		`"web":{"paths":[]}`:                          `"web" in stele.config.json: paths must list`,
		`"web":{}`:                                    `"web" in stele.config.json: paths must list`,
		`"web":{"paths":["web/**"],"owner":"me"}`:     `"web" in stele.config.json: unknown field "owner"`,
		`"web":{"paths":["/web/**"]}`:                 `invalid path pattern "/web/**"`,
		`"web":{"paths":["web/a**"]}`:                 "`**` must be a whole path segment",
		`"web":{"paths":["web/../api"]}`:              "`..` segment",
		`"web":{"paths":[""]}`:                        "it is empty",
		`"web":{"paths":["web/**"],"evidenceOnly":1}`: `invalid target "web"`,
	} {
		root := evidenceFixture(t)
		writeFixture(t, root, "stele.config.json", `{"change":"example","targets":{`+definition+`}}`)
		code, stdout, stderr := runCommand(t, "verify", "--root", root)
		if code != 2 || stdout != "" || !strings.Contains(stderr, want) {
			t.Fatalf("%s: verify = %d, %q, %q", definition, code, stdout, stderr)
		}
	}
}

// @verifies scn.verificationtargets.e8cd45d85205.unit
func TestSpecificationTargetsApplyToEveryScenario(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", strings.Replace(shareSpec,
		"Targets: ios\n", "", 1))
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{
		"ios": "ios/**", "android": "android/**",
	})
	for _, scenario := range parsed.Requirements[0].Scenarios {
		if !scenario.Targeted || !slices.Equal(scenario.Targets, []string{"android", "ios"}) {
			t.Fatalf("scenario targets = %#v", scenario)
		}
	}
	if found := targetDiagnostics(parsed.Diagnostics); len(found) != 0 {
		t.Fatalf("target diagnostics = %#v", found)
	}
}

// @verifies scn.verificationtargets.975c05dc71d9.unit
func TestUnknownAndMalformedTargetListsFail(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md",
		strings.Replace(shareSpec, "targets: ios, android", "targets: ios, anroid", 1))
	writeFixture(t, root, "openspec/changes/example/specs/sheet/spec.md",
		strings.Replace(strings.Replace(shareSpec, "targets: ios, android", "targets: ios, ios", 1),
			"scn.share.", "scn.sheet.", 2))
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{
		"ios": "ios/**", "android": "android/**",
	})
	unknown := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGET_UNKNOWN")
	if len(unknown) != 1 || !strings.Contains(unknown[0].Message, "anroid") ||
		!strings.Contains(unknown[0].Message, "configured targets: android, ios") ||
		unknown[0].Source.Path != "openspec/changes/example/specs/share/spec.md" {
		t.Fatalf("SPEC_TARGET_UNKNOWN = %#v", unknown)
	}
	malformed := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_MALFORMED")
	if len(malformed) != 1 || !strings.Contains(malformed[0].Message, `"ios" twice`) {
		t.Fatalf("SPEC_TARGETS_MALFORMED = %#v", malformed)
	}
	if _, _, verdict := diagnosticSummary(parsed.Diagnostics); verdict != "fail" {
		t.Fatal("unknown and malformed target lists did not fail")
	}
	for value, problem := range map[string]string{
		"": "the list is empty", "ios,,web": "an empty name",
		"Web": `the invalid name "Web"`, "unit": `the invalid name "unit"`,
	} {
		if _, found := parseTargetList(value); !strings.Contains(found, problem) {
			t.Fatalf("parseTargetList(%q) = %q", value, found)
		}
	}
}

// @verifies scn.verificationtargets.ca80f845a9cc.unit
func TestChangingTargetsMustCoverEveryAffectedRequirement(t *testing.T) {
	root := fixtureRoot(t)
	current := `<!-- stele: spec v1; targets: ios, android -->
### Requirement: Share a list
Verification-ID: req.share.111111111111
#### Scenario: Share
Verification-ID: scn.share.222222222222
- **WHEN** shared
- **THEN** text
### Requirement: Copy a list
Verification-ID: req.share.333333333333
#### Scenario: Copy
Verification-ID: scn.share.444444444444
- **WHEN** copied
- **THEN** text
### Requirement: Print a list
Verification-ID: req.share.555555555555
Targets: ios
#### Scenario: Print
Verification-ID: scn.share.666666666666
- **WHEN** printed
- **THEN** paper
### Requirement: Rename a list
Verification-ID: req.share.777777777777
#### Scenario: Rename
Verification-ID: scn.share.888888888888
- **WHEN** renamed
- **THEN** title
`
	writeFixture(t, root, "openspec/specs/share/spec.md", current)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", `<!-- stele: spec v1; targets: ios -->
## MODIFIED Requirements
### Requirement: Share a list
Verification-ID: req.share.111111111111
#### Scenario: Share
Verification-ID: scn.share.222222222222
- **WHEN** shared
- **THEN** text
## REMOVED Requirements
### Requirement: Rename a list
`)
	paths := map[string]string{"ios": "ios/**", "android": "android/**"}
	parsed := parseTargetedScope(t, root, changeScope("example"), paths)
	uncovered := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_CHANGE_UNCOVERED")
	if len(uncovered) != 1 || identityOf(uncovered[0]) != "req.share.333333333333" ||
		!strings.Contains(uncovered[0].Message, `"Copy a list"`) ||
		!strings.Contains(uncovered[0].Message, "from android, ios to ios") {
		t.Fatalf("SPEC_TARGETS_CHANGE_UNCOVERED = %#v", uncovered)
	}

	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", `## MODIFIED Requirements
### Requirement: Share a list
Verification-ID: req.share.111111111111
#### Scenario: Share
Verification-ID: scn.share.222222222222
- **WHEN** shared
- **THEN** text
`)
	parsed = parseTargetedScope(t, root, changeScope("example"), paths)
	if mismatch := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_MISMATCH"); len(mismatch) != 1 ||
		!strings.Contains(mismatch[0].Message, "declares no targets, but the current specification "+
			"openspec/specs/share/spec.md declares the targets ios, android") {
		t.Fatalf("SPEC_TARGETS_MISMATCH = %#v", mismatch)
	}
}

// @verifies scn.verificationtargets.577d723898cd.unit
func TestScenarioNarrowsToOnePlatform(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", shareSpec+`
#### Scenario: Share through an Android share intent
Verification-ID: scn.share.dddddddddddd
Targets: android

- **WHEN** the user taps Share
- **THEN** a chooser opens
`)
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{
		"ios": "ios/**", "android": "android/**",
	})
	want := [][]string{{"android", "ios"}, {"ios"}, {"android"}}
	for index, scenario := range parsed.Requirements[0].Scenarios {
		if !slices.Equal(scenario.Targets, want[index]) {
			t.Fatalf("scenario %d targets = %v", index, scenario.Targets)
		}
	}
	if found := targetDiagnostics(parsed.Diagnostics); len(found) != 0 {
		t.Fatalf("target diagnostics = %#v", found)
	}
	if narrowed := parsed.Requirements[0].Scenarios[1]; !slices.Equal(narrowed.DeclaredTargets, []string{"ios"}) ||
		strings.Contains(narrowed.Text, "Targets:") || strings.Contains(narrowed.Body, "Targets:") {
		t.Fatalf("the Targets: line leaked into the scenario: %#v", narrowed)
	}
}

// @verifies scn.verificationtargets.6616ae0098fb.unit
func TestRequirementsSplitByResponsibility(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/checkout/spec.md", `<!-- stele: spec v1; targets: api, web -->
### Requirement: Validate promo codes
Verification-ID: req.checkout.111111111111
Targets: api

The API SHALL reject expired codes.

#### Scenario: Reject an expired code
Verification-ID: scn.checkout.222222222222
- **WHEN** an expired code is posted
- **THEN** the API answers 410

### Requirement: Show the promo discount
Targets: web
Verification-ID: req.checkout.333333333333

#### Scenario: Show the discount line
Verification-ID: scn.checkout.444444444444
- **WHEN** a code is accepted
- **THEN** the page shows the discount

### Requirement: Promo contract
Verification-ID: req.checkout.555555555555

#### Scenario: Apply a valid code
Verification-ID: scn.checkout.666666666666
- **WHEN** a valid code is submitted
- **THEN** the response has the discount
`)
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{"api": "api/**", "web": "web/**"})
	want := [][]string{{"api"}, {"web"}, {"api", "web"}}
	for index, requirement := range parsed.Requirements {
		if !slices.Equal(requirement.Targets, want[index]) ||
			!slices.Equal(requirement.Scenarios[0].Targets, want[index]) {
			t.Fatalf("requirement %d = %v, scenario = %v", index, requirement.Targets, requirement.Scenarios[0].Targets)
		}
	}
	if parsed.Requirements[0].Text != "The API SHALL reject expired codes." {
		t.Fatalf("requirement text = %q", parsed.Requirements[0].Text)
	}
	if found := targetDiagnostics(parsed.Diagnostics); len(found) != 0 {
		t.Fatalf("target diagnostics = %#v", found)
	}
}

// @verifies scn.verificationtargets.7e8b768b3da0.unit
func TestScenarioCannotWidenItsRequirement(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/checkout/spec.md", `<!-- stele: spec v1; targets: api, web -->
### Requirement: Validate promo codes
Verification-ID: req.checkout.111111111111
Targets: api
#### Scenario: Reject an expired code
Verification-ID: scn.checkout.222222222222
Targets: api, web
- **WHEN** an expired code is posted
- **THEN** it is rejected
`)
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{"api": "api/**", "web": "web/**"})
	widened := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_WIDENED")
	if len(widened) != 1 || widened[0].Source.Line != 7 || !strings.Contains(widened[0].Message, "names web") {
		t.Fatalf("SPEC_TARGETS_WIDENED = %#v", widened)
	}
	if _, _, verdict := diagnosticSummary(parsed.Diagnostics); verdict != "fail" {
		t.Fatal("a widening scenario did not fail")
	}
}

// @verifies scn.verificationtargets.e324fcddf99b.unit
func TestMisplacedAndUndeclaredTargetLinesAreReported(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", strings.Replace(shareSpec,
		"- **WHEN** the user shares a list with three items\n",
		"- **WHEN** the user shares a list with three items\nTargets: ios\n", 1))
	writeFixture(t, root, "openspec/changes/example/specs/plain/spec.md", `<!-- stele: spec v1 -->
### Requirement: Plain
Verification-ID: req.plain.111111111111
Targets: ios
#### Scenario: Plain
Verification-ID: scn.plain.222222222222
- **WHEN** it runs
- **THEN** it passes
`)
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{"ios": "ios/**", "android": "a/**"})
	misplaced := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_MISPLACED")
	if len(misplaced) != 1 || misplaced[0].Source.Path != "openspec/changes/example/specs/share/spec.md" ||
		misplaced[0].Source.Line != 13 {
		t.Fatalf("SPEC_TARGETS_MISPLACED = %#v", misplaced)
	}
	undeclared := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_UNDECLARED")
	if len(undeclared) != 1 || undeclared[0].Source.Path != "openspec/changes/example/specs/plain/spec.md" ||
		undeclared[0].Source.Line != 4 {
		t.Fatalf("SPEC_TARGETS_UNDECLARED = %#v", undeclared)
	}
	twice := `<!-- stele: spec v1; targets: ios -->
### Requirement: Twice
Targets: ios
Targets: ios
Verification-ID: req.twice.111111111111
#### Scenario: Twice
Verification-ID: scn.twice.222222222222
- **WHEN** it runs
- **THEN** it passes
`
	writeFixture(t, root, "openspec/changes/example/specs/plain/spec.md", twice)
	parsed = parseTargetedScope(t, root, changeScope("example"), map[string]string{"ios": "ios/**", "android": "a/**"})
	if malformed := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_MALFORMED"); len(malformed) != 1 ||
		malformed[0].Source.Line != 4 {
		t.Fatalf("a second Targets: line = %#v", malformed)
	}
}

// @verifies scn.verificationtargets.bb95c914ccac.unit
func TestNarrowingKeepsTheApprovalsOfRemainingTargets(t *testing.T) {
	root := targetedFixture(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", strings.Replace(shareSpec,
		"Targets: ios\n", "", 1))
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "android", "unit"),
		targetedEntry(shareSheetID, "ios", "unit"), targetedEntry(shareSheetID, "android", "unit"))
	if report := verifyTargeted(t, root, "proposal"); report.Verdicts.Linkage != "pass" {
		t.Fatalf("the approved plan did not pass: %v", diagnosticCodes(report.Diagnostics))
	}
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", shareSpec)
	report := verifyTargeted(t, root, "proposal")
	codes := diagnosticCodes(report.Diagnostics)
	if !slices.Equal(codes, []string{"PLAN_TARGET_NOT_APPLICABLE"}) ||
		identityOf(report.Diagnostics[0]) != shareSheetID+".android.unit" {
		t.Fatalf("narrowing reported %v: %#v", codes, report.Diagnostics)
	}
	for _, entry := range report.Requirements[0].Scenarios[1].Evidence {
		if entry.Target == "ios" && entry.Approval != "approved" {
			t.Fatalf("the ios entry lost its approval: %#v", entry)
		}
	}
}

// @verifies scn.verificationtargets.1a22d82ecf41.unit
func TestAnchorsReadTargetedEvidenceIDs(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"targets":{"web":{"paths":["web/**"]},"api":{"paths":["api/**"]}}}`)
	writeTargetedTest(t, root, "web/share.test.mts", shareTextID+".web.unit", "web share")
	writeFixture(t, root, "api/share_test.go", "package api\n\nimport \"testing\"\n\n// @verifies "+
		shareTextID+".api.integration.2\nfunc TestShare(t *testing.T) {}\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0)
	for _, anchor := range anchors {
		got = append(got, anchor.ID+"|"+anchor.EvidenceID+"|"+anchor.Target+"|"+anchor.Level+"|"+
			anchor.Path+"#"+pointerValue(anchor.Selector))
	}
	want := []string{
		shareTextID + "|" + shareTextID + ".api.integration.2|api|integration|api/share_test.go#TestShare",
		shareTextID + "|" + shareTextID + ".web.unit|web|unit|web/share.test.mts#web share",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("anchors = %v", got)
	}
	for suffix, want := range map[string]string{
		".unit": "||unit", ".e2e.2": "||e2e", ".ios.unit": "|ios|unit", ".android.e2e.3": "|android|e2e",
	} {
		evidence, target, level := anchorEvidence(shareTextID, suffix)
		if evidence != shareTextID+suffix || "|"+target+"|"+level != want {
			t.Fatalf("anchorEvidence(%q) = %q, %q, %q", suffix, evidence, target, level)
		}
	}
	if level, _, valid := splitEvidenceID(shareTextID, shareTextID+".ios.unit"); valid {
		t.Fatalf("a targeted ID passed as untargeted: %q", level)
	}
	if target, level, ordinal, valid := splitTargetedEvidenceID(shareTextID, shareTextID+".unit.e2e"); valid {
		t.Fatalf("a level name was read as a target: %q %q %d", target, level, ordinal)
	}
}

// @verifies scn.verificationtargets.880647fbb533.unit
func TestEvidenceInTheWrongTargetIsReported(t *testing.T) {
	root := completeTargetedFixture(t)
	writeFixture(t, root, "android/share.test.mts", "import { test } from \"node:test\";\n\n// @verifies "+
		shareTextID+".ios.unit\ntest(\"copied from ios\", () => {});\n")
	report := verifyTargeted(t, root, "implementation")
	outside := diagnosticsWithCode(report.Diagnostics, "ANCHOR_TARGET_OUTSIDE_PATHS")
	if len(outside) != 1 || outside[0].Source.Path != "android/share.test.mts" || outside[0].Source.Line != 3 ||
		!strings.Contains(outside[0].Message, "target ios") ||
		!strings.Contains(outside[0].Message, "ios/**, shared/**") {
		t.Fatalf("ANCHOR_TARGET_OUTSIDE_PATHS = %#v", outside)
	}
	if report.Verdicts.Linkage != "fail" {
		t.Fatal("evidence in the wrong target did not fail")
	}
}

// @verifies scn.verificationtargets.c3f36c37e37a.unit
func TestEveryReplicaNeedsAnImplementation(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"change":"example","targets":{"vscode":{"paths":["vscode/**"]},`+
		`"zed":{"paths":["zed/**"]},"jetbrains":{"paths":["jetbrains/**"]}}}`)
	writeFixture(t, root, "openspec/changes/example/specs/lens/spec.md",
		`<!-- stele: spec v1; targets: vscode, zed, jetbrains -->
### Requirement: Show a lens
Verification-ID: req.lens.aaaaaaaaaaaa
#### Scenario: Lens on a function
Verification-ID: scn.lens.bbbbbbbbbbbb
- **WHEN** a function is anchored
- **THEN** a lens names the requirement
`)
	writeFixture(t, root, "vscode/lens.mts", "// @implements req.lens.aaaaaaaaaaaa\nexport function lens() {}\n")
	writeFixture(t, root, "zed/lens.mts", "// @implements req.lens.aaaaaaaaaaaa\nexport function lens() {}\n")
	report := verifyTargeted(t, root, "implementation")
	missing := diagnosticsWithCode(report.Diagnostics, "LINK_TARGET_IMPLEMENTATION_MISSING")
	if len(missing) != 1 || identityOf(missing[0]) != "req.lens.aaaaaaaaaaaa" ||
		!strings.Contains(missing[0].Message, "target jetbrains") {
		t.Fatalf("LINK_TARGET_IMPLEMENTATION_MISSING = %#v", missing)
	}
}

// @verifies scn.verificationtargets.3574271b7f35.unit
func TestSharedCodeAndJourneysPass(t *testing.T) {
	root := targetedFixture(t)
	writeFixture(t, root, "stele.config.json", strings.Replace(targetsConfig, `"system"`,
		`"api":{"paths":["api/**"]},"system"`, 1))
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md",
		strings.Replace(strings.Replace(shareSpec, "targets: ios, android", "targets: ios, android, system", 1),
			"Verification-ID: req.share.aaaaaaaaaaaa\n",
			"Verification-ID: req.share.aaaaaaaaaaaa\nTargets: ios, android\n", 1)+`
### Requirement: Complete a purchase
Verification-ID: req.share.dddddddddddd
Targets: system

#### Scenario: Pay the discounted total
Verification-ID: scn.share.eeeeeeeeeeee

- **WHEN** a shopper pays
- **THEN** the total is discounted
`)
	writeFixture(t, root, "api/purchase.mts", "// @implements req.share.dddddddddddd\nexport function pay() {}\n")
	writeFixture(t, root, "shared/share.test.mts", "import { test } from \"node:test\";\n\n// @verifies "+
		shareTextID+".ios.unit\n// @verifies "+shareTextID+".android.unit\ntest(\"shared share text\", () => {});\n")
	writeTargetedTest(t, root, "ios/sheet.test.mts", shareSheetID+".ios.unit", "ios share sheet")
	writeTargetedTest(t, root, "e2e/purchase.spec.ts", "scn.share.eeeeeeeeeeee.system.e2e", "pays the total")
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "android", "unit"),
		targetedEntry(shareSheetID, "ios", "unit"), targetedEntry("scn.share.eeeeeeeeeeee", "system", "e2e"))
	report := verifyTargeted(t, root, "implementation")
	if found := targetDiagnostics(report.Diagnostics); len(found) != 0 || report.Verdicts.Linkage != "pass" {
		t.Fatalf("shared code or a journey was reported: %#v", report.Diagnostics)
	}
}

// @verifies scn.verificationtargets.fc749b62bad7.unit
func TestOneTargetIsVerifiedWithoutTheOthersGaps(t *testing.T) {
	root := targetedFixture(t)
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "android", "unit"),
		targetedEntry(shareSheetID, "ios", "unit"))
	writeTargetedTest(t, root, "ios/share.test.mts", shareTextID+".ios.unit", "ios share text")
	writeTargetedTest(t, root, "ios/sheet.test.mts", shareSheetID+".ios.unit", "ios share sheet")
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", shareSpec+`
#### Scenario: Share an empty list
- **WHEN** the list is empty
- **THEN** nothing is shared
`)
	report := verifyTargeted(t, root, "implementation", "ios")
	for _, item := range report.Diagnostics {
		if item.Code == "LINK_EVIDENCE_MISSING" || strings.Contains(identityOf(item), ".android.") {
			t.Fatalf("an android finding was reported for --target ios: %#v", item)
		}
	}
	if !hasDiagnostic(report.Diagnostics, "ID_SCENARIO_MISSING") {
		t.Fatalf("the missing Verification-ID was hidden: %v", diagnosticCodes(report.Diagnostics))
	}
	if !slices.Equal(report.SelectedTargets, []string{"ios"}) {
		t.Fatalf("selected targets = %v", report.SelectedTargets)
	}
	var output strings.Builder
	renderHumanReport(&output, buildHumanReport(humanReportInput{
		command: "verify", scope: configuredScope(t, root, changeScope("example"), "ios"), report: &report,
	}), reportStyle{})
	if !strings.HasPrefix(output.String(), "stele "+Version+" · verify · change example · target ios\n") {
		t.Fatalf("the report does not name the target:\n%s", output.String())
	}
}

// @verifies scn.verificationtargets.857e3746c99d.unit
func TestUnknownTargetSelectionsStopBeforeRunning(t *testing.T) {
	root := completeTargetedFixture(t)
	ran := recordTests(t)
	code, stdout, stderr := runCommand(t, "test", "--target", "androd", "--root", root)
	if code != 2 || stdout != "" || !strings.Contains(stderr, `unknown target "androd"`) ||
		!strings.Contains(stderr, "configured targets: android, ios, system") {
		t.Fatalf("test --target androd = %d, %q, %q", code, stdout, stderr)
	}
	plain := evidenceFixture(t)
	code, stdout, stderr = runCommand(t, "verify", "--target", "ios", "--root", plain, "--change", "example")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "configures none") {
		t.Fatalf("verify --target in a project without targets = %d, %q, %q", code, stdout, stderr)
	}
	if len(*ran) != 0 || fileExists(filepath.Join(root, defaultEvidencePath)) {
		t.Fatalf("tests ran for an invalid selection: %v", *ran)
	}
}

// @verifies scn.verify.1bab272a56e4.unit
func TestPerTargetRulesApplyOnlyToTargetedSpecifications(t *testing.T) {
	root := targetedFixture(t)
	writeFixture(t, root, "openspec/changes/example/specs/plain/spec.md", `<!-- stele: spec v1 -->
## ADDED Requirements

### Requirement: Plain
Verification-ID: req.plain.111111111111

#### Scenario: Plain works
Verification-ID: scn.plain.222222222222

- **WHEN** it runs
- **THEN** it passes
`)
	writeFixture(t, root, "src/plain.mts", "// @implements req.plain.111111111111\nexport function plain() {}\n")
	writeTargetedTest(t, root, "tests/plain.test.mts", "scn.plain.222222222222.unit", "plain works")
	plainEntry := EvidenceEntry{ID: "scn.plain.222222222222.unit", Level: "unit", Rationale: "Pure logic."}
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "android", "unit"),
		targetedEntry(shareSheetID, "ios", "unit"), plainEntry)
	writeTargetedTest(t, root, "ios/share.test.mts", shareTextID+".ios.unit", "ios share text")
	writeTargetedTest(t, root, "ios/sheet.test.mts", shareSheetID+".ios.unit", "ios share sheet")
	report := verifyTargeted(t, root, "implementation")
	missing := diagnosticsWithCode(report.Diagnostics, "LINK_EVIDENCE_MISSING")
	if len(missing) != 1 || identityOf(missing[0]) != shareTextID+".android.unit" || len(report.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	plain := report.Requirements[0].Scenarios[0]
	if plain.Linkage != "linked" || len(plain.Evidence) != 1 || plain.Evidence[0].Target != "" {
		t.Fatalf("the untargeted specification changed: %#v", plain)
	}
}

// @verifies scn.verify.05d59c6b97af.unit
func TestVerdictsArePerTarget(t *testing.T) {
	root := completeTargetedFixture(t)
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{SchemaVersion: evidenceSchemaVersion, InputDigest: digest, Outcome: "failed"}
	for _, test := range []struct{ path, name, evidence, outcome string }{
		{"ios/share.test.mts", "ios share text", shareTextID + ".ios.unit", "passed"},
		{"ios/sheet.test.mts", "ios share sheet", shareSheetID + ".ios.unit", "passed"},
		{"android/share.test.mts", "android share text", shareTextID + ".android.unit", "failed"},
	} {
		evidence.Executions = append(evidence.Executions, execution(test.path, test.name, test.evidence,
			test.outcome, digest))
	}
	evidence.Scenarios = []ScenarioOutcome{{ID: shareTextID, Outcome: "failed"}, {ID: shareSheetID, Outcome: "passed"}}
	report, err := verifyScope(verifyRequest{
		root: root, scope: configuredScope(t, root, changeScope("example")), mode: "implementation",
		evidence: &evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Targets["ios"] != (ReportVerdicts{Linkage: "pass", Execution: "passed", Overall: "pass"}) ||
		report.Targets["android"] != (ReportVerdicts{Linkage: "pass", Execution: "failed", Overall: "fail"}) ||
		report.Verdict != "fail" {
		t.Fatalf("verdicts = %#v, %s", report.Targets, report.Verdict)
	}
	content, _ := MarshalDeterministic(report)
	if !strings.Contains(string(content), `"targets": {`) ||
		!strings.Contains(string(content), `"evidenceTarget": "ios"`) {
		t.Fatalf("the JSON report lacks target verdicts:\n%s", content)
	}
	plain := verifyFixture(t, approvedEvidenceFixture(t), "implementation")
	if plain.Targets != nil || plain.matrix != nil {
		t.Fatalf("an untargeted report has target verdicts: %#v", plain.Targets)
	}
}

func TestTargetFixtureHelpersStayConsistent(t *testing.T) {
	root := completeTargetedFixture(t)
	if _, err := os.Stat(filepath.Join(root, "openspec/changes/example/linkage-plan.json")); err != nil {
		t.Fatal(err)
	}
	if report := verifyTargeted(t, root, "implementation"); report.Verdicts.Linkage != "pass" {
		t.Fatalf("the complete fixture does not pass: %#v", report.Diagnostics)
	}
}

func TestTargetRegistryWithoutTargets(t *testing.T) {
	var none *projectTargets
	if none.configured("ios") || none.matches("ios", "ios/a.ts") || none.evidenceOnly("ios") ||
		none.paths("ios") != "" || none.list() != "none" || none.scanRoots() != nil {
		t.Fatal("a project without targets reports targets")
	}
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"targets":`)
	if roots := configuredTargetRoots(diskFiles{}, root); roots != nil {
		t.Fatalf("an unreadable configuration has roots %v", roots)
	}
	writeFixture(t, root, "stele.config.json", `{"targets":{"iOS":{"paths":["ios/**"]}}}`)
	if roots := configuredTargetRoots(diskFiles{}, root); roots != nil {
		t.Fatalf("an invalid configuration has roots %v", roots)
	}
	if selected, err := validateTargetSelection(nil, nil); selected != nil || err != nil {
		t.Fatalf("no selection = %v, %v", selected, err)
	}
	if targetListText(nil) != "no targets" {
		t.Fatal("an empty target list has no text")
	}
}

func TestTargetLinesNameOnlyConfiguredTargets(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", strings.Replace(shareSpec,
		"Targets: ios\n", "Targets: ios, watch, ios\n", 1))
	parsed := parseTargetedScope(t, root, changeScope("example"), map[string]string{"ios": "ios/**", "android": "a/**"})
	unknown := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGET_UNKNOWN")
	malformed := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_MALFORMED")
	if len(unknown) != 1 || !strings.Contains(unknown[0].Message, "watch") || unknown[0].Source.Line != 17 ||
		len(malformed) != 1 || !strings.Contains(malformed[0].Message, `"ios" twice`) {
		t.Fatalf("diagnostics = %#v", parsed.Diagnostics)
	}
	if scenario := parsed.Requirements[0].Scenarios[1]; !slices.Equal(scenario.Targets, []string{"ios"}) {
		t.Fatalf("the narrowed scenario applies to %v", scenario.Targets)
	}
}

func TestRenamedRequirementsCoverATargetChange(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/share/spec.md", `<!-- stele: spec v1; targets: ios, android -->
### Requirement: Share a list
Verification-ID: req.share.111111111111
Targets: android
#### Scenario: Share
Verification-ID: scn.share.222222222222
- **WHEN** shared
- **THEN** text
`)
	delta := `<!-- stele: spec v1; targets: ios -->
## RENAMED Requirements
- FROM: ` + "`### Requirement: Share a list`" + `
- TO: ` + "`### Requirement: Share a whole list`" + `
`
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", delta)
	paths := map[string]string{"ios": "ios/**", "android": "android/**"}
	parsed := parseTargetedScope(t, root, changeScope("example"), paths)
	if uncovered := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_CHANGE_UNCOVERED"); len(uncovered) != 0 ||
		len(parsed.Renamed) != 1 || parsed.Renamed[0].Name != "Share a list" {
		t.Fatalf("a renamed requirement was not listed: %#v, %#v", uncovered, parsed.Renamed)
	}
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", strings.Replace(delta,
		"## RENAMED", "## OTHER", 1))
	parsed = parseTargetedScope(t, root, changeScope("example"), paths)
	if uncovered := diagnosticsWithCode(parsed.Diagnostics, "SPEC_TARGETS_CHANGE_UNCOVERED"); len(uncovered) != 1 ||
		!strings.Contains(uncovered[0].Message, "from android to no targets") {
		t.Fatalf("SPEC_TARGETS_CHANGE_UNCOVERED = %#v", uncovered)
	}
}

func TestTargetSelectionLeavesOutOtherTargets(t *testing.T) {
	root := completeTargetedFixture(t)
	writeFixture(t, root, "openspec/changes/example/specs/share/spec.md", shareSpec+`
### Requirement: Print a list
Verification-ID: req.share.dddddddddddd
Targets: ios

#### Scenario: Print on iOS
Verification-ID: scn.share.eeeeeeeeeeee

- **WHEN** the user prints
- **THEN** a page comes out
`)
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{SchemaVersion: evidenceSchemaVersion, InputDigest: digest, Executions: []TestExecution{
		execution("android/share.test.mts", "android share text", shareTextID+".android.unit", "passed", digest),
		execution("ios/share.test.mts", "ios share text", shareTextID+".ios.unit", "failed", "older"),
	}}
	report, err := verifyScope(verifyRequest{
		root: root, scope: configuredScope(t, root, changeScope("example"), "android"), mode: "implementation",
		evidence: &evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Requirements) != 1 || len(report.Requirements[0].Scenarios) != 1 ||
		report.Requirements[0].Scenarios[0].Execution.Outcome != "passed" || report.Verdict != "pass" {
		t.Fatalf("--target android report = %#v", report.Requirements)
	}
	iosInput := func() matrixInput {
		scenario := Scenario{ID: shareTextID, Targeted: true, Targets: []string{"ios"}}
		return matrixInput{
			parsed: ParsedSpecs{
				Requirements: []Requirement{{Scenarios: []Scenario{scenario}}},
				Annotations:  []SpecAnnotation{{TargetsDeclared: true, Targets: []string{"ios"}}},
			},
			plan: LinkagePlan{Evidence: map[string][]EvidenceEntry{
				shareTextID: {targetedEntry(shareTextID, "ios", "unit")},
			}},
			anchors: []Anchor{{
				ID: shareTextID, Kind: "test", Path: "ios/share.test.mts", EvidenceID: shareTextID + ".ios.unit",
				Target: "ios", Selector: stringPointer("ios share text"),
			}},
			executions: indexExecutions(&evidence), inputDigest: digest,
			targets: testTargets(t, map[string]string{"ios": "ios/**"}),
		}
	}
	if cell := cellOf(t, matrixRow(t, buildTargetMatrix(iosInput()), shareTextID), "ios"); cell.State != "failed" {
		t.Fatalf("a failed stale cell = %#v", cell)
	}
	evidence.Executions[1].Outcome = "passed"
	if cell := cellOf(t, matrixRow(t, buildTargetMatrix(iosInput()), shareTextID), "ios"); cell.State != "unapproved" {
		t.Fatalf("an unapproved stale cell = %#v", cell)
	}
	stale := iosInput()
	entry := stale.plan.Evidence[shareTextID][0]
	if state := entryExecution(stale, Scenario{ID: shareTextID}, entry); state != "stale" {
		t.Fatalf("a stale entry = %s", state)
	}
	index := runIndex(t, "--change", "example", "--root", root, "--target", "android")
	for _, requirement := range index.Requirements {
		if requirement.ID == "req.share.dddddddddddd" {
			t.Fatal("a requirement narrowed to ios was indexed for android")
		}
	}
}

func TestTargetsFromKeepsAndRemovesTargets(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/kept/spec.md", "<!-- stele: spec v1; targets: ios -->\n# kept\n")
	writeFixture(t, root, "openspec/specs/removed/spec.md", "<!-- stele: spec v1; targets: ios -->\n# removed\n")
	writeFixture(t, root, "openspec/specs/plain/spec.md", "# plain\n")
	writeFixture(t, root, "archive/specs/removed/spec.md", "<!-- stele: spec v1 -->\n## ADDED Requirements\n")
	if code, stdout, stderr := runAnnotate(t, "--root", root, "--specs", "--targets-from", "archive"); code != 0 {
		t.Fatalf("annotate = %d, %q, %q", code, stdout, stderr)
	}
	for file, want := range map[string]string{
		"kept":    "<!-- stele: spec v1; targets: ios -->\n# kept\n",
		"removed": annotationCanonical + "\n# removed\n",
		"plain":   annotationCanonical + "\n# plain\n",
	} {
		if content := readTestFile(t, root, "openspec/specs/"+file+"/spec.md"); content != want {
			t.Fatalf("%s = %q", file, content)
		}
	}
}
