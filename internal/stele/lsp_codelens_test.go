package stele

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// lensesOf returns a file's CodeLens items as line: title (command).
func lensesOf(t *testing.T, client *lspTestClient, root, relative string) []string {
	t.Helper()
	var lenses []lspCodeLens
	client.result(client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, relative))},
	}), &lenses)
	found := make([]string, 0, len(lenses))
	for _, lens := range lenses {
		found = append(found, fmt.Sprintf("%d: %s (%s)", lens.Range.Start.Line+1, lens.Command.Title,
			lens.Command.Command))
	}
	return found
}

// withCommand keeps the lenses of one command.
func withCommand(lenses []string, command string) []string {
	kept := make([]string, 0)
	for _, lens := range lenses {
		if strings.HasSuffix(lens, "("+command+")") {
			kept = append(kept, lens)
		}
	}
	return kept
}

// @verifies scn.languageserver.28e44f61a51f.unit
func TestLSPSummarizesTheLinkedScenarioAboveATest(t *testing.T) {
	root := lspFixture(t)
	long := "a value that contains “quoted” text and émojis 😀 is stored after the user has typed it " +
		"into the form, pressed save, and waited"
	writeFixture(t, root, lspChangeSpec, strings.Replace(evidenceSpec,
		"- **WHEN** a value is stored\n- **THEN** it is returned\n",
		"- **WHEN** "+long+"\n- **THEN** it is returned\n- **AND** the log is empty\n", 1))
	client := openLSP(t, root, lspCapabilities())
	lenses := lensesOf(t, client, root, "tests/value.test.mts")
	full := "Scenario: Value is returned — WHEN " + long + " · THEN it is returned"
	want := string([]rune(full)[:lspLensLength-1]) + "…"
	if len(lenses) != 2 || lenses[0] != "2: "+want+" (stele.showSpecification)" {
		t.Fatalf("lenses = %q, want %q", lenses, want)
	}
	if utf8.RuneCountInString(want) != lspLensLength || strings.Contains(want, "AND") {
		t.Fatalf("summary = %q", want)
	}
	short := lensesOf(t, client, root, "src/demo.mts")
	if len(short) != 1 || short[0] != "1: Requirement: Return value — The system SHALL return the stored value. "+
		"(stele.showSpecification)" {
		t.Fatalf("requirement summary = %q", short)
	}
}

// @verifies scn.languageserver.53ed0e506ce6.unit
func TestLSPShowsTheSpecificationFromASummaryLens(t *testing.T) {
	root := lspFixture(t)
	arguments := []map[string]string{{"root": fileURI(root), "id": evidenceScenarioID}}
	showing := openLSP(t, root, lspCapabilities())
	response := showing.request("workspace/executeCommand", map[string]any{
		"command": "stele.showSpecification", "arguments": arguments,
	})
	if response.Error != nil || string(response.Result) != "null" {
		t.Fatalf("showSpecification = %#v", response)
	}
	shown := showing.take("window/showDocument")
	if len(shown) != 1 {
		t.Fatalf("showDocument requests = %#v", showing.received)
	}
	var document struct {
		URI       string   `json:"uri"`
		TakeFocus bool     `json:"takeFocus"`
		Selection lspRange `json:"selection"`
	}
	if err := json.Unmarshal(shown[0].Params, &document); err != nil || !document.TakeFocus ||
		document.URI != fileURI(filepath.Join(root, lspChangeSpec)) || document.Selection.Start.Line != 8 {
		t.Fatalf("showDocument = %s", shown[0].Params)
	}
	messaging := openLSP(t, root, map[string]any{})
	messaging.request("workspace/executeCommand", map[string]any{
		"command": "stele.showSpecification", "arguments": arguments,
	})
	messages := messaging.take("window/showMessage")
	if len(messages) != 1 || len(messaging.take("window/showDocument")) != 0 {
		t.Fatalf("messages = %#v", messaging.received)
	}
	var message struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(messages[0].Params, &message); err != nil || message.Type != 3 ||
		!strings.Contains(message.Message, "Scenario: Value is returned · example\n\nWHEN a value is stored") ||
		strings.Contains(message.Message, "**") {
		t.Fatalf("showMessage = %s", messages[0].Params)
	}
}

func TestLSPStatusLenses(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, defaultEvidencePath, `{"schemaVersion":3,"runner":"node","testedRevision":"r",`+
		`"inputDigest":"old","outcome":"failed","scenarios":[],"executions":[`+
		`{"path":"tests/value.test.mts","selector":"returns the value","scenarioIds":["scn.demo.bbbbbbbbbbbb"],`+
		`"outcome":"passed","reason":null,"inputDigest":"old"},`+
		`{"path":"tests/e2e/value.test.mts","selector":"shows the value","scenarioIds":["scn.demo.bbbbbbbbbbbb"],`+
		`"outcome":"failed","reason":null,"inputDigest":"old"}]}`+"\n")
	client := openLSP(t, root, lspCapabilities())
	lenses := withCommand(lensesOf(t, client, root, lspChangeSpec), "stele.showStatus")
	want := []string{
		"4: ✗ failed · 1 failed · 1 not run · 1 unapproved (stele.showStatus)",
		"9: unit ✓ · e2e ✗ · stale (stele.showStatus)",
		"15: unit – not run (unapproved) (stele.showStatus)",
	}
	if strings.Join(lenses, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lenses:\n%s", strings.Join(lenses, "\n"))
	}
	response := client.request("workspace/executeCommand", map[string]any{
		"command":   "stele.showStatus",
		"arguments": []map[string]string{{"root": fileURI(root), "id": evidenceScenarioID}},
	})
	messages := client.take("window/showMessage")
	if response.Error != nil || len(messages) != 1 ||
		!strings.Contains(string(messages[0].Params), "e2e · approved · failed (stale) — tests/e2e/value.test.mts:2") {
		t.Fatalf("showStatus = %#v %#v", response, messages)
	}
}

func TestLSPCombinedStatus(t *testing.T) {
	scenario := func(requirement, outcome, state string) IndexScenario {
		return IndexScenario{ID: "scn." + outcome, Scope: "specs", Requirement: requirement, Evidence: []IndexEvidence{
			{Level: "unit", Approval: "approved", Execution: ExecutionState{State: state, Outcome: outcome}},
		}}
	}
	index := Index{Scenarios: []IndexScenario{
		scenario("req.a", "passed", "executed"), scenario("req.a", "not-run", "not-run"),
		scenario("req.b", "passed", "executed"), scenario("req.c", "passed", "stale"),
		{ID: "scn.none", Scope: "specs", Requirement: "req.d"},
		{ID: "scn.v1", Scope: "specs", Requirement: "req.e", Evidence: []IndexEvidence{
			{Approval: "v1", Execution: ExecutionState{State: "executed", Outcome: "passed"}},
		}},
	}}
	for _, want := range []struct{ requirement, status string }{
		{"req.a", "not run · 1 passed · 1 not run"},
		{"req.b", "✓ passed · 1 passed"},
		{"req.c", "✓ stale · 1 stale"},
		{"req.d", ""},
	} {
		status, _ := requirementStatus(index, IndexRequirement{ID: want.requirement, Scope: "specs"})
		if status != want.status {
			t.Fatalf("%s status = %q, want %q", want.requirement, status, want.status)
		}
	}
	unplanned := Index{Requirements: []IndexRequirement{{ID: "req.x", Source: Source{Path: "a.md", Line: 1}}}}
	if lenses := lspLenses(unplanned, "a.md"); len(lenses) != 0 {
		t.Fatalf("lenses of a requirement without evidence = %#v", lenses)
	}
	if status := scenarioStatus(index.Scenarios[5]); status != "tests ✓" {
		t.Fatalf("v1 status = %q", status)
	}
	if summaryTitle(Index{Requirements: []IndexRequirement{{ID: "req.x", Title: "Empty"}}}, "req.x") !=
		"Requirement: Empty" || stepSummary(nil) != "" {
		t.Fatal("summaries without text")
	}
}

func TestLSPCommandErrors(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	for _, params := range []any{
		"invalid",
		map[string]any{"command": "stele.showStatus", "arguments": []any{}},
		map[string]any{"command": "stele.other", "arguments": []map[string]string{{"root": fileURI(root)}}},
		map[string]any{"command": "stele.showStatus", "arguments": []map[string]string{{"root": fileURI(t.TempDir())}}},
		map[string]any{"command": "stele.showStatus", "arguments": []map[string]string{
			{"root": fileURI(root), "id": "req.x"},
		}},
		map[string]any{"command": "stele.showSpecification", "arguments": []map[string]string{
			{"root": fileURI(root), "id": "req.x"},
		}},
	} {
		if response := client.request("workspace/executeCommand", params); response.Error == nil {
			t.Fatalf("executeCommand %v = %#v", params, response)
		}
	}
	if response := client.request("textDocument/codeLens", "invalid"); response.Error == nil {
		t.Fatalf("codeLens with invalid params = %#v", response)
	}
	if lenses := lensesOf(t, client, root, "elsewhere.mts"); len(lenses) != 0 {
		t.Fatalf("lenses of an unknown file = %q", lenses)
	}
}

func TestLSPRequirementStatusMessage(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, map[string]any{})
	client.request("workspace/executeCommand", map[string]any{
		"command":   "stele.showStatus",
		"arguments": []map[string]string{{"root": fileURI(root), "id": "req.demo.aaaaaaaaaaaa"}},
	})
	messages := client.take("window/showMessage")
	if len(messages) != 1 || !strings.Contains(string(messages[0].Params), "Implementations") {
		t.Fatalf("requirement status = %#v", messages)
	}
	response := client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": "untitled:1"},
	})
	if string(response.Result) != "[]" {
		t.Fatalf("lenses of an untitled document = %s", response.Result)
	}
}

// runLenses returns a file's run lenses as line: title → targets @ scope.
func runLenses(t *testing.T, client *lspTestClient, root, relative string) []string {
	t.Helper()
	var lenses []struct {
		Range   lspRange `json:"range"`
		Command struct {
			Title     string                `json:"title"`
			Command   string                `json:"command"`
			Arguments []lspCommandArguments `json:"arguments"`
		} `json:"command"`
	}
	client.result(client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, relative))},
	}), &lenses)
	found := make([]string, 0)
	for _, lens := range lenses {
		if lens.Command.Command != "stele.runTests" {
			continue
		}
		argument := lens.Command.Arguments[0]
		if argument.Root != fileURI(root) {
			t.Fatalf("run lens root = %q", argument.Root)
		}
		found = append(found, fmt.Sprintf("%d: %s → %s @ %s", lens.Range.Start.Line+1, lens.Command.Title,
			strings.Join(argument.Targets, ","), argument.Scope))
	}
	return found
}

// currentEvidence stores an outcome for each of the fixture's two tests of
// scenario bbbb, recorded with the current inputs.
func currentEvidence(t *testing.T, root, unit, e2e string) {
	t.Helper()
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, defaultEvidencePath, `{"schemaVersion":3,"runner":"node","testedRevision":"r",`+
		`"inputDigest":"`+digest+`","outcome":"failed","scenarios":[],"executions":[`+
		`{"path":"tests/value.test.mts","selector":"returns the value","scenarioIds":["scn.demo.bbbbbbbbbbbb"],`+
		`"outcome":"`+unit+`","reason":null,"inputDigest":"`+digest+`"},`+
		`{"path":"tests/e2e/value.test.mts","selector":"shows the value","scenarioIds":["scn.demo.bbbbbbbbbbbb"],`+
		`"outcome":"`+e2e+`","reason":null,"inputDigest":"`+digest+`"}]}`+"\n")
}

// @verifies scn.languageserver.f584d3ec6c44.unit
func TestLSPShowsRunActionsAndStatusOnAScenario(t *testing.T) {
	root := lspFixture(t)
	currentEvidence(t, root, "passed", "failed")
	client := openLSP(t, root, lspCapabilities())
	id := evidenceScenarioID
	if lenses := runLenses(t, client, root, lspChangeSpec); !slices.Equal(lenses[3:6], []string{
		"9: ▶ Run all → " + id + " @ example",
		"9: ▶ Run unit → " + id + ".unit @ example",
		"9: ▶ Run e2e → " + id + ".e2e @ example",
	}) {
		t.Fatalf("scenario run lenses = %q", lenses)
	}
	all := lensesOf(t, client, root, lspChangeSpec)
	index := slices.Index(all, "9: ▶ Run all (stele.runTests)")
	if index < 0 || !slices.Equal(all[index:index+4], []string{
		"9: ▶ Run all (stele.runTests)", "9: ▶ Run unit (stele.runTests)", "9: ▶ Run e2e (stele.runTests)",
		"9: unit ✓ · e2e ✗ (stele.showStatus)",
	}) {
		t.Fatalf("scenario lenses = %q", all)
	}
}

// @verifies scn.languageserver.f1cf94d0c1d6.unit
func TestLSPSummarizesARequirement(t *testing.T) {
	root := lspFixture(t)
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	currentEvidence(t, root, "passed", "passed")
	client := openLSP(t, root, lspCapabilities())
	lenses := runLenses(t, client, root, lspChangeSpec)
	if len(lenses) < 1 || lenses[0] != "4: ▶ Run all → req.demo.aaaaaaaaaaaa @ example" {
		t.Fatalf("requirement run lenses = %q", lenses)
	}
	status := withCommand(lensesOf(t, client, root, lspChangeSpec), "stele.showStatus")
	if len(status) < 1 || status[0] != "4: not run · 1 passed · 1 not run · 1 unapproved (stele.showStatus)" {
		t.Fatalf("requirement status = %q", status)
	}
}

// @verifies scn.languageserver.cdca0ea9f34c.unit
func TestLSPRunsOneEvidenceEntryFromATest(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/bare.test.mts", "import test from \"node:test\";\n// @verifies "+
		"scn.demo.cccccccccccc\nvoid test(\"reports\", () => {});\n")
	client := openLSP(t, root, lspCapabilities())
	if lenses := runLenses(t, client, root, "tests/value.test.mts"); !slices.Equal(lenses, []string{
		"2: ▶ Run → " + evidenceScenarioID + ".unit @ example",
	}) {
		t.Fatalf("test anchor run lenses = %q", lenses)
	}
	if lenses := runLenses(t, client, root, "src/demo.mts"); len(lenses) != 0 {
		t.Fatalf("code anchor run lenses = %q", lenses)
	}
	if lenses := runLenses(t, client, root, "tests/bare.test.mts"); !slices.Equal(lenses, []string{
		"2: ▶ Run → scn.demo.cccccccccccc @ example",
	}) {
		t.Fatalf("bare anchor run lenses = %q", lenses)
	}
}
