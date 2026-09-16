package stele

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// @verifies scn.init.e841b29256e0.unit
func TestInitializeIsIdempotent(t *testing.T) {
	root := fixtureRoot(t)
	created, err := Initialize(root, "example")
	if err != nil || len(created) != 3 {
		t.Fatalf("first Initialize() = %#v, %v", created, err)
	}
	created, err = Initialize(root, "example")
	if err != nil || len(created) != 0 {
		t.Fatalf("second Initialize() = %#v, %v", created, err)
	}
}

func TestInitializeReportsFilesystemFailures(t *testing.T) {
	t.Run("config encoding", func(t *testing.T) {
		original := marshalConfig
		t.Cleanup(func() { marshalConfig = original })
		marshalConfig = func(any, string, string) ([]byte, error) { return nil, errors.New("encoding failed") }
		if _, err := Initialize(fixtureRoot(t), "example"); err == nil {
			t.Fatal("expected config encoding error")
		}
	})

	t.Run("config", func(t *testing.T) {
		if _, err := Initialize(filepath.Join(fixtureRoot(t), "missing"), "example"); err == nil {
			t.Fatal("expected config write error")
		}
	})

	t.Run("template read", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "stele.config.json", "{}")
		original := readSkillTemplate
		t.Cleanup(func() { readSkillTemplate = original })
		readSkillTemplate = func(string) ([]byte, error) { return nil, errors.New("template failed") }
		if _, err := Initialize(root, "example"); err == nil {
			t.Fatal("expected template read error")
		}
	})

	t.Run("skill directory", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "stele.config.json", "{}")
		writeFixture(t, root, ".agents", "blocking file")
		if _, err := Initialize(root, "example"); err == nil {
			t.Fatal("expected skill directory error")
		}
	})

	t.Run("skill file", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "stele.config.json", "{}")
		destination := filepath.Join(root, ".agents", "skills", "stele-plan", "SKILL.md")
		if err := os.MkdirAll(destination, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Initialize(root, "example"); err == nil {
			t.Fatal("expected skill write error")
		}
	})

	t.Run("artifacts directory", func(t *testing.T) {
		root := fixtureRoot(t)
		if _, err := Initialize(root, "example"); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, "artifacts")); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, "artifacts", "blocking file")
		if _, err := Initialize(root, "example"); err == nil {
			t.Fatal("expected artifacts directory error")
		}
	})
}
