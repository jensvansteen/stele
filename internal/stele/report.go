package stele

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// reportItemLimit is how many findings, failed tests, and stale tests a group
// shows before "… N more (--details)".
const reportItemLimit = 5

// Marks of the human report.
const (
	markPass = "✓"
	markFail = "✗"
	markWarn = "!"
)

var evidenceIDPattern = regexp.MustCompile(
	`^((?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12})\.(?:[a-z][a-z0-9-]*\.)?(unit|integration|e2e)(?:\.\d+)?$`)

// humanReportInput is what a command knows when it prints its report.
type humanReportInput struct {
	command string
	scope   verificationScope
	// report is the verification report; test has none.
	report *Report
	// specs replaces the report's requirements when there is no report.
	specs []Requirement
	// executions are the tests to count, and currentDigest decides which of
	// them are stale.
	executions    []TestExecution
	currentDigest string
	testsRelevant bool
	// openSpec is the OpenSpec result of validate, and duration the time the
	// tests took, when they ran in this command.
	openSpec *bool
	duration *time.Duration
	passed   bool
	// notes are verdict remarks that do not fail the run.
	notes []string
	// incomplete are the test batches whose process did not complete.
	incomplete []batchEvent
}

// humanReport is the rendered content of the report, sorted when it is built.
type humanReport struct {
	header       string
	capabilities []capabilityRow
	checks       []checkLine
	matrix       *matrixView
	groups       []problemGroup
	failed       []testLine
	stale        []testLine
	incomplete   []batchEvent
	verdict      string
	passed       bool
}

type capabilityRow struct {
	name, scenarios, tests, plan, mark string
}

type checkLine struct {
	mark, name, detail string
	// reason explains a failed stage on the verdict line.
	reason string
}

type problemGroup struct {
	code, severity, meaning, fix string
	items                        []problemItem
}

// problemItem is one finding; path and line locate it for annotations.
type problemItem struct {
	id, title, location string
	path                string
	line                int
}

// testLine is one failed or stale test; path and line locate it for
// annotations.
type testLine struct {
	location, name, detail string
	path                   string
	line                   int
}

// behavior is what the report knows about one requirement or scenario.
type behavior struct {
	title, capability string
	source            Source
}

// reportIndex maps IDs to behavior, capabilities in specification order, and
// scenarios to their capability.
type reportIndex struct {
	behaviors    map[string]behavior
	capabilities []string
	scenarios    map[string][]string
	tests        map[string]int
}

// buildHumanReport turns a command's results into the report model. Every
// list is sorted here, so rendering never depends on collection order.
//
// @implements req.terminalreport.483a267b2c1a
func buildHumanReport(input humanReportInput) humanReport {
	index := newReportIndex(input)
	name, _, label := scopeName(input.scope)
	result := humanReport{
		header: fmt.Sprintf("stele %s · %s · %s", Version, input.command, label),
		passed: input.passed,
	}
	if input.scope.selected.active() {
		result.header += " · " + choose(len(input.scope.selected) == 1, "target ", "targets ") +
			strings.Join(input.scope.selected, ", ")
	}
	if input.report != nil && input.report.matrix != nil {
		result.matrix = newMatrixView(*input.report.matrix)
	}
	tests := classifyTests(input, index)
	diagnostics := reportDiagnostics(input)
	result.capabilities = capabilityRows(input, index, tests, diagnostics)
	result.checks = checkLines(input, tests, diagnostics)
	result.groups = problemGroups(index, diagnostics, scopeFlag(input.scope, name))
	result.failed, result.stale = tests.failedLines(index), tests.staleLines(index)
	result.incomplete = input.incomplete
	if !tests.ran && input.report != nil {
		result.failed = failedScenarios(*input.report)
	}
	result.verdict = verdictText(input, label, result.checks, diagnostics)
	return result
}

func scopeFlag(scope verificationScope, name string) string {
	if scope.currentSpecs {
		return "--specs"
	}
	return "--change " + name
}

func reportDiagnostics(input humanReportInput) []Diagnostic {
	if input.report == nil {
		return nil
	}
	return input.report.Diagnostics
}

func newReportIndex(input humanReportInput) reportIndex {
	index := reportIndex{behaviors: map[string]behavior{}, scenarios: map[string][]string{}, tests: map[string]int{}}
	add := func(id, title string, source Source) string {
		capability := capabilityOf(source.Path)
		index.behaviors[id] = behavior{title: title, capability: capability, source: source}
		if _, seen := index.scenarios[capability]; !seen {
			index.capabilities = append(index.capabilities, capability)
			index.scenarios[capability] = []string{}
		}
		return capability
	}
	requirements := input.specs
	if input.report != nil {
		requirements = requirementsOfReport(*input.report, index.tests)
	}
	for _, requirement := range requirements {
		add(requirement.ID, requirement.Title, requirement.Source)
		for _, scenario := range requirement.Scenarios {
			capability := add(scenario.ID, scenario.Title, scenario.Source)
			index.scenarios[capability] = append(index.scenarios[capability], scenario.ID)
		}
	}
	return index
}

// requirementsOfReport returns a report's requirements as parsed requirements
// and records the line of each linked test.
func requirementsOfReport(report Report, lines map[string]int) []Requirement {
	requirements := make([]Requirement, 0, len(report.Requirements))
	for _, requirement := range report.Requirements {
		converted := Requirement{ID: requirement.ID, Title: requirement.Title, Source: requirement.Source}
		for _, scenario := range requirement.Scenarios {
			converted.Scenarios = append(converted.Scenarios,
				Scenario{ID: scenario.ID, Title: scenario.Title, Source: scenario.Source})
			for _, link := range scenario.TestLinks {
				lines[link.Path+"\x00"+pointerValue(link.Selector)] = link.Line
			}
		}
		requirements = append(requirements, converted)
	}
	return requirements
}

// capabilityOf returns the capability of a specification path: the directory
// below the last specs/ segment.
func capabilityOf(path string) string {
	rooted := "/" + path
	start := strings.LastIndex(rooted, "/specs/")
	if start < 0 {
		return ""
	}
	within := rooted[start+len("/specs/"):]
	if slash := strings.LastIndex(within, "/"); slash >= 0 {
		return within[:slash]
	}
	return strings.TrimSuffix(within, ".md")
}

// baseIdentityOf returns the requirement or scenario an ID names, stripping an
// evidence level suffix.
func baseIdentityOf(id string) string {
	if match := evidenceIDPattern.FindStringSubmatch(id); match != nil {
		return match[1]
	}
	return id
}

// levelOf returns the evidence level of an evidence ID, or "".
func levelOf(id string) string {
	if match := evidenceIDPattern.FindStringSubmatch(id); match != nil {
		return match[2]
	}
	return ""
}

// testSummary classifies the counted tests.
type testSummary struct {
	passed, failed, stale []TestExecution
	levels                map[string]int
	ran                   bool
	lines                 map[string]int
}

func classifyTests(input humanReportInput, index reportIndex) testSummary {
	summary := testSummary{levels: map[string]int{}, ran: len(input.executions) > 0, lines: index.tests}
	executions := append([]TestExecution{}, input.executions...)
	sort.Slice(executions, func(i, j int) bool {
		left, right := executionKey(executions[i]), executionKey(executions[j])
		return left.Path+"\x00"+left.Selector < right.Path+"\x00"+right.Selector
	})
	for _, execution := range executions {
		switch {
		case input.currentDigest != "" && execution.InputDigest != input.currentDigest:
			summary.stale = append(summary.stale, execution)
		case execution.Outcome == "passed":
			summary.passed = append(summary.passed, execution)
		default:
			summary.failed = append(summary.failed, execution)
		}
		if len(execution.EvidenceIDs) > 0 {
			summary.levels[levelOf(execution.EvidenceIDs[0])]++
		}
	}
	return summary
}

// forCapability counts the tests that cover a capability's scenarios.
func (summary testSummary) forCapability(scenarios []string) (int, int, int) {
	covers := func(execution TestExecution) bool {
		for _, id := range execution.ScenarioIDs {
			if slices.Contains(scenarios, id) {
				return true
			}
		}
		return false
	}
	count := func(executions []TestExecution) int {
		total := 0
		for _, execution := range executions {
			if covers(execution) {
				total++
			}
		}
		return total
	}
	return count(summary.passed), count(summary.failed), count(summary.stale)
}

func (summary testSummary) total() int {
	return len(summary.passed) + len(summary.failed) + len(summary.stale)
}

func (summary testSummary) failedLines(index reportIndex) []testLine {
	lines := make([]testLine, 0, len(summary.failed))
	for _, execution := range summary.failed {
		lines = append(lines, summary.line(index, execution, pointerValue(execution.Reason)))
	}
	return lines
}

func (summary testSummary) staleLines(index reportIndex) []testLine {
	lines := make([]testLine, 0, len(summary.stale))
	for _, execution := range summary.stale {
		target := firstOf(execution.EvidenceIDs, firstOf(execution.ScenarioIDs, execution.Path))
		lines = append(lines, summary.line(index, execution, "run again: stele test "+target))
	}
	return lines
}

// firstOf returns the first value, or fallback for an empty list.
func firstOf(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return values[0]
}

func (summary testSummary) line(index reportIndex, execution TestExecution, remark string) testLine {
	location := execution.Path
	line := summary.lines[execution.Path+"\x00"+pointerValue(execution.Selector)]
	if line > 0 {
		location = fmt.Sprintf("%s:%d", execution.Path, line)
	}
	name := pointerValue(execution.Selector)
	if name == "" {
		name = "(no exact test name)"
	}
	details := append([]string{}, execution.EvidenceIDs...)
	if len(details) == 0 {
		details = append(details, execution.ScenarioIDs...)
	}
	if title := index.behaviors[firstOf(execution.ScenarioIDs, "")].title; title != "" {
		details = append(details, title)
	}
	if remark != "" {
		details = append(details, remark)
	}
	return testLine{
		location: location, name: name, detail: strings.Join(details, " · "),
		path: execution.Path, line: line,
	}
}

func capabilityRows(
	input humanReportInput,
	index reportIndex,
	tests testSummary,
	diagnostics []Diagnostic,
) []capabilityRow {
	approvals := planApprovals(input.report)
	rows := make([]capabilityRow, 0, len(index.capabilities))
	for _, capability := range index.capabilities {
		scenarios := index.scenarios[capability]
		passed, failed, stale := tests.forCapability(scenarios)
		approved, planned := 0, 0
		for _, id := range scenarios {
			approved += approvals[id][0]
			planned += approvals[id][1]
		}
		errors, warnings := 0, 0
		for _, item := range diagnostics {
			if diagnosticCapability(index, item) == capability {
				errors += boolCount(item.Severity == "error")
				warnings += boolCount(item.Severity == "warning")
			}
		}
		mark := markPass
		switch {
		case failed > 0 || errors > 0:
			mark = markFail
		case warnings > 0 || (input.testsRelevant && (stale > 0 || passed == 0)):
			mark = markWarn
		}
		rows = append(rows, capabilityRow{
			name:      capability,
			scenarios: fmt.Sprint(len(scenarios)),
			tests:     testCounts(passed, failed, stale),
			plan:      approvalText(approved, planned),
			mark:      mark,
		})
	}
	return rows
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func testCounts(passed, failed, stale int) string {
	if passed+failed+stale == 0 {
		return "not run"
	}
	parts := []string{fmt.Sprintf("%d passed", passed)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", stale))
	}
	return strings.Join(parts, ", ")
}

func approvalText(approved, planned int) string {
	if planned == 0 {
		return "–"
	}
	return fmt.Sprintf("%d/%d approved", approved, planned)
}

// planApprovals counts approved and planned evidence entries per scenario.
func planApprovals(report *Report) map[string][2]int {
	approvals := make(map[string][2]int)
	if report == nil {
		return approvals
	}
	for _, requirement := range report.Requirements {
		for _, scenario := range requirement.Scenarios {
			counts := [2]int{0, len(scenario.Evidence)}
			for _, entry := range scenario.Evidence {
				counts[0] += boolCount(entry.Approval == "approved")
			}
			approvals[scenario.ID] = counts
		}
	}
	return approvals
}

// diagnosticCapability attributes a diagnostic to a capability through its
// identity, or through its location when that is a specification file.
func diagnosticCapability(index reportIndex, item Diagnostic) string {
	if item.IdentityID != nil {
		if found, known := index.behaviors[baseIdentityOf(*item.IdentityID)]; known {
			return found.capability
		}
	}
	if item.Source != nil && strings.Contains("/"+item.Source.Path, "/specs/") {
		return capabilityOf(item.Source.Path)
	}
	return ""
}

func checkLines(input humanReportInput, tests testSummary, diagnostics []Diagnostic) []checkLine {
	lines := make([]checkLine, 0, 5)
	if input.openSpec != nil {
		lines = append(lines, checkLine{
			mark: choose(*input.openSpec, markPass, markFail),
			name: "OpenSpec strict validation", detail: choose(*input.openSpec, "passed", "failed"),
		})
	}
	if input.report != nil {
		report := *input.report
		approved, planned := 0, 0
		for _, counts := range planApprovals(input.report) {
			approved, planned = approved+counts[0], planned+counts[1]
		}
		planDetail := "no evidence entries"
		if planned > 0 {
			planDetail = fmt.Sprintf("%d/%d evidence entries approved", approved, planned)
		}
		lines = append(lines,
			stageLine(stageSpecifications, diagnostics,
				plural(report.Summary.Requirements, "requirement")+", "+plural(report.Summary.Scenarios, "scenario")),
			stageLine(stagePlan, diagnostics, planDetail),
			stageLine(stageLinkage, diagnostics, fmt.Sprintf("%d/%d requirements and %d/%d scenarios linked",
				report.Summary.LinkedRequirements, report.Summary.Requirements,
				report.Summary.LinkedScenarios, report.Summary.Scenarios)))
	}
	if input.testsRelevant {
		lines = append(lines, testExecutionLine(input, tests))
	}
	return lines
}

// stageLine marks a stage failed for its errors, warned for its warnings, and
// adds the short labels of its problems to the detail.
func stageLine(stage string, diagnostics []Diagnostic, detail string) checkLine {
	errors, warnings := stageCounts(stage, diagnostics)
	mark := markPass
	switch {
	case len(errors) > 0:
		mark = markFail
	case len(warnings) > 0:
		mark = markWarn
	}
	problems := append(append([]string{}, errors...), warnings...)
	if len(problems) > 0 {
		detail += "; " + strings.Join(problems, ", ")
	}
	return checkLine{mark: mark, name: stage, detail: detail, reason: strings.Join(errors, ", ")}
}

// stageCounts returns "<count> <short label>" for each code of a stage, for
// errors and for warnings, ordered by code.
func stageCounts(stage string, diagnostics []Diagnostic) ([]string, []string) {
	counts := map[string]map[string]int{"error": {}, "warning": {}}
	for _, item := range diagnostics {
		if guideFor(item.Code).stage == stage && counts[item.Severity] != nil {
			counts[item.Severity][item.Code]++
		}
	}
	describe := func(severity string) []string {
		codes := make([]string, 0, len(counts[severity]))
		for code := range counts[severity] {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		labels := make([]string, 0, len(codes))
		for _, code := range codes {
			labels = append(labels, fmt.Sprintf("%d %s", counts[severity][code],
				guideFor(code).label(counts[severity][code])))
		}
		return labels
	}
	return describe("error"), describe("warning")
}

func testExecutionLine(input humanReportInput, tests testSummary) checkLine {
	if !tests.ran {
		return scenarioExecutionLine(input.report)
	}
	parts := []string{fmt.Sprintf("%d/%d passed", len(tests.passed), tests.total())}
	if len(tests.failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(tests.failed)))
	}
	if len(tests.stale) > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", len(tests.stale)))
	}
	for _, level := range evidenceLevels {
		if count := tests.levels[level]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", level, count))
		}
	}
	if input.duration != nil {
		parts = append(parts, formatDuration(*input.duration))
	}
	mark := markPass
	switch {
	case len(tests.failed) > 0:
		mark = markFail
	case len(tests.stale) > 0:
		mark = markWarn
	}
	return checkLine{
		mark: mark, name: "Test execution", detail: strings.Join(parts, " · "),
		reason: plural(len(tests.failed), "failed test"),
	}
}

// scenarioExecutionLine describes execution from a report's scenario outcomes,
// for a verification without stored test executions.
func scenarioExecutionLine(report *Report) checkLine {
	if report == nil || report.Verdicts.Execution == "not-run" {
		return checkLine{mark: markWarn, name: "Test execution", detail: "not run"}
	}
	failed := len(failedScenarios(*report))
	detail := fmt.Sprintf("%d/%d scenarios passed; %s", report.Summary.PassedScenarios, report.Summary.Scenarios,
		report.Verdicts.Execution)
	mark := choose(report.Verdicts.Execution == "passed", markPass, markWarn)
	if failed > 0 {
		mark = markFail
	}
	return checkLine{mark: mark, name: "Test execution", detail: detail, reason: plural(failed, "failed scenario")}
}

// failedScenarios lists a report's failed scenarios as test lines.
func failedScenarios(report Report) []testLine {
	lines := make([]testLine, 0)
	for _, requirement := range report.Requirements {
		for _, scenario := range requirement.Scenarios {
			if scenario.Execution.Outcome == "failed" {
				lines = append(lines, testLine{
					location: fmt.Sprintf("%s:%d", scenario.Source.Path, scenario.Source.Line),
					name:     scenario.Title, detail: scenario.ID,
					path: scenario.Source.Path, line: scenario.Source.Line,
				})
			}
		}
	}
	return lines
}

// formatDuration prints a duration with one decimal of seconds.
func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%.1fs", duration.Seconds())
}

func problemGroups(index reportIndex, diagnostics []Diagnostic, flag string) []problemGroup {
	byCode := make(map[string]*problemGroup)
	keys := make([]string, 0)
	for _, item := range diagnostics {
		key := choose(item.Severity == "error", "0", "1") + item.Code
		group, exists := byCode[key]
		if !exists {
			guide := guideFor(item.Code)
			group = &problemGroup{
				code: item.Code, severity: item.Severity, meaning: guide.meaning,
				fix: guide.fixFor(flag),
			}
			byCode[key] = group
			keys = append(keys, key)
		}
		group.items = append(group.items, problemItemOf(index, item))
	}
	sort.Strings(keys)
	groups := make([]problemGroup, 0, len(keys))
	for _, key := range keys {
		group := *byCode[key]
		sort.SliceStable(group.items, func(i, j int) bool {
			left, right := group.items[i], group.items[j]
			return left.location+"\x00"+left.id < right.location+"\x00"+right.id
		})
		groups = append(groups, group)
	}
	return groups
}

func problemItemOf(index reportIndex, item Diagnostic) problemItem {
	result := problemItem{}
	var found behavior
	if item.IdentityID != nil {
		result.id = *item.IdentityID
		found = index.behaviors[baseIdentityOf(result.id)]
		result.title = found.title
	}
	source := item.Source
	if source == nil && found.source.Path != "" {
		source = &found.source
	}
	if source != nil {
		result.location = fmt.Sprintf("%s:%d", source.Path, source.Line)
		result.path, result.line = source.Path, source.Line
	}
	if result.id == "" {
		result.title = item.Message
	}
	return result
}

func verdictText(input humanReportInput, label string, checks []checkLine, diagnostics []Diagnostic) string {
	reasons := make([]string, 0)
	for _, check := range checks {
		if check.mark == markFail {
			name := check.name
			if !strings.HasPrefix(name, "OpenSpec") {
				name = strings.ToLower(name)
			}
			reasons = append(reasons, strings.TrimSuffix(name+" ("+check.reason+")", " ()"))
		}
	}
	notes := append([]string{}, input.notes...)
	if _, warnings, _ := diagnosticSummary(diagnostics); warnings > 0 {
		notes = append(notes, plural(warnings, "warning"))
	}
	text := "PASSED  " + label
	if !input.passed {
		text = "FAILED  " + label
		if len(reasons) > 0 {
			text += " — " + strings.Join(reasons, ", ")
		}
	}
	for _, note := range notes {
		text += " · " + note
	}
	return text
}

func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// reportStyle decides color and truncation.
type reportStyle struct {
	color   bool
	details bool
}

// paint colors a mark or a verdict word when color is on.
func (style reportStyle) paint(text string) string {
	if !style.color {
		return text
	}
	code := "32"
	switch text {
	case markFail, "FAILED":
		code = "31"
	case markWarn:
		code = "33"
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (style reportStyle) dim(text string) string {
	if !style.color || text == "" {
		return text
	}
	return "\x1b[2m" + text + "\x1b[0m"
}

// renderHumanReport prints the report model.
//
// @implements req.terminalreport.c2e0f6607fd6
func renderHumanReport(writer io.Writer, report humanReport, style reportStyle) {
	var out strings.Builder
	out.WriteString(report.header + "\n\n")
	renderCapabilities(&out, report.capabilities, style)
	for _, check := range report.checks {
		fmt.Fprintf(&out, "  %s %s%s\n", style.paint(check.mark), pad(check.name, 28), check.detail)
	}
	renderMatrix(&out, report.matrix, style)
	renderGroups(&out, "Errors", "error", report.groups, style)
	renderGroups(&out, "Warnings", "warning", report.groups, style)
	renderTests(&out, "Failed tests", markFail, report.failed, style)
	renderTests(&out, "Stale tests", markWarn, report.stale, style)
	renderIncomplete(&out, report.incomplete, style)
	out.WriteString("\n" + verdictLineText(report, style) + "\n")
	_, _ = io.WriteString(writer, out.String())
}

func verdictLineText(report humanReport, style reportStyle) string {
	word, rest, _ := strings.Cut(report.verdict, "  ")
	return style.paint(choose(report.passed, markPass, markFail)) + " " + style.paint(word) + "  " + rest
}

func renderCapabilities(out *strings.Builder, rows []capabilityRow, style reportStyle) {
	if len(rows) == 0 {
		return
	}
	header := capabilityRow{"Capability", "Scenarios", "Tests", "Plan", "Status"}
	widths := [4]int{}
	for _, row := range append([]capabilityRow{header}, rows...) {
		for column, value := range []string{row.name, row.scenarios, row.tests, row.plan} {
			widths[column] = max(widths[column], utf8.RuneCountInString(value))
		}
	}
	format := func(row capabilityRow, mark string) {
		fmt.Fprintf(out, "  %s  %s  %s  %s  %s\n", pad(row.name, widths[0]),
			strings.Repeat(" ", widths[1]-utf8.RuneCountInString(row.scenarios))+row.scenarios,
			pad(row.tests, widths[2]), pad(row.plan, widths[3]), mark)
	}
	format(header, header.mark)
	for _, row := range rows {
		format(row, style.paint(row.mark))
	}
	out.WriteString("\n")
}

func renderGroups(out *strings.Builder, title, severity string, groups []problemGroup, style reportStyle) {
	first := true
	for _, group := range groups {
		if group.severity != severity {
			continue
		}
		if first {
			out.WriteString("\n" + title + "\n")
			first = false
		}
		fmt.Fprintf(out, "  %s ×%d  %s\n    → Fix: %s\n", group.code, len(group.items), group.meaning, group.fix)
		shown := limited(len(group.items), style)
		idWidth, titleWidth := 0, 0
		for _, item := range group.items[:shown] {
			idWidth = max(idWidth, utf8.RuneCountInString(item.id))
			titleWidth = max(titleWidth, utf8.RuneCountInString(truncate(item.title, 48)))
		}
		for _, item := range group.items[:shown] {
			line := "    " + style.dim(item.location) + "  " + item.title
			if item.id != "" {
				line = "    " + pad(item.id, idWidth) + "  " + pad(truncate(item.title, 48), titleWidth) + "  " +
					style.dim(item.location)
			}
			out.WriteString(strings.TrimRight(line, " ") + "\n")
		}
		writeMore(out, len(group.items)-shown, "    ")
	}
}

func renderTests(out *strings.Builder, title, mark string, lines []testLine, style reportStyle) {
	if len(lines) == 0 {
		return
	}
	out.WriteString("\n" + title + "\n")
	shown := limited(len(lines), style)
	for _, line := range lines[:shown] {
		fmt.Fprintf(out, "  %s %s  %s\n      %s\n", style.paint(mark), style.dim(line.location), line.name, line.detail)
	}
	writeMore(out, len(lines)-shown, "  ")
}

// renderIncomplete names each test batch that did not complete, its exit
// status, and its last output lines.
func renderIncomplete(out *strings.Builder, batches []batchEvent, style reportStyle) {
	if len(batches) == 0 {
		return
	}
	out.WriteString("\nIncomplete test processes\n")
	for _, batch := range batches {
		fmt.Fprintf(out, "  %s %s  %s, %s without a result\n", style.paint(markFail), batch.label, batch.status,
			plural(batch.unreported, "test"))
		for _, line := range batch.tail {
			out.WriteString(strings.TrimRight("      "+style.dim(line), " ") + "\n")
		}
	}
}

// limited returns how many of count items to show.
func limited(count int, style reportStyle) int {
	if style.details {
		return count
	}
	return min(count, reportItemLimit)
}

func writeMore(out *strings.Builder, hidden int, indent string) {
	if hidden > 0 {
		fmt.Fprintf(out, "%s… %d more (--details)\n", indent, hidden)
	}
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(text)))
}

func truncate(text string, width int) string {
	if utf8.RuneCountInString(text) <= width {
		return text
	}
	return string([]rune(text)[:width-1]) + "…"
}

// renderVerdictOnly prints the final line for --quiet.
func renderVerdictOnly(writer io.Writer, report humanReport, style reportStyle) {
	_, _ = fmt.Fprintln(writer, verdictLineText(report, style))
}

// matrixRowLimit is how many scenario rows of the target matrix the report
// shows before "… N more (--details)".
const matrixRowLimit = 10

// matrixMarks are the report marks of the matrix cell states.
var matrixMarks = map[string]string{
	"passed": markPass + " passed", "failed": markFail + " failed", "missing": markFail + " missing",
	"unapproved": markWarn + " unapproved", "stale": "~ stale", "not-run": "○ not run",
	matrixNotApplicable: matrixNotApplicable,
}

// matrixView is the target matrix of the human report: a summary row per
// capability with passed and applicable scenarios per target, and the
// scenarios with a gap, or with --details every targeted scenario.
type matrixView struct {
	targets      []string
	capabilities [][]string
	gaps         [][]string
	all          [][]string
}

// newMatrixView turns a scope's matrix into report rows. Every row starts with
// its name, followed by one value per target.
//
// @implements req.terminalreport.c2e0f6607fd6
func newMatrixView(matrix TargetMatrix) *matrixView {
	view := &matrixView{targets: matrix.Targets}
	counts := make(map[string][][2]int)
	order := make([]string, 0)
	for _, row := range matrix.Rows {
		if _, seen := counts[row.Capability]; !seen {
			order = append(order, row.Capability)
			counts[row.Capability] = make([][2]int, len(matrix.Targets))
		}
		marks := []string{row.Title}
		gap := false
		for column, cell := range row.Cells {
			marks = append(marks, matrixMarks[cell.State])
			if cell.State != matrixNotApplicable {
				counts[row.Capability][column][1]++
				counts[row.Capability][column][0] += boolCount(cell.State == "passed")
			}
			gap = gap || (cell.State != "passed" && cell.State != matrixNotApplicable)
		}
		view.all = append(view.all, marks)
		if gap {
			view.gaps = append(view.gaps, marks)
		}
	}
	for _, capability := range order {
		row := []string{capability}
		for _, count := range counts[capability] {
			row = append(row, fmt.Sprintf("%d/%d", count[0], count[1]))
		}
		view.capabilities = append(view.capabilities, row)
	}
	return view
}

// renderMatrix prints the target matrix: capability rows, then scenario rows
// with a gap, at most ten of them unless --details lists every scenario.
func renderMatrix(out *strings.Builder, view *matrixView, style reportStyle) {
	if view == nil {
		return
	}
	scenarios := view.gaps
	if style.details {
		scenarios = view.all
	}
	shown := len(scenarios)
	if !style.details {
		shown = min(shown, matrixRowLimit)
	}
	width := utf8.RuneCountInString("Target matrix")
	for _, row := range view.capabilities {
		width = max(width, utf8.RuneCountInString(row[0]))
	}
	for _, row := range scenarios[:shown] {
		width = max(width, utf8.RuneCountInString(truncate(row[0], 48))+2)
	}
	widths := make([]int, len(view.targets))
	for column, target := range view.targets {
		widths[column] = utf8.RuneCountInString(target)
		for _, row := range append(append([][]string{}, view.capabilities...), scenarios[:shown]...) {
			widths[column] = max(widths[column], utf8.RuneCountInString(row[column+1]))
		}
	}
	painted := []string{markPass, markFail, markWarn}
	line := func(name string, values []string, paint bool) {
		var text strings.Builder
		text.WriteString("  " + pad(name, width))
		for column, value := range values {
			cell := pad(value, widths[column])
			if mark, rest, _ := strings.Cut(value, " "); paint && slices.Contains(painted, mark) {
				cell = style.paint(mark) + " " + pad(rest, widths[column]-utf8.RuneCountInString(mark)-1)
			}
			text.WriteString("  " + cell)
		}
		out.WriteString(strings.TrimRight(text.String(), " ") + "\n")
	}
	out.WriteString("\n")
	line("Target matrix", view.targets, false)
	for _, row := range view.capabilities {
		line(row[0], row[1:], false)
	}
	for _, row := range scenarios[:shown] {
		line("  "+truncate(row[0], 48), row[1:], true)
	}
	writeMore(out, len(scenarios)-shown, "  ")
}
