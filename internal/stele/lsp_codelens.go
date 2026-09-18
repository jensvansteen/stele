package stele

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// lspLensLength is the length of an anchor's summary lens, in characters.
const lspLensLength = 120

// Command names the server executes.
const (
	commandShowStatus        = "stele.showStatus"
	commandShowSpecification = "stele.showSpecification"
	commandAddAnnotation     = "stele.addAnnotation"
)

// lspLens is a CodeLens before it gets a range: a one-based line, a title,
// and the command with the identity it shows, or the targets it runs in a
// scope.
type lspLens struct {
	line    int
	title   string
	command string
	id      string
	targets []string
	scope   string
}

// lspLenses returns a file's CodeLens items, ordered by line and then
// summary, run all, unit, integration, e2e, and status: on each requirement
// and scenario heading with planned evidence or tests, run actions and a
// status lens; on each anchor a summary lens, and on a test anchor a run
// action for its own evidence entry.
//
// @implements req.languageserver.a798154d936e
func lspLenses(index Index, path string) []lspLens {
	lenses := make([]lspLens, 0)
	for _, requirement := range index.Requirements {
		if requirement.Source.Path == path && requirement.ID != "" {
			lenses = append(lenses, requirementLenses(index, requirement)...)
		}
	}
	for _, scenario := range index.Scenarios {
		if scenario.Source.Path == path && scenario.ID != "" && len(scenario.Evidence) > 0 {
			lenses = append(lenses, headingLenses(scenario.Source.Line, scenario.ID, scenario.Scope,
				scenario.Evidence, scenarioStatus(scenario))...)
		}
	}
	lenses = append(lenses, anchorLenses(index, path)...)
	slices.SortStableFunc(lenses, func(left, right lspLens) int { return left.line - right.line })
	return lenses
}

// requirementLenses are a requirement heading's lenses, when any of its
// scenarios has planned evidence or tests.
func requirementLenses(index Index, requirement IndexRequirement) []lspLens {
	status, planned := requirementStatus(index, requirement)
	if !planned {
		return nil
	}
	evidence := make([]IndexEvidence, 0)
	for _, scenario := range index.Scenarios {
		if scenario.Requirement == requirement.ID && scenario.Scope == requirement.Scope {
			evidence = append(evidence, scenario.Evidence...)
		}
	}
	return headingLenses(requirement.Source.Line, requirement.ID, requirement.Scope, evidence, status)
}

// anchorLenses are a summary lens on each anchor of a file, in the first
// scope that declares it, and a run action on each test anchor.
func anchorLenses(index Index, path string) []lspLens {
	lenses := make([]lspLens, 0)
	seen := make(map[string]bool)
	for _, anchor := range index.Anchors {
		name := anchorName(Anchor{ID: anchor.ID, EvidenceID: anchor.EvidenceID})
		key := fmt.Sprintf("%d\x00%s", anchor.Line, name)
		if anchor.Path != path || anchor.Scope == "" || seen[key] {
			continue
		}
		seen[key] = true
		summary := summaryTitle(index, anchor.ID)
		lenses = append(lenses, lspLens{
			line: anchor.Line, title: summary, command: commandShowSpecification,
			id: anchor.ID,
		})
		if anchor.Kind == "test" {
			lenses = append(lenses, lspLens{
				line: anchor.Line, title: "▶ Run", command: commandRunTests, targets: []string{name},
				scope: anchor.Scope,
			})
		}
	}
	return lenses
}

// headingLenses are a heading's run actions, "Run all" and one per evidence
// level present, and its status lens.
func headingLenses(line int, id, scope string, evidence []IndexEvidence, status string) []lspLens {
	lenses := []lspLens{{
		line: line, title: "▶ Run all", command: commandRunTests, targets: []string{id}, scope: scope,
	}}
	for _, level := range evidenceLevels {
		targets := make([]string, 0)
		for _, entry := range evidence {
			if entry.Level == level {
				targets = append(targets, entry.ID)
			}
		}
		if len(targets) > 0 {
			lenses = append(lenses, lspLens{
				line: line, title: "▶ Run " + level, command: commandRunTests, targets: targets, scope: scope,
			})
		}
	}
	return append(lenses, lspLens{line: line, title: status, command: commandShowStatus, id: id})
}

// evidenceValue is one evidence entry's outcome for the combined status:
// failed, stale, not-run, or passed.
func evidenceValue(evidence IndexEvidence) string {
	switch {
	case evidence.Execution.Outcome == "failed":
		return "failed"
	case evidence.Execution.State == "stale":
		return "stale"
	default:
		return evidence.Execution.Outcome
	}
}

// scenarioValue combines a scenario's evidence with the rule of the execution
// verdict.
func scenarioValue(scenario IndexScenario) string {
	values := make([]string, 0, len(scenario.Evidence))
	for _, evidence := range scenario.Evidence {
		values = append(values, evidenceValue(evidence))
	}
	return aggregateExecution(values)
}

// statusMark is the short form of a combined outcome.
func statusMark(value string) string {
	switch value {
	case "passed", "stale":
		return "✓"
	case "failed":
		return "✗"
	default:
		return "– not run"
	}
}

// scenarioStatus is a scenario's status, such as `unit ✓ · e2e ✗ · stale`:
// each level's combined outcome, unapproved levels marked, and a stale suffix
// when any outcome is stale.
func scenarioStatus(scenario IndexScenario) string {
	levels := make([]string, 0)
	values := make(map[string][]string)
	unapproved := make(map[string]bool)
	stale := false
	for _, evidence := range scenario.Evidence {
		level := choose(evidence.Level == "", "tests", evidence.Level)
		if _, listed := values[level]; !listed {
			levels = append(levels, level)
		}
		values[level] = append(values[level], evidenceValue(evidence))
		unapproved[level] = unapproved[level] || evidence.Approval == "unapproved"
		stale = stale || evidence.Execution.State == "stale"
	}
	parts := make([]string, 0, len(levels)+1)
	for _, level := range levels {
		part := level + " " + statusMark(aggregateExecution(values[level]))
		if unapproved[level] {
			part += " (unapproved)"
		}
		parts = append(parts, part)
	}
	if stale {
		parts = append(parts, "stale")
	}
	return strings.Join(parts, " · ")
}

// requirementStatus combines a requirement's scenarios like the execution
// verdict and counts them, such as `not run · 1 passed · 1 not run`. It
// reports false when no scenario has planned evidence or tests.
func requirementStatus(index Index, requirement IndexRequirement) (string, bool) {
	values := make([]string, 0)
	unapproved := 0
	for _, scenario := range index.Scenarios {
		if scenario.Requirement != requirement.ID || scenario.Scope != requirement.Scope ||
			len(scenario.Evidence) == 0 {
			continue
		}
		values = append(values, scenarioValue(scenario))
		for _, evidence := range scenario.Evidence {
			if evidence.Approval == "unapproved" {
				unapproved++
			}
		}
	}
	if len(values) == 0 {
		return "", false
	}
	combined := aggregateExecution(values)
	parts := []string{combinedStatus[combined]}
	for _, value := range []string{"passed", "failed", "stale", "not-run"} {
		if count := countOf(values, value); count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, strings.ReplaceAll(value, "-", " ")))
		}
	}
	if unapproved > 0 {
		parts = append(parts, fmt.Sprintf("%d unapproved", unapproved))
	}
	return strings.Join(parts, " · "), true
}

// combinedStatus names a requirement's combined outcome.
var combinedStatus = map[string]string{
	"passed": "✓ passed", "failed": "✗ failed", "stale": "✓ stale", "not-run": "not run",
}

func countOf(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}

// summaryTitle summarizes the behavior an anchor names, from the first scope
// that declares it: a scenario's title and first WHEN and THEN steps, or a
// requirement's title and first line of text, cut to the lens length.
func summaryTitle(index Index, id string) string {
	title := ""
	for _, scenario := range index.Scenarios {
		if scenario.ID == id {
			title = "Scenario: " + scenario.Title + stepSummary(scenario.Steps)
			break
		}
	}
	for _, requirement := range index.Requirements {
		if requirement.ID == id && title == "" {
			line, _, _ := strings.Cut(strings.TrimSpace(requirement.Text), "\n")
			title = "Requirement: " + requirement.Title + choose(line == "", "", " — "+line)
		}
	}
	return truncateRunes(title, lspLensLength)
}

// stepSummary is " — WHEN <text> · THEN <text>" from a scenario's first WHEN
// and THEN steps.
func stepSummary(steps []ScenarioStep) string {
	parts := make([]string, 0, 2)
	for _, keyword := range []string{"WHEN", "THEN"} {
		for _, step := range steps {
			if step.Keyword == keyword {
				parts = append(parts, keyword+" "+step.Text)
				break
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " — " + strings.Join(parts, " · ")
}

// codeLens answers textDocument/codeLens.
func (server *lspServer) codeLens(raw json.RawMessage) ([]byte, error) {
	var params struct {
		TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, relative, lines, found := server.document(params.TextDocument.URI)
	if !found {
		return []byte("[]"), nil
	}
	lenses := make([]lspCodeLens, 0)
	for _, lens := range lspLenses(project.build.index, relative) {
		lenses = append(lenses, lspCodeLens{
			Range: lspLineRange(lines, lens.line, server.encoding),
			Command: lspCommand{Title: lens.title, Command: lens.command, Arguments: []any{
				lspCommandArguments{Root: fileURI(project.root), ID: lens.id, Targets: lens.targets, Scope: lens.scope},
			}},
		})
	}
	return json.Marshal(lenses)
}

// executeCommand runs one of the server's commands. Every command's logic is
// on the server; a client only forwards the click.
func (server *lspServer) executeCommand(raw json.RawMessage) ([]byte, error) {
	var params lspExecuteCommandParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if len(params.Arguments) != 1 {
		return nil, newRPCError(rpcInvalidParams, "%s takes one argument", params.Command)
	}
	argument := params.Arguments[0]
	switch params.Command {
	case commandShowStatus, commandShowSpecification:
		project, err := server.projectOfRoot(argument.Root)
		if err != nil {
			return nil, err
		}
		if params.Command == commandShowStatus {
			return server.showStatus(project, argument.ID)
		}
		return server.showSpecification(project, argument.ID)
	case commandAddAnnotation:
		return server.addAnnotation(argument.URI)
	case commandRunTests:
		return server.runTests(argument, params.WorkDoneToken)
	default:
		return nil, newRPCError(rpcInvalidParams, "unknown command %s", params.Command)
	}
}

// showStatus shows each evidence entry of a requirement or scenario as a
// message.
func (server *lspServer) showStatus(project *lspProject, id string) ([]byte, error) {
	source, found := declaringSource(project.build.index, id)
	if !found {
		return nil, newRPCError(rpcInvalidParams, "no specification declares %s", id)
	}
	scope := ""
	for _, requirement := range project.build.index.Requirements {
		if requirement.ID == id && requirement.Source == source {
			scope = requirement.Scope
		}
	}
	for _, scenario := range project.build.index.Scenarios {
		if scenario.ID == id && scenario.Source == source {
			scope = scenario.Scope
		}
	}
	target := lspTarget{id: id, scope: scope}
	server.sendNotification("window/showMessage", map[string]any{
		"type": lspMessageInfo, "message": lspHoverText(project.build.index, target, false),
	})
	return []byte("null"), nil
}

// showSpecification shows the heading that declares a behavior: in the
// editor when the client can show documents, and otherwise as a message with
// the plain-text card.
//
// @implements req.languageserver.a798154d936e
func (server *lspServer) showSpecification(project *lspProject, id string) ([]byte, error) {
	source, found := declaringSource(project.build.index, id)
	if !found {
		return nil, newRPCError(rpcInvalidParams, "no specification declares %s", id)
	}
	if server.client.Window.ShowDocument.Support {
		lines := lspLines(server.text(project, source.Path))
		server.sendRequest("window/showDocument", map[string]any{
			"uri":       fileURI(filepath.Join(project.root, filepath.FromSlash(source.Path))),
			"takeFocus": true,
			"selection": lspLineRange(lines, source.Line, server.encoding),
		})
		return []byte("null"), nil
	}
	server.sendNotification("window/showMessage", map[string]any{
		"type": lspMessageInfo, "message": lspHoverText(project.build.index, lspTarget{id: id}, false),
	})
	return []byte("null"), nil
}

// declaringSource returns the heading of the first scope that declares an
// identity.
func declaringSource(index Index, id string) (Source, bool) {
	places := declarationPlaces(index, id)
	if len(places) == 0 {
		return Source{}, false
	}
	return Source{Path: places[0].path, Line: places[0].line}, true
}
