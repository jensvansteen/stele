package stele

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	evidencePlanVersion = 2
	v1RemovalRelease    = "0.2.0"
)

var evidenceLevels = []string{"unit", "integration", "e2e"}

var approvalVias = map[string]bool{"cli": true, "agent-confirmed": true}

// evidenceSuffixPattern matches the level suffix of an evidence ID, such as
// ".unit" or ".e2e.2".
var evidenceSuffixPattern = regexp.MustCompile(`^\.(unit|integration|e2e)(?:\.([1-9][0-9]*))?$`)

// evidencePlanFile is the on-disk form of a v2 linkage plan.
type evidencePlanFile struct {
	SchemaVersion int                         `json:"schemaVersion"`
	ChangeID      string                      `json:"changeId"`
	Scenarios     map[string]ScenarioEvidence `json:"scenarios"`
}

type rawLinkagePlan struct {
	SchemaVersion int                        `json:"schemaVersion"`
	ChangeID      string                     `json:"changeId"`
	Requirements  map[string]string          `json:"requirements"`
	Scenarios     map[string]json.RawMessage `json:"scenarios"`
}

var errInvalidPlan = errors.New("invalid linkage plan")

// parseLinkagePlan reads a v1 path-based plan or a v2 evidence plan. A v2 plan
// lists evidence entries per scenario and no code or test locations.
//
// @implements req.verificationstrategy.04519b98b8bb
func parseLinkagePlan(content []byte) (LinkagePlan, error) {
	var raw rawLinkagePlan
	if err := json.Unmarshal(content, &raw); err != nil {
		return LinkagePlan{}, fmt.Errorf("%w: %w", errInvalidPlan, err)
	}
	plan := emptyLinkagePlan()
	plan.SchemaVersion = raw.SchemaVersion
	plan.ChangeID = raw.ChangeID
	if raw.SchemaVersion == evidencePlanVersion {
		plan.Evidence = map[string][]EvidenceEntry{}
		plan.EvidenceOnly = true
		for id, value := range raw.Scenarios {
			var scenario ScenarioEvidence
			if err := json.Unmarshal(value, &scenario); err != nil {
				return LinkagePlan{}, fmt.Errorf("%w: scenario %s: %w", errInvalidPlan, id, err)
			}
			plan.Evidence[id] = scenario.Evidence
		}
		return plan, nil
	}
	maps.Copy(plan.Requirements, raw.Requirements)
	for id, value := range raw.Scenarios {
		var target string
		if err := json.Unmarshal(value, &target); err != nil {
			return LinkagePlan{}, fmt.Errorf("%w: scenario %s: %w", errInvalidPlan, id, err)
		}
		plan.Scenarios[id] = target
	}
	return plan, nil
}

// readLinkagePlan reads and parses a plan file.
func readLinkagePlan(path string) (LinkagePlan, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return LinkagePlan{}, err
	}
	return parseLinkagePlan(content)
}

// usesEvidence reports whether a scenario follows the v2 rules.
func (plan LinkagePlan) usesEvidence(scenarioID string) bool {
	if plan.EvidenceOnly {
		return true
	}
	_, listed := plan.Evidence[scenarioID]
	return listed
}

// requirementUsesEvidence reports whether a requirement follows the v2 rules:
// it has no v1 target and its scenarios are planned as evidence.
func (plan LinkagePlan) requirementUsesEvidence(requirement Requirement) bool {
	if plan.EvidenceOnly {
		return true
	}
	if _, planned := plan.Requirements[requirement.ID]; planned {
		return false
	}
	for _, scenario := range requirement.Scenarios {
		if plan.usesEvidence(scenario.ID) {
			return true
		}
	}
	return false
}

func deprecatedPlanDiagnostic(source string) Diagnostic {
	return diagnostic(
		"PLAN_V1_DEPRECATED",
		"warning",
		fmt.Sprintf(
			"Linkage plan %s uses schema version 1, which is supported until %s. Run stele plan migrate to convert it.",
			source,
			v1RemovalRelease,
		),
		source,
		1,
		"",
	)
}

// evidenceID builds the first evidence ID of a level for a scenario.
func evidenceID(scenarioID, level string) string {
	return scenarioID + "." + level
}

// splitEvidenceID returns the level and ordinal of an evidence ID that belongs
// to scenarioID, and false when the suffix is not a valid level suffix.
func splitEvidenceID(scenarioID, id string) (string, int, bool) {
	if !strings.HasPrefix(id, scenarioID+".") {
		return "", 0, false
	}
	match := evidenceSuffixPattern.FindStringSubmatch(id[len(scenarioID):])
	if match == nil {
		return "", 0, false
	}
	if match[2] == "" {
		return match[1], 1, true
	}
	ordinal, _ := strconv.Atoi(match[2])
	return match[1], ordinal, ordinal >= 2
}

// normalizedScenarioText collapses every run of whitespace, so whitespace-only
// edits keep an approval.
func normalizedScenarioText(scenario Scenario) string {
	return strings.Join(strings.Fields(scenario.Text), " ")
}

// evidenceDigest fingerprints what a reviewer approved: the entry's level and
// rationale together with the scenario's wording.
func evidenceDigest(scenario Scenario, entry EvidenceEntry) string {
	payload, _ := json.Marshal(struct {
		ID           string `json:"id"`
		Level        string `json:"level"`
		Rationale    string `json:"rationale"`
		ScenarioID   string `json:"scenarioId"`
		ScenarioText string `json:"scenarioText"`
	}{entry.ID, entry.Level, entry.Rationale, scenario.ID, normalizedScenarioText(scenario)})
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// approvalState returns "approved", "unapproved", or "stale" for an entry.
func approvalState(scenario Scenario, entry EvidenceEntry) string {
	switch {
	case entry.Approval == nil:
		return "unapproved"
	case entry.Approval.Digest != evidenceDigest(scenario, entry):
		return "stale"
	default:
		return "approved"
	}
}

// evidenceEntryProblem explains why an entry is malformed, or returns "".
func evidenceEntryProblem(scenario Scenario, entry EvidenceEntry, seen map[string]bool) string {
	level, _, suffixValid := splitEvidenceID(scenario.ID, entry.ID)
	switch {
	case !contains(evidenceLevels, entry.Level):
		return fmt.Sprintf("unknown level %q", entry.Level)
	case !suffixValid || level != entry.Level:
		expected := evidenceID(scenario.ID, entry.Level)
		return fmt.Sprintf("the ID must be %s, or %s.<n> with n of 2 or more", expected, expected)
	case strings.TrimSpace(entry.Rationale) == "":
		return "missing rationale"
	case seen[entry.ID]:
		return "duplicate evidence ID"
	case entry.Approval != nil && (entry.Approval.Approver == "" || entry.Approval.Date == "" ||
		!approvalVias[entry.Approval.Via]):
		return "approval needs an approver, a date, and via cli or agent-confirmed"
	}
	return ""
}

// planEntryDiagnostics validates a scenario's evidence entries and their
// approvals. It applies in both stages.
func planEntryDiagnostics(scenario Scenario, entries []EvidenceEntry) []Diagnostic {
	report := func(code, identity, message string) Diagnostic {
		return diagnostic(code, "error", message, scenario.Source.Path, scenario.Source.Line, identity)
	}
	if len(entries) == 0 {
		return []Diagnostic{report("PLAN_EVIDENCE_MISSING", scenario.ID,
			fmt.Sprintf("No planned evidence for %s.", scenario.ID))}
	}
	diagnostics := make([]Diagnostic, 0)
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.ID, scenario.ID+".") {
			diagnostics = append(diagnostics, report("PLAN_UNKNOWN_ID", entry.ID,
				fmt.Sprintf("Evidence %s is listed under %s but names another scenario.", entry.ID, scenario.ID)))
			continue
		}
		if problem := evidenceEntryProblem(scenario, entry, seen); problem != "" {
			diagnostics = append(diagnostics, report("PLAN_EVIDENCE_INVALID", entry.ID,
				fmt.Sprintf("Evidence %s is invalid: %s.", entry.ID, problem)))
			seen[entry.ID] = true
			continue
		}
		seen[entry.ID] = true
		switch approvalState(scenario, entry) {
		case "unapproved":
			diagnostics = append(diagnostics, report("PLAN_UNAPPROVED", entry.ID,
				fmt.Sprintf("Evidence %s (%s) is not approved.", entry.ID, entry.Level)))
		case "stale":
			diagnostics = append(diagnostics, report("PLAN_APPROVAL_STALE", entry.ID,
				fmt.Sprintf("Evidence %s changed since it was approved; approve it again.", entry.ID)))
		}
	}
	return diagnostics
}

// unknownPlanDiagnostics reports v2 plan scenarios that the scope does not declare.
// Removed behavior has its own diagnostic, so it is not reported again here.
func unknownPlanDiagnostics(
	plan LinkagePlan,
	scenarios map[string]bool,
	removed map[string]removedIdentity,
	source string,
) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	if !plan.EvidenceOnly {
		return diagnostics
	}
	for id := range plan.Evidence {
		if _, isRemoved := removed[id]; !scenarios[id] && !isRemoved {
			diagnostics = append(diagnostics, diagnostic(
				"PLAN_UNKNOWN_ID",
				"error",
				fmt.Sprintf("The plan lists %s, which this change does not declare as a scenario.", id),
				source,
				1,
				id,
			))
		}
	}
	return diagnostics
}

// plannedEvidence summarizes a scenario's entries for the report.
func plannedEvidence(scenario Scenario, entries []EvidenceEntry) []PlannedEvidence {
	result := make([]PlannedEvidence, 0, len(entries))
	for _, entry := range entries {
		result = append(result, PlannedEvidence{
			ID:        entry.ID,
			Level:     entry.Level,
			Approval:  approvalState(scenario, entry),
			Placement: entry.Placement,
		})
	}
	return result
}
