package stele

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// @implements req.execution.f9056cdc6fe6
func RunScenarioTests(root, changeID, evidencePath string) (Evidence, error) {
	return runScopeTests(root, changeScope(changeID), evidencePath)
}

func runScopeTests(root string, scope verificationScope, evidencePath string) (Evidence, error) {
	if err := requireScopeSpecs(root, scope); err != nil {
		return Evidence{}, err
	}
	inputDigest, err := computeScenarioDigest(root)
	if err != nil {
		return Evidence{}, err
	}
	parsed, err := parseScenarioSpecs(root, scope)
	if err != nil {
		return Evidence{}, err
	}
	anchors, err := scanScenarioAnchors(root)
	if err != nil {
		return Evidence{}, err
	}
	groups := groupScenarioTests(selectScenarioTests(parsed, anchors))
	executions := executeTestGroups(root, groups)
	evidence := assembleEvidence(root, inputDigest, parsed, executions)
	if evidencePath != "" {
		if err := writeJSON(resolveWithin(root, evidencePath), evidence); err != nil {
			return Evidence{}, err
		}
	}
	return evidence, nil
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

func executeTestGroups(root string, groups []testGroup) []TestExecution {
	executions := make([]TestExecution, 0, len(groups))
	for _, group := range groups {
		executions = append(executions, executeTestGroup(root, group))
	}
	return executions
}

func executeTestGroup(root string, group testGroup) TestExecution {
	execution := TestExecution{
		Path:        group.Key.Path,
		Selector:    group.Selector,
		ScenarioIDs: append([]string{}, group.IDs...),
		EvidenceIDs: group.EvidenceIDs,
		Outcome:     "failed",
	}
	if group.Selector == nil {
		execution.Reason = stringPointer("target-not-resolved")
		return execution
	}

	passed, executed, err := runExactTest(root, group.Key.Path, group.Key.Selector)
	switch {
	case errors.Is(err, errUnsupportedTestExtension):
		execution.Reason = stringPointer("unsupported-test-extension")
	case errors.Is(err, errTestSkipped):
		execution.Reason = stringPointer("test-skipped")
	case err != nil:
		execution.Reason = stringPointer("test-process-failed")
	case !executed:
		execution.Reason = stringPointer("test-not-executed")
	case passed:
		execution.Outcome = "passed"
	default:
		execution.Reason = stringPointer("test-process-failed")
	}
	return execution
}

func assembleEvidence(root, inputDigest string, parsed ParsedSpecs, executions []TestExecution) Evidence {
	scenarios := scenarioOutcomes(parsed, executions)
	revision, _ := gitState(root)
	return Evidence{
		SchemaVersion:  2,
		Runner:         "stele-go/exact-scenario",
		TestedRevision: revision,
		InputDigest:    inputDigest,
		Outcome:        aggregateScenarioOutcome(scenarios),
		Scenarios:      scenarios,
		Executions:     executions,
	}
}

func scenarioOutcomes(parsed ParsedSpecs, executions []TestExecution) []ScenarioOutcome {
	outcomes := make(map[string]string)
	failedEvidence := make(map[string][]string)
	for _, execution := range executions {
		for _, id := range execution.ScenarioIDs {
			// A scenario passes only when every test linked to it passed.
			if outcomes[id] == "" || outcomes[id] == "passed" {
				outcomes[id] = execution.Outcome
			}
			if execution.Outcome != "passed" {
				failedEvidence[id] = append(failedEvidence[id], evidenceOfScenario(execution.EvidenceIDs, id)...)
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

func executeExactTest(root, path, selector string) (bool, bool, error) {
	switch filepath.Ext(path) {
	case ".mts", ".ts":
		return executeNodeTest(root, path, selector)
	case ".go":
		if strings.HasSuffix(path, "_test.go") {
			return executeGoTest(root, path, selector)
		}
		fallthrough
	default:
		return false, false, fmt.Errorf(
			"%w %q; supported extensions: .mts, .ts, _test.go",
			errUnsupportedTestExtension,
			filepath.Ext(path),
		)
	}
}

func executeNodeTest(root, path, selector string) (bool, bool, error) {
	pattern := "^" + regexp.QuoteMeta(selector) + "$"
	command := exec.Command(
		"node",
		"--test",
		"--test-reporter=tap",
		"--test-name-pattern="+pattern,
		filepath.ToSlash(path),
	)
	command.Dir = root
	command.Env = nodeTestEnvironment()
	output, err := command.CombinedOutput()
	executed := false
	passPattern := regexp.MustCompile(`^ok \d+ - ` + regexp.QuoteMeta(selector) + `$`)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if passPattern.MatchString(line) {
			executed = true
			break
		}
	}
	return err == nil && executed, executed, err
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
