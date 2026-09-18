package stele

import (
	"encoding/json"
	"strings"
	"testing"
)

// lspCurrentSpec declares requirement aaaa and scenario bbbb in the current
// specifications.
const lspCurrentSpec = `<!-- stele: spec v1 -->
# Demo

## Requirements

### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa

The system SHALL return the value that was stored last.

#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb

- **WHEN** a value is stored
- **THEN** it is returned
`

// lspSpecsFixture returns current specifications with an implementation
// anchor on line 1 of src/demo.mts.
func lspSpecsFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/demo/spec.md", lspCurrentSpec)
	writeFixture(t, root, "src/demo.mts",
		"// @implements "+"req.demo.aaaaaaaaaaaa\nexport function value() { return 1; }\n")
	writeFixture(t, root, "package.json", "{\"type\":\"module\"}\n")
	return root
}

// lspStoredEvidence writes an evidence file with one execution of the unit
// test of the fixture, recorded with other inputs.
func lspStoredEvidence(t *testing.T, root, outcome, digest string) {
	t.Helper()
	writeFixture(t, root, defaultEvidencePath, `{"schemaVersion":3,"runner":"node","testedRevision":"r",`+
		`"inputDigest":"`+digest+`","outcome":"`+outcome+`",`+
		`"scenarios":[{"id":"scn.demo.bbbbbbbbbbbb","outcome":"`+outcome+`"}],`+
		`"executions":[{"path":"tests/value.test.mts","selector":"returns the value",`+
		`"scenarioIds":["scn.demo.bbbbbbbbbbbb"],"evidenceIds":["scn.demo.bbbbbbbbbbbb.unit"],`+
		`"outcome":"`+outcome+`","reason":null,"inputDigest":"`+digest+`"}]}`+"\n")
}

// hoverAt returns the hover at a position, or "" when there is none.
func hoverAt(t *testing.T, client *lspTestClient, root, relative string, line, character int) (string, string) {
	t.Helper()
	response := client.request("textDocument/hover", position(root, relative, line, character))
	var hover *lspHover
	client.result(response, &hover)
	if hover == nil {
		return "", ""
	}
	return hover.Contents.Kind, hover.Contents.Value
}

func requireLines(t *testing.T, text string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if !strings.Contains("\n"+text+"\n", "\n"+line+"\n") {
			t.Fatalf("missing line %q in:\n%s", line, text)
		}
	}
}

// @verifies scn.languageserver.59068d53a952.unit
func TestLSPHoverImplementationAnchor(t *testing.T) {
	root := lspSpecsFixture(t)
	client := openLSP(t, root, lspCapabilities())
	kind, text := hoverAt(t, client, root, "src/demo.mts", 0, 20)
	if kind != "markdown" {
		t.Fatalf("kind = %q", kind)
	}
	requireLines(t, text, "**Requirement:** Return value · `specs`",
		"The system SHALL return the value that was stored last.")
}

// @verifies scn.languageserver.015bb7af18f8.unit
func TestLSPHoverEvidenceAnchor(t *testing.T) {
	root := lspFixture(t)
	lspStoredEvidence(t, root, "passed", "an-older-digest")
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, "tests/value.test.mts", 1, 20)
	requireLines(t, text, "**Scenario:** Value is returned · `example`",
		"- **WHEN** a value is stored", "- **THEN** it is returned",
		"`unit` · approved · passed (stale)")
}

// @verifies scn.languageserver.5c3579497b2b.unit
func TestLSPHoverRendersStepsAsACard(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, lspChangeSpec, strings.Replace(evidenceSpec,
		"- **WHEN** a value is stored\n- **THEN** it is returned\n",
		"- **WHEN** a value is stored\n  and saved twice\n- **THEN** it is returned\n"+
			"- **AND** the log is empty\n- the cache stays warm\n", 1))
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, "tests/value.test.mts", 1, 20)
	requireLines(t, text, "- **WHEN** a value is stored and saved twice\n- **THEN** it is returned\n"+
		"- **AND** the log is empty\n- the cache stays warm")
	_, plain := hoverAt(t, startedPlain(t, root), root, "tests/value.test.mts", 1, 20)
	requireLines(t, plain, "WHEN a value is stored and saved twice\nTHEN it is returned\n"+
		"AND the log is empty\nthe cache stays warm")
}

// startedPlain opens a session whose client declares no hover formats.
func startedPlain(t *testing.T, root string) *lspTestClient {
	t.Helper()
	return openLSP(t, root, map[string]any{})
}

// @verifies scn.languageserver.aa36d54d66ec.unit
func TestLSPHoverScenarioHeading(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/e2e/value.test.mts", "export const nothing = 1;\n")
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, lspChangeSpec, 8, 5)
	requireLines(t, text, "**Scenario:** Value is returned · `example`", "**Tests**",
		"- `unit` · approved · not run — `tests/value.test.mts:2` returns the value",
		"- `e2e` · approved · planned, no test")
	_, fromIdentity := hoverAt(t, client, root, lspChangeSpec, 9, 5)
	if fromIdentity != text {
		t.Fatalf("Verification-ID hover = %q, heading hover = %q", fromIdentity, text)
	}
}

// @verifies scn.languageserver.cd81e42944ee.unit
func TestLSPHoverShowsBothScopes(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/specs/demo/spec.md", lspCurrentSpec)
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, "src/demo.mts", 0, 20)
	requireLines(t, text, "**Requirement:** Return value · `specs`",
		"The system SHALL return the value that was stored last.",
		"**Requirement:** Return value · `example`", "The system SHALL return the stored value.")
	if strings.Index(text, "`specs`") > strings.Index(text, "`example`") {
		t.Fatalf("scopes out of order:\n%s", text)
	}
}

// @verifies scn.languageserver.62861d0e5875.unit
func TestLSPHoverFallsBackToPlainText(t *testing.T) {
	root := lspFixture(t)
	lspStoredEvidence(t, root, "passed", "an-older-digest")
	kind, text := hoverAt(t, startedPlain(t, root), root, "tests/value.test.mts", 1, 20)
	if kind != "plaintext" || strings.Contains(text, "**") || strings.Contains(text, "`") {
		t.Fatalf("plain hover = %q %q", kind, text)
	}
	requireLines(t, text, "Scenario: Value is returned · example", "WHEN a value is stored",
		"THEN it is returned", "unit · approved · passed (stale)")
}

func TestLSPHoverRequirementHeadingAndMisses(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "src/extra.mts", "// @implements "+"req.demo.000000000000\nexport function extra() {}\n"+
		"export const text = \"@implements "+"req.demo.aaaaaaaaaaaa\";\n")
	writeFixture(t, root, "tests/bare.test.mts", "import test from \"node:test\";\n// @verifies "+
		"scn.demo.cccccccccccc\nvoid test(\"reports\", () => {});\n")
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, lspChangeSpec, 3, 5)
	requireLines(t, text, "**Implementations**", "- `src/demo.mts:1` value",
		"- Value is returned: `unit` · approved · not run — `tests/value.test.mts:2` returns the value",
		"- Missing value is reported: `unit` · unapproved · planned, no test")
	_, bare := hoverAt(t, client, root, "tests/bare.test.mts", 1, 20)
	if !strings.Contains(bare, "Missing value is reported") || strings.Contains(bare, "---") {
		t.Fatalf("bare scenario anchor hover = %q", bare)
	}
	for _, miss := range []struct {
		path            string
		line, character int
	}{
		{"src/extra.mts", 0, 20},
		{"src/extra.mts", 2, 30},
		{"src/extra.mts", 1, 0},
		{"src/extra.mts", 40, 0},
		{"src/missing.mts", 0, 0},
		{"README.md", 0, 0},
	} {
		if _, text := hoverAt(t, client, root, miss.path, miss.line, miss.character); text != "" {
			t.Fatalf("hover %s:%d = %q", miss.path, miss.line, text)
		}
	}
	response := client.request("textDocument/hover", map[string]any{
		"textDocument": map[string]string{"uri": "untitled:1"}, "position": map[string]int{},
	})
	if string(response.Result) != "null" {
		t.Fatalf("hover of an untitled document = %s", response.Result)
	}
}

func TestLSPHoverWithoutSteps(t *testing.T) {
	card := &lspCard{markdown: true}
	scenarioCard(card, IndexScenario{Title: "Raw", Scope: "specs", Text: "Some text."})
	evidenceLines(card, IndexScenario{}, "")
	if text := card.String(); !strings.Contains(text, "Some text.") ||
		!strings.Contains(text, "- no planned evidence or tests") {
		t.Fatalf("card = %q", text)
	}
	if approvalLabel("stale") != "approval stale" || approvalLabel("v1") != "v1 plan" {
		t.Fatal("approval labels")
	}
	content, _ := json.Marshal(lspHover{})
	if !strings.Contains(string(content), `"range"`) {
		t.Fatalf("hover = %s", content)
	}
}

func TestLSPHoverScenarioInTwoScopesAndUnimplementedRequirement(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/specs/demo/spec.md", lspCurrentSpec)
	writeFixture(t, root, "src/demo.mts", "export function value() { return 1; }\n")
	client := openLSP(t, root, lspCapabilities())
	_, text := hoverAt(t, client, root, "tests/value.test.mts", 1, 20)
	if strings.Count(text, "**Scenario:** Value is returned") != 2 || !strings.Contains(text, "\n---\n") {
		t.Fatalf("scenario hover in two scopes = %q", text)
	}
	_, heading := hoverAt(t, client, root, "openspec/specs/demo/spec.md", 5, 4)
	requireLines(t, heading, "**Implementations**", "none yet")
}
