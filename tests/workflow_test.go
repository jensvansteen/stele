//go:build integration

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests guard this repository's two CI gates and the workflow that the
// user guide offers. They read the fixed structure of these files line by
// line, because the repository has no YAML library, and fail loudly when that
// structure changes. They are ordinary tests: YAML and documentation cannot
// carry Stele anchors.

const (
	selfGateRun   = "run: npm run verify:self"
	changeGateRun = "run: npm run stele -- check --all"
	guideCheckRun = "run: npx stele check --all"
	guideNodeLine = "node-version: 24"
	linuxRunner   = "runs-on: ubuntu-latest"
	selfGateJob   = "  self-verify:"
	changeGateJob = "  check-changes:"
	stepCondition = "if:"
	continueOnErr = "continue-on-error"
)

func TestContinuousIntegrationRunsBothGates(t *testing.T) {
	root := testRepositoryRoot(t)
	workflow := readText(t, filepath.Join(root, ".github", "workflows", "ci.yml"))
	if problems := ciGateProblems(workflow); len(problems) > 0 {
		t.Fatalf("ci.yml: %s", strings.Join(problems, "; "))
	}
	for name, broken := range map[string]string{
		"without verify:self":    strings.Replace(workflow, selfGateRun, "run: true", 1),
		"without the check step": strings.Replace(workflow, changeGateRun, "run: true", 1),
		"without the self job":   strings.Replace(workflow, selfGateJob, "  other:", 1),
		"without the check job":  strings.Replace(workflow, changeGateJob, "  other:", 1),
		"on macOS":               replaceInJob(workflow, changeGateJob, linuxRunner, "runs-on: macos-14"),
		"in a matrix":            replaceInJob(workflow, selfGateJob, linuxRunner, "runs-on: ${{ matrix.os }}"),
		"with a job condition": replaceInJob(workflow, changeGateJob, linuxRunner,
			linuxRunner+"\n    if: github.event_name == 'push'"),
		"allowed to fail": strings.Replace(workflow, changeGateRun,
			changeGateRun+"\n        continue-on-error: true", 1),
		"skipped by a condition": strings.Replace(workflow, selfGateRun,
			selfGateRun+"\n        if: github.event_name == 'push'", 1),
		"without pull requests": strings.Replace(workflow, "  pull_request:", "  workflow_dispatch:", 1),
		"without main pushes":   strings.Replace(workflow, "    branches: [main]", "    branches: [dev]", 1),
	} {
		if len(ciGateProblems(broken)) == 0 {
			t.Errorf("the ci.yml guard misses a workflow %s", name)
		}
	}

	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	readJSONFile(t, filepath.Join(root, "package.json"), &manifest)
	if manifest.Scripts["stele"] != "npm run build --silent && ./dist/stele" ||
		manifest.Scripts["verify:self"] != "npm run build --silent && stele check --specs --annotations=never" {
		t.Fatalf("package.json gate scripts: stele = %q, verify:self = %q",
			manifest.Scripts["stele"], manifest.Scripts["verify:self"])
	}
}

func TestContinuousIntegrationGuideOffersTheGate(t *testing.T) {
	guide := readText(t, filepath.Join(testRepositoryRoot(t), "docs", "guide", "continuous-integration.md"))
	if problems := guideWorkflowProblems(guide); len(problems) > 0 {
		t.Fatalf("continuous-integration.md: %s", strings.Join(problems, "; "))
	}
	for name, broken := range map[string]string{
		"without the check":  strings.Replace(guide, guideCheckRun, "run: npm test", 1),
		"with another Node":  strings.Replace(guide, guideNodeLine, "node-version: 22", 1),
		"with no YAML block": strings.ReplaceAll(guide, "```yaml", "```text"),
	} {
		if len(guideWorkflowProblems(broken)) == 0 {
			t.Errorf("the guide guard misses a workflow %s", name)
		}
	}
}

// replaceInJob replaces the first occurrence of old within one job.
func replaceInJob(workflow, job, old, replacement string) string {
	original := strings.TrimRight(block(workflow, job), "\n")
	return strings.Replace(workflow, original, strings.Replace(original, old, replacement, 1), 1)
}

// ciGateProblems lists what is missing from the two gates of ci.yml: each
// runs in its own Linux job, for pull requests and pushes to main, and can
// neither be skipped by a condition nor fail without failing its job.
func ciGateProblems(workflow string) []string {
	problems := make([]string, 0)
	triggers := block(workflow, "on:")
	if !containsLine(triggers, "  pull_request:") {
		problems = append(problems, "no pull_request trigger")
	}
	if !strings.Contains(triggers, "  push:\n    branches: [main]\n") {
		problems = append(problems, "no trigger for pushes to main")
	}
	jobs := block(workflow, "jobs:")
	for _, gate := range []struct{ job, run, name string }{
		{selfGateJob, selfGateRun, "verify:self"},
		{changeGateJob, changeGateRun, "the check"},
	} {
		job := block(jobs, gate.job)
		if job == "" {
			problems = append(problems, "no "+strings.TrimSpace(gate.job)+" job")
			continue
		}
		if !containsLine(job, linuxRunner) || strings.Contains(job, "matrix") {
			problems = append(problems, gate.name+" does not run on Linux only")
		}
		found := false
		for _, step := range jobSteps(job) {
			if !containsLine(step, gate.run) {
				continue
			}
			found = true
			if strings.Contains(step, stepCondition) || strings.Contains(step, continueOnErr) {
				problems = append(problems, gate.name+" may be skipped or fail without failing its job")
			}
		}
		if !found {
			problems = append(problems, "the "+strings.TrimSpace(gate.job)+" job does not "+gate.run)
		}
		if strings.Contains(strings.SplitN(job, "steps:", 2)[0], stepCondition) {
			problems = append(problems, "the "+strings.TrimSpace(gate.job)+" job has a condition")
		}
	}
	return problems
}

// guideWorkflowProblems lists what is missing from the guide's workflow.
func guideWorkflowProblems(guide string) []string {
	_, workflow, found := strings.Cut(guide, "```yaml\n")
	if !found {
		return []string{"no YAML workflow block"}
	}
	workflow, _, _ = strings.Cut(workflow, "```")
	problems := make([]string, 0)
	if !containsLine(workflow, guideCheckRun) {
		problems = append(problems, "the workflow does not run npx stele check --all")
	}
	if !containsLine(workflow, guideNodeLine) {
		problems = append(problems, "the workflow does not use Node 24")
	}
	return problems
}

// block returns the lines from a heading line to the next line with the same
// or less indentation.
func block(content, heading string) string {
	lines := strings.Split(content, "\n")
	indent := len(heading) - len(strings.TrimLeft(heading, " "))
	for index, line := range lines {
		if line != heading {
			continue
		}
		end := index + 1
		for end < len(lines) && (strings.TrimSpace(lines[end]) == "" ||
			len(lines[end])-len(strings.TrimLeft(lines[end], " ")) > indent) {
			end++
		}
		return strings.Join(lines[index:end], "\n") + "\n"
	}
	return ""
}

// jobSteps splits a job into its steps, each starting at "      - ".
func jobSteps(job string) []string {
	steps := make([]string, 0)
	for line := range strings.SplitSeq(job, "\n") {
		if strings.HasPrefix(line, "      - ") {
			steps = append(steps, "")
		}
		if len(steps) > 0 {
			steps[len(steps)-1] += line + "\n"
		}
	}
	return steps
}

// containsLine reports whether a line, without its indentation and after an
// optional list marker, is exactly the wanted text and is not a comment.
func containsLine(content, wanted string) bool {
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "- ")
		if trimmed == strings.TrimSpace(wanted) {
			return true
		}
	}
	return false
}

func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
