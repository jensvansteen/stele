package stele

import "fmt"

// evidenceDiagnostics applies the v2 rules to one scenario: its plan entries
// must be complete and approved, and in the implementation stage every
// approved entry needs a `@verifies <evidence-id>` anchor on a resolvable test
// while no test may claim unplanned evidence. Locations are never compared.
// With a target selection, only the selected targets' entries and anchors
// are checked.
//
// @implements req.verificationstrategy.c852cba36427
func evidenceDiagnostics(input linkageValidationInput, scenario Scenario) []Diagnostic {
	entries := input.Plan.Evidence[scenario.ID]
	selection := input.Selection
	diagnostics := planEntryDiagnostics(scenario, entries, selection)
	if input.Mode == "proposal" {
		return diagnostics
	}
	anchors := anchorsFor(input.Anchors, scenario.ID, "test")
	planned := make(map[string]bool, len(entries))
	for _, entry := range entries {
		planned[entry.ID] = true
		if entry.Approval == nil || hasEvidenceAnchor(anchors, entry.ID) || !selection.includes(entry.Target) ||
			(scenario.Targeted && !contains(scenario.Targets, entry.Target)) {
			continue
		}
		missing := diagnostic(
			"LINK_EVIDENCE_MISSING",
			"error",
			fmt.Sprintf("No test anchor resolves for evidence %s (%s).", entry.ID, entry.Level),
			scenario.Source.Path,
			scenario.Source.Line,
			entry.ID,
		)
		missing.target = entry.Target
		diagnostics = append(diagnostics, missing)
	}
	for _, anchor := range anchors {
		if (anchor.EvidenceID != "" && planned[anchor.EvidenceID]) ||
			(anchor.Target != "" && !selection.includes(anchor.Target)) {
			continue
		}
		named := anchor.EvidenceID
		if named == "" {
			named = anchor.ID
		}
		unplanned := diagnostic(
			"ANCHOR_EVIDENCE_UNPLANNED",
			"error",
			fmt.Sprintf("@verifies %s names evidence that the plan for %s does not list.", named, scenario.ID),
			anchor.Path,
			anchor.Line,
			named,
		)
		unplanned.target = anchor.Target
		diagnostics = append(diagnostics, unplanned)
	}
	return diagnostics
}

func hasEvidenceAnchor(anchors []Anchor, id string) bool {
	for _, anchor := range anchors {
		if anchor.EvidenceID == id && anchor.Selector != nil {
			return true
		}
	}
	return false
}

// evidenceLinks returns the report links of a v2 scenario and its linkage
// state, for the selected targets.
func evidenceLinks(mode string, anchors []Anchor, entries []EvidenceEntry, selection targetSelection) ([]Link,
	string,
) {
	selected := make([]EvidenceEntry, 0, len(entries))
	for _, entry := range entries {
		if selection.includes(entry.Target) {
			selected = append(selected, entry)
		}
	}
	if mode == "proposal" {
		links := make([]Link, 0, len(selected))
		for _, entry := range selected {
			links = append(links, Link{
				Kind: "test", State: "planned", EvidenceID: entry.ID, Level: entry.Level,
				EvidenceTarget: entry.Target,
			})
		}
		return links, choose(len(selected) > 0, "planned", "missing")
	}
	links := make([]Link, 0, len(anchors))
	for _, anchor := range anchors {
		if anchor.Target == "" || selection.includes(anchor.Target) {
			links = append(links, resolvedLink(anchor))
		}
	}
	linked := len(selected) > 0
	for _, entry := range selected {
		linked = linked && hasEvidenceAnchor(anchors, entry.ID)
	}
	return links, choose(linked, "linked", "missing")
}
