package stele

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type testGroup struct {
	Key         testGroupKey
	Selector    *string
	IDs         []string
	EvidenceIDs []string
}

type testGroupKey struct {
	Path     string
	Selector string
}

var runExactTest = executeExactTest

var errUnsupportedTestExtension = errors.New("unsupported test file extension")

var (
	computeScenarioDigest = ComputeInputDigest
	parseScenarioSpecs    = parseScopeSpecs
	scanScenarioAnchors   = ScanAnchors
)

func DiscoverScenarioTests(root, changeID string) ([]Anchor, error) {
	parsed, err := ParseSpecs(root, changeID)
	if err != nil {
		return nil, err
	}
	anchors, err := ScanAnchors(root)
	if err != nil {
		return nil, err
	}
	return selectScenarioTests(parsed, anchors), nil
}

func selectScenarioTests(parsed ParsedSpecs, anchors []Anchor) []Anchor {
	known := make(map[string]bool)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			known[scenario.ID] = true
		}
	}
	result := make([]Anchor, 0)
	for _, anchor := range anchors {
		if anchor.Kind == "test" && known[anchor.ID] {
			result = append(result, anchor)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return scenarioAnchorSortKey(result[i]) < scenarioAnchorSortKey(result[j])
	})
	return result
}

func scenarioAnchorSortKey(anchor Anchor) string {
	return anchor.Path + ":" + pointerValue(anchor.Selector) + ":" + anchor.ID
}

// evidenceSchemaVersion is the evidence file format: version 3 records the
// input digest of every execution.
const evidenceSchemaVersion = 3

// testRequest selects what `stele test` runs and where its evidence goes.
type testRequest struct {
	root         string
	scope        verificationScope
	evidencePath string
	// targets are requirement, scenario, or evidence IDs, or spec file paths.
	targets []string
	// merge keeps the stored evidence of tests that do not run now.
	merge bool
	// observer receives progress; nil shows none.
	observer testObserver
	// cancel, when set, stops the run: the language server sets it, the CLI
	// never does. A cancelled run writes no evidence.
	cancel context.Context
}

// notCancellable is the cancel context of a run that cannot be cancelled,
// such as every run of the CLI.
var notCancellable context.Context

// errRunCancelled reports a run stopped through its cancel context.
var errRunCancelled = errors.New("the test run was cancelled")

// testRun is the result of running a scope's tests: the scope's evidence and,
// for a targeted run, the executions that ran.
type testRun struct {
	evidence Evidence
	selected []TestExecution
	targeted bool
	// parsed holds the scope's specifications, and incomplete the batches
	// that did not complete, for the human report.
	parsed     ParsedSpecs
	incomplete []batchEvent
}

// @implements req.execution.f9056cdc6fe6
func RunScenarioTests(root, changeID, evidencePath string) (Evidence, error) {
	run, err := runScopeTests(testRequest{root: root, scope: changeScope(changeID), evidencePath: evidencePath})
	return run.evidence, err
}

func runScopeTests(request testRequest) (testRun, error) {
	root := request.root
	if err := requireScopeSpecs(root, request.scope); err != nil {
		return testRun{}, err
	}
	inputDigest, err := computeScenarioDigest(root)
	if err != nil {
		return testRun{}, err
	}
	parsed, err := parseScenarioSpecs(root, request.scope)
	if err != nil {
		return testRun{}, err
	}
	anchors, err := scanScenarioAnchors(root)
	if err != nil {
		return testRun{}, err
	}
	scopeTests := selectScenarioTests(parsed, anchors)
	groups := groupScenarioTests(scopeTests)
	run := testRun{targeted: len(request.targets) > 0}
	selectedGroups := groups
	if run.targeted {
		plan, _ := loadScopePlan(root, request.scope)
		selected, err := selectBehaviorTests(root, parsed, plan, scopeTests, request.targets)
		if err != nil {
			return testRun{}, err
		}
		selectedGroups = groupsOf(groups, selected)
	}
	observer := request.observer
	if observer == nil {
		observer = silentProgress{}
	}
	executions, incomplete := executeCancellableGroups(request.cancel, root, selectedGroups, inputDigest, observer,
		parsed)
	if request.cancel != nil && request.cancel.Err() != nil {
		return testRun{}, errRunCancelled
	}
	run.parsed, run.incomplete = parsed, incomplete
	if run.targeted {
		run.selected = executions
	}

	var stored Evidence
	merging := (request.merge || run.targeted) && request.evidencePath != "" &&
		readJSON(resolveWithin(root, request.evidencePath), &stored)
	if run.targeted {
		executions = withStoredExecutions(groups, executions, stored)
	}
	run.evidence = assembleEvidence(root, inputDigest, parsed, groups, executions)
	if request.evidencePath == "" {
		return run, nil
	}
	file := run.evidence
	if merging {
		file = mergeEvidence(stored, run.evidence)
	}
	if err := writeJSON(resolveWithin(root, request.evidencePath), file); err != nil {
		return testRun{}, err
	}
	return run, nil
}

var errUnknownTarget = errors.New("unknown test target")

// behaviorCatalog lists what a scope declares, for resolving test targets.
type behaviorCatalog struct {
	requirements map[string][]string
	scenarios    map[string]bool
	files        map[string][]string
	evidence     map[string]bool
}

func newBehaviorCatalog(parsed ParsedSpecs, plan LinkagePlan, tests []Anchor) behaviorCatalog {
	catalog := behaviorCatalog{
		requirements: make(map[string][]string),
		scenarios:    make(map[string]bool),
		files:        make(map[string][]string),
		evidence:     make(map[string]bool),
	}
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			catalog.requirements[requirement.ID] = append(catalog.requirements[requirement.ID], scenario.ID)
			catalog.scenarios[scenario.ID] = true
			catalog.files[scenario.Source.Path] = append(catalog.files[scenario.Source.Path], scenario.ID)
			for _, entry := range plan.Evidence[scenario.ID] {
				catalog.evidence[entry.ID] = true
			}
		}
	}
	for _, anchor := range tests {
		catalog.evidence[anchor.EvidenceID] = anchor.EvidenceID != ""
	}
	return catalog
}

// resolve returns the scenarios, or the one evidence entry, a target selects.
func (catalog behaviorCatalog) resolve(root string, parsed ParsedSpecs, target string) ([]string, string, error) {
	switch {
	case strings.HasPrefix(target, "req."):
		if identities, known := catalog.requirements[target]; known {
			return identities, "", nil
		}
		return nil, "", fmt.Errorf("%w %s: the scope declares no such requirement", errUnknownTarget, target)
	case catalog.scenarios[target]:
		return []string{target}, "", nil
	case catalog.evidence[target]:
		return nil, target, nil
	case strings.HasPrefix(target, "scn."):
		return nil, "", fmt.Errorf("%w %s: the scope declares no such scenario or evidence",
			errUnknownTarget, target)
	default:
		identities, err := specFileScenarios(root, target, parsed, catalog.files)
		return identities, "", err
	}
}

// selectBehaviorTests returns the scope's test anchors selected by the
// targets: a requirement ID selects its scenarios, a scenario ID the scenario,
// an evidence ID that entry, and a spec file path every scenario in the file.
// Every target must name something the scope declares.
//
// @implements req.linkindex.860a4d91b9fe
func selectBehaviorTests(
	root string,
	parsed ParsedSpecs,
	plan LinkagePlan,
	tests []Anchor,
	targets []string,
) ([]Anchor, error) {
	catalog := newBehaviorCatalog(parsed, plan, tests)
	selected := make(map[string]bool)
	for _, target := range targets {
		scenarios, evidence, err := catalog.resolve(root, parsed, target)
		if err != nil {
			return nil, err
		}
		for _, scenario := range scenarios {
			selected[scenario] = true
		}
		if evidence != "" {
			selected[evidence] = true
		}
	}
	result := make([]Anchor, 0)
	for _, anchor := range tests {
		if selected[anchor.ID] || (anchor.EvidenceID != "" && selected[anchor.EvidenceID]) {
			result = append(result, anchor)
		}
	}
	return result, nil
}

// specFileScenarios resolves a spec file target against the repository root
// and returns the scenarios the scope parsed from it.
func specFileScenarios(root, target string, parsed ParsedSpecs, files map[string][]string) ([]string, error) {
	absolute := resolveWithin(root, target)
	relative := repositoryPath(root, filepath.Clean(absolute))
	if !strings.HasPrefix(relative, "openspec/") || path.Base(relative) != "spec.md" {
		return nil, fmt.Errorf("%w %s: expected a requirement, scenario, or evidence ID, "+
			"or a spec.md file under openspec/", errUnknownTarget, target)
	}
	if !fileExists(absolute) {
		return nil, fmt.Errorf("%w %s: the file does not exist", errUnknownTarget, target)
	}
	if !slices.Contains(parsed.Files, relative) {
		return nil, fmt.Errorf("%w %s: the file is not part of the selected scope", errUnknownTarget, target)
	}
	return files[relative], nil
}

// groupsOf returns the groups whose test a selected anchor names, so a test
// shared by several scenarios runs once and records all of them.
func groupsOf(groups []testGroup, selected []Anchor) []testGroup {
	keys := make(map[testGroupKey]bool, len(selected))
	for _, anchor := range selected {
		keys[testGroupKey{Path: anchor.Path, Selector: pointerValue(anchor.Selector)}] = true
	}
	result := make([]testGroup, 0)
	for _, group := range groups {
		if keys[group.Key] {
			result = append(result, group)
		}
	}
	return result
}

func executionKey(execution TestExecution) testGroupKey {
	return testGroupKey{Path: execution.Path, Selector: pointerValue(execution.Selector)}
}

// withStoredExecutions completes the executions of a targeted run with the
// stored executions of the scope's other tests. Stored executions without
// their own digest take the digest of their file.
func withStoredExecutions(groups []testGroup, executions []TestExecution, stored Evidence) []TestExecution {
	fresh := make(map[testGroupKey]TestExecution, len(executions))
	for _, execution := range executions {
		fresh[executionKey(execution)] = execution
	}
	previous := make(map[testGroupKey]TestExecution, len(stored.Executions))
	for _, execution := range stored.Executions {
		if execution.InputDigest == "" {
			execution.InputDigest = stored.InputDigest
		}
		previous[executionKey(execution)] = execution
	}
	result := make([]TestExecution, 0, len(groups))
	for _, group := range groups {
		if execution, ran := fresh[group.Key]; ran {
			result = append(result, execution)
			continue
		}
		if execution, found := previous[group.Key]; found {
			execution.ScenarioIDs = append([]string{}, group.IDs...)
			execution.EvidenceIDs = group.EvidenceIDs
			result = append(result, execution)
		}
	}
	return result
}

// mergeEvidence combines a scope's evidence with the stored evidence of other
// tests and scenarios. Stored scenario outcomes from other inputs are stale,
// and the aggregate outcome covers every scenario in the file.
func mergeEvidence(stored, scope Evidence) Evidence {
	merged := scope
	keys := make(map[testGroupKey]bool, len(scope.Executions))
	for _, execution := range scope.Executions {
		keys[executionKey(execution)] = true
	}
	merged.Executions = append([]TestExecution{}, scope.Executions...)
	for _, execution := range stored.Executions {
		if keys[executionKey(execution)] {
			continue
		}
		if execution.InputDigest == "" {
			execution.InputDigest = stored.InputDigest
		}
		merged.Executions = append(merged.Executions, execution)
	}
	sort.Slice(merged.Executions, func(i, j int) bool {
		left, right := executionKey(merged.Executions[i]), executionKey(merged.Executions[j])
		return left.Path+"\x00"+left.Selector < right.Path+"\x00"+right.Selector
	})
	identities := make(map[string]bool, len(scope.Scenarios))
	for _, scenario := range scope.Scenarios {
		identities[scenario.ID] = true
	}
	merged.Scenarios = append([]ScenarioOutcome{}, scope.Scenarios...)
	for _, scenario := range stored.Scenarios {
		if identities[scenario.ID] {
			continue
		}
		if stored.InputDigest != scope.InputDigest {
			scenario = ScenarioOutcome{ID: scenario.ID, Outcome: "stale"}
		}
		merged.Scenarios = append(merged.Scenarios, scenario)
	}
	sort.Slice(merged.Scenarios, func(i, j int) bool { return merged.Scenarios[i].ID < merged.Scenarios[j].ID })
	merged.Outcome = aggregateScenarioOutcome(merged.Scenarios)
	return merged
}

// groupScenarioTests ensures scenarios sharing one exact test target execute once.
func groupScenarioTests(anchors []Anchor) []testGroup {
	groupIndexes := make(map[testGroupKey]int)
	groups := make([]testGroup, 0)
	for _, anchor := range anchors {
		key := testGroupKey{
			Path:     anchor.Path,
			Selector: pointerValue(anchor.Selector),
		}
		index, exists := groupIndexes[key]
		if !exists {
			index = len(groups)
			groupIndexes[key] = index
			groups = append(groups, testGroup{Key: key, Selector: anchor.Selector, IDs: []string{}})
		}
		groups[index].IDs = append(groups[index].IDs, anchor.ID)
		if anchor.EvidenceID != "" {
			groups[index].EvidenceIDs = append(groups[index].EvidenceIDs, anchor.EvidenceID)
		}
	}
	for index := range groups {
		sort.Strings(groups[index].IDs)
		groups[index].IDs = slices.Compact(groups[index].IDs)
		sort.Strings(groups[index].EvidenceIDs)
		groups[index].EvidenceIDs = slices.Compact(groups[index].EvidenceIDs)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Key.Path != groups[j].Key.Path {
			return groups[i].Key.Path < groups[j].Key.Path
		}
		return groups[i].Key.Selector < groups[j].Key.Selector
	})
	return groups
}

// runTestGroups runs the selected groups and returns their executions and the
// batches that did not complete. It runs batches; tests replace it with
// executeGroupsOneByOne, the per-test oracle.
var runTestGroups = executeGroupsInBatches

// executeTestGroups runs the groups and reports each test to the observer as
// it starts and finishes. Executions follow the order of the groups, so the
// order the tests ran in never reaches evidence.
func executeTestGroups(
	root string,
	groups []testGroup,
	inputDigest string,
	observer testObserver,
	parsed ParsedSpecs,
) ([]TestExecution, []batchEvent) {
	return executeCancellableGroups(notCancellable, root, groups, inputDigest, observer, parsed)
}

// executeCancellableGroups runs the groups like executeTestGroups; a set
// cancel context stops the running batch and starts no further batch.
func executeCancellableGroups(
	cancel context.Context,
	root string,
	groups []testGroup,
	inputDigest string,
	observer testObserver,
	parsed ParsedSpecs,
) ([]TestExecution, []batchEvent) {
	reporter := groupReporter{observer: observer, titles: make(map[string]string), cancel: cancel}
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			reporter.titles[scenario.ID] = scenario.Title
		}
	}
	files := make(map[string]int)
	for _, group := range groups {
		files[group.Key.Path]++
	}
	observer.planned(len(groups), files)
	byKey, incomplete := runTestGroups(root, groups, reporter)
	executions := make([]TestExecution, 0, len(groups))
	for _, group := range groups {
		execution := byKey[group.Key]
		execution.InputDigest = inputDigest
		executions = append(executions, execution)
	}
	return executions, incomplete
}

// groupReporter turns test groups into progress events.
type groupReporter struct {
	observer testObserver
	titles   map[string]string
	// cancel is the run's cancel context, nil when the run cannot be cancelled.
	cancel context.Context
}

// cancelled reports whether the run was cancelled.
func (reporter groupReporter) cancelled() bool {
	return reporter.cancel != nil && reporter.cancel.Err() != nil
}

func (reporter groupReporter) event(group testGroup, batch string) progressEvent {
	return progressEvent{
		group:       "default",
		batch:       batch,
		path:        group.Key.Path,
		selector:    group.Key.Selector,
		evidenceIDs: group.EvidenceIDs,
		scenarioIDs: group.IDs,
		title:       reporter.titles[firstOf(group.IDs, "")],
		level:       levelOf(firstOf(group.EvidenceIDs, "")),
	}
}

func (reporter groupReporter) started(group testGroup, batch string) {
	reporter.observer.started(reporter.event(group, batch))
}

func (reporter groupReporter) finished(group testGroup, batch string, execution TestExecution) {
	event := reporter.event(group, batch)
	event.outcome, event.reason = execution.Outcome, pointerValue(execution.Reason)
	reporter.observer.finished(event)
}

// batchEnded passes the end of a batch to observers that show it.
func (reporter groupReporter) batchEnded(event batchEvent) {
	if observer, shows := reporter.observer.(batchObserver); shows {
		observer.batchEnded(event)
	}
}

// executeGroupsOneByOne is the per-test oracle: one process per test, one at
// a time, through the same result readers as the batches.
func executeGroupsOneByOne(
	root string,
	groups []testGroup,
	reporter groupReporter,
) (map[testGroupKey]TestExecution, []batchEvent) {
	executions := make(map[testGroupKey]TestExecution, len(groups))
	for _, group := range groups {
		reporter.started(group, group.Key.Path)
		execution := executeTestGroup(root, group)
		executions[group.Key] = execution
		reporter.finished(group, group.Key.Path, execution)
	}
	return executions, nil
}

func executeTestGroup(root string, group testGroup) TestExecution {
	if group.Selector == nil {
		return groupExecution(group, exactResult{})
	}
	passed, executed, err := runExactTest(root, group.Key.Path, group.Key.Selector)
	return groupExecution(group, exactResult{passed: passed, executed: executed, err: err})
}

// groupExecution records a test's result for every scenario of its group.
func groupExecution(group testGroup, result exactResult) TestExecution {
	execution := TestExecution{
		Path:        group.Key.Path,
		Selector:    group.Selector,
		ScenarioIDs: append([]string{}, group.IDs...),
		EvidenceIDs: group.EvidenceIDs,
		Outcome:     "failed",
	}
	switch {
	case group.Selector == nil:
		execution.Reason = stringPointer("target-not-resolved")
	case errors.Is(result.err, errUnsupportedTestExtension):
		execution.Reason = stringPointer("unsupported-test-extension")
	case errors.Is(result.err, errTestSkipped):
		execution.Reason = stringPointer("test-skipped")
	case result.err != nil:
		execution.Reason = stringPointer("test-process-failed")
	case !result.executed:
		execution.Reason = stringPointer("test-not-executed")
	case result.passed:
		execution.Outcome = "passed"
	default:
		execution.Reason = stringPointer("test-process-failed")
	}
	return execution
}

func assembleEvidence(
	root, inputDigest string,
	parsed ParsedSpecs,
	groups []testGroup,
	executions []TestExecution,
) Evidence {
	scenarios := scenarioOutcomes(parsed, groups, executions, inputDigest)
	revision, _ := gitState(root)
	return Evidence{
		SchemaVersion:  evidenceSchemaVersion,
		Runner:         "stele-go/exact-scenario",
		TestedRevision: revision,
		InputDigest:    inputDigest,
		Outcome:        aggregateScenarioOutcome(scenarios),
		Scenarios:      scenarios,
		Executions:     executions,
	}
}

// outcomeRank orders scenario outcomes: a scenario takes the worst outcome of
// its tests, so it passes only when every linked test passed with the current
// inputs.
var outcomeRank = map[string]int{"": 0, "passed": 1, "not-run": 2, "stale": 3, "failed": 4}

// scenarioOutcomes decides each scenario's outcome from the executions of the
// scope's test groups. A group without an execution has not run, and an
// execution recorded for other inputs is stale.
func scenarioOutcomes(
	parsed ParsedSpecs,
	groups []testGroup,
	executions []TestExecution,
	inputDigest string,
) []ScenarioOutcome {
	byKey := make(map[testGroupKey]TestExecution, len(executions))
	for _, execution := range executions {
		byKey[executionKey(execution)] = execution
	}
	outcomes := make(map[string]string)
	failedEvidence := make(map[string][]string)
	for _, group := range groups {
		execution, found := byKey[group.Key]
		outcome := execution.Outcome
		switch {
		case !found:
			outcome = "not-run"
		case execution.InputDigest != inputDigest:
			outcome = "stale"
		}
		for _, id := range group.IDs {
			if outcomeRank[outcome] > outcomeRank[outcomes[id]] {
				outcomes[id] = outcome
			}
			if outcome == "failed" {
				failedEvidence[id] = append(failedEvidence[id], evidenceOfScenario(group.EvidenceIDs, id)...)
			}
		}
	}

	scenarios := make([]ScenarioOutcome, 0)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			outcome := outcomes[scenario.ID]
			if outcome == "" {
				outcome = "not-run"
			}
			failed := failedEvidence[scenario.ID]
			sort.Strings(failed)
			scenarios = append(scenarios, ScenarioOutcome{ID: scenario.ID, Outcome: outcome, FailedEvidence: failed})
		}
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })
	return scenarios
}

// evidenceOfScenario returns the evidence IDs that belong to one scenario.
func evidenceOfScenario(evidenceIDs []string, scenarioID string) []string {
	result := make([]string, 0)
	for _, evidence := range evidenceIDs {
		if strings.HasPrefix(evidence, scenarioID+".") {
			result = append(result, evidence)
		}
	}
	return result
}

func aggregateScenarioOutcome(scenarios []ScenarioOutcome) string {
	if len(scenarios) == 0 {
		return "failed"
	}
	for _, scenario := range scenarios {
		if scenario.Outcome != "passed" {
			return "failed"
		}
	}
	return "passed"
}

// executeExactTest runs one test alone in its own process, as a batch of one.
// It is the per-test oracle that batched runs are compared with.
func executeExactTest(root, path, selector string) (bool, bool, error) {
	group := testGroup{Key: testGroupKey{Path: path, Selector: selector}, Selector: &selector}
	plan := planTestBatches(root, []testGroup{group})
	result := exactResult{}
	if len(plan.batches) == 0 {
		result = plan.settled[0].result
	} else {
		result = runTestBatch(plan.batches[0], func(string, exactResult) {}).resultOf(selector)
	}
	return result.passed, result.executed, result.err
}

func unsupportedExtension(path string) error {
	return fmt.Errorf(
		"%w %q; supported extensions: .mts, .ts, _test.go",
		errUnsupportedTestExtension,
		filepath.Ext(path),
	)
}

// nodeBatchCommand runs the selected tests of one Node test file.
func nodeBatchCommand(batch testBatch) *exec.Cmd {
	command := exec.Command(
		"node",
		"--test",
		"--test-reporter=tap",
		"--test-name-pattern="+namePattern(batch.names()),
		batch.target,
	)
	command.Dir = batch.directory
	command.Env = nodeTestEnvironment()
	return command
}

// nodeTestEnvironment returns the environment for a linked Node test. Node sets
// NODE_TEST_CONTEXT for children of `node --test`; a nested test that inherits
// it reports to that outer runner instead of printing TAP for Stele.
//
// @implements req.execution.c27e3b85223e
func nodeTestEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NODE_TEST_CONTEXT=") {
			environment = append(environment, entry)
		}
	}
	return append(environment, "STELE_CHILD_TEST=1")
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPointer(value string) *string {
	return &value
}
