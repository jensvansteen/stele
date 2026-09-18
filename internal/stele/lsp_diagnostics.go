package stele

import (
	"fmt"
	"sort"
)

// LSP diagnostic severities.
const (
	lspSeverityError       = 1
	lspSeverityWarning     = 2
	lspSeverityInformation = 3
)

// lspMessageInfo is the window/showMessage type of an information message.
const lspMessageInfo = 3

// lspFinding is one diagnostic of a file before it gets a range: a one-based
// line, the severity, the code, and the full message.
type lspFinding struct {
	line     int
	severity int
	code     string
	message  string
}

// lspBuild is the state the server answers from: the link index over the
// open documents and the saved files, and the findings of every file.
type lspBuild struct {
	index    Index
	findings map[string][]lspFinding
}

// lspScopeCheck is one served scope's verification report.
type lspScopeCheck struct {
	name   string
	flag   string
	report Report
}

// buildLSPProject checks every scope of a project over its snapshot, like
// `stele verify` does for each scope of --all, and indexes them. Staleness
// comes from the saved files, because tests run against them.
func buildLSPProject(files *lspFiles) (*lspBuild, error) {
	return buildLSPViews(files.root, files.view(true), files.view(false))
}

// buildLSPViews builds a project from its files with open documents, view,
// and its saved files.
func buildLSPViews(root string, view, saved repoFiles) (*lspBuild, error) {
	config, err := readConfigFrom(view, root)
	if err != nil {
		return nil, err
	}
	backend, err := resolveBackend(config.Adapter)
	if err != nil {
		return nil, err
	}
	backend = backendWithFiles(backend, view)
	shared, err := loadSharedVerifyInput(root, verificationScope{currentSpecs: true, backend: backend}, saved)
	if err != nil {
		return nil, err
	}
	input := IndexInput{
		Anchors: shared.anchors, Declared: shared.declared, Evidence: shared.evidence, InputDigest: shared.inputDigest,
	}
	checks := make([]lspScopeCheck, 0)
	for _, scope := range everyScope(root, backend) {
		scope.unannotated = config.UnannotatedSpecs
		loaded, err := scopeVerifyInput(root, scope, "implementation", shared)
		if err != nil {
			return nil, err
		}
		name, kind, _ := scopeName(scope)
		input.Scopes = append(input.Scopes, IndexScopeInput{
			Scope: IndexScope{ID: name, Kind: kind}, Parsed: loaded.linkage.Parsed, Plan: loaded.linkage.Plan,
		})
		if len(loaded.linkage.Parsed.Files) == 0 {
			// Like `stele verify`, a scope without specification files is not checked.
			continue
		}
		loaded.linkage.Mode = lspStage(scope, loaded.linkage.Plan)
		checks = append(checks, lspScopeCheck{name: name, flag: scopeFlag(scope, name), report: verifyLoaded(loaded)})
	}
	build := &lspBuild{index: BuildIndex(input)}
	build.findings = lspFindings(build.index, checks)
	return build, nil
}

// lspStage is the stage a scope is checked at in the editor: the current
// specifications at implementation, and a change at implementation once its
// plan has an approved entry and at proposal before that.
//
// @implements req.languageserver.c4e7f3210359
func lspStage(scope verificationScope, plan LinkagePlan) string {
	if scope.currentSpecs {
		return "implementation"
	}
	for _, entries := range plan.Evidence {
		for _, entry := range entries {
			if entry.Approval != nil {
				return "implementation"
			}
		}
	}
	return "proposal"
}

// lspFindings turns every scope's verification findings, except
// PLAN_UNAPPROVED, and the failed and stale outcomes into findings per file.
// A finding several scopes report at the same place is kept once, from the
// first scope.
//
// @implements req.languageserver.c4e7f3210359
func lspFindings(index Index, checks []lspScopeCheck) map[string][]lspFinding {
	findings := make(map[string][]lspFinding)
	seen := make(map[string]bool)
	add := func(path string, line int, item Diagnostic, flag string) {
		key := fmt.Sprintf("%s\x00%d\x00%s\x00%s", path, line, item.Code, item.Message)
		if seen[key] {
			return
		}
		seen[key] = true
		guide := guideFor(item.Code)
		findings[path] = append(findings[path], lspFinding{
			line:     line,
			severity: lspSeverity(item.Severity),
			code:     item.Code,
			message:  item.Message + "\n" + guide.meaning + "\nFix: " + guide.fixFor(flag),
		})
	}
	for _, check := range checks {
		for _, item := range check.report.Diagnostics {
			if item.Code == "PLAN_UNAPPROVED" {
				continue
			}
			if path, line, found := lspFindingSource(index, check.name, item); found {
				add(path, line, item, check.flag)
			}
		}
	}
	for _, scenario := range index.Scenarios {
		flag := choose(scenario.Scope == "specs", "--specs", "--change "+scenario.Scope)
		for _, item := range executionFindings(scenario) {
			add(scenario.Source.Path, scenario.Source.Line, item.Diagnostic, flag)
			for _, test := range item.tests {
				add(test.Path, test.Line, item.Diagnostic, flag)
			}
		}
	}
	for path := range findings {
		sort.SliceStable(findings[path], func(i, j int) bool {
			left, right := findings[path][i], findings[path][j]
			return fmt.Sprintf("%09d\x00%s\x00%s", left.line, left.code, left.message) <
				fmt.Sprintf("%09d\x00%s\x00%s", right.line, right.code, right.message)
		})
	}
	return findings
}

// lspFindingSource places a finding at its source, or without one at the
// heading of the identity it names in the scope.
func lspFindingSource(index Index, scope string, item Diagnostic) (string, int, bool) {
	if item.Source != nil {
		return item.Source.Path, item.Source.Line, true
	}
	if item.IdentityID == nil {
		return "", 0, false
	}
	source, found := indexHeading(index, scope, baseIdentityOf(*item.IdentityID))
	return source.Path, source.Line, found
}

// indexHeading returns the heading that declares an identity in a scope.
func indexHeading(index Index, scope, id string) (Source, bool) {
	for _, requirement := range index.Requirements {
		if requirement.ID == id && requirement.Scope == scope {
			return requirement.Source, true
		}
	}
	for _, scenario := range index.Scenarios {
		if scenario.ID == id && scenario.Scope == scope {
			return scenario.Source, true
		}
	}
	return Source{}, false
}

// executionFinding is an execution finding of one evidence entry with the
// tests it is also shown on.
type executionFinding struct {
	Diagnostic
	tests []IndexLocation
}

// executionFindings reports each evidence entry whose last outcome failed
// (EXECUTION_FAILED) or is stale (EXECUTION_STALE).
func executionFindings(scenario IndexScenario) []executionFinding {
	findings := make([]executionFinding, 0)
	for _, evidence := range scenario.Evidence {
		if evidence.Execution.Outcome == "failed" {
			findings = append(findings, executionFinding{Diagnostic: diagnostic("EXECUTION_FAILED", "warning",
				fmt.Sprintf("The last run of %s failed.", evidence.ID), "", 0, evidence.ID), tests: evidence.Tests})
		}
		if evidence.Execution.State == "stale" {
			findings = append(findings, executionFinding{Diagnostic: diagnostic("EXECUTION_STALE", "information",
				fmt.Sprintf("The last result of %s is stale: it was recorded before the verified inputs changed.",
					evidence.ID), "", 0, evidence.ID), tests: evidence.Tests})
		}
	}
	return findings
}

// lspSeverity maps a Stele severity to the protocol's.
func lspSeverity(severity string) int {
	switch severity {
	case "error":
		return lspSeverityError
	case "warning":
		return lspSeverityWarning
	default:
		return lspSeverityInformation
	}
}

// lspDiagnostics gives a file's findings their ranges: the whole line.
func lspDiagnostics(findings []lspFinding, lines []string, encoding string) []lspDiagnostic {
	diagnostics := make([]lspDiagnostic, 0, len(findings))
	for _, finding := range findings {
		diagnostics = append(diagnostics, lspDiagnostic{
			Range:    lspLineRange(lines, finding.line, encoding),
			Severity: finding.severity,
			Code:     finding.code,
			Source:   "stele",
			Message:  finding.message,
		})
	}
	return diagnostics
}
