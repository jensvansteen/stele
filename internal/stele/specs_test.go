package stele

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies scn.verify.6c22483ab3c3.unit
func TestParseSpecsPreservesRelationships(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `## ADDED Requirements

### Requirement: Store value
Verification-ID: req.demo.aaaaaaaaaaaa

#### Scenario: Read value
Verification-ID: scn.demo.bbbbbbbbbbbb

- **WHEN** a value exists
- **THEN** it is returned
`)
	parsed, err := ParseSpecs(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", parsed.Diagnostics)
	}
	if len(parsed.Requirements) != 1 || len(parsed.Requirements[0].Scenarios) != 1 {
		t.Fatalf("unexpected relationships: %#v", parsed.Requirements)
	}
	if parsed.Requirements[0].Scenarios[0].ID != "scn.demo.bbbbbbbbbbbb" {
		t.Fatalf("scenario identity was not preserved")
	}
}

// @verifies scn.verify.c5fd3656da59.unit
func TestParseSpecsReportsAllShapeDiagnostics(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", `#### Scenario: Ignored before requirement
Verification-ID: ignored
### Requirement: Missing everything
### Requirement: No scenarios
Verification-ID: req.demo.111111111111
### Requirement: Bad requirement kind
Verification-ID: scn.demo.222222222222
#### Scenario: Bad scenario kind
Verification-ID: req.demo.333333333333
Verification-ID: scn.demo.444444444444
### Requirement: Scenario missing ID
Verification-ID: req.demo.555555555555
#### Scenario: Missing ID
### Requirement: Duplicate scenarios
Verification-ID: req.demo.666666666666
#### Scenario: First
Verification-ID: scn.demo.777777777777
#### Scenario: Second
Verification-ID: scn.demo.777777777777
`)
	parsed, err := ParseSpecs(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	expectedCodes := []string{
		"ID_REQUIREMENT_MISSING",
		"SCENARIO_MISSING",
		"ID_FORMAT",
		"ID_MULTIPLE",
		"ID_SCENARIO_MISSING",
		"ID_DUPLICATE",
	}
	for _, code := range expectedCodes {
		if !hasDiagnostic(parsed.Diagnostics, code) {
			t.Errorf("missing %s in %#v", code, parsed.Diagnostics)
		}
	}
	if got := diagnosticKey(Diagnostic{Code: "A"}); got != "A::000000000" {
		t.Fatalf("diagnosticKey without source = %q", got)
	}
}

func TestParseSpecsReturnsFileErrors(t *testing.T) {
	t.Run("relative path", func(t *testing.T) {
		root := fixtureRoot(t)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", "### Requirement: Demo\n")
		original := relativePath
		t.Cleanup(func() { relativePath = original })
		relativePath = func(string, string) (string, error) { return "", errors.New("relative failed") }
		if _, err := ParseSpecs(root, "example"); err == nil {
			t.Fatal("expected relative path error")
		}
	})

	t.Run("open", func(t *testing.T) {
		root := fixtureRoot(t)
		directory := filepath.Join(root, "openspec", "changes", "example", "specs")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing.md"), filepath.Join(directory, "broken.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := ParseSpecs(root, "example"); err == nil {
			t.Fatal("expected open error")
		}
	})

	t.Run("scanner", func(t *testing.T) {
		root := fixtureRoot(t)
		content := "### Requirement: Parsed before scanner error\n" +
			"Verification-ID: req.demo.aaaaaaaaaaaa\n" +
			strings.Repeat("x", 70_000)
		writeFixture(t, root, "openspec/changes/example/specs/demo/spec.md", content)
		parsed, err := ParseSpecs(root, "example")
		if err == nil {
			t.Fatal("expected scanner token error")
		}
		if len(parsed.Requirements) != 1 || parsed.Requirements[0].ID != "req.demo.aaaaaaaaaaaa" {
			t.Fatalf("expected partial parse result, got %#v", parsed.Requirements)
		}
	})
}

func TestParseSpecsReportsStableDuplicateDiagnostic(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/example/specs/demo/one.md", `### Requirement: One
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: First
Verification-ID: scn.demo.bbbbbbbbbbbb
`)
	writeFixture(t, root, "openspec/changes/example/specs/demo/two.md", `### Requirement: Two
Verification-ID: req.demo.aaaaaaaaaaaa
#### Scenario: Second
Verification-ID: scn.demo.cccccccccccc
`)
	parsed, err := ParseSpecs(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(parsed.Diagnostics, "ID_DUPLICATE") {
		t.Fatalf("expected duplicate diagnostic, got %#v", parsed.Diagnostics)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, item := range diagnostics {
		if item.Code == code {
			return true
		}
	}
	return false
}
