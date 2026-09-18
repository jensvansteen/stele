package stele

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var marshalJSON = json.MarshalIndent

// reportSchemaVersion 2.1 adds separate linkage, execution, and overall verdicts.
const reportSchemaVersion = "2.1"

func readJSON(path string, target any) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(content, target) == nil
}

func plannedTarget(value string) (string, string) {
	path, selector, found := strings.Cut(value, "#")
	if !found {
		return value, ""
	}
	return path, selector
}

func matchesPlanned(anchor Anchor, value string) bool {
	path, selector := plannedTarget(value)
	return anchor.Path == path && anchor.Selector != nil && *anchor.Selector == selector
}

func gitState(root string) (string, bool) {
	revisionCommand := exec.Command("git", "rev-parse", "HEAD")
	revisionCommand.Dir = root
	revisionBytes, err := revisionCommand.Output()
	if err != nil {
		return "uncommitted", true
	}
	statusCommand := exec.Command("git", "status", "--porcelain")
	statusCommand.Dir = root
	statusBytes, statusErr := statusCommand.Output()
	return strings.TrimSpace(string(revisionBytes)), statusErr != nil || strings.TrimSpace(string(statusBytes)) != ""
}

type linkageValidationInput struct {
	Mode     string
	Parsed   ParsedSpecs
	Anchors  []Anchor
	Plan     LinkagePlan
	Declared map[string]bool
}

type reportBuildInput struct {
	Root        string
	ChangeID    string
	Mode        string
	InputDigest string
	Parsed      ParsedSpecs
	Anchors     []Anchor
	Plan        LinkagePlan
	Diagnostics []Diagnostic
	Evidence    *Evidence
}

type reportContribution struct {
	LinkedRequirement bool
	LinkedScenarios   int
	PassedScenarios   int
	ExecutionOutcomes []string
}

// verifyRequest selects the scope, stage, and output of a verification.
type verifyRequest struct {
	root       string
	scope      verificationScope
	mode       string
	reportPath string
	// evidence, when set, replaces the stored evidence file.
	evidence *Evidence
}

// defaultEvidencePath is where test runs store evidence and verification reads it.
const defaultEvidencePath = "artifacts/test-results.json"

// @implements req.verify.1d6031f2d3dd
func RunVerification(root, changeID, mode, reportPath string) (Report, error) {
	return verifyScope(verifyRequest{root: root, scope: changeScope(changeID), mode: mode, reportPath: reportPath})
}

func verifyScope(request verifyRequest) (Report, error) {
	root, scope, mode, reportPath := request.root, request.scope, request.mode, request.reportPath
	if err := requireScopeSpecs(root, scope); err != nil {
		return Report{}, err
	}
	parsed, err := parseScopeSpecs(root, scope)
	if err != nil {
		return Report{}, err
	}
	anchors, err := ScanAnchors(root)
	if err != nil {
		return Report{}, err
	}
	declared, err := scope.spec().DeclaredIdentities(root)
	if err != nil {
		return Report{}, err
	}
	plan, planDiagnostics := loadScopePlan(root, scope)
	parsed.Diagnostics = append(parsed.Diagnostics, planDiagnostics...)
	diagnostics := validateLinkage(linkageValidationInput{
		Mode:     mode,
		Parsed:   parsed,
		Anchors:  anchors,
		Plan:     plan,
		Declared: declared,
	})
	inputDigest, err := ComputeInputDigest(root)
	if err != nil {
		return Report{}, err
	}
	evidence := request.evidence
	var loaded Evidence
	if evidence == nil && readJSON(filepath.Join(root, defaultEvidencePath), &loaded) {
		evidence = &loaded
	}
	report := BuildReport(root, scope.changeID, mode, inputDigest, parsed, anchors, plan, diagnostics, evidence)
	if reportPath != "" {
		absolute := resolveWithin(root, reportPath)
		if err := writeJSON(absolute, report); err != nil {
			return Report{}, err
		}
	}
	return report, nil
}

func validateLinkage(input linkageValidationInput) []Diagnostic {
	diagnostics := append([]Diagnostic{}, input.Parsed.Diagnostics...)
	known := knownIdentities(input.Parsed)
	diagnostics = append(diagnostics, anchorDiagnostics(input.Mode, input.Anchors, known, input.Declared)...)
	diagnostics = append(diagnostics, unknownPlanDiagnostics(input.Plan, scenarioIdentities(input.Parsed),
		input.Plan.Source)...)
	for _, requirement := range input.Parsed.Requirements {
		diagnostics = append(diagnostics, requirementLinkDiagnostics(input, requirement)...)
		for _, scenario := range requirement.Scenarios {
			if input.Plan.usesEvidence(scenario.ID) {
				diagnostics = append(diagnostics, evidenceDiagnostics(input, scenario)...)
				continue
			}
			diagnostics = append(diagnostics, scenarioLinkDiagnostics(input, scenario)...)
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		return diagnosticKey(diagnostics[i]) < diagnosticKey(diagnostics[j])
	})
	return diagnostics
}

func knownIdentities(parsed ParsedSpecs) map[string]bool {
	known := make(map[string]bool)
	for _, requirement := range parsed.Requirements {
		if requirement.ID != "" {
			known[requirement.ID] = true
		}
		for _, scenario := range requirement.Scenarios {
			if scenario.ID != "" {
				known[scenario.ID] = true
			}
		}
	}
	return known
}

func scenarioIdentities(parsed ParsedSpecs) map[string]bool {
	scenarios := make(map[string]bool)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			scenarios[scenario.ID] = true
		}
	}
	return scenarios
}

func anchorDiagnostics(mode string, anchors []Anchor, known, declared map[string]bool) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, anchor := range anchors {
		if !known[anchor.ID] && declared[anchor.ID] {
			// The identity belongs to another change or to the current specifications.
			continue
		}
		if !known[anchor.ID] {
			diagnostics = append(diagnostics, diagnostic(
				"ANCHOR_DANGLING",
				"error",
				fmt.Sprintf("%s names undeclared identity %s.", anchor.Annotation, anchor.ID),
				anchor.Path,
				anchor.Line,
				anchor.ID,
			))
		}
		if (anchor.Annotation == "implements") != strings.HasPrefix(anchor.ID, "req.") {
			diagnostics = append(diagnostics, diagnostic(
				"ANCHOR_KIND",
				"error",
				fmt.Sprintf("%s cannot target %s.", anchor.Annotation, anchor.ID),
				anchor.Path,
				anchor.Line,
				anchor.ID,
			))
		}
		if mode == "implementation" && known[anchor.ID] && anchor.Selector == nil {
			diagnostics = append(diagnostics, diagnostic(
				"ANCHOR_TARGET_MISSING",
				"error",
				fmt.Sprintf("%s %s is not attached to a nearby compatible declaration.", anchor.Annotation, anchor.ID),
				anchor.Path,
				anchor.Line,
				anchor.ID,
			))
		}
	}
	return diagnostics
}

func requirementLinkDiagnostics(input linkageValidationInput, requirement Requirement) []Diagnostic {
	links := anchorsFor(input.Anchors, requirement.ID, "code")
	planned := input.Plan.Requirements[requirement.ID]
	evidencePlan := input.Plan.requirementUsesEvidence(requirement)
	if input.Mode == "proposal" {
		if planned == "" && !evidencePlan {
			return []Diagnostic{diagnostic(
				"PLAN_CODE_MISSING",
				"error",
				fmt.Sprintf("No planned code target for %s.", requirement.ID),
				requirement.Source.Path,
				requirement.Source.Line,
				requirement.ID,
			)}
		}
		return nil
	}
	if len(links) == 0 {
		return []Diagnostic{diagnostic(
			"LINK_CODE_MISSING",
			"error",
			fmt.Sprintf("No code anchor resolves for %s.", requirement.ID),
			requirement.Source.Path,
			requirement.Source.Line,
			requirement.ID,
		)}
	}
	if planned != "" && !anyPlannedMatch(links, planned) {
		return []Diagnostic{diagnostic(
			"LINK_TARGET_MISMATCH",
			"error",
			fmt.Sprintf("%s does not resolve to planned target %s.", requirement.ID, planned),
			requirement.Source.Path,
			requirement.Source.Line,
			requirement.ID,
		)}
	}
	return nil
}

func scenarioLinkDiagnostics(input linkageValidationInput, scenario Scenario) []Diagnostic {
	links := anchorsFor(input.Anchors, scenario.ID, "test")
	planned := input.Plan.Scenarios[scenario.ID]
	if input.Mode == "proposal" {
		if planned == "" {
			return []Diagnostic{diagnostic(
				"PLAN_TEST_MISSING",
				"error",
				fmt.Sprintf("No planned test target for %s.", scenario.ID),
				scenario.Source.Path,
				scenario.Source.Line,
				scenario.ID,
			)}
		}
		return nil
	}
	if len(links) == 0 {
		return []Diagnostic{diagnostic(
			"LINK_TEST_MISSING",
			"error",
			fmt.Sprintf("No test anchor resolves for %s.", scenario.ID),
			scenario.Source.Path,
			scenario.Source.Line,
			scenario.ID,
		)}
	}
	if planned != "" && !anyPlannedMatch(links, planned) {
		return []Diagnostic{diagnostic(
			"LINK_TARGET_MISMATCH",
			"error",
			fmt.Sprintf("%s does not resolve to planned target %s.", scenario.ID, planned),
			scenario.Source.Path,
			scenario.Source.Line,
			scenario.ID,
		)}
	}
	return nil
}

func anchorsFor(anchors []Anchor, identity, kind string) []Anchor {
	result := make([]Anchor, 0)
	for _, anchor := range anchors {
		if anchor.ID == identity && anchor.Kind == kind {
			result = append(result, anchor)
		}
	}
	return result
}

func anyPlannedMatch(anchors []Anchor, planned string) bool {
	for _, anchor := range anchors {
		if matchesPlanned(anchor, planned) {
			return true
		}
	}
	return false
}

func BuildReport(
	root string,
	changeID string,
	mode string,
	inputDigest string,
	parsed ParsedSpecs,
	anchors []Anchor,
	plan LinkagePlan,
	diagnostics []Diagnostic,
	evidence *Evidence,
) Report {
	return buildReport(reportBuildInput{
		Root:        root,
		ChangeID:    changeID,
		Mode:        mode,
		InputDigest: inputDigest,
		Parsed:      parsed,
		Anchors:     anchors,
		Plan:        plan,
		Diagnostics: diagnostics,
		Evidence:    evidence,
	})
}

func buildReport(input reportBuildInput) Report {
	revision, dirty := gitState(input.Root)
	outcomes := evidenceOutcomes(input.Evidence, input.InputDigest)
	report := initialReport(input, revision, dirty)
	report.Summary.Errors, report.Summary.Warnings, report.Verdict = diagnosticSummary(input.Diagnostics)
	report.Stages.Proposal.Status, report.Stages.Linkage.Status = stageSummary(
		input.Parsed.Diagnostics,
		input.Mode,
		report.Summary.Errors,
	)

	executionOutcomes := make([]string, 0)
	for _, requirement := range input.Parsed.Requirements {
		requirementReport, contribution := buildRequirementReport(input, requirement, outcomes)
		report.Requirements = append(report.Requirements, requirementReport)
		if contribution.LinkedRequirement {
			report.Summary.LinkedRequirements++
		}
		report.Summary.LinkedScenarios += contribution.LinkedScenarios
		report.Summary.PassedScenarios += contribution.PassedScenarios
		executionOutcomes = append(executionOutcomes, contribution.ExecutionOutcomes...)
	}
	report.Summary.Requirements = len(report.Requirements)
	report.Summary.Scenarios = len(executionOutcomes)
	report.Stages.Execution.Status = aggregateExecution(executionOutcomes)
	report.Verdicts = reportVerdicts(input.Mode, report.Verdict, report.Stages.Execution.Status)
	report.Verdict = report.Verdicts.Overall
	return report
}

// reportVerdicts keeps linkage and execution apart and derives the overall
// verdict: the proposal stage follows linkage; otherwise a linkage or
// execution failure fails, missing or stale evidence is incomplete, and only
// passing linkage with passing execution passes.
//
// @implements req.verify.27a52b8cfbd6
func reportVerdicts(mode, linkage, execution string) ReportVerdicts {
	verdicts := ReportVerdicts{Linkage: linkage, Execution: execution}
	switch {
	case mode == "proposal":
		verdicts.Overall = linkage
	case linkage == "fail" || execution == "failed":
		verdicts.Overall = "fail"
	case execution == "passed":
		verdicts.Overall = "pass"
	default:
		verdicts.Overall = "incomplete"
	}
	return verdicts
}

func evidenceOutcomes(evidence *Evidence, inputDigest string) map[string]string {
	outcomes := make(map[string]string)
	if evidence == nil {
		return outcomes
	}
	executionCurrent := evidence.InputDigest == inputDigest
	for _, scenario := range evidence.Scenarios {
		if executionCurrent {
			outcomes[scenario.ID] = scenario.Outcome
		} else {
			outcomes[scenario.ID] = "stale"
		}
	}
	return outcomes
}

func initialReport(input reportBuildInput, revision string, dirty bool) Report {
	report := Report{
		SchemaVersion: reportSchemaVersion,
		Complete:      true,
		Requirements:  []RequirementReport{},
		Diagnostics:   input.Diagnostics,
	}
	report.Verifier.Name = "stele"
	report.Verifier.Version = Version
	report.OpenSpec.Version = OpenSpecVersion
	report.OpenSpec.ChangeID = input.ChangeID
	report.Mode = input.Mode
	report.Repository.Revision = revision
	report.Repository.Dirty = dirty
	report.Repository.InputDigest = input.InputDigest
	report.Stages.Review.Status = "not-reviewed"
	report.Stages.Review.ReviewedRevision = nil
	if input.Evidence != nil {
		value := input.Evidence.TestedRevision
		report.Stages.Execution.TestedRevision = &value
	}
	return report
}

func diagnosticSummary(diagnostics []Diagnostic) (int, int, string) {
	errors := 0
	warnings := 0
	for _, item := range diagnostics {
		switch item.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}
	if errors > 0 {
		return errors, warnings, "fail"
	}
	return errors, warnings, "pass"
}

func buildRequirementReport(
	input reportBuildInput,
	requirement Requirement,
	outcomes map[string]string,
) (RequirementReport, reportContribution) {
	codeAnchors := anchorsFor(input.Anchors, requirement.ID, "code")
	codeLinks, linkage := linksForMode(input.Mode, "code", codeAnchors, input.Plan.Requirements[requirement.ID])
	if input.Mode == "proposal" && input.Plan.requirementUsesEvidence(requirement) {
		// A v2 plan does not list requirements; their anchors are checked later.
		linkage = "planned"
	}
	report := RequirementReport{
		ID:        requirement.ID,
		Title:     requirement.Title,
		Status:    "proposed",
		Source:    requirement.Source,
		CodeLinks: codeLinks,
		Linkage:   linkage,
		Scenarios: []ScenarioReport{},
	}
	contribution := reportContribution{LinkedRequirement: linkageComplete(linkage), ExecutionOutcomes: []string{}}
	for _, scenario := range requirement.Scenarios {
		scenarioReport, outcome := buildScenarioReport(input, scenario, outcomes)
		report.Scenarios = append(report.Scenarios, scenarioReport)
		if linkageComplete(scenarioReport.Linkage) {
			contribution.LinkedScenarios++
		}
		if outcome == "passed" {
			contribution.PassedScenarios++
		}
		contribution.ExecutionOutcomes = append(contribution.ExecutionOutcomes, outcome)
	}
	return report, contribution
}

func buildScenarioReport(
	input reportBuildInput,
	scenario Scenario,
	outcomes map[string]string,
) (ScenarioReport, string) {
	testAnchors := anchorsFor(input.Anchors, scenario.ID, "test")
	testLinks, linkage := linksForMode(input.Mode, "test", testAnchors, input.Plan.Scenarios[scenario.ID])
	var evidence []PlannedEvidence
	if input.Plan.usesEvidence(scenario.ID) {
		entries := input.Plan.Evidence[scenario.ID]
		testLinks, linkage = evidenceLinks(input.Mode, testAnchors, entries)
		evidence = plannedEvidence(scenario, entries)
	}
	outcome := outcomes[scenario.ID]
	if outcome == "" {
		outcome = "not-run"
	}
	return ScenarioReport{
		ID:        scenario.ID,
		Title:     scenario.Title,
		Source:    scenario.Source,
		TestLinks: testLinks,
		Linkage:   linkage,
		Execution: executionState(outcome),
		Evidence:  evidence,
	}, outcome
}

func linksForMode(mode, kind string, anchors []Anchor, planned string) ([]Link, string) {
	if mode == "proposal" {
		var target *string
		linkage := "missing"
		if planned != "" {
			value := planned
			target = &value
			linkage = "planned"
		}
		return []Link{{Kind: kind, State: "planned", Target: target}}, linkage
	}

	links := make([]Link, 0, len(anchors))
	for _, anchor := range anchors {
		links = append(links, resolvedLink(anchor))
	}
	if len(links) == 0 {
		return links, "missing"
	}
	return links, "linked"
}

func linkageComplete(linkage string) bool {
	return linkage == "linked" || linkage == "planned"
}

func executionState(outcome string) ExecutionState {
	state := "executed"
	switch outcome {
	case "not-run":
		state = "not-run"
	case "stale":
		state = "stale"
	}
	return ExecutionState{State: state, Outcome: outcome}
}

func stageSummary(proposalDiagnostics []Diagnostic, mode string, errors int) (string, string) {
	proposalStatus := "pass"
	if proposalErrors, _, _ := diagnosticSummary(proposalDiagnostics); proposalErrors > 0 {
		proposalStatus = "fail"
	}
	switch {
	case errors > 0:
		return proposalStatus, "fail"
	case mode == "proposal":
		return proposalStatus, "planned"
	default:
		return proposalStatus, "pass"
	}
}

func resolvedLink(anchor Anchor) Link {
	return Link{
		ID:              anchor.ID,
		Annotation:      anchor.Annotation,
		Kind:            anchor.Kind,
		Path:            anchor.Path,
		Line:            anchor.Line,
		Selector:        anchor.Selector,
		DeclarationLine: anchor.DeclarationLine,
		State:           "resolved",
		EvidenceID:      anchor.EvidenceID,
		Level:           anchor.Level,
	}
}

func aggregateExecution(outcomes []string) string {
	if len(outcomes) > 0 && every(outcomes, "passed") {
		return "passed"
	}
	if contains(outcomes, "failed") {
		return "failed"
	}
	if contains(outcomes, "stale") {
		return "stale"
	}
	return "not-run"
}

func every(values []string, target string) bool {
	for _, value := range values {
		if value != target {
			return false
		}
	}
	return true
}

func resolveWithin(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := MarshalDeterministic(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// @implements req.verify.aa9f017c4cb4
func MarshalDeterministic(value any) ([]byte, error) {
	content, err := marshalJSON(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
