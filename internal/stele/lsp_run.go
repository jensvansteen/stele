package stele

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Run errors of the protocol.
const rpcRequestCancelled = -32800

// commandRunTests runs the tests of targets.
const commandRunTests = "stele.runTests"

// errRespondLater marks a request whose response a running command sends.
var errRespondLater = errors.New("the response follows when the command ends")

// lspRun is one run of stele.runTests in a project.
type lspRun struct {
	id      json.RawMessage
	token   json.RawMessage
	targets []string
	project *lspProject
	cancel  context.CancelFunc
	scopes  []lspRunScope
}

// lspRunScope is the part of a run in one scope: its targets and its result.
type lspRunScope struct {
	name    string
	scope   verificationScope
	targets []string
	run     testRun
}

// lspRunDone reports the end of a run to the server's loop.
type lspRunDone struct {
	run *lspRun
	err error
}

// lspRunResult is the result of stele.runTests.
type lspRunResult struct {
	Executions []lspRunExecution  `json:"executions"`
	Incomplete []lspRunIncomplete `json:"incomplete"`
	Scopes     []lspRunVerdicts   `json:"scopes"`
}

type lspRunExecution struct {
	EvidenceID string  `json:"evidenceId"`
	Path       string  `json:"path"`
	Line       int     `json:"line"`
	Selector   *string `json:"selector"`
	Outcome    string  `json:"outcome"`
	Reason     *string `json:"reason"`
}

type lspRunIncomplete struct {
	Batch      string `json:"batch"`
	Status     string `json:"status"`
	Unreported int    `json:"unreported"`
}

type lspRunVerdicts struct {
	Scope    string         `json:"scope"`
	Verdicts ReportVerdicts `json:"verdicts"`
}

// runTests starts stele.runTests: it checks the targets against the index,
// refuses a second run in the project, and runs the tests in the background
// through the runner of `stele test <targets...>`. The response follows when
// the run ends.
//
// @implements req.languageserver.e3ab377dc59f
func (server *lspServer) runTests(argument lspCommandArguments, token json.RawMessage) ([]byte, error) {
	project, err := server.projectOfRoot(argument.Root)
	if err != nil {
		return nil, err
	}
	if running := server.runs[project.root]; running != nil {
		return nil, newRPCError(rpcRequestFailed, "a run of %s is in progress in this project",
			strings.Join(running.targets, ", "))
	}
	scopes, err := runScopes(project.build, argument.Targets, argument.Scope)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &lspRun{
		id: server.requestID, token: token, targets: argument.Targets, project: project, cancel: cancel,
		scopes: scopes,
	}
	if len(run.token) == 0 && server.client.Window.WorkDoneProgress {
		run.token = json.RawMessage(fmt.Sprintf(`"stele-run-%d"`, server.requests+1))
		server.sendRequest("window/workDoneProgress/create", map[string]json.RawMessage{"token": run.token})
	}
	server.runs[project.root] = run
	progress := &lspProgress{server: server, token: run.token}
	go func() {
		err := executeRun(ctx, project.root, run, progress)
		progress.end(err)
		server.done <- lspRunDone{run: run, err: err}
	}()
	return nil, errRespondLater
}

// runScopes assigns every target to the scope that runs it: the given scope,
// or the first scope in scope order that declares the target, so a test never
// runs twice in one command. An unknown target fails before any test runs.
func runScopes(build *lspBuild, targets []string, only string) ([]lspRunScope, error) {
	if len(targets) == 0 {
		return nil, newRPCError(rpcInvalidParams, "%s needs at least one target", commandRunTests)
	}
	declared := declaredTargets(build.index)
	chosen := make(map[string][]string)
	for _, target := range targets {
		name := ""
		for _, scope := range build.index.Scopes {
			if declared[scope.ID][target] && (only == "" || only == scope.ID) {
				name = scope.ID
				break
			}
		}
		if name == "" {
			return nil, newRPCError(rpcInvalidParams, "unknown test target %s", target)
		}
		chosen[name] = append(chosen[name], target)
	}
	scopes := make([]lspRunScope, 0, len(chosen))
	for _, scope := range build.index.Scopes {
		if chosen[scope.ID] != nil {
			scopes = append(scopes, lspRunScope{name: scope.ID, targets: chosen[scope.ID], scope: verificationScope{
				currentSpecs: scope.Kind == "specs", changeID: choose(scope.Kind == "specs", "", scope.ID),
				backend: build.backend,
			}})
		}
	}
	return scopes, nil
}

// declaredTargets lists, per scope, the targets `stele test` accepts there:
// requirement, scenario, and evidence IDs, and specification files.
func declaredTargets(index Index) map[string]map[string]bool {
	declared := make(map[string]map[string]bool)
	add := func(scope, target string) {
		if declared[scope] == nil {
			declared[scope] = make(map[string]bool)
		}
		declared[scope][target] = true
	}
	for _, requirement := range index.Requirements {
		add(requirement.Scope, requirement.ID)
	}
	for _, scenario := range index.Scenarios {
		add(scenario.Scope, scenario.ID)
		for _, evidence := range scenario.Evidence {
			add(scenario.Scope, evidence.ID)
		}
	}
	for _, file := range index.SpecFiles {
		add(file.Scope, file.Path)
	}
	return declared
}

// executeRun runs each scope's targets, one scope after another, and stops at
// the first error or cancellation.
func executeRun(ctx context.Context, root string, run *lspRun, progress *lspProgress) error {
	for index := range run.scopes {
		scope := &run.scopes[index]
		result, err := runProjectScenarios(testRequest{
			root: root, scope: scope.scope, evidencePath: defaultEvidencePath, targets: scope.targets,
			merge: true, observer: progress, cancel: ctx,
		})
		if err != nil {
			return err
		}
		scope.run = result
	}
	return nil
}

// runFinished ends a run on the server's loop: it answers the command,
// cancelled or with the result built from the refreshed state, and refreshes
// CodeLens items and diagnostics.
func (server *lspServer) runFinished(done lspRunDone) {
	run := done.run
	delete(server.runs, run.project.root)
	run.cancel()
	switch {
	case errors.Is(done.err, errRunCancelled):
		server.respondError(run.id, newRPCError(rpcRequestCancelled, "the run of %s was cancelled",
			strings.Join(run.targets, ", ")))
		return
	case done.err != nil:
		server.respondError(run.id, newRPCError(rpcRequestFailed, "%s", done.err))
		return
	}
	run.project.files.update(defaultEvidencePath)
	run.project.dirty = true
	server.refresh = true
	server.flush()
	content, _ := json.Marshal(runResult(run))
	server.respondRaw(run.id, content)
}

// runResult lists the run's executions by path and selector, the batches that
// did not complete in the order they ran, and each scope's verdicts after the
// run.
func runResult(run *lspRun) lspRunResult {
	result := lspRunResult{
		Executions: []lspRunExecution{}, Incomplete: []lspRunIncomplete{}, Scopes: []lspRunVerdicts{},
	}
	build := run.project.build
	lines := make(map[string]int)
	for _, anchor := range build.index.Anchors {
		lines[anchor.Path+"\x00"+pointerValue(anchor.Selector)+"\x00"+anchorName(Anchor{
			ID:         anchor.ID,
			EvidenceID: anchor.EvidenceID,
		})] = anchor.Line
	}
	for _, scope := range run.scopes {
		for _, execution := range scope.run.selected {
			for _, id := range firstNonEmpty(execution.EvidenceIDs, execution.ScenarioIDs) {
				result.Executions = append(result.Executions, lspRunExecution{
					EvidenceID: id, Path: execution.Path,
					Line:     lines[execution.Path+"\x00"+pointerValue(execution.Selector)+"\x00"+id],
					Selector: execution.Selector, Outcome: execution.Outcome, Reason: execution.Reason,
				})
			}
		}
		for _, event := range scope.run.incomplete {
			result.Incomplete = append(result.Incomplete, lspRunIncomplete{
				Batch: event.label, Status: event.status, Unreported: event.unreported,
			})
		}
		result.Scopes = append(result.Scopes, lspRunVerdicts{Scope: scope.name, Verdicts: build.verdicts[scope.name]})
	}
	sort.SliceStable(result.Executions, func(i, j int) bool {
		left, right := result.Executions[i], result.Executions[j]
		return left.Path+"\x00"+pointerValue(left.Selector)+"\x00"+left.EvidenceID <
			right.Path+"\x00"+pointerValue(right.Selector)+"\x00"+right.EvidenceID
	})
	return result
}

func firstNonEmpty(values, fallback []string) []string {
	if len(values) > 0 {
		return values
	}
	return fallback
}

// cancelRun cancels the run whose request or progress token matches.
func (server *lspServer) cancelRun(raw json.RawMessage, field string) {
	var params map[string]json.RawMessage
	_ = json.Unmarshal(raw, &params)
	for _, run := range server.runs {
		key := run.id
		if field == "token" {
			key = run.token
		}
		if len(key) > 0 && string(key) == string(params[field]) {
			run.cancel()
		}
	}
}

// lspProgress reports a run as work-done progress: it begins with the number
// of tests, reports each finished test and each batch that did not complete,
// and ends with a summary. Without a token it reports nothing.
type lspProgress struct {
	server  *lspServer
	token   json.RawMessage
	mutex   sync.Mutex
	begun   bool
	total   int
	run     int
	passed  int
	failed  int
	message string
}

func (progress *lspProgress) send(value map[string]any) {
	if len(progress.token) == 0 {
		return
	}
	progress.server.sendNotification("$/progress", map[string]any{"token": progress.token, "value": value})
}

func (progress *lspProgress) percentage() int {
	return progress.run * 100 / max(progress.total, 1)
}

func (progress *lspProgress) planned(total int, _ map[string]int) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	progress.total += total
	if progress.begun {
		return
	}
	progress.begun = true
	progress.send(map[string]any{
		"kind": "begin", "title": fmt.Sprintf("Stele: running %d tests", total), "cancellable": true,
		"percentage": 0,
	})
}

func (progress *lspProgress) started(progressEvent) {}

func (progress *lspProgress) finished(event progressEvent) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	progress.run++
	if event.outcome == "passed" {
		progress.passed++
	} else {
		progress.failed++
	}
	progress.message = fmt.Sprintf("%d/%d · %d passed · %d failed", progress.run, progress.total, progress.passed,
		progress.failed)
	message := progress.message
	if event.outcome != "passed" {
		message += fmt.Sprintf(" · failed: %s (%s)", event.title, choose(event.level == "", "tests", event.level))
	}
	progress.send(map[string]any{"kind": "report", "message": message, "percentage": progress.percentage()})
}

// batchEnded reports a batch that did not complete, in the terminal's words.
func (progress *lspProgress) batchEnded(event batchEvent) {
	if event.completed {
		return
	}
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	progress.send(map[string]any{
		"kind": "report", "message": incompleteText(event), "percentage": progress.percentage(),
	})
}

// end ends the progress with the summary, or says the run was cancelled or
// failed.
func (progress *lspProgress) end(err error) {
	progress.mutex.Lock()
	defer progress.mutex.Unlock()
	message := choose(progress.message == "", "no tests ran", progress.message)
	switch {
	case errors.Is(err, errRunCancelled):
		message = "cancelled"
	case err != nil:
		message = err.Error()
	}
	if !progress.begun {
		progress.send(map[string]any{"kind": "begin", "title": "Stele: running tests", "percentage": 0})
	}
	progress.send(map[string]any{"kind": "end", "message": message})
}
