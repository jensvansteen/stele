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
	"sort"
	"strings"
)

type testGroup struct {
	Key      testGroupKey
	Selector *string
	IDs      []string
}

type testGroupKey struct {
	Path     string
	Selector string
}

var runExactTest = executeExactTest

var errUnsupportedTestExtension = errors.New("unsupported test file extension")

var (
	computeScenarioDigest = ComputeInputDigest
	parseScenarioSpecs    = ParseSpecs
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

func RunScenarioTests(root, changeID, evidencePath string) (Evidence, error) {
	inputDigest, err := computeScenarioDigest(root)
	if err != nil {
		return Evidence{}, err
	}
	parsed, err := parseScenarioSpecs(root, changeID)
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
	}
	for index := range groups {
		sort.Strings(groups[index].IDs)
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
	for _, execution := range executions {
		for _, id := range execution.ScenarioIDs {
			outcomes[id] = execution.Outcome
		}
	}

	scenarios := make([]ScenarioOutcome, 0)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			outcome := outcomes[scenario.ID]
			if outcome == "" {
				outcome = "not-run"
			}
			scenarios = append(scenarios, ScenarioOutcome{ID: scenario.ID, Outcome: outcome})
		}
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })
	return scenarios
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
	default:
		return false, false, fmt.Errorf(
			"%w %q; supported extensions: .mts, .ts",
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
	command.Env = append(os.Environ(), "STELE_CHILD_TEST=1")
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

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPointer(value string) *string {
	return &value
}
