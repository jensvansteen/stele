package stele

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// notDiagnostics are code-like literals in the package that are environment
// variables, not diagnostic codes.
var notDiagnostics = map[string]bool{
	"GITHUB_ACTIONS": true, "GITHUB_WORKSPACE": true, "NODE_TEST_CONTEXT": true, "NO_COLOR": true,
	"STELE_CHILD_TEST": true, "STELE_FAKE_EXIT": true, "STELE_FAKE_OPENSPEC_VERSION": true,
	"STELE_OPENSPEC_EXTEND": true,
}

// @verifies scn.terminalreport.78fda44eee93.unit
func TestEveryDiagnosticCodeHasGuidance(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`"([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+)"`)
	found := make(map[string]bool)
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") || source == "diagnostics.go" {
			continue
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range literal.FindAllStringSubmatch(string(content), -1) {
			if !notDiagnostics[match[1]] {
				found[match[1]] = true
			}
		}
	}
	for _, code := range []string{
		"PLAN_UNAPPROVED", "SPEC_ANNOTATION_MISSING", "LINK_REMOVED_BEHAVIOR_ANCHORED",
		"PLAN_REMOVED_BEHAVIOR_PLANNED", "SPEC_REMOVED_UNMATCHED", "PLAN_ARCHIVE_ORDER_AMBIGUOUS",
	} {
		if !found[code] {
			t.Fatalf("the source scan missed %s; found %v", code, found)
		}
	}
	for code := range found {
		guide, known := diagnosticGuides[code]
		if !known {
			t.Errorf("diagnostic %s has no entry in the catalogue", code)
			continue
		}
		if guide.meaning == "" || guide.fix == "" || guide.short == "" ||
			!slices.Contains([]string{stageSpecifications, stagePlan, stageLinkage, stageExecution}, guide.stage) {
			t.Errorf("diagnostic %s has incomplete guidance: %#v", code, guide)
		}
	}
	reference, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "cli.md"))
	if err != nil {
		t.Fatal(err)
	}
	for code := range diagnosticGuides {
		if !found[code] {
			t.Errorf("the catalogue lists %s, which Stele never reports", code)
		}
		if !strings.Contains(string(reference), "`"+code+"`") {
			t.Errorf("the CLI reference does not document %s", code)
		}
	}
}
