package stele

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

const linkagePlanFile = "linkage-plan.json"

var errConflictingScope = errors.New("--specs cannot be combined with --change")

// verificationScope selects what a command verifies: one OpenSpec change, or the
// current specifications in openspec/specs.
type verificationScope struct {
	changeID     string
	currentSpecs bool
}

func changeScope(changeID string) verificationScope {
	return verificationScope{changeID: changeID}
}

func (scope verificationScope) specsRoot(root string) string {
	if scope.currentSpecs {
		return filepath.Join(root, "openspec", "specs")
	}
	return filepath.Join(root, "openspec", "changes", scope.changeID, "specs")
}

// resolveScope turns parsed options into the scope that verify, test, and
// validate run against.
//
// @implements req.verificationscope.bee89d6750ed
func resolveScope(parsed options) verificationScope {
	if parsed.specs {
		return verificationScope{currentSpecs: true}
	}
	return changeScope(parsed.changeID)
}

// requireScopeSpecs rejects a scope without specification files, so a gate
// never passes after checking zero requirements.
//
// @implements req.verificationscope.270b822fff6b
func requireScopeSpecs(root string, scope verificationScope) error {
	files := walkFiles(scope.specsRoot(root), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".md")
	})
	if len(files) > 0 {
		return nil
	}
	if scope.currentSpecs {
		return errors.New("no current specifications in openspec/specs")
	}
	return fmt.Errorf("change %s has no delta specs", scope.changeID)
}

func emptyLinkagePlan() LinkagePlan {
	return LinkagePlan{Requirements: map[string]string{}, Scenarios: map[string]string{}}
}

func loadScopePlan(root string, scope verificationScope) (LinkagePlan, []Diagnostic) {
	if scope.currentSpecs {
		return loadArchivedPlans(root)
	}
	return loadChangePlan(root, scope.changeID)
}

// loadChangePlan reads the plan stored with the change, falling back to the
// shared artifacts plan, and rejects a plan written for a different change.
//
// @implements req.verificationscope.713ceb31377c
func loadChangePlan(root, changeID string) (LinkagePlan, []Diagnostic) {
	source := "openspec/changes/" + changeID + "/" + linkagePlanFile
	if !fileExists(filepath.Join(root, filepath.FromSlash(source))) {
		source = "artifacts/" + linkagePlanFile
	}
	plan, err := readLinkagePlan(filepath.Join(root, filepath.FromSlash(source)))
	if err != nil {
		return emptyLinkagePlan(), nil
	}
	if plan.ChangeID != "" && plan.ChangeID != changeID {
		return emptyLinkagePlan(), []Diagnostic{diagnostic(
			"PLAN_CHANGE_MISMATCH",
			"error",
			fmt.Sprintf("Linkage plan %s belongs to change %s, not %s.", source, plan.ChangeID, changeID),
			source,
			1,
			"",
		)}
	}
	plan.Source = source
	if plan.SchemaVersion != evidencePlanVersion {
		return plan, []Diagnostic{deprecatedPlanDiagnostic(source)}
	}
	return plan, nil
}

// loadArchivedPlans combines the plans of archived changes. OpenSpec prefixes
// archive directories with their date, so path order is archive order and a
// later plan replaces an earlier entry for the same identity, whatever the
// schema version of either plan.
func loadArchivedPlans(root string) (LinkagePlan, []Diagnostic) {
	combined := emptyLinkagePlan()
	combined.Evidence = map[string][]EvidenceEntry{}
	diagnostics := make([]Diagnostic, 0)
	archive := filepath.Join(root, "openspec", "changes", "archive")
	for _, file := range walkFiles(archive, func(path string) bool { return filepath.Base(path) == linkagePlanFile }) {
		plan, err := readLinkagePlan(file)
		if err != nil {
			continue
		}
		if plan.SchemaVersion == evidencePlanVersion {
			for id, entries := range plan.Evidence {
				combined.Evidence[id] = entries
				delete(combined.Scenarios, id)
			}
			continue
		}
		source := strings.TrimPrefix(filepath.ToSlash(file), filepath.ToSlash(root)+"/")
		diagnostics = append(diagnostics, deprecatedPlanDiagnostic(source))
		maps.Copy(combined.Requirements, plan.Requirements)
		for id, target := range plan.Scenarios {
			combined.Scenarios[id] = target
			delete(combined.Evidence, id)
		}
	}
	return combined, diagnostics
}

// declaredIdentities returns every Verification-ID declared anywhere under
// openspec/: active changes, archived changes, and current specifications.
//
// @implements req.verificationscope.9c81618619df
func declaredIdentities(root string) (map[string]bool, error) {
	declared := make(map[string]bool)
	files := walkFiles(filepath.Join(root, "openspec"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".md")
	})
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for line := range strings.SplitSeq(string(content), "\n") {
			if match := verificationIDPattern.FindStringSubmatch(line); match != nil {
				declared[match[1]] = true
			}
		}
	}
	return declared, nil
}
