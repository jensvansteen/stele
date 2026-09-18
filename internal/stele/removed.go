package stele

import (
	"fmt"
	"sort"
)

// removedIdentity is one identity that a change removes: the requirement's ID
// or one of its scenarios' IDs, with the name of the removed requirement.
type removedIdentity struct {
	requirement string
	title       string
}

// removedBehavior resolves the requirements that a change lists under
// `## REMOVED Requirements` against the current specification of the same
// capability. Identities the change declares again moved to another
// requirement and are not removed. A name that matches nothing is reported,
// because nothing would be checked for it.
//
// @implements req.verify.511302d1be0b
func removedBehavior(root string, scope verificationScope, parsed ParsedSpecs) (map[string]removedIdentity,
	[]Diagnostic,
) {
	removed := make(map[string]removedIdentity)
	diagnostics := make([]Diagnostic, 0)
	if scope.currentSpecs {
		return removed, diagnostics
	}
	declared := knownIdentities(parsed)
	current := make(map[string][]Requirement)
	for _, item := range parsed.Removed {
		capability := scope.spec().Capability(scope, item.Source.Path)
		requirements, loaded := current[capability]
		if !loaded {
			requirements = currentRequirements(scope.files(), root, scope.spec().CurrentSpecFile(root, capability))
			current[capability] = requirements
		}
		requirement, found := requirementNamed(requirements, item.Name)
		if !found {
			diagnostics = append(diagnostics, diagnostic(
				"SPEC_REMOVED_UNMATCHED",
				"error",
				fmt.Sprintf("Removed requirement %q matches no requirement in the current %s specification, "+
					"so nothing is checked for it.", item.Name, capability),
				item.Source.Path,
				item.Source.Line,
				"",
			))
			continue
		}
		addRemoved(removed, declared, requirement.ID, removedIdentity{requirement: item.Name, title: item.Name})
		for _, scenario := range requirement.Scenarios {
			addRemoved(removed, declared, scenario.ID, removedIdentity{requirement: item.Name, title: scenario.Title})
		}
	}
	return removed, diagnostics
}

func addRemoved(removed map[string]removedIdentity, declared map[string]bool, id string, identity removedIdentity) {
	if id != "" && !declared[id] {
		removed[id] = identity
	}
}

// currentRequirements parses one current specification, or returns nothing
// when the capability has none.
func currentRequirements(repo repoFiles, root, file string) []Requirement {
	if !repo.isFile(file) {
		return nil
	}
	parsed, _ := parseSpecFiles(repo, root, []string{file}, "")
	return parsed.Requirements
}

// requirementNamed finds a requirement by name, as OpenSpec matches names:
// exactly, after trimming surrounding whitespace.
func requirementNamed(requirements []Requirement, name string) (Requirement, bool) {
	for _, requirement := range requirements {
		if requirement.Title == name {
			return requirement, true
		}
	}
	return Requirement{}, false
}

// removedAnchorDiagnostics reports code and tests still anchored to removed
// behavior. Anchor IDs are identities, so an evidence ID such as
// <scenario>.unit.2 is matched through its scenario.
func removedAnchorDiagnostics(anchors []Anchor, removed map[string]removedIdentity) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, anchor := range anchors {
		identity, found := removed[anchor.ID]
		if !found {
			continue
		}
		diagnostics = append(diagnostics, removedAnchorDiagnostic(anchor,
			fmt.Sprintf("@%s %s still names %q, which this change removes with requirement %q.",
				anchor.Annotation, anchorName(anchor), identity.title, identity.requirement)))
	}
	return diagnostics
}

// retiredAnchorDiagnostics reports, for the current specifications, anchors
// whose identity only archived changes declare: behavior an archived change
// removed.
func retiredAnchorDiagnostics(anchors []Anchor, retired map[string]bool) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, anchor := range anchors {
		if retired[anchor.ID] {
			diagnostics = append(diagnostics, removedAnchorDiagnostic(anchor,
				fmt.Sprintf("@%s %s names behavior that only archived changes declare; "+
					"an archived change removed it.", anchor.Annotation, anchorName(anchor))))
		}
	}
	return diagnostics
}

func removedAnchorDiagnostic(anchor Anchor, message string) Diagnostic {
	return diagnostic("LINK_REMOVED_BEHAVIOR_ANCHORED", "error", message, anchor.Path, anchor.Line, anchorName(anchor))
}

// anchorName is the name an anchor was written with: its evidence ID when it
// has one, otherwise its identity.
func anchorName(anchor Anchor) string {
	if anchor.EvidenceID != "" {
		return anchor.EvidenceID
	}
	return anchor.ID
}

// removedPlanDiagnostics reports plan entries that still name removed behavior.
func removedPlanDiagnostics(plan LinkagePlan, removed map[string]removedIdentity) []Diagnostic {
	identities := make([]string, 0)
	for id := range plan.Evidence {
		identities = append(identities, id)
	}
	for id := range plan.Scenarios {
		identities = append(identities, id)
	}
	for id := range plan.Requirements {
		identities = append(identities, id)
	}
	sort.Strings(identities)
	diagnostics := make([]Diagnostic, 0)
	for _, id := range identities {
		identity, found := removed[id]
		if !found {
			continue
		}
		diagnostics = append(diagnostics, diagnostic(
			"PLAN_REMOVED_BEHAVIOR_PLANNED",
			"error",
			fmt.Sprintf("The plan still lists %s (%q), which this change removes with requirement %q.",
				id, identity.title, identity.requirement),
			plan.Source,
			1,
			id,
		))
	}
	return diagnostics
}
