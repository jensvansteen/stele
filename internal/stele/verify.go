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

func RunVerification(root, changeID, mode, reportPath string) (Report, error) {
	parsed, err := ParseSpecs(root, changeID)
	if err != nil {
		return Report{}, err
	}
	anchors, err := ScanAnchors(root)
	if err != nil {
		return Report{}, err
	}
	plan := LinkagePlan{Requirements: map[string]string{}, Scenarios: map[string]string{}}
	_ = readJSON(filepath.Join(root, "artifacts", "linkage-plan.json"), &plan)
	diagnostics := append([]Diagnostic{}, parsed.Diagnostics...)
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
	for _, anchor := range anchors {
		if !known[anchor.ID] {
			diagnostics = append(diagnostics, diagnostic("ANCHOR_DANGLING", "error", fmt.Sprintf("%s names undeclared identity %s.", anchor.Annotation, anchor.ID), anchor.Path, anchor.Line, anchor.ID))
		}
		if (anchor.Annotation == "implements") != strings.HasPrefix(anchor.ID, "req.") {
			diagnostics = append(diagnostics, diagnostic("ANCHOR_KIND", "error", fmt.Sprintf("%s cannot target %s.", anchor.Annotation, anchor.ID), anchor.Path, anchor.Line, anchor.ID))
		}
		if mode == "implementation" && known[anchor.ID] && anchor.Selector == nil {
			diagnostics = append(diagnostics, diagnostic("ANCHOR_TARGET_MISSING", "error", fmt.Sprintf("%s %s is not attached to a nearby compatible declaration.", anchor.Annotation, anchor.ID), anchor.Path, anchor.Line, anchor.ID))
		}
	}
	for _, requirement := range parsed.Requirements {
		codeLinks := anchorsFor(anchors, requirement.ID, "code")
		planned := plan.Requirements[requirement.ID]
		if mode == "proposal" && planned == "" {
			diagnostics = append(diagnostics, diagnostic("PLAN_CODE_MISSING", "error", fmt.Sprintf("No planned code target for %s.", requirement.ID), requirement.Source.Path, requirement.Source.Line, requirement.ID))
		}
		if mode == "implementation" && len(codeLinks) == 0 {
			diagnostics = append(diagnostics, diagnostic("LINK_CODE_MISSING", "error", fmt.Sprintf("No code anchor resolves for %s.", requirement.ID), requirement.Source.Path, requirement.Source.Line, requirement.ID))
		}
		if mode == "implementation" && len(codeLinks) > 0 && planned != "" && !anyPlannedMatch(codeLinks, planned) {
			diagnostics = append(diagnostics, diagnostic("LINK_TARGET_MISMATCH", "error", fmt.Sprintf("%s does not resolve to planned target %s.", requirement.ID, planned), requirement.Source.Path, requirement.Source.Line, requirement.ID))
		}
		for _, scenario := range requirement.Scenarios {
			testLinks := anchorsFor(anchors, scenario.ID, "test")
			plannedTest := plan.Scenarios[scenario.ID]
			if mode == "proposal" && plannedTest == "" {
				diagnostics = append(diagnostics, diagnostic("PLAN_TEST_MISSING", "error", fmt.Sprintf("No planned test target for %s.", scenario.ID), scenario.Source.Path, scenario.Source.Line, scenario.ID))
			}
			if mode == "implementation" && len(testLinks) == 0 {
				diagnostics = append(diagnostics, diagnostic("LINK_TEST_MISSING", "error", fmt.Sprintf("No test anchor resolves for %s.", scenario.ID), scenario.Source.Path, scenario.Source.Line, scenario.ID))
			}
			if mode == "implementation" && len(testLinks) > 0 && plannedTest != "" && !anyPlannedMatch(testLinks, plannedTest) {
				diagnostics = append(diagnostics, diagnostic("LINK_TARGET_MISMATCH", "error", fmt.Sprintf("%s does not resolve to planned target %s.", scenario.ID, plannedTest), scenario.Source.Path, scenario.Source.Line, scenario.ID))
			}
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool { return diagnosticKey(diagnostics[i]) < diagnosticKey(diagnostics[j]) })
	inputDigest, err := ComputeInputDigest(root)
	if err != nil {
		return Report{}, err
	}
	var evidence *Evidence
	var loaded Evidence
	if readJSON(filepath.Join(root, "artifacts", "test-results.json"), &loaded) {
		evidence = &loaded
	}
	report := BuildReport(root, changeID, mode, inputDigest, parsed, anchors, plan, diagnostics, evidence)
	if reportPath != "" {
		absolute := resolveWithin(root, reportPath)
		if err := writeJSON(absolute, report); err != nil {
			return Report{}, err
		}
	}
	return report, nil
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

func BuildReport(root, changeID, mode, inputDigest string, parsed ParsedSpecs, anchors []Anchor, plan LinkagePlan, diagnostics []Diagnostic, evidence *Evidence) Report {
	revision, dirty := gitState(root)
	executionCurrent := evidence != nil && evidence.InputDigest == inputDigest
	outcomes := make(map[string]string)
	if evidence != nil {
		for _, scenario := range evidence.Scenarios {
			if executionCurrent {
				outcomes[scenario.ID] = scenario.Outcome
			} else {
				outcomes[scenario.ID] = "stale"
			}
		}
	}
	report := Report{SchemaVersion: "2.0", Complete: true, Requirements: []RequirementReport{}, Diagnostics: diagnostics}
	report.Verifier.Name = "stele"
	report.Verifier.Version = Version
	report.OpenSpec.Version = OpenSpecVersion
	report.OpenSpec.ChangeID = changeID
	report.Mode = mode
	report.Repository.Revision = revision
	report.Repository.Dirty = dirty
	report.Repository.InputDigest = inputDigest
	report.Stages.Review.Status = "not-reviewed"
	report.Stages.Review.ReviewedRevision = nil
	if evidence != nil {
		value := evidence.TestedRevision
		report.Stages.Execution.TestedRevision = &value
	}
	for _, item := range diagnostics {
		switch item.Severity {
		case "error":
			report.Summary.Errors++
		case "warning":
			report.Summary.Warnings++
		}
	}
	if report.Summary.Errors == 0 {
		report.Verdict = "pass"
	} else {
		report.Verdict = "fail"
	}
	if len(parsed.Diagnostics) == 0 {
		report.Stages.Proposal.Status = "pass"
	} else {
		report.Stages.Proposal.Status = "fail"
	}
	switch {
	case report.Summary.Errors != 0:
		report.Stages.Linkage.Status = "fail"
	case mode == "proposal":
		report.Stages.Linkage.Status = "planned"
	default:
		report.Stages.Linkage.Status = "pass"
	}
	executionOutcomes := make([]string, 0)
	for _, requirement := range parsed.Requirements {
		codeAnchors := anchorsFor(anchors, requirement.ID, "code")
		requirementReport := RequirementReport{ID: requirement.ID, Title: requirement.Title, Status: "proposed", Source: requirement.Source, CodeLinks: []Link{}, Scenarios: []ScenarioReport{}}
		if mode == "proposal" {
			target := plan.Requirements[requirement.ID]
			var targetPointer *string
			if target != "" {
				value := target
				targetPointer = &value
				requirementReport.Linkage = "planned"
			} else {
				requirementReport.Linkage = "missing"
			}
			requirementReport.CodeLinks = append(requirementReport.CodeLinks, Link{Kind: "code", State: "planned", Target: targetPointer})
		} else {
			for _, anchor := range codeAnchors {
				requirementReport.CodeLinks = append(requirementReport.CodeLinks, resolvedLink(anchor))
			}
			if len(codeAnchors) > 0 {
				requirementReport.Linkage = "linked"
			} else {
				requirementReport.Linkage = "missing"
			}
		}
		if requirementReport.Linkage == "linked" || requirementReport.Linkage == "planned" {
			report.Summary.LinkedRequirements++
		}
		for _, scenario := range requirement.Scenarios {
			testAnchors := anchorsFor(anchors, scenario.ID, "test")
			scenarioReport := ScenarioReport{ID: scenario.ID, Title: scenario.Title, Source: scenario.Source, TestLinks: []Link{}}
			if mode == "proposal" {
				target := plan.Scenarios[scenario.ID]
				var targetPointer *string
				if target != "" {
					value := target
					targetPointer = &value
					scenarioReport.Linkage = "planned"
				} else {
					scenarioReport.Linkage = "missing"
				}
				scenarioReport.TestLinks = append(scenarioReport.TestLinks, Link{Kind: "test", State: "planned", Target: targetPointer})
			} else {
				for _, anchor := range testAnchors {
					scenarioReport.TestLinks = append(scenarioReport.TestLinks, resolvedLink(anchor))
				}
				if len(testAnchors) > 0 {
					scenarioReport.Linkage = "linked"
				} else {
					scenarioReport.Linkage = "missing"
				}
			}
			outcome := outcomes[scenario.ID]
			if outcome == "" {
				outcome = "not-run"
			}
			scenarioReport.Execution = ExecutionState{State: "executed", Outcome: outcome}
			switch outcome {
			case "not-run":
				scenarioReport.Execution.State = "not-run"
			case "stale":
				scenarioReport.Execution.State = "stale"
			}
			if scenarioReport.Linkage == "linked" || scenarioReport.Linkage == "planned" {
				report.Summary.LinkedScenarios++
			}
			if outcome == "passed" {
				report.Summary.PassedScenarios++
			}
			executionOutcomes = append(executionOutcomes, outcome)
			requirementReport.Scenarios = append(requirementReport.Scenarios, scenarioReport)
		}
		report.Requirements = append(report.Requirements, requirementReport)
	}
	report.Summary.Requirements = len(report.Requirements)
	report.Summary.Scenarios = len(executionOutcomes)
	report.Stages.Execution.Status = aggregateExecution(executionOutcomes)
	return report
}

func resolvedLink(anchor Anchor) Link {
	return Link{ID: anchor.ID, Annotation: anchor.Annotation, Kind: anchor.Kind, Path: anchor.Path, Line: anchor.Line, Selector: anchor.Selector, DeclarationLine: anchor.DeclarationLine, State: "resolved"}
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

func MarshalDeterministic(value any) ([]byte, error) {
	content, err := marshalJSON(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
