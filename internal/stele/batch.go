package stele

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// errBatchIncomplete marks a test without a result in a batch whose process
// never reported the end of its run.
var errBatchIncomplete = errors.New("test process did not complete")

// batchTailLines is how many of a batch's last output lines the human report
// shows when the batch did not complete.
const batchTailLines = 20

// exactResult is the outcome of one exact test, as the per-test runner has
// always returned it: whether it passed, whether it ran, and why it failed.
type exactResult struct {
	passed   bool
	executed bool
	err      error
}

// testBatch is the selected tests that share one test process: the tests of
// one Go package with one set of build tags, or of one Node test file.
type testBatch struct {
	runner string
	// path is the package directory or the test file, relative to the root.
	path string
	tags []string
	// directory and target are the working directory and the package or file
	// argument of the command.
	directory string
	target    string
	groups    []testGroup
}

// label names the batch in progress and in the human report.
func (batch testBatch) label() string {
	if len(batch.tags) == 0 {
		return batch.path
	}
	return batch.path + " (-tags=" + strings.Join(batch.tags, ",") + ")"
}

// names returns the batch's exact test names, sorted and without duplicates.
func (batch testBatch) names() []string {
	names := make([]string, 0, len(batch.groups))
	for _, group := range batch.groups {
		names = append(names, group.Key.Selector)
	}
	sort.Strings(names)
	return slices.Compact(names)
}

// namePattern matches exactly the given names and nothing else.
func namePattern(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// command returns the batch's test process.
func (batch testBatch) command() *exec.Cmd {
	if batch.runner == "go" {
		return goBatchCommand(batch)
	}
	return nodeBatchCommand(batch)
}

// settledGroup is a selected test whose outcome is known without starting a
// process: its target is unresolved, its file has no runner or cannot be
// read, or its build constraint does not hold on this platform.
type settledGroup struct {
	group  testGroup
	result exactResult
}

type batchPlan struct {
	batches []testBatch
	settled []settledGroup
}

// planTestBatches puts every selected test in exactly one batch, or settles
// it without a process. Batches are ordered by path, then build tags.
//
// @implements req.execution.5aa38806b077
func planTestBatches(root string, groups []testGroup) batchPlan {
	plan := batchPlan{}
	indexes := make(map[string]int)
	for _, group := range groups {
		if group.Selector == nil {
			plan.settled = append(plan.settled, settledGroup{group: group})
			continue
		}
		batch, result, batched := planTestGroup(root, group)
		if !batched {
			plan.settled = append(plan.settled, settledGroup{group: group, result: result})
			continue
		}
		key := batch.path + "\x00" + strings.Join(batch.tags, ",")
		index, found := indexes[key]
		if !found {
			index = len(plan.batches)
			indexes[key] = index
			plan.batches = append(plan.batches, batch)
		}
		plan.batches[index].groups = append(plan.batches[index].groups, group)
	}
	sort.Slice(plan.batches, func(i, j int) bool {
		left, right := plan.batches[i], plan.batches[j]
		if left.path != right.path {
			return left.path < right.path
		}
		return strings.Join(left.tags, ",") < strings.Join(right.tags, ",")
	})
	return plan
}

// planTestGroup returns the batch a test belongs to, without its groups, or
// the test's settled result.
func planTestGroup(root string, group testGroup) (testBatch, exactResult, bool) {
	path := group.Key.Path
	switch {
	case filepath.Ext(path) == ".mts" || filepath.Ext(path) == ".ts":
		file := filepath.ToSlash(path)
		return testBatch{runner: "node", path: file, directory: root, target: file}, exactResult{}, true
	case strings.HasSuffix(path, "_test.go"):
		return planGoTest(root, path)
	default:
		return testBatch{}, exactResult{err: unsupportedExtension(path)}, false
	}
}

// batchOutcome is what a batch's process reported.
type batchOutcome struct {
	results   map[string]exactResult
	completed bool
	// status is the exit status, or why the process did not start.
	status string
	// tail is the last output lines, for the human report only.
	tail []string
}

// resultOf returns a test's result. A test without one did not run when the
// batch completed, and has no result because of the crash otherwise.
func (outcome batchOutcome) resultOf(name string) exactResult {
	if result, found := outcome.results[name]; found {
		return result
	}
	if outcome.completed {
		return exactResult{}
	}
	return exactResult{err: errBatchIncomplete}
}

// batchLine is what one output line of a batch reveals.
type batchLine struct {
	name    string
	result  exactResult
	found   bool
	display string
	shown   bool
}

// batchReader decides per-test results and completion from a batch's output,
// one line at a time. It reports each test's first final result only.
type batchReader interface {
	read(line string) batchLine
	completed() bool
}

func newBatchReader(batch testBatch) batchReader {
	names := make(map[string]bool)
	for _, name := range batch.names() {
		names[name] = name != ""
	}
	if batch.runner == "go" {
		return &goBatchReader{names: names, reported: map[string]bool{}}
	}
	return &nodeBatchReader{file: batch.target, names: names, reported: map[string]bool{}}
}

// goBatchReader reads `go test -json`: the pass, fail, or skip event of each
// exact top-level test, and the test binary's final PASS or FAIL line.
type goBatchReader struct {
	names    map[string]bool
	reported map[string]bool
	done     bool
}

func (reader *goBatchReader) read(line string) batchLine {
	var event goTestEvent
	if json.Unmarshal([]byte(line), &event) != nil {
		return batchLine{display: line, shown: true}
	}
	result := batchLine{}
	if event.Action == "output" {
		result.display, result.shown = strings.TrimSuffix(event.Output, "\n"), true
		if event.Test == "" && (event.Output == "PASS\n" || event.Output == "FAIL\n") {
			reader.done = true
		}
	}
	if !reader.names[event.Test] || reader.reported[event.Test] {
		return result
	}
	switch event.Action {
	case "pass":
		result.result = exactResult{passed: true, executed: true}
	case "fail":
		result.result = exactResult{executed: true}
	case "skip":
		result.result = exactResult{executed: true, err: errTestSkipped}
	default:
		return result
	}
	reader.reported[event.Test] = true
	result.name, result.found = event.Test, true
	return result
}

func (reader *goBatchReader) completed() bool {
	return reader.done
}

// tapResultPattern matches a TAP test point at any indentation, with an
// optional SKIP or TODO directive.
var tapResultPattern = regexp.MustCompile(`^(not )?ok \d+ - (.*?)(?: # (SKIP|TODO)\b.*)?$`)

// nodeBatchReader reads Node TAP: `ok`, `not ok`, and `# SKIP` test points,
// the `# tests` summary, and a failed entry named after the test file, which
// Node prints when the file's process ended early.
type nodeBatchReader struct {
	file       string
	names      map[string]bool
	reported   map[string]bool
	summary    bool
	fileFailed bool
}

func (reader *nodeBatchReader) read(line string) batchLine {
	result := batchLine{display: line, shown: true}
	if strings.HasPrefix(line, "# tests ") {
		reader.summary = true
	}
	match := tapResultPattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return result
	}
	failed, name, directive := match[1] != "", match[2], match[3]
	if failed && (name == reader.file || strings.HasSuffix(name, "/"+reader.file)) {
		reader.fileFailed = true
	}
	if !reader.names[name] || reader.reported[name] {
		return result
	}
	switch {
	case directive == "SKIP":
		result.result = exactResult{executed: true, err: errTestSkipped}
	case failed:
		result.result = exactResult{executed: true}
	default:
		result.result = exactResult{passed: true, executed: true}
	}
	reader.reported[name] = true
	result.name, result.found = name, true
	return result
}

func (reader *nodeBatchReader) completed() bool {
	return reader.summary && !reader.fileFailed
}

// batchProcess is a started test process: its standard output, and a wait
// that returns once the process ended and its output was read.
type batchProcess struct {
	output io.Reader
	wait   func() error
}

// startBatchProcess starts a batch's command, with standard error going to
// the given writer. Tests replace it with a fake output stream.
var startBatchProcess = startCommand

func startCommand(command *exec.Cmd, stderr io.Writer) (batchProcess, error) {
	output, err := command.StdoutPipe()
	if err != nil {
		return batchProcess{}, err
	}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return batchProcess{}, err
	}
	return batchProcess{output: output, wait: command.Wait}, nil
}

// runTestBatch runs one batch and reports each test's result as soon as the
// output shows it, so a failure is known before the batch ends.
//
// @implements req.execution.5aa38806b077
func runTestBatch(batch testBatch, onResult func(name string, result exactResult)) batchOutcome {
	outcome := batchOutcome{results: map[string]exactResult{}}
	var stderr bytes.Buffer
	process, err := startBatchProcess(batch.command(), &stderr)
	if err != nil {
		outcome.status = err.Error()
		return outcome
	}
	reader := newBatchReader(batch)
	tail := make([]string, 0, batchTailLines)
	keep := func(line string) {
		tail = append(tail, line)
		if len(tail) > batchTailLines {
			tail = tail[1:]
		}
	}
	buffered := bufio.NewReader(process.output)
	for {
		line, readErr := buffered.ReadString('\n')
		if line != "" {
			read := reader.read(strings.TrimRight(line, "\r\n"))
			if read.shown {
				keep(read.display)
			}
			if read.found {
				outcome.results[read.name] = read.result
				onResult(read.name, read.result)
			}
		}
		if readErr != nil {
			break
		}
	}
	outcome.status = exitStatus(process.wait())
	for line := range strings.SplitSeq(strings.TrimRight(stderr.String(), "\n"), "\n") {
		if line != "" {
			keep(line)
		}
	}
	outcome.completed, outcome.tail = reader.completed(), tail
	return outcome
}

// exitStatus describes how a process ended.
func exitStatus(err error) string {
	if err == nil {
		return "exit status 0"
	}
	return err.Error()
}

// batchEvent reports the end of a batch's process to the observer and, when
// the batch did not complete, to the human report. It never reaches evidence.
type batchEvent struct {
	label     string
	status    string
	completed bool
	// unreported is how many tests had no result when the process ended.
	unreported int
	tail       []string
}

// executeGroupsInBatches runs the groups one batch at a time: every test of a
// batch starts with its process, and finishes as soon as its result appears.
func executeGroupsInBatches(
	root string,
	groups []testGroup,
	reporter groupReporter,
) (map[testGroupKey]TestExecution, []batchEvent) {
	plan := planTestBatches(root, groups)
	executions := make(map[testGroupKey]TestExecution, len(groups))
	for _, settled := range plan.settled {
		reporter.started(settled.group, settled.group.Key.Path)
		execution := groupExecution(settled.group, settled.result)
		executions[settled.group.Key] = execution
		reporter.finished(settled.group, settled.group.Key.Path, execution)
	}
	incomplete := make([]batchEvent, 0)
	for _, batch := range plan.batches {
		if event := executeBatch(batch, executions, reporter); !event.completed {
			incomplete = append(incomplete, event)
		}
	}
	return executions, incomplete
}

// executeBatch runs one batch, records the execution of each of its tests,
// and returns the end of its process.
func executeBatch(batch testBatch, executions map[testGroupKey]TestExecution, reporter groupReporter) batchEvent {
	label := batch.label()
	record := func(group testGroup, result exactResult) {
		execution := groupExecution(group, result)
		executions[group.Key] = execution
		reporter.finished(group, label, execution)
	}
	for _, group := range batch.groups {
		reporter.started(group, label)
	}
	outcome := runTestBatch(batch, func(name string, result exactResult) {
		for _, group := range batch.groups {
			if group.Key.Selector == name {
				record(group, result)
			}
		}
	})
	unreported := make([]testGroup, 0)
	for _, group := range batch.groups {
		if _, done := executions[group.Key]; !done {
			unreported = append(unreported, group)
		}
	}
	event := batchEvent{
		label: label, status: outcome.status, completed: outcome.completed,
		unreported: len(unreported), tail: outcome.tail,
	}
	reporter.batchEnded(event)
	for _, group := range unreported {
		record(group, outcome.resultOf(group.Key.Selector))
	}
	return event
}
