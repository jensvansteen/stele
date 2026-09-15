package stele

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type testGroup struct {
	Path     string
	Selector *string
	IDs      []string
}

var runExactTest = executeExactTest

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
		left, right := result[i].Path+":"+pointerValue(result[i].Selector)+":"+result[i].ID, result[j].Path+":"+pointerValue(result[j].Selector)+":"+result[j].ID
		return left < right
	})
	return result
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
	anchors = selectScenarioTests(parsed, anchors)
	groupsByKey := make(map[string]*testGroup)
	for _, anchor := range anchors {
		key := anchor.Path + "\x00" + pointerValue(anchor.Selector)
		if groupsByKey[key] == nil {
			groupsByKey[key] = &testGroup{Path: anchor.Path, Selector: anchor.Selector, IDs: []string{}}
		}
		groupsByKey[key].IDs = append(groupsByKey[key].IDs, anchor.ID)
	}
	groups := make([]*testGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		sort.Strings(group.IDs)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Path+":"+pointerValue(groups[i].Selector) < groups[j].Path+":"+pointerValue(groups[j].Selector)
	})
	outcomes := make(map[string]string)
	executions := make([]TestExecution, 0, len(groups))
	for _, group := range groups {
		outcome := "failed"
		reasonValue := "target-not-resolved"
		reason := &reasonValue
		if group.Selector != nil {
			passed, executed, runErr := runExactTest(root, group.Path, *group.Selector)
			switch {
			case runErr == nil && passed && executed:
				outcome = "passed"
				reason = nil
			case runErr == nil && !executed:
				reasonValue = "test-not-executed"
			default:
				reasonValue = "test-process-failed"
			}
		}
		for _, id := range group.IDs {
			outcomes[id] = outcome
		}
		executions = append(executions, TestExecution{Path: group.Path, Selector: group.Selector, ScenarioIDs: append([]string{}, group.IDs...), Outcome: outcome, Reason: reason})
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
	outcome := "failed"
	if len(scenarios) > 0 {
		allPassed := true
		for _, scenario := range scenarios {
			allPassed = allPassed && scenario.Outcome == "passed"
		}
		if allPassed {
			outcome = "passed"
		}
	}
	revision, _ := gitState(root)
	evidence := Evidence{SchemaVersion: 2, Runner: "stele-go/exact-scenario", TestedRevision: revision, InputDigest: inputDigest, Outcome: outcome, Scenarios: scenarios, Executions: executions}
	if evidencePath != "" {
		if err := writeJSON(resolveWithin(root, evidencePath), evidence); err != nil {
			return Evidence{}, err
		}
	}
	return evidence, nil
}

func executeExactTest(root, path, selector string) (bool, bool, error) {
	if strings.HasSuffix(path, "_test.go") {
		return executeGoTest(root, path, selector)
	}
	return executeNodeTest(root, path, selector)
}

func executeNodeTest(root, path, selector string) (bool, bool, error) {
	pattern := "^" + regexp.QuoteMeta(selector) + "$"
	command := exec.Command("node", "--test", "--test-reporter=tap", "--test-name-pattern="+pattern, filepath.ToSlash(path))
	command.Dir = root
	command.Env = append(os.Environ(), "STELE_CHILD_TEST=1")
	output, err := command.CombinedOutput()
	executed := false
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if regexp.MustCompile(`^ok \d+ - ` + regexp.QuoteMeta(selector) + `$`).MatchString(line) {
			executed = true
			break
		}
	}
	return err == nil && executed, executed, err
}

func executeGoTest(root, path, selector string) (bool, bool, error) {
	directory := filepath.Dir(path)
	command := exec.Command("go", "test", "-json", "./"+filepath.ToSlash(directory), "-run", "^"+regexp.QuoteMeta(selector)+"$")
	command.Dir = root
	output, err := command.CombinedOutput()
	executed, passed := false, false
	for line := range bytes.SplitSeq(output, []byte{'\n'}) {
		var event struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if json.Unmarshal(line, &event) == nil && event.Test == selector {
			executed = true
			if event.Action == "pass" {
				passed = true
			}
			if event.Action == "fail" {
				passed = false
			}
		}
	}
	return err == nil && executed && passed, executed, err
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
