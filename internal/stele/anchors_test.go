package stele

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanAnchorsResolvesJavaScriptAndGoDeclarations(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/store.mjs", "// @implements "+"req.demo.aaaaaaaaaaaa\nexport function store() {}\n")
	writeFixture(t, root, "internal/demo/service.go", "package demo\n\n// @implements "+"req.demo.bbbbbbbbbbbb\nfunc Store() {}\n")
	writeFixture(t, root, "tests/store.test.mjs", "// @verifies "+"scn.demo.cccccccccccc\ntest(\"stores value\", () => {})\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	assertAnchor(t, anchors, "req.demo.aaaaaaaaaaaa", "src/store.mjs", "store")
	assertAnchor(t, anchors, "req.demo.bbbbbbbbbbbb", "internal/demo/service.go", "Store")
	assertAnchor(t, anchors, "scn.demo.cccccccccccc", "tests/store.test.mjs", "stores value")
}

func TestScanAnchorsSupportsDeclarationsAndServerFiles(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "server.js", "// @implements req.demo.111111111111\nclass Server {}\n")
	writeFixture(t, root, "src/functions.js", `// @implements req.demo.222222222222
// explanation
export async function load() {}
// @implements req.demo.333333333333
export class Store {}
// @implements req.demo.444444444444
save() {}
// @implements req.demo.555555555555
if (true) {}
`)
	writeFixture(t, root, "internal/demo/types.go", `package demo
// @implements req.demo.666666666666
type Record struct{}
// @implements req.demo.777777777777
var value = 1
`)
	writeFixture(t, root, "tests/demo_test.go", `package demo
// @verifies scn.demo.888888888888
func TestWorks(t *testing.T) {}
// @verifies scn.demo.999999999999
func BenchmarkIgnored(b *testing.B) {}
`)
	writeFixture(t, root, "tests/demo.spec.ts", "// @verifies scn.demo.aaaaaaaaaaaa\nit(`works too`, () => {})\n")

	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct{ id, path, selector string }{
		{"req.demo.111111111111", "server.js", "Server"},
		{"req.demo.222222222222", "src/functions.js", "load"},
		{"req.demo.333333333333", "src/functions.js", "Store"},
		{"req.demo.444444444444", "src/functions.js", "save"},
		{"req.demo.666666666666", "internal/demo/types.go", "Record"},
		{"scn.demo.888888888888", "tests/demo_test.go", "TestWorks"},
		{"scn.demo.aaaaaaaaaaaa", "tests/demo.spec.ts", "works too"},
	} {
		assertAnchor(t, anchors, expected.id, expected.path, expected.selector)
	}
	for _, id := range []string{"req.demo.555555555555", "req.demo.777777777777", "scn.demo.999999999999"} {
		for _, anchor := range anchors {
			if anchor.ID == id && anchor.Selector != nil {
				t.Fatalf("%s unexpectedly resolved to %q", id, *anchor.Selector)
			}
		}
	}
}

func TestScanAnchorsReturnsReadAndPathErrors(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		root := fixtureRoot(t)
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.js"), filepath.Join(root, "src", "broken.js")); err != nil {
			t.Fatal(err)
		}
		if _, err := ScanAnchors(root); err == nil {
			t.Fatal("expected read error")
		}
	})

	t.Run("relative path", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "src/demo.js", "// @implements req.demo.aaaaaaaaaaaa\nfunction demo() {}\n")
		original := relativePath
		t.Cleanup(func() { relativePath = original })
		relativePath = func(string, string) (string, error) { return "", errors.New("relative failed") }
		if _, err := ScanAnchors(root); err == nil {
			t.Fatal("expected relative path error")
		}
	})
}

func TestAdjacentDeclarationReturnsNilAfterComments(t *testing.T) {
	selector, line := adjacentDeclaration([]string{"// anchor", "// comment", "", "/* note */", "* detail"}, 0, "code", ".js")
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
