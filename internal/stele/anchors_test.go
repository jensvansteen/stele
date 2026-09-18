package stele

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// @verifies scn.verify.36f0ef1f337f.unit
func TestScanAnchorsResolvesTypeScriptDeclarations(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/store.ts", `// @implements req.demo.aaaaaaaaaaaa
export function store() {}
`)
	writeFixture(t, root, "src/view.tsx", `// @implements req.demo.bbbbbbbbbbbb
export class View {}
`)
	writeFixture(t, root, "src/module.mts", `// @implements req.demo.cccccccccccc
export async function loadModule() {}
`)
	writeFixture(t, root, "tests/store.test.ts", `// @verifies scn.demo.dddddddddddd
test("stores value", () => {})
`)
	writeFixture(t, root, "tests/view.spec.tsx", `// @verifies scn.demo.eeeeeeeeeeee
it("renders view", () => {})
`)
	writeFixture(t, root, "tests/module.test.mts", `// @verifies scn.demo.ffffffffffff
test("loads module", () => {})
`)

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	expected := []struct {
		identity string
		path     string
		selector string
	}{
		{"req.demo.aaaaaaaaaaaa", "src/store.ts", "store"},
		{"req.demo.bbbbbbbbbbbb", "src/view.tsx", "View"},
		{"req.demo.cccccccccccc", "src/module.mts", "loadModule"},
		{"scn.demo.dddddddddddd", "tests/store.test.ts", "stores value"},
		{"scn.demo.eeeeeeeeeeee", "tests/view.spec.tsx", "renders view"},
		{"scn.demo.ffffffffffff", "tests/module.test.mts", "loads module"},
	}
	if len(anchors) != len(expected) {
		t.Fatalf("expected %d anchors, got %#v", len(expected), anchors)
	}
	for index, item := range expected {
		assertAnchor(t, anchors, item.identity, item.path, item.selector)
		if anchors[index].ID != item.identity {
			t.Fatalf("anchors are not deterministically ordered: %#v", anchors)
		}
	}
}

func TestScanAnchorsSupportsTypeScriptDeclarationsAndServerFiles(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "server.ts", `// @implements req.demo.111111111111
class Server {}
`)
	writeFixture(t, root, "server.tsx", `// @implements req.demo.222222222222
class ServerView {}
`)
	writeFixture(t, root, "server.mts", `// @implements req.demo.333333333333
async function startServer() {}
`)
	writeFixture(t, root, "src/functions.ts", `// @implements req.demo.444444444444
// explanation
export async function load() {}
// @implements req.demo.555555555555
export class Store {}
// @implements req.demo.666666666666
save() {}
// @implements req.demo.777777777777
if (true) {}
`)
	writeFixture(t, root, "tests/demo.spec.ts", `// @verifies scn.demo.888888888888
it(`+"`works too`"+`, () => {})
// @verifies scn.demo.999999999999
describe("not an exact test", () => {})
`)

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct {
		identity string
		path     string
		selector string
	}{
		{"req.demo.111111111111", "server.ts", "Server"},
		{"req.demo.222222222222", "server.tsx", "ServerView"},
		{"req.demo.333333333333", "server.mts", "startServer"},
		{"req.demo.444444444444", "src/functions.ts", "load"},
		{"req.demo.555555555555", "src/functions.ts", "Store"},
		{"req.demo.666666666666", "src/functions.ts", "save"},
		{"scn.demo.888888888888", "tests/demo.spec.ts", "works too"},
	} {
		assertAnchor(t, anchors, expected.identity, expected.path, expected.selector)
	}

	for _, identity := range []string{"req.demo.777777777777", "scn.demo.999999999999"} {
		assertAnchorHasNoSelector(t, anchors, identity)
	}
}

func TestSupportedSourceUsesConsumerScope(t *testing.T) {
	for _, path := range []string{"source.ts", "component.tsx", "module.mts", "UPPER.TS", "source.go"} {
		if !supportedSource(path) {
			t.Errorf("expected %s to be supported", path)
		}
	}
	for _, path := range []string{
		"source.js",
		"source.jsx",
		"source.mjs",
		"source.cts",
		"README.md",
	} {
		if supportedSource(path) {
			t.Errorf("expected %s to be unsupported", path)
		}
	}
}

// @verifies scn.verify.0d43596abe4a.unit
func TestScanAnchorsIgnoresUnsupportedConsumerLanguages(t *testing.T) {
	root := fixtureRoot(t)
	for _, path := range []string{
		"src/source.js",
		"src/source.jsx",
		"src/source.mjs",
		"src/source.cts",
	} {
		writeFixture(t, root, path, "// @implements req.demo.aaaaaaaaaaaa\nfunction ignored() {}\n")
	}

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 0 {
		t.Fatalf("expected unsupported files to be ignored, got %#v", anchors)
	}
}

func TestScanAnchorsReturnsReadAndPathErrors(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		root := fixtureRoot(t)
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		missing := filepath.Join(root, "missing.ts")
		broken := filepath.Join(root, "src", "broken.ts")
		if err := os.Symlink(missing, broken); err != nil {
			t.Fatal(err)
		}
		if _, err := ScanAnchors(root); err == nil {
			t.Fatal("expected read error")
		}
	})

	t.Run("relative path", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "src/demo.ts", `// @implements req.demo.aaaaaaaaaaaa
function demo() {}
`)
		original := relativePath
		t.Cleanup(func() { relativePath = original })
		relativePath = func(string, string) (string, error) {
			return "", errors.New("relative failed")
		}
		if _, err := ScanAnchors(root); err == nil {
			t.Fatal("expected relative path error")
		}
	})
}

func TestAdjacentDeclarationReturnsNilAfterComments(t *testing.T) {
	selector, line := adjacentDeclaration(
		[]string{"// anchor", "// comment", "", "/* note */", "* detail"},
		0,
		"code",
	)
	if selector != nil || line != nil {
		t.Fatalf("adjacentDeclaration = %v, %v", selector, line)
	}
}

func assertAnchor(t *testing.T, anchors []Anchor, identity, path, selector string) {
	t.Helper()
	for _, anchor := range anchors {
		if anchor.ID == identity && anchor.Path == path && pointerValue(anchor.Selector) == selector {
			return
		}
	}
	t.Fatalf("anchor %s at %s#%s not found in %#v", identity, path, selector, anchors)
}

func assertAnchorHasNoSelector(t *testing.T, anchors []Anchor, identity string) {
	t.Helper()
	for _, anchor := range anchors {
		if anchor.ID == identity {
			if anchor.Selector != nil {
				t.Fatalf("%s unexpectedly resolved to %q", identity, *anchor.Selector)
			}
			return
		}
	}
	t.Fatalf("anchor %s not found in %#v", identity, anchors)
}

// @verifies scn.tsanchors.f4c1eb23c2ab.unit
func TestScanAnchorsResolvesExpressionTestCalls(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/forms.test.mts", `import test, { it } from "node:test";
// @verifies scn.demo.aaaaaaaaaaaa
void test("adds a todo", (): void => {});
// @verifies scn.demo.bbbbbbbbbbbb
await it("removes a todo", (): void => {});
// @verifies scn.demo.cccccccccccc
test("lists todos", (): void => {});
// @verifies scn.demo.dddddddddddd.e2e
void   test('keeps evidence ids', (): void => {});
`)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	for identity, selector := range map[string]string{
		"scn.demo.aaaaaaaaaaaa": "adds a todo",
		"scn.demo.bbbbbbbbbbbb": "removes a todo",
		"scn.demo.cccccccccccc": "lists todos",
		"scn.demo.dddddddddddd": "keeps evidence ids",
	} {
		assertAnchor(t, anchors, identity, "tests/forms.test.mts", selector)
	}
}

// @verifies scn.tsanchors.f5c807c92cad.unit
func TestScanAnchorsIgnoresOtherExpressionCalls(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/other.test.mts", `// @verifies scn.demo.aaaaaaaaaaaa
void run("adds a todo");
// @verifies scn.demo.bbbbbbbbbbbb
voidtest("joined", () => {});
`)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	assertAnchorHasNoSelector(t, anchors, "scn.demo.aaaaaaaaaaaa")
	assertAnchorHasNoSelector(t, anchors, "scn.demo.bbbbbbbbbbbb")
}

// @verifies scn.tsanchors.e3be49f14532.unit
func TestScanAnchorsIgnoresTypeScriptStringLiterals(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/strings.mts", "const single = '// @implements req.demo.111111111111';\n"+
		"const double = \"// @implements req.demo.222222222222 \\\" still text\";\n"+
		"const escaped = '\\' // @implements req.demo.333333333333';\n"+
		"const template = `first line\n"+
		"// @implements req.demo.444444444444\n"+
		"${ { nested: \"} // @implements req.demo.555555555555\" }.nested } after\n"+
		"${`inner ${1} // @implements req.demo.666666666666`} \\` // @implements req.demo.999999999999`;\n"+
		"const unterminated = \"// @implements req.demo.777777777777\n"+
		"// @implements req.demo.888888888888\n"+
		"export function after(): void {}\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 1 || anchors[0].ID != "req.demo.888888888888" {
		t.Fatalf("expected only the real comment anchor, got %#v", anchors)
	}
	assertAnchor(t, anchors, "req.demo.888888888888", "src/strings.mts", "after")
}

// @verifies scn.tsanchors.e6114f49ffdc.unit
func TestScanAnchorsReadsEveryCommentForm(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/comments.ts", `const value = 1; // @implements req.demo.111111111111
/* @implements req.demo.222222222222 */ const other = 2;
/**
 * Stores a value.
 * @implements req.demo.333333333333
 */
export function store(): void {}
/* first */ /* @implements req.demo.444444444444 */
export class Box {}
`)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 4 {
		t.Fatalf("expected four anchors, got %#v", anchors)
	}
	assertAnchor(t, anchors, "req.demo.333333333333", "src/comments.ts", "store")
	assertAnchor(t, anchors, "req.demo.444444444444", "src/comments.ts", "Box")
	for _, anchor := range anchors {
		if anchor.ID == "req.demo.111111111111" && anchor.Line != 1 {
			t.Fatalf("line comment anchor reported at line %d", anchor.Line)
		}
	}
}

func TestAnchorsSplitEvidenceIDs(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "tests/levels.test.mts", "import test from \"node:test\";\n"+
		"// @verifies "+"scn.demo.bbbbbbbbbbbb.unit\n"+
		"void test(\"unit\", () => {});\n"+
		"// @verifies "+"scn.demo.bbbbbbbbbbbb.e2e.2 and prose after it\n"+
		"void test(\"second e2e\", () => {});\n"+
		"// @verifies "+"scn.demo.bbbbbbbbbbbb.unitary is not a level\n"+
		"void test(\"bare\", () => {});\n")
	writeFixture(t, root, "tests/levels_test.go", "package tests\n\n"+
		"// @verifies "+"scn.demo.cccccccccccc.integration\n"+
		"func TestLevels(t *testing.T) {}\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		got = append(got, anchor.ID+"|"+anchor.EvidenceID+"|"+anchor.Level)
	}
	want := []string{
		"scn.demo.bbbbbbbbbbbb|scn.demo.bbbbbbbbbbbb.unit|unit",
		"scn.demo.bbbbbbbbbbbb|scn.demo.bbbbbbbbbbbb.e2e.2|e2e",
		"scn.demo.bbbbbbbbbbbb||",
		"scn.demo.cccccccccccc|scn.demo.cccccccccccc.integration|integration",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("anchors =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// @verifies scn.linkindex.57dcee8c30a8.unit
func TestScanAnchorsLeavesParameterizedTestsUnresolved(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Demo
Verification-ID: req.demo.ffffffffffff
#### Scenario: Interpolated
Verification-ID: scn.demo.aaaaaaaaaaaa
#### Scenario: Table
Verification-ID: scn.demo.bbbbbbbbbbbb
#### Scenario: Literal template
Verification-ID: scn.demo.cccccccccccc
`)
	writeFixture(t, root, "tests/names.test.mts", "import test from \"node:test\";\n"+
		"const size = 2;\n"+
		"// @verifies scn.demo.aaaaaaaaaaaa\n"+
		"test(`adds ${size} todos`, () => {});\n"+
		"// @verifies scn.demo.bbbbbbbbbbbb\n"+
		"test.each([[1], [2]])(\"adds %i todos\", () => {});\n"+
		"// @verifies scn.demo.cccccccccccc\n"+
		"test(`adds todos`, () => {});\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	assertAnchorHasNoSelector(t, anchors, "scn.demo.aaaaaaaaaaaa")
	assertAnchorHasNoSelector(t, anchors, "scn.demo.bbbbbbbbbbbb")
	assertAnchor(t, anchors, "scn.demo.cccccccccccc", "tests/names.test.mts", "adds todos")

	original := runExactTest
	t.Cleanup(func() { runExactTest = original })
	runOneByOne(t)
	ran := make([]string, 0)
	runExactTest = func(_, _, selector string) (bool, bool, error) {
		ran = append(ran, selector)
		return true, true, nil
	}
	evidence, err := RunScenarioTests(root, "example", "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ran, []string{"adds todos"}) {
		t.Fatalf("tests run by a partial name: %v", ran)
	}
	unresolved := 0
	for _, execution := range evidence.Executions {
		if execution.Selector == nil && pointerValue(execution.Reason) == "target-not-resolved" {
			unresolved++
		}
	}
	if unresolved != 1 || evidence.Scenarios[0].Outcome != "failed" || evidence.Scenarios[1].Outcome != "failed" {
		t.Fatalf("parameterized tests were not reported as not selectable: %#v", evidence)
	}
}
