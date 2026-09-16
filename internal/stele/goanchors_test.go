package stele

import (
	"bytes"
	"go/ast"
	"strings"
	"testing"
)

const goCodeFixture = `package store

// Store keeps values.
//
// @implements req.demo.111111111111
type Store[T any] struct{}

// @implements req.demo.222222222222
func (store *Store[T]) Save(value T) {}

// @implements req.demo.333333333333
func Load() {}

/* @implements req.demo.444444444444 */
func (Plain) Name() string { return "" }

type Plain struct{}

// @implements req.demo.555555555555
func (plain (*Plain)) Paren() {}

// @implements req.demo.666666666666
func (pair Pair[K, V]) Keys() {}

type Pair[K comparable, V any] struct{}

/*
Multi-line block comment.
@implements req.demo.777777777777
*/
func Multi() {}

// @implements req.demo.888888888888
type (
	First  int
	Second int
)

// @implements req.demo.999999999999
func (values []int) Sum() int { return 0 }
`

// @verifies scn.gosupport.7798d2c7e562.unit
func TestScanAnchorsResolvesGoDeclarations(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "internal/store/store.go", goCodeFixture)
	writeFixture(t, root, "pkg/api/api.go",
		"package api\r\n\r\n// @implements req.demo.aaaaaaaaaaaa\r\nfunc Serve() {}\r\n")
	writeFixture(t, root, "main.go", "package main\n\n// @implements req.demo.bbbbbbbbbbbb\nfunc main() {}\n")
	writeFixture(t, root, "tests/helpers.go",
		"package tests\n\n// @implements req.demo.cccccccccccc\nfunc Helper() {}\n")
	writeFixture(t, root, "main_test.go", "package main\n")
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	for identity, selector := range map[string]string{
		"req.demo.111111111111": "Store",
		"req.demo.222222222222": "Store.Save",
		"req.demo.333333333333": "Load",
		"req.demo.444444444444": "Plain.Name",
		"req.demo.555555555555": "Plain.Paren",
		"req.demo.666666666666": "Pair.Keys",
		"req.demo.777777777777": "Multi",
	} {
		assertAnchor(t, anchors, identity, "internal/store/store.go", selector)
	}
	assertAnchor(t, anchors, "req.demo.aaaaaaaaaaaa", "pkg/api/api.go", "Serve")
	assertAnchor(t, anchors, "req.demo.bbbbbbbbbbbb", "main.go", "main")
	assertAnchorHasNoSelector(t, anchors, "req.demo.888888888888")
	assertAnchorHasNoSelector(t, anchors, "req.demo.999999999999")
	for _, anchor := range anchors {
		if anchor.ID == "req.demo.cccccccccccc" {
			t.Fatalf("non-test Go file under tests/ was scanned: %#v", anchor)
		}
		if anchor.ID == "req.demo.777777777777" && anchor.Line != 29 {
			t.Fatalf("block comment anchor line = %d, want 29", anchor.Line)
		}
		if anchor.Kind != "code" {
			t.Fatalf("code file anchor has kind %q", anchor.Kind)
		}
	}
}

const goTestFixture = `package store

import "testing"

// @verifies scn.demo.aaaaaaaaaaaa
func TestSave(t *testing.T) {}

// @verifies scn.demo.bbbbbbbbbbbb
// @verifies scn.demo.cccccccccccc.unit
func Test(t *testing.T) {}

// @verifies scn.demo.dddddddddddd
func helper(t *testing.T) {}

// @verifies scn.demo.eeeeeeeeeeee
func Testing(t *testing.T) {}

// @verifies scn.demo.ffffffffffff
func TestBench(b *testing.B) {}

// @verifies scn.demo.111111111111
func TestTwo(t *testing.T, extra int) {}

// @verifies scn.demo.222222222222
func TestValue(t testing.T) {}

// @verifies scn.demo.333333333333
func TestLocal(t *T) {}

// @verifies scn.demo.444444444444
func TestOther(t *other.T) {}

// @verifies scn.demo.555555555555
func (s suite) TestMethod(t *testing.T) {}

// @verifies scn.demo.666666666666
func TestGeneric[P any](t *testing.T) {}

// @verifies scn.demo.777777777777
func TestPair(first, second *testing.T) {}

// @verifies scn.demo.888888888888
type TestType struct{}

// @verifies scn.demo.999999999999
func TestSlice(t *[]int) {}
`

// @verifies scn.gosupport.9bd4f6844031.unit
func TestScanAnchorsResolvesGoTestFunctions(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "internal/store/store_test.go", goTestFixture)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	assertAnchor(t, anchors, "scn.demo.aaaaaaaaaaaa", "internal/store/store_test.go", "TestSave")
	assertAnchor(t, anchors, "scn.demo.bbbbbbbbbbbb", "internal/store/store_test.go", "Test")
	assertAnchor(t, anchors, "scn.demo.cccccccccccc", "internal/store/store_test.go", "Test")
	for _, identity := range []string{
		"scn.demo.dddddddddddd", "scn.demo.eeeeeeeeeeee", "scn.demo.ffffffffffff",
		"scn.demo.111111111111", "scn.demo.222222222222", "scn.demo.333333333333",
		"scn.demo.444444444444", "scn.demo.555555555555", "scn.demo.666666666666",
		"scn.demo.777777777777", "scn.demo.888888888888", "scn.demo.999999999999",
	} {
		assertAnchorHasNoSelector(t, anchors, identity)
	}
	for _, anchor := range anchors {
		if anchor.Kind != "test" {
			t.Fatalf("test file anchor has kind %q", anchor.Kind)
		}
	}
}

// @verifies scn.gosupport.351e919a25ad.unit
func TestScanAnchorsIgnoresGoStringLiterals(t *testing.T) {
	root := fixtureRoot(t)
	source := "package fixtures\n\n" +
		"const raw = `\n// @implements req.demo.111111111111\nfunc Fake() {}\n`\n\n" +
		"var quoted = \"// @implements req.demo.222222222222\"\n\n" +
		"var character = '@' // @implements req.demo.333333333333\n" +
		"func Real() {}\n"
	writeFixture(t, root, "internal/fixtures/fixtures.go", source)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) != 1 || anchors[0].ID != "req.demo.333333333333" {
		t.Fatalf("expected only the comment anchor, got %#v", anchors)
	}
	assertAnchor(t, anchors, "req.demo.333333333333", "internal/fixtures/fixtures.go", "Real")
}

// @verifies scn.gosupport.29a77eab096c.unit
func TestScanAnchorsLeavesUnattachedGoAnchorsUnresolved(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `### Requirement: Body
Verification-ID: req.demo.111111111111
#### Scenario: Unused
Verification-ID: scn.demo.444444444444
`)
	writeFixture(t, root, "internal/demo/demo.go", `package demo

func Outer() {
	// @implements req.demo.111111111111
	println("statement")
}

// @implements req.demo.222222222222
var value = 1

// @implements req.demo.333333333333
//
//
//
//
//
//
func TooFar() {}
`)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"req.demo.111111111111", "req.demo.222222222222", "req.demo.333333333333"} {
		assertAnchorHasNoSelector(t, anchors, identity)
	}
	selector, line := goAdjacentDeclaration([]string{"// anchor", "", "// more"}, 0, map[int]string{})
	if selector != nil || line != nil {
		t.Fatalf("goAdjacentDeclaration past the end = %v, %v", selector, line)
	}

	report, err := RunVerification(root, "example", "implementation", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(report.Diagnostics, "ANCHOR_TARGET_MISSING") {
		t.Fatalf("expected ANCHOR_TARGET_MISSING, got %#v", report.Diagnostics)
	}
}

// @verifies scn.gosupport.8954a51cba0f.unit
func TestVerifyRejectsInvalidGoSource(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
	writeFixture(t, root, "internal/broken/broken.go", "package broken\n\nfunc Broken( {\n")
	var stderr bytes.Buffer
	code := Run([]string{"verify", "--root", root, "--change", "example"}, &bytes.Buffer{}, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "internal/broken/broken.go") {
		t.Fatalf("verify = %d, %q", code, stderr.String())
	}
	if selector := goReceiverName(&ast.FieldList{}); selector != "" {
		t.Fatalf("empty receiver list resolved to %q", selector)
	}
}
