package stele

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func runIndex(t *testing.T, arguments ...string) Index {
	t.Helper()
	code, stdout, stderr := runCommand(t, append([]string{"index"}, arguments...)...)
	if code != 0 {
		t.Fatalf("index %v = %d, %q", arguments, code, stderr)
	}
	var index Index
	if err := json.Unmarshal([]byte(stdout), &index); err != nil {
		t.Fatalf("index output is not JSON: %v, %q", err, stdout)
	}
	return index
}

func indexedScenario(t *testing.T, index Index, scope, id string) IndexScenario {
	t.Helper()
	for _, scenario := range index.Scenarios {
		if scenario.Scope == scope && scenario.ID == id {
			return scenario
		}
	}
	t.Fatalf("scenario %s of scope %s not indexed: %#v", id, scope, index.Scenarios)
	return IndexScenario{}
}

func indexedAnchors(index Index, id string) []IndexAnchor {
	found := make([]IndexAnchor, 0)
	for _, anchor := range index.Anchors {
		if anchor.ID == id {
			found = append(found, anchor)
		}
	}
	return found
}

// linkedChangeFixture returns change "example" with an approved v2 plan, an
// implementation anchor, and unit and e2e tests for scenario bbbb.
func linkedChangeFixture(t *testing.T) string {
	t.Helper()
	root := evidenceFixture(t)
	writeEvidencePlan(t, root, map[string][]EvidenceEntry{
		evidenceScenarioID: {
			approved(t, root, entry(evidenceScenarioID, "unit", "Pure logic.")),
			approved(t, root, entry(evidenceScenarioID, "e2e", "The user journey.")),
		},
		otherScenarioID: {entry(otherScenarioID, "unit", "Pure logic.")},
	})
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/e2e/value.test.mts", evidenceScenarioID+".e2e", "shows the value")
	return root
}

// @verifies scn.linkindex.30ab15bc8293.unit
func TestIndexListsLinksForAChange(t *testing.T) {
	root := linkedChangeFixture(t)
	index := runIndex(t, "--root", root, "--change", "example", "--json")
	if index.SchemaVersion != 1 || !slices.Equal(index.Scopes, []IndexScope{{ID: "example", Kind: "change"}}) {
		t.Fatalf("scopes = %#v", index.Scopes)
	}
	requirement := index.Requirements[0]
	if requirement.ID != "req.demo.aaaaaaaaaaaa" || requirement.Scope != "example" ||
		requirement.Text != "The system SHALL return the stored value." ||
		!slices.Equal(requirement.Scenarios, []string{evidenceScenarioID, otherScenarioID}) ||
		len(requirement.Implementations) != 1 || requirement.Implementations[0].Path != "src/demo.mts" ||
		pointerValue(requirement.Implementations[0].Selector) != "value" {
		t.Fatalf("requirement = %#v", requirement)
	}
	scenario := indexedScenario(t, index, "example", evidenceScenarioID)
	if scenario.Requirement != requirement.ID ||
		scenario.Text != "- **WHEN** a value is stored\n- **THEN** it is returned" ||
		len(scenario.Steps) != 2 || scenario.Steps[1] != (ScenarioStep{Keyword: "THEN", Text: "it is returned"}) ||
		scenario.Source.Line != 8 {
		t.Fatalf("scenario = %#v", scenario)
	}
	if len(scenario.Evidence) != 2 {
		t.Fatalf("evidence = %#v", scenario.Evidence)
	}
	for _, want := range []struct{ level, path, selector string }{
		{"unit", "tests/value.test.mts", "returns the value"},
		{"e2e", "tests/e2e/value.test.mts", "shows the value"},
	} {
		index := slices.IndexFunc(scenario.Evidence, func(item IndexEvidence) bool { return item.Level == want.level })
		item := scenario.Evidence[index]
		if item.ID != evidenceScenarioID+"."+want.level || item.Approval != "approved" || len(item.Tests) != 1 ||
			item.Tests[0].Path != want.path || pointerValue(item.Tests[0].Selector) != want.selector ||
			item.Execution != (ExecutionState{State: "not-run", Outcome: "not-run"}) {
			t.Fatalf("%s evidence = %#v", want.level, item)
		}
	}
	if other := indexedScenario(t, index, "example", otherScenarioID); other.Evidence[0].Approval != "unapproved" {
		t.Fatalf("unapproved evidence = %#v", other.Evidence)
	}
	anchors := indexedAnchors(index, evidenceScenarioID)
	if len(anchors) != 2 || anchors[0].Path != "tests/e2e/value.test.mts" || anchors[0].Level != "e2e" ||
		anchors[0].EvidenceID != evidenceScenarioID+".e2e" || anchors[0].Status != "linked" ||
		anchors[1].Level != "unit" {
		t.Fatalf("test anchors = %#v", anchors)
	}
	if code := indexedAnchors(index, "req.demo.aaaaaaaaaaaa"); len(code) != 1 || code[0].Kind != "code" ||
		code[0].Annotation != "implements" || code[0].Scope != "example" {
		t.Fatalf("code anchors = %#v", code)
	}

	recordTests(t, "shows the value")
	runTargets(t, root, evidenceScenarioID)
	tested := indexedScenario(t, runIndex(t, "--root", root, "--change", "example"), "example", evidenceScenarioID)
	for _, item := range tested.Evidence {
		want := ExecutionState{State: "executed", Outcome: choose(item.Level == "unit", "passed", "failed")}
		if item.Execution != want {
			t.Fatalf("executed %s evidence = %#v", item.Level, item.Execution)
		}
	}
}

// @verifies scn.linkindex.48b3c24773d9.unit
func TestIndexIncludesCurrentSpecifications(t *testing.T) {
	root := archivedFixture(t)
	writeFixture(t, root, "openspec/changes/rework/specs/demo/spec.md", `## MODIFIED Requirements
### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa

The system SHALL return the value twice.

#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	index := runIndex(t, "--root", root, "--change", "rework")
	if !slices.Equal(index.Scopes, []IndexScope{{ID: "rework", Kind: "change"}, {ID: "specs", Kind: "specs"}}) {
		t.Fatalf("scopes = %#v", index.Scopes)
	}
	if len(index.Requirements) != 2 || index.Requirements[0].Scope != "rework" ||
		index.Requirements[0].Text != "The system SHALL return the value twice." ||
		index.Requirements[1].Scope != "specs" || index.Requirements[1].ID != index.Requirements[0].ID {
		t.Fatalf("requirements = %#v", index.Requirements)
	}
	current := indexedScenario(t, index, "specs", "scn.demo.bbbbbbbbbbbb")
	if current.Source.Path != "openspec/specs/demo/spec.md" || current.Evidence[0].Approval != "unplanned" {
		t.Fatalf("current scenario = %#v", current)
	}
	scopes := make([]string, 0)
	for _, anchor := range indexedAnchors(index, "scn.demo.bbbbbbbbbbbb") {
		scopes = append(scopes, anchor.Scope)
	}
	if !slices.Equal(scopes, []string{"rework", "specs"}) {
		t.Fatalf("anchor scopes = %v", scopes)
	}

	specsOnly := runIndex(t, "--root", root, "--specs")
	if !slices.Equal(specsOnly.Scopes, []IndexScope{{ID: "specs", Kind: "specs"}}) {
		t.Fatalf("--specs scopes = %#v", specsOnly.Scopes)
	}
	without := linkedChangeFixture(t)
	if scopes := runIndex(t, "--root", without, "--change", "example").Scopes; len(scopes) != 1 {
		t.Fatalf("a project without current specifications = %#v", scopes)
	}
}

// @verifies scn.linkindex.331076ad0227.unit
func TestIndexMarksStaleOutcomes(t *testing.T) {
	scenario := Scenario{ID: "scn.demo.bbbbbbbbbbbb", Title: "Works"}
	parsed := ParsedSpecs{Requirements: []Requirement{{ID: "req.demo.aaaaaaaaaaaa", Scenarios: []Scenario{scenario}}}}
	first, second := "first", "second"
	anchors := []Anchor{
		{
			ID: scenario.ID, Annotation: "verifies", Kind: "test", Path: "a.test.ts", Selector: &first,
			EvidenceID: scenario.ID + ".unit", Level: "unit",
		},
		{
			ID: scenario.ID, Annotation: "verifies", Kind: "test", Path: "b.test.ts", Selector: &second,
			EvidenceID: scenario.ID + ".e2e", Level: "e2e",
		},
	}
	plan := emptyLinkagePlan()
	plan.Scenarios[scenario.ID] = "a.test.ts#first"
	evidence := &Evidence{InputDigest: "old", Executions: []TestExecution{
		{Path: "a.test.ts", Selector: &first, Outcome: "passed"},
		{Path: "b.test.ts", Selector: &second, Outcome: "failed", InputDigest: "current"},
	}}
	scopes := []IndexScopeInput{{Scope: IndexScope{ID: "example", Kind: "change"}, Parsed: parsed, Plan: plan}}
	build := func(digest string) []IndexEvidence {
		index := BuildIndex(IndexInput{
			Scopes:      scopes,
			Anchors:     anchors,
			Declared:    map[string]bool{},
			Evidence:    evidence,
			InputDigest: digest,
		})
		return index.Scenarios[0].Evidence
	}
	items := build("current")
	if items[0].ID != scenario.ID+".e2e" || items[0].Approval != "v1" || items[0].Target != "a.test.ts#first" ||
		items[0].Execution != (ExecutionState{State: "executed", Outcome: "failed"}) {
		t.Fatalf("current e2e = %#v", items[0])
	}
	if items[1].ID != scenario.ID+".unit" || items[1].Execution != (ExecutionState{State: "stale", Outcome: "passed"}) {
		t.Fatalf("stale unit = %#v", items[1])
	}
	if items := build("newer"); items[0].Execution != (ExecutionState{State: "stale", Outcome: "failed"}) {
		t.Fatalf("every outcome is stale after a change: %#v", items)
	}

	// A v1 target without anchors still lists its evidence as not run.
	bare := BuildIndex(IndexInput{Scopes: []IndexScopeInput{{
		Scope: IndexScope{ID: "example", Kind: "change"}, Parsed: parsed, Plan: plan,
	}}})
	if items := bare.Scenarios[0].Evidence; len(items) != 1 || items[0].ID != scenario.ID ||
		items[0].Execution.State != "not-run" {
		t.Fatalf("planned evidence without anchors = %#v", items)
	}
	partial := &Evidence{Executions: []TestExecution{{Path: "a.test.ts", Selector: &first, Outcome: "passed"}}}
	unrun := BuildIndex(IndexInput{
		Scopes: scopes,
		Anchors: []Anchor{anchors[0], {
			ID: scenario.ID, Kind: "test", Path: "c.test.ts", Selector: &first, EvidenceID: scenario.ID + ".unit",
		}},
		Evidence: partial,
	})
	if state := unrun.Scenarios[0].Evidence[0].Execution; state.State != "not-run" {
		t.Fatalf("evidence with a test that never ran = %#v", state)
	}
}

// @verifies scn.linkindex.560aeb5e3970.unit
func TestIndexFlagsUndeclaredAnchors(t *testing.T) {
	root := linkedChangeFixture(t)
	writeFixture(t, root, "openspec/changes/other/specs/other/spec.md", `### Requirement: Other
Verification-ID: req.other.111111111111
#### Scenario: Other
Verification-ID: scn.other.222222222222
`)
	writeFixture(t, root, "src/typo.mts", "// @implements "+"req.demo.999999999999\nexport function typo() {}\n"+
		"// @implements "+"req.other.111111111111\nexport function other() {}\n")
	writeFixture(t, root, "tests/loose.test.mts", "// @verifies "+otherScenarioID+"\nconst loose = 1;\n")
	index := runIndex(t, "--root", root, "--change", "example")
	for id, status := range map[string]string{
		"req.demo.999999999999":  "undeclared",
		"req.other.111111111111": "other-scope",
		otherScenarioID:          "unresolved",
	} {
		anchors := indexedAnchors(index, id)
		if len(anchors) != 1 || anchors[0].Status != status {
			t.Fatalf("%s anchors = %#v, want %s", id, anchors, status)
		}
	}
	last := index.Anchors[len(index.Anchors)-2:]
	if last[0].Scope != "" || last[0].ID != "req.demo.999999999999" || last[1].Scope != "" {
		t.Fatalf("anchors outside the scopes are not listed last: %#v", index.Anchors)
	}
}

// @verifies scn.linkindex.bda4420352ef.unit
func TestIndexCoversEveryScope(t *testing.T) {
	root := archivedFixture(t)
	for _, change := range []string{"alpha", "beta"} {
		writeFixture(t, root, "openspec/changes/"+change+"/specs/"+change+"/spec.md",
			"### Requirement: "+change+"\nVerification-ID: req."+change+".111111111111\n"+
				"#### Scenario: "+change+"\nVerification-ID: scn."+change+".222222222222\n")
	}
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	index := runIndex(t, "--root", root, "--all")
	want := []IndexScope{{ID: "specs", Kind: "specs"}, {ID: "alpha", Kind: "change"}, {ID: "beta", Kind: "change"}}
	if !slices.Equal(index.Scopes, want) {
		t.Fatalf("scopes = %#v", index.Scopes)
	}
	scopes := make([]string, 0)
	for _, requirement := range index.Requirements {
		scopes = append(scopes, requirement.Scope+":"+requirement.ID)
	}
	if !slices.Equal(scopes, []string{
		"specs:req.demo.aaaaaaaaaaaa", "alpha:req.alpha.111111111111",
		"beta:req.beta.111111111111",
	}) {
		t.Fatalf("requirements = %v", scopes)
	}

	output := filepath.Join(root, "out", "index.json")
	if code, stdout, stderr := runCommand(t, "index", "--root", root, "--all", "--output-file", output); code != 0 ||
		stdout != "" {
		t.Fatalf("index --output-file = %d, %q, %q", code, stdout, stderr)
	}
	_, printed, _ := runCommand(t, "index", "--root", root, "--all")
	if content, err := os.ReadFile(output); err != nil || string(content) != printed {
		t.Fatalf("the written index differs from the printed one: %v", err)
	}
}

func TestIndexReportsFailures(t *testing.T) {
	empty := fixtureRoot(t)
	if code, _, stderr := runCommand(t, "index", "--root", empty, "--all"); code != 2 ||
		!strings.Contains(stderr, "no current specifications and no active changes") {
		t.Fatalf("index --all without scopes = %d, %q", code, stderr)
	}
	if code, _, _ := runCommand(t, "index", "--root", empty, "--change", "missing"); code != 2 {
		t.Fatalf("index of a missing change = %d", code)
	}
	root := linkedChangeFixture(t)
	writeFixture(t, root, "blocked", "file")
	if code, _, stderr := runCommand(t, "index", "--root", root, "--change", "example",
		"--output-file", "blocked/index.json"); code != 2 || !strings.Contains(stderr, "writing the index") {
		t.Fatalf("index write failure = %d, %q", code, stderr)
	}
	writeFixture(t, root, "openspec/changes/example/specs/broken/spec.md", string(make([]byte, 70_000)))
	if code, _, _ := runCommand(t, "index", "--root", root, "--change", "example"); code != 2 {
		t.Fatalf("index of an unreadable spec = %d", code)
	}
	if code, _, _ := runCommand(t, "index", "--root", root, "--change", "example", "extra"); code != 2 {
		t.Fatalf("index accepted a positional argument: %d", code)
	}

	for name, breakInput := range map[string]func(){
		"spec": func() {
			writeFixture(t, root, "openspec/changes/example/specs/broken/spec.md", string(make([]byte, 70_000)))
		},
		"anchor": func() {
			link := filepath.Join(root, "src", "broken.ts")
			if err := os.Symlink(filepath.Join(root, "missing.ts"), link); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root = linkedChangeFixture(t)
			breakInput()
			if _, err := loadIndex(root, []verificationScope{changeScope("example")}); err == nil {
				t.Fatal("expected an index error")
			}
		})
	}

	root = linkedChangeFixture(t)
	calls := make([]string, 0)
	failing := failingDeclarations{memoryBackend{calls: &calls}}
	if _, err := loadIndex(root, []verificationScope{{changeID: "example", backend: failing}}); err == nil {
		t.Fatal("expected a declaration error")
	}
	original := relativePath
	t.Cleanup(func() { relativePath = original })
	relativePath = func(base, target string) (string, error) {
		if strings.HasSuffix(target, "package.json") {
			return "", errors.New("relative failed")
		}
		return original(base, target)
	}
	if _, err := loadIndex(root, []verificationScope{changeScope("example")}); err == nil {
		t.Fatal("expected a digest error")
	}
}

// failingDeclarations is a backend whose declared identities cannot be read.
type failingDeclarations struct {
	memoryBackend
}

func (failingDeclarations) DeclaredIdentities(string) (map[string]bool, error) {
	return nil, errors.New("declarations failed")
}
