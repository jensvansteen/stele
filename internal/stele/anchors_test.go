package stele

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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

func TestSupportedSourceUsesTypeScriptConsumerScope(t *testing.T) {
	for _, path := range []string{"source.ts", "component.tsx", "module.mts", "UPPER.TS"} {
		if !supportedSource(path) {
			t.Errorf("expected %s to be supported", path)
		}
	}
	for _, path := range []string{
		"source.go",
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

func TestScanAnchorsIgnoresUnsupportedConsumerLanguages(t *testing.T) {
	root := fixtureRoot(t)
	for _, path := range []string{
		"src/source.go",
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
