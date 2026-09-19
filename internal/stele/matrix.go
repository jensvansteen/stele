package stele

import "slices"

// Matrix cell states, from the most to the least urgent: when the entries of
// a cell differ, the first state in this order wins.
var matrixStateOrder = []string{"failed", "missing", "unapproved", "stale", "not-run", "passed"}

// matrixNotApplicable marks a scenario that does not apply to a target.
const matrixNotApplicable = "n/a"

// TargetMatrix is the scenario by target matrix of one scope: a column per
// covered target, ordered by name, and a row per targeted scenario in
// specification order.
type TargetMatrix struct {
	Targets []string          `json:"targets"`
	Rows    []TargetMatrixRow `json:"rows"`
}

// TargetMatrixRow is one targeted scenario and its cell per target.
type TargetMatrixRow struct {
	Scenario    string             `json:"scenario"`
	Requirement string             `json:"requirement"`
	Title       string             `json:"title"`
	Capability  string             `json:"capability"`
	Cells       []TargetMatrixCell `json:"cells"`
}

// TargetMatrixCell is one scenario on one target: its state and the evidence
// IDs of its plan entries.
type TargetMatrixCell struct {
	Target   string   `json:"target"`
	State    string   `json:"state"`
	Evidence []string `json:"evidence"`
}

// matrixInput is what the matrix is computed from; computing does no I/O.
type matrixInput struct {
	parsed      ParsedSpecs
	plan        LinkagePlan
	anchors     []Anchor
	executions  map[testGroupKey]TestExecution
	inputDigest string
	targets     *projectTargets
	selection   targetSelection
}

// buildTargetMatrix computes a scope's matrix, or nil when the scope has no
// targeted specification. The index, the report, and later the language
// server all use it.
//
// @implements req.verificationtargets.4ed90b4188d7
func buildTargetMatrix(input matrixInput) *TargetMatrix {
	columns := matrixColumns(input)
	if len(columns) == 0 {
		return nil
	}
	matrix := &TargetMatrix{Targets: columns, Rows: []TargetMatrixRow{}}
	for _, requirement := range input.parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			if !scenario.Targeted || !input.selection.appliesTo(true, scenario.Targets) {
				continue
			}
			row := TargetMatrixRow{
				Scenario: scenario.ID, Requirement: requirement.ID, Title: scenario.Title,
				Capability: capabilityOf(scenario.Source.Path), Cells: make([]TargetMatrixCell, 0, len(columns)),
			}
			for _, target := range columns {
				row.Cells = append(row.Cells, matrixCell(input, scenario, target))
			}
			matrix.Rows = append(matrix.Rows, row)
		}
	}
	return matrix
}

// matrixColumns returns the configured targets that a specification of the
// scope declares, within the selection, ordered by name.
func matrixColumns(input matrixInput) []string {
	columns := make([]string, 0)
	for _, annotation := range input.parsed.Annotations {
		for _, target := range annotation.Targets {
			if annotation.TargetsDeclared && input.targets.configured(target) && input.selection.includes(target) &&
				!slices.Contains(columns, target) {
				columns = append(columns, target)
			}
		}
	}
	return sortedTargets(columns)
}

// matrixCell decides one cell: n/a when the scenario does not apply, missing
// without entries, and otherwise the most urgent state of its entries.
func matrixCell(input matrixInput, scenario Scenario, target string) TargetMatrixCell {
	cell := TargetMatrixCell{Target: target, State: matrixNotApplicable, Evidence: []string{}}
	if !slices.Contains(scenario.Targets, target) {
		return cell
	}
	cell.State = "missing"
	rank := slices.Index(matrixStateOrder, cell.State)
	entries := targetEntries(input.plan, scenario, target)
	for position, entry := range entries {
		cell.Evidence = append(cell.Evidence, entry.ID)
		state := entryMatrixState(input, scenario, entry)
		if entryRank := slices.Index(matrixStateOrder, state); position == 0 || entryRank < rank {
			cell.State, rank = state, entryRank
		}
	}
	return cell
}

// targetEntries returns a scenario's plan entries for one target.
func targetEntries(plan LinkagePlan, scenario Scenario, target string) []EvidenceEntry {
	entries := make([]EvidenceEntry, 0)
	for _, entry := range plan.Evidence[scenario.ID] {
		if entry.Target == target {
			entries = append(entries, entry)
		}
	}
	return entries
}

// entryMatrixState is the state of one plan entry: failed when its test
// failed, unapproved before approval, missing without a resolvable anchor,
// and otherwise its execution state.
func entryMatrixState(input matrixInput, scenario Scenario, entry EvidenceEntry) string {
	execution := entryExecution(input, scenario, entry)
	switch {
	case execution == "failed":
		return "failed"
	case approvalState(scenario, entry) != "approved":
		return "unapproved"
	case !hasEvidenceAnchor(anchorsFor(input.anchors, scenario.ID, "test"), entry.ID):
		return "missing"
	}
	return execution
}

// entryExecution returns failed, stale, not-run, or passed for the tests of
// one plan entry.
func entryExecution(input matrixInput, scenario Scenario, entry EvidenceEntry) string {
	tests := make([]Anchor, 0)
	for _, anchor := range anchorsFor(input.anchors, scenario.ID, "test") {
		if anchor.EvidenceID == entry.ID && anchor.Selector != nil {
			tests = append(tests, anchor)
		}
	}
	state := evidenceExecution(tests, input.executions, input.inputDigest)
	switch {
	case state.Outcome == "failed":
		return "failed"
	case state.State == "stale":
		return "stale"
	case state.State == "not-run":
		return "not-run"
	}
	return "passed"
}

// targetVerdicts gives each matrix column its own verdicts: linkage fails for
// errors of that target or of no target, and execution follows the target's
// entries.
//
// @implements req.verify.27a52b8cfbd6
func targetVerdicts(input matrixInput, matrix *TargetMatrix, mode string,
	diagnostics []Diagnostic,
) map[string]ReportVerdicts {
	verdicts := make(map[string]ReportVerdicts, len(matrix.Targets))
	for _, target := range matrix.Targets {
		linkage := "pass"
		for _, item := range diagnostics {
			if item.Severity == "error" && (item.target == "" || item.target == target) {
				linkage = "fail"
			}
		}
		verdicts[target] = reportVerdicts(mode, linkage, aggregateExecution(targetOutcomes(input, target)))
	}
	return verdicts
}

// targetOutcomes lists the execution outcome of every plan entry of a target.
func targetOutcomes(input matrixInput, target string) []string {
	outcomes := make([]string, 0)
	for _, requirement := range input.parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			if !slices.Contains(scenario.Targets, target) {
				continue
			}
			for _, entry := range targetEntries(input.plan, scenario, target) {
				outcomes = append(outcomes, entryExecution(input, scenario, entry))
			}
		}
	}
	return outcomes
}

// selectedScenarioOutcome is a targeted scenario's outcome over the entries
// of the selected targets, for a command with --target.
func selectedScenarioOutcome(input matrixInput, scenario Scenario) string {
	outcomes := make([]string, 0)
	for _, entry := range input.plan.Evidence[scenario.ID] {
		if input.selection.includes(entry.Target) && slices.Contains(scenario.Targets, entry.Target) {
			outcomes = append(outcomes, entryExecution(input, scenario, entry))
		}
	}
	return aggregateExecution(outcomes)
}
