package stele

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkFilesFiltersAndSkipsGeneratedTrees(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/keep.go", "package demo\n")
	writeFixture(t, root, "src/drop.txt", "ignored\n")
	writeFixture(t, root, "src/node_modules/drop.go", "package demo\n")
	writeFixture(t, root, "src/.git/drop.go", "package demo\n")
	writeFixture(t, root, "src/dist/drop.go", "package demo\n")

	files := walkFiles(filepath.Join(root, "src"), func(path string) bool {
		return filepath.Ext(path) == ".go"
	})
	if len(files) != 1 || filepath.Base(files[0]) != "keep.go" {
		t.Fatalf("walkFiles() = %#v, want only keep.go", files)
	}
	if got := walkFiles(filepath.Join(root, "missing"), func(string) bool { return true }); len(got) != 0 {
		t.Fatalf("missing root returned %#v", got)
	}
}

func TestComputeInputDigestTracksAcceptedInputs(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/demo.ts", "export const value = 1\n")
	writeFixture(t, root, "src/ignored.txt", "first\n")
	first, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "src/ignored.txt", "second\n")
	second, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("ignored extension changed the digest")
	}
	writeFixture(t, root, "src/demo.ts", "export const value = 2\n")
	third, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("accepted input did not change the digest")
	}
}

func TestComputeInputDigestReturnsPathAndReadErrors(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "src/demo.go", "package demo\n")
	originalRelativePath := relativePath
	t.Cleanup(func() { relativePath = originalRelativePath })
	relativePath = func(string, string) (string, error) { return "", errors.New("relative failed") }
	if _, err := ComputeInputDigest(root); err == nil {
		t.Fatal("expected relative path error")
	}

	relativePath = originalRelativePath
	broken := filepath.Join(root, "src", "broken.go")
	if err := os.Symlink(filepath.Join(root, "missing.go"), broken); err != nil {
		t.Fatal(err)
	}
	if _, err := ComputeInputDigest(root); err == nil {
		t.Fatal("expected broken symlink read error")
	}
}
