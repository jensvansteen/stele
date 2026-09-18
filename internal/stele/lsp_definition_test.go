package stele

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// locationsAt runs a definition or references request and returns each
// location as path:line:start-end, with the path relative to the root.
func locationsAt(t *testing.T, client *lspTestClient, root, method string, params map[string]any) []string {
	t.Helper()
	var locations []lspLocation
	client.result(client.request(method, params), &locations)
	found := make([]string, 0, len(locations))
	for _, location := range locations {
		path, _ := uriPath(location.URI)
		relative, _ := filepath.Rel(root, path)
		found = append(found, fmt.Sprintf("%s:%d:%d-%d", filepath.ToSlash(relative), location.Range.Start.Line+1,
			location.Range.Start.Character, location.Range.End.Character))
	}
	return found
}

func requireLocations(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("locations = %q, want %q", got, want)
	}
}

// @verifies scn.languageserver.08b3c3878af0.unit
func TestLSPDefinitionFromAnchorToSpecification(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, lspChangeSpec, strings.Replace(evidenceSpec,
		"#### Scenario: Value is returned", "#### Scenario: Valeur rendue 😀 é", 1))
	writeFixture(t, root, "tests/e2e/value.test.mts", "import test from \"node:test\";\n"+
		"// ✓ café 😀 @verifies "+"scn.demo.bbbbbbbbbbbb.e2e\nvoid test(\"shows the value\", () => {});\n")
	utf16 := openLSP(t, root, lspCapabilities())
	requireLocations(t, locationsAt(t, utf16, root, "textDocument/definition",
		position(root, "tests/e2e/value.test.mts", 1, 25)), lspChangeSpec+":9:0-33")
	var hover lspHover
	utf16.result(utf16.request("textDocument/hover", position(root, "tests/e2e/value.test.mts", 1, 25)), &hover)
	if hover.Range.Start.Character != 23 || hover.Range.End.Character != 23+len("scn.demo.bbbbbbbbbbbb.e2e") {
		t.Fatalf("UTF-16 anchor span = %#v", hover.Range)
	}
	capabilities := lspCapabilities()
	capabilities["general"] = map[string]any{"positionEncodings": []string{"utf-8"}}
	utf8 := openLSP(t, root, capabilities)
	requireLocations(t, locationsAt(t, utf8, root, "textDocument/definition",
		position(root, "tests/e2e/value.test.mts", 1, 30)), lspChangeSpec+":9:0-36")
	requireLocations(t, locationsAt(t, utf8, root, "textDocument/definition",
		position(root, "tests/e2e/value.test.mts", 1, 10)))
}

// @verifies scn.languageserver.3f9633e42fd4.unit
func TestLSPDefinitionFromRequirementToImplementations(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "src/another.mts", "export const first = 1;\n\n// @implements "+
		"req.demo.aaaaaaaaaaaa\nexport function another() { return 2; }\n")
	client := openLSP(t, root, lspCapabilities())
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 3, 4)),
		"src/another.mts:3:15-36", "src/demo.mts:1:15-36")
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 4, 0)),
		"src/another.mts:3:15-36", "src/demo.mts:1:15-36")
}

// @verifies scn.languageserver.28e4bd385efa.unit
func TestLSPDefinitionFromScenarioToTests(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 8, 4)),
		"tests/e2e/value.test.mts:2:13-38", "tests/value.test.mts:2:13-39")
}

// @verifies scn.languageserver.100060104a1b.unit
func TestLSPReferencesOfARequirement(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/specs/demo/spec.md", lspCurrentSpec)
	writeFixture(t, root, "tests/e2e/value.test.mts", "export const nothing = 1;\n")
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	client := openLSP(t, root, lspCapabilities())
	params := position(root, lspChangeSpec, 3, 4)
	params["context"] = map[string]bool{"includeDeclaration": true}
	requireLocations(t, locationsAt(t, client, root, "textDocument/references", params),
		lspChangeSpec+":4:0-29", "openspec/specs/demo/spec.md:6:0-29", "src/demo.mts:1:15-36",
		"tests/missing.test.mts:2:13-39", "tests/value.test.mts:2:13-39")
}

// @verifies scn.languageserver.e42ad028df53.unit
func TestLSPReferencesOfAScenarioFromATest(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	params := position(root, "tests/value.test.mts", 1, 20)
	params["context"] = map[string]bool{"includeDeclaration": false}
	requireLocations(t, locationsAt(t, client, root, "textDocument/references", params),
		"tests/e2e/value.test.mts:2:13-38", "tests/value.test.mts:2:13-39")
	params["context"] = map[string]bool{"includeDeclaration": true}
	requireLocations(t, locationsAt(t, client, root, "textDocument/references", params),
		lspChangeSpec+":9:0-32", "tests/e2e/value.test.mts:2:13-38", "tests/value.test.mts:2:13-39")
}

func TestLSPNavigationMisses(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "src/extra.mts", "// @implements "+"req.demo.000000000000\nexport function extra() {}\n")
	client := openLSP(t, root, lspCapabilities())
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, "src/extra.mts", 0, 20)))
	params := position(root, "src/extra.mts", 0, 20)
	params["context"] = map[string]bool{"includeDeclaration": true}
	requireLocations(t, locationsAt(t, client, root, "textDocument/references", params), "src/extra.mts:1:15-36")
	for _, method := range []string{"textDocument/definition", "textDocument/references"} {
		requireLocations(t, locationsAt(t, client, root, method, position(root, "src/extra.mts", 1, 0)))
		requireLocations(t, locationsAt(t, client, root, method, position(root, "elsewhere.mts", 0, 0)))
		if response := client.request(method, "invalid"); response.Error == nil {
			t.Fatalf("%s with invalid params = %#v", method, response)
		}
	}
	places := sortPlaces([]lspPlace{{path: "a", line: 2}, {path: "a", line: 2}, {path: "a", line: 1}})
	if len(places) != 2 {
		t.Fatalf("places = %#v", places)
	}
	lines := []string{"x"}
	if lspLineRange(lines, 5, encodingUTF16) != (lspRange{Start: lspPosition{Line: 4}, End: lspPosition{Line: 4}}) {
		t.Fatal("a line past the end has an empty range")
	}
}

func TestLSPPositionEncodings(t *testing.T) {
	line := "a😀é"
	if lspColumn(line, len(line), encodingUTF16) != 4 || lspColumn(line, 99, encodingUTF8) != len(line) ||
		lspColumn(line, -1, encodingUTF16) != 0 {
		t.Fatal("columns")
	}
	if lspOffset(line, 3, encodingUTF16) != 5 || lspOffset(line, 99, encodingUTF16) != len(line) ||
		lspOffset(line, 99, encodingUTF8) != len(line) || lspOffset(line, 2, encodingUTF8) != 2 {
		t.Fatal("offsets")
	}
	if _, local := uriPath("file://%zz"); local {
		t.Fatal("an invalid URI is a path")
	}
	if path, local := uriPath(fileURI("/a b/c")); !local || path != filepath.FromSlash("/a b/c") {
		t.Fatalf("round trip = %q", path)
	}
	if truncateRunes("éé", 2) != "éé" || truncateRunes("ééé", 2) != "é…" {
		t.Fatal("truncation")
	}
}

func TestLSPNavigationOfUntitledDocuments(t *testing.T) {
	client := openLSP(t, lspFixture(t), lspCapabilities())
	for _, method := range []string{"textDocument/definition", "textDocument/references"} {
		response := client.request(method, map[string]any{
			"textDocument": map[string]string{"uri": "untitled:1"}, "position": map[string]int{},
		})
		if string(response.Result) != "[]" {
			t.Fatalf("%s of an untitled document = %s", method, response.Result)
		}
	}
}
