package stele

import "fmt"

// evidenceDiagnostics applies the v2 rules to one scenario: its plan entries
// must be complete and approved, and in the implementation stage every
// approved entry needs a `@verifies <evidence-id>` anchor on a resolvable test
// while no test may claim unplanned evidence. Locations are never compared.
//
// @implements req.verificationstrategy.c852cba36427
func evidenceDiagnostics(input linkageValidationInput, scenario Scenario) []Diagnostic {
	entries := input.Plan.Evidence[scenario.ID]
	diagnostics := planEntryDiagnostics(scenario, entries)
	if input.Mode == "proposal" {
		return diagnostics
	}
	anchors := anchorsFor(input.Anchors, scenario.ID, "test")
	planned := make(map[string]bool, len(entries))
	for _, entry := range entries {
		planned[entry.ID] = true
		if entry.Approval == nil || hasEvidenceAnchor(anchors, entry.ID) {
			continue
		}
		diagnostics = append(diagnostics, diagnostic(
			"LINK_EVIDENCE_MISSING",
			"error",
			fmt.Sprintf("No test anchor resolves for evidence %s (%s).", entry.ID, entry.Level),
			scenario.Source.Path,
			scenario.Source.Line,
			entry.ID,
		))
	}
	for _, anchor := range anchors {
		if anchor.EvidenceID != "" && planned[anchor.EvidenceID] {
			continue
		}
		named := anchor.EvidenceID
		if named == "" {
			named = anchor.ID
		}
		diagnostics = append(diagnostics, diagnostic(
			"ANCHOR_EVIDENCE_UNPLANNED",
			"error",
			fmt.Sprintf("@verifies %s names evidence that the plan for %s does not list.", named, scenario.ID),
			anchor.Path,
			anchor.Line,
			named,
		))
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

// evidenceLinks returns the report links of a v2 scenario and its linkage state.
func evidenceLinks(mode string, anchors []Anchor, entries []EvidenceEntry) ([]Link, string) {
	if mode == "proposal" {
		links := make([]Link, 0, len(entries))
		for _, entry := range entries {
			links = append(links, Link{Kind: "test", State: "planned", EvidenceID: entry.ID, Level: entry.Level})
		}
		return links, choose(len(entries) > 0, "planned", "missing")
	}
	links := make([]Link, 0, len(anchors))
	for _, anchor := range anchors {
		links = append(links, resolvedLink(anchor))
	}
	linked := len(entries) > 0
	for _, entry := range entries {
		linked = linked && hasEvidenceAnchor(anchors, entry.ID)
	}
	return links, choose(linked, "linked", "missing")
}
