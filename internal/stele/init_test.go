package stele

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies scn.init.e841b29256e0.unit
func TestInitializeIsIdempotent(t *testing.T) {
	root := fixtureRoot(t)
	created, err := Initialize(root, "example")
	if err != nil || len(created) != 1+len(steleSkills) {
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

// installedSkill initializes a project and returns the body of an installed skill.
func installedSkill(t *testing.T, name string) string {
	t.Helper()
	root := fixtureRoot(t)
	if _, err := Initialize(root, ""); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, ".agents", "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// assertOrdered fails unless every step appears in the skill after the previous one.
func assertOrdered(t *testing.T, skill string, steps ...string) {
	t.Helper()
	position := 0
	for _, step := range steps {
		index := strings.Index(skill[position:], step)
		if index < 0 {
			t.Fatalf("step %q is missing or out of order in:\n%s", step, skill)
		}
		position += index + len(step)
	}
}

// @verifies scn.lifecycle.5a216e97cca6.unit
func TestProposeSkillOrdersPlanningSteps(t *testing.T) {
	skill := installedSkill(t, "stele-propose")
	assertOrdered(t, skill,
		"name: stele-propose",
		"do not write implementation code",
		"`openspec-propose` skill",
		"`stele` schema, its `verification` artifact",
		"`stele ids --change <change>`",
		"`stele-plan` skill",
		"`stele verify --stage proposal --change <change>`",
		`"Plan ready. Ask me to apply the change; I'll show the levels to confirm first."`,
	)
}

// @verifies scn.lifecycle.b91a6d6dd399.unit
func TestApplySkillConfirmsBeforeDelegating(t *testing.T) {
	skill := installedSkill(t, "stele-apply")
	assertOrdered(t, skill,
		"name: stele-apply",
		"approval step in the `stele-plan` skill",
		"show the verification levels",
		"only after an explicit yes",
		"Never treat the request to apply as confirmation",
		"`openspec-apply-change` skill",
		"`@implements` and `@verifies` anchors",
		"`stele validate --change <change>`",
	)
}

// @verifies scn.lifecycle.08814fe1e646.unit
func TestArchiveSkillGatesOnValidation(t *testing.T) {
	skill := installedSkill(t, "stele-archive")
	assertOrdered(t, skill,
		"name: stele-archive",
		"`stele validate --change <change>`",
		"If it fails, stop",
		"`openspec-archive-change` skill",
		"`stele validate --specs`",
	)
}

// lifecycleDelegates maps each lifecycle skill to the OpenSpec skill it uses.
var lifecycleDelegates = map[string]string{
	"stele-propose": "openspec-propose",
	"stele-apply":   "openspec-apply-change",
	"stele-archive": "openspec-archive-change",
}

// @verifies scn.lifecycle.5df1b597ff80.unit
func TestLifecycleSkillsStayThin(t *testing.T) {
	forbidden := []string{"schemaVersion", "linkage-plan", "#selector", "Verification-ID:", "evidence ID"}
	for skillName, openSpecName := range lifecycleDelegates {
		skill := installedSkill(t, skillName)
		for _, term := range forbidden {
			if strings.Contains(skill, term) {
				t.Fatalf("%s describes plan-format details (%q)", skillName, term)
			}
		}
		if steps := countOrderedSteps(t, skillName, skill); steps < 3 ||
			!strings.Contains(skill, "`"+openSpecName+"` skill") {
			t.Fatalf("%s does not delegate in ordered steps to %s:\n%s", skillName, openSpecName, skill)
		}
		assertNoCopiedText(t, skillName, skill, openSpecName)
	}
}

// countOrderedSteps counts the numbered steps of a skill body and fails on any
// other content besides the title and the planning-only note.
func countOrderedSteps(t *testing.T, skillName, skill string) int {
	t.Helper()
	body := skill[strings.Index(skill[3:], "---")+6:]
	steps := 0
	for line := range strings.SplitSeq(strings.TrimSpace(body), "\n") {
		switch {
		case line == "", strings.HasPrefix(line, "# "), strings.HasPrefix(line, "Planning only:"):
		case len(line) > 3 && line[0] >= '1' && line[0] <= '9' && line[1] == '.':
			steps++
		default:
			t.Fatalf("%s has a line that is not an ordered step: %q", skillName, line)
		}
	}
	return steps
}

// assertNoCopiedText compares a skill with the OpenSpec skill installed in this
// repository and fails on any shared line of real content.
func assertNoCopiedText(t *testing.T, skillName, skill, openSpecName string) {
	t.Helper()
	reference, err := os.ReadFile(filepath.Join("..", "..", ".agents", "skills", openSpecName, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(reference), "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 30 && strings.Contains(skill, line) {
			t.Fatalf("%s copies OpenSpec skill text: %q", skillName, line)
		}
	}
}
