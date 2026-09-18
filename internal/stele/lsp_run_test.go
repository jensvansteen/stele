package stele

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// stubTestGroups replaces the batch runner with a recording stub: every
// group passes unless its selector is failing, and the stub reports each
// test as it starts and finishes. release, when set, holds the run until
// it is closed.
func stubTestGroups(t *testing.T, release chan struct{}, failing ...string) *[]string {
	t.Helper()
	original := runTestGroups
	t.Cleanup(func() { runTestGroups = original })
	var mutex sync.Mutex
	ran := make([]string, 0)
	runTestGroups = func(_ string, groups []testGroup, reporter groupReporter) (map[testGroupKey]TestExecution,
		[]batchEvent,
	) {
		if release != nil {
			select {
			case <-release:
			case <-reporter.cancel.Done():
				return map[testGroupKey]TestExecution{}, nil
			}
		}
		executions := make(map[testGroupKey]TestExecution)
		for _, group := range groups {
			mutex.Lock()
			ran = append(ran, group.Key.Selector)
			mutex.Unlock()
			reporter.started(group, group.Key.Path)
			result := exactResult{passed: !slices.Contains(failing, group.Key.Selector), executed: true}
			executions[group.Key] = groupExecution(group, result)
			reporter.finished(group, group.Key.Path, executions[group.Key])
		}
		return executions, nil
	}
	return &ran
}

// runArguments are the arguments of stele.runTests.
func runArguments(root string, targets ...string) map[string]any {
	return map[string]any{
		"command": "stele.runTests", "arguments": []map[string]any{{"root": fileURI(root), "targets": targets}},
	}
}

// startRun sends stele.runTests without waiting and returns its request ID.
func startRun(client *lspTestClient, params map[string]any) string {
	client.next++
	id := itoa(client.next)
	content, _ := json.Marshal(params)
	client.sendRaw(`{"jsonrpc":"2.0","id":` + id + `,"method":"workspace/executeCommand","params":` +
		string(content) + `}`)
	return id
}

// runResultOf decodes the result of stele.runTests.
func runResultOf(t *testing.T, client *lspTestClient, response lspTestMessage) lspRunResult {
	t.Helper()
	var result lspRunResult
	client.result(response, &result)
	return result
}

// @verifies scn.languageserver.074136f1f47f.unit
func TestLSPRunsAScenarioFromItsHeading(t *testing.T) {
	root := lspFixture(t)
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	writeFixture(t, root, defaultEvidencePath, `{"schemaVersion":3,"runner":"node","testedRevision":"r",`+
		`"inputDigest":"old","outcome":"failed","scenarios":[],"executions":[`+
		`{"path":"tests/missing.test.mts","selector":"reports a missing value","scenarioIds":["`+otherScenarioID+`"],`+
		`"evidenceIds":["`+otherScenarioID+`.unit"],"outcome":"failed","reason":"test-process-failed",`+
		`"inputDigest":"old"},`+
		`{"path":"tests/value.test.mts","selector":"returns the value","scenarioIds":["`+evidenceScenarioID+`"],`+
		`"evidenceIds":["`+evidenceScenarioID+`.unit"],"outcome":"failed","reason":null,"inputDigest":"old"}]}`+"\n")
	ran := stubTestGroups(t, nil)
	client := openLSP(t, root, lspCapabilities())
	result := runResultOf(t, client, client.request("workspace/executeCommand", runArguments(root, evidenceScenarioID)))
	if !slices.Equal(*ran, []string{"shows the value", "returns the value"}) {
		t.Fatalf("ran = %q", *ran)
	}
	if len(result.Executions) != 2 || result.Executions[0].EvidenceID != evidenceScenarioID+".e2e" ||
		result.Executions[0].Line != 2 || result.Executions[1].Outcome != "passed" {
		t.Fatalf("result = %#v", result)
	}
	stored := storedEvidence(t, root)
	if executionOutcome(stored, "reports a missing value") != "failed" ||
		executionOutcome(stored, "returns the value") != "passed" ||
		executionOutcome(stored, "shows the value") != "passed" {
		t.Fatalf("stored evidence = %#v", stored.Executions)
	}
	lenses := withCommand(lensesOf(t, client, root, lspChangeSpec), "stele.showStatus")
	if !slices.Contains(lenses, "9: unit ✓ · e2e ✓ (stele.showStatus)") {
		t.Fatalf("refreshed status = %q", lenses)
	}
	if len(client.take("workspace/codeLens/refresh")) == 0 {
		t.Fatal("no CodeLens refresh after the run")
	}
}

// @verifies scn.languageserver.a12e36dfdfb8.unit
func TestLSPRejectsAnUnknownTarget(t *testing.T) {
	root := lspFixture(t)
	ran := stubTestGroups(t, nil)
	client := openLSP(t, root, lspCapabilities())
	for _, target := range []string{"scn.demo.000000000000", "openspec/specs/none/spec.md"} {
		response := client.request("workspace/executeCommand", runArguments(root, evidenceScenarioID, target))
		if response.Error == nil || response.Error.Code != rpcInvalidParams ||
			!strings.Contains(response.Error.Message, target) {
			t.Fatalf("unknown target %s = %#v", target, response)
		}
	}
	if response := client.request("workspace/executeCommand", runArguments(root)); response.Error == nil {
		t.Fatalf("a run without targets = %#v", response)
	}
	if len(*ran) != 0 {
		t.Fatalf("tests ran: %q", *ran)
	}
}

// writeBlockingTest writes a Node test that starts a child process, records
// the child's process ID, and waits until it is killed.
func writeBlockingTest(t *testing.T, root, relative, anchor, title, pidFile string) {
	t.Helper()
	target, _ := json.Marshal(pidFile)
	writeFixture(t, root, relative, "import test from \"node:test\";\n"+
		"import { spawn } from \"node:child_process\";\n"+
		"import { writeFileSync } from \"node:fs\";\n"+
		"// @verifies "+anchor+"\n"+
		"void test(\""+title+"\", async () => {\n"+
		"  const child = spawn(\"sleep\", [\"60\"], { stdio: \"ignore\" });\n"+
		"  writeFileSync("+string(target)+", String(child.pid));\n"+
		"  await new Promise(() => { setInterval(() => {}, 1000); });\n"+
		"});\n")
}

// processGone waits until a process, or with a negative pid a process group,
// no longer exists.
func processGone(pid int) bool {
	for range 250 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// @verifies scn.languageserver.3c56123372c1.integration
func TestLSPCancelsARunningCommand(t *testing.T) {
	root := lspFixture(t)
	pidFile, marker := filepath.Join(root, "child.pid"), filepath.Join(root, "second-batch-ran")
	writeBlockingTest(t, root, "tests/e2e/value.test.mts", evidenceScenarioID+".e2e", "shows the value", pidFile)
	quoted, _ := json.Marshal(marker)
	writeFixture(t, root, "tests/value.test.mts", "import test from \"node:test\";\n"+
		"import { writeFileSync } from \"node:fs\";\n// @verifies "+evidenceScenarioID+".unit\n"+
		"void test(\"returns the value\", () => { writeFileSync("+string(quoted)+", \"yes\"); });\n")
	lspStoredEvidence(t, root, "passed", "an-older-digest")
	before, _ := os.ReadFile(filepath.Join(root, defaultEvidencePath))
	client := openLSP(t, root, lspCapabilities())
	id := startRun(client, runArguments(root, evidenceScenarioID))
	var content []byte
	for range 1500 {
		if content, _ = os.ReadFile(pidFile); len(content) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	child, err := strconv.Atoi(string(content))
	if err != nil {
		t.Fatalf("the blocking test did not start its child: %q", content)
	}
	client.notify("$/cancelRequest", map[string]any{"id": json.RawMessage(id)})
	response := client.response(id)
	if response.Error == nil || response.Error.Code != rpcRequestCancelled {
		t.Fatalf("cancelled run = %#v", response)
	}
	if !processGone(child) {
		t.Fatalf("the test's child process %d survived the cancel", child)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a batch started after the cancel")
	}
	if after, _ := os.ReadFile(filepath.Join(root, defaultEvidencePath)); string(after) != string(before) {
		t.Fatalf("the evidence file changed:\n%s", after)
	}
}

func TestStopOnCancelKillsWhatIgnoresTerminate(t *testing.T) {
	original := batchCancelGrace
	t.Cleanup(func() { batchCancelGrace = original })
	batchCancelGrace = 100 * time.Millisecond
	command := exec.Command("sh", "-c", "trap '' TERM; sleep 30 & wait")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	stop := stopOnCancel(ctx, command.Process.Pid)
	cancel()
	_ = command.Wait()
	stop()
	// The killed background child can linger as a zombie until init reaps it,
	// and a zombie still belongs to the group, so wait for the group to go.
	if !processGone(-command.Process.Pid) {
		t.Fatalf("the process group survived: %v", syscall.Kill(-command.Process.Pid, 0))
	}
	for _, pid := range []int{0, command.Process.Pid} {
		ctx, cancel := context.WithCancel(context.Background())
		stop := stopOnCancel(ctx, pid)
		cancel()
		time.Sleep(20 * time.Millisecond)
		stop()
	}
	stopOnCancel(t.Context(), command.Process.Pid)()
}

// @verifies scn.languageserver.ddba3d8c9626.unit
func TestLSPRefusesASecondConcurrentRun(t *testing.T) {
	root := lspFixture(t)
	release := make(chan struct{})
	ran := stubTestGroups(t, release)
	client := openLSP(t, root, lspCapabilities())
	first := startRun(client, runArguments(root, evidenceScenarioID))
	client.sync()
	second := client.request("workspace/executeCommand", runArguments(root, otherScenarioID))
	if second.Error == nil || second.Error.Code != rpcRequestFailed ||
		!strings.Contains(second.Error.Message, evidenceScenarioID) {
		t.Fatalf("second run = %#v", second)
	}
	close(release)
	if result := runResultOf(t, client, client.response(first)); len(result.Executions) != 2 || len(*ran) != 2 {
		t.Fatalf("first run = %#v, ran %q", result, *ran)
	}
}

// progressValues returns the values of the $/progress notifications received.
func progressValues(t *testing.T, client *lspTestClient) []map[string]any {
	t.Helper()
	values := make([]map[string]any, 0)
	for _, message := range client.take("$/progress") {
		var params struct {
			Token string         `json:"token"`
			Value map[string]any `json:"value"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil || params.Token == "" {
			t.Fatalf("progress = %s", message.Params)
		}
		values = append(values, params.Value)
	}
	return values
}

// @verifies scn.languageserver.159800f00b8d.unit
func TestLSPReportsProgressWhileTestsRun(t *testing.T) {
	root := lspFixture(t)
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	stubTestGroups(t, nil, "reports a missing value")
	capabilities := lspCapabilities()
	capabilities["window"] = map[string]any{"workDoneProgress": true}
	client := openLSP(t, root, capabilities)
	response := client.request("workspace/executeCommand", runArguments(root, "req.demo.aaaaaaaaaaaa"))
	if response.Error != nil || len(client.take("window/workDoneProgress/create")) != 1 {
		t.Fatalf("run = %#v, %#v", response, client.received)
	}
	values := progressValues(t, client)
	messages := make([]string, 0, len(values))
	for _, value := range values {
		messages = append(messages, stringOf(value["kind"])+": "+
			choose(value["title"] != nil, stringOf(value["title"]), stringOf(value["message"])))
	}
	want := []string{
		"begin: Stele: running 3 tests",
		"report: 1/3 · 1 passed · 0 failed",
		"report: 2/3 · 1 passed · 1 failed · failed: Missing value is reported (unit)",
		"report: 3/3 · 2 passed · 1 failed",
		"end: 3/3 · 2 passed · 1 failed",
	}
	if !slices.Equal(messages, want) || values[0]["cancellable"] != true || values[3]["percentage"] != float64(100) {
		t.Fatalf("progress:\n%s", strings.Join(messages, "\n"))
	}
}

func stringOf(value any) string {
	text, _ := value.(string)
	return text
}

// @verifies scn.languageserver.dfffd1e421b4.unit
func TestLSPReportsTheVerdictsOfTheScopeAfterARun(t *testing.T) {
	root := approvedEvidenceFixture(t)
	plan := readEvidencePlan(t, root, "openspec/changes/example/linkage-plan.json")
	scenario := plan.Scenarios[otherScenarioID]
	scenario.Evidence = append(scenario.Evidence, approved(t, root, entry(otherScenarioID, "e2e", "The journey.")))
	plan.Scenarios[otherScenarioID] = scenario
	entries := map[string][]EvidenceEntry{}
	for id, value := range plan.Scenarios {
		entries[id] = value.Evidence
	}
	writeEvidencePlan(t, root, entries)
	writeEvidenceTest(t, root, "tests/value.test.mts", evidenceScenarioID+".unit", "returns the value")
	writeEvidenceTest(t, root, "tests/e2e/value.test.mts", evidenceScenarioID+".e2e", "shows the value")
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, defaultEvidencePath, `{"schemaVersion":3,"runner":"node","testedRevision":"r",`+
		`"inputDigest":"`+digest+`","outcome":"passed","scenarios":[{"id":"`+otherScenarioID+`","outcome":"passed"}],`+
		`"executions":[{"path":"tests/missing.test.mts","selector":"reports a missing value",`+
		`"scenarioIds":["`+otherScenarioID+`"],"evidenceIds":["`+otherScenarioID+`.unit"],"outcome":"passed",`+
		`"reason":null,"inputDigest":"`+digest+`"}]}`+"\n")
	stubTestGroups(t, nil)
	client := openLSP(t, root, lspCapabilities())
	result := runResultOf(t, client, client.request("workspace/executeCommand", runArguments(root, evidenceScenarioID)))
	if len(result.Executions) != 2 || result.Executions[0].Outcome != "passed" ||
		result.Executions[1].Outcome != "passed" {
		t.Fatalf("executions = %#v", result.Executions)
	}
	want := []lspRunVerdicts{{Scope: "example", Verdicts: ReportVerdicts{
		Linkage: "fail", Execution: "passed",
		Overall: "fail",
	}}}
	if !slices.Equal(result.Scopes, want) {
		t.Fatalf("scopes = %#v", result.Scopes)
	}
}

// @verifies scn.languageserver.40c7e3ae9243.unit
func TestLSPReportsABatchThatDidNotComplete(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/value.test.mts", "import test from \"node:test\";\n// @verifies "+
		evidenceScenarioID+".unit\nvoid test(\"returns the value\", () => {});\n// @verifies "+otherScenarioID+
		".unit\nvoid test(\"reports a missing value\", () => {});\n")
	original := startBatchProcess
	t.Cleanup(func() { startBatchProcess = original })
	startBatchProcess = func(command *exec.Cmd, _ io.Writer) (batchProcess, error) {
		if slices.Contains(command.Args, "tests/e2e/value.test.mts") {
			return batchProcess{
				output: strings.NewReader("ok 1 - shows the value\n# tests 1\n"),
				wait:   func() error { return nil },
			}, nil
		}
		return batchProcess{output: strings.NewReader("ok 1 - returns the value\n"), wait: func() error {
			return errors.New("exit status 2")
		}}, nil
	}
	capabilities := lspCapabilities()
	capabilities["window"] = map[string]any{"workDoneProgress": true}
	client := openLSP(t, root, capabilities)
	result := runResultOf(t, client, client.request("workspace/executeCommand",
		runArguments(root, evidenceScenarioID, otherScenarioID)))
	if len(result.Incomplete) != 1 || result.Incomplete[0] != (lspRunIncomplete{
		Batch: "tests/value.test.mts", Status: "exit status 2", Unreported: 1,
	}) {
		t.Fatalf("incomplete = %#v", result.Incomplete)
	}
	outcomes := map[string]string{}
	for _, execution := range result.Executions {
		outcomes[execution.EvidenceID] = execution.Outcome + " " + pointerValue(execution.Reason)
	}
	if outcomes[evidenceScenarioID+".unit"] != "passed " ||
		outcomes[otherScenarioID+".unit"] != "failed test-process-failed" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
	found := false
	for _, value := range progressValues(t, client) {
		found = found || value["message"] == "tests/value.test.mts did not complete (exit status 2)"
	}
	if !found {
		t.Fatal("no progress report for the batch that did not complete")
	}
}

func TestLSPRunAcrossScopesAndEdgeCases(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "openspec/specs/demo/spec.md", lspCurrentSpec)
	writeFixture(t, root, "tests/bare.test.mts", "import test from \"node:test\";\n// @verifies "+
		evidenceScenarioID+"\nvoid test(\"returns it bare\", () => {});\n")
	writeEvidenceTest(t, root, "tests/missing.test.mts", otherScenarioID+".unit", "reports a missing value")
	release := make(chan struct{})
	close(release)
	ran := stubTestGroups(t, release)
	client := openLSP(t, root, lspCapabilities())
	params := runArguments(root, evidenceScenarioID, otherScenarioID)
	params["workDoneToken"] = "client-token"
	result := runResultOf(t, client, client.request("workspace/executeCommand", params))
	if len(result.Scopes) != 2 || result.Scopes[0].Scope != "specs" || result.Scopes[1].Scope != "example" {
		t.Fatalf("scopes = %#v", result.Scopes)
	}
	bare := result.Executions[0]
	if bare.EvidenceID != evidenceScenarioID || bare.Path != "tests/bare.test.mts" || bare.Line != 2 ||
		pointerValue(bare.Selector) != "returns it bare" ||
		strings.Count(strings.Join(*ran, ","), "returns the value") != 1 {
		t.Fatalf("executions = %#v, ran %q", result.Executions, *ran)
	}
	begins := 0
	for _, value := range progressValues(t, client) {
		if value["kind"] == "begin" {
			begins++
		}
	}
	if begins != 1 {
		t.Fatalf("progress began %d times", begins)
	}
	for _, failing := range []map[string]any{
		{"command": "stele.runTests", "arguments": []map[string]any{
			{"root": fileURI(t.TempDir()), "targets": []string{"x"}},
		}},
		{"command": "stele.runTests", "arguments": []map[string]any{
			{"root": fileURI(root), "targets": []string{otherScenarioID}, "scope": "specs"},
		}},
	} {
		if response := client.request("workspace/executeCommand", failing); response.Error == nil {
			t.Fatalf("run %v = %#v", failing, response)
		}
	}
	original := runProjectScenarios
	t.Cleanup(func() { runProjectScenarios = original })
	runProjectScenarios = func(testRequest) (testRun, error) { return testRun{}, errors.New("the runner broke") }
	response := client.request("workspace/executeCommand", params)
	values := progressValues(t, client)
	if response.Error == nil || response.Error.Code != rpcRequestFailed || len(values) != 2 ||
		values[1]["message"] != "the runner broke" {
		t.Fatalf("failed run = %#v, progress %#v", response, values)
	}
}

func TestLSPCancelsThroughTheProgressTokenAndOnExit(t *testing.T) {
	root := lspFixture(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	stubTestGroups(t, release)
	client := openLSP(t, root, lspCapabilities())
	params := runArguments(root, evidenceScenarioID)
	params["workDoneToken"] = 7
	id := startRun(client, params)
	client.sync()
	client.notify("window/workDoneProgress/cancel", map[string]any{"token": 8})
	client.notify("window/workDoneProgress/cancel", map[string]any{"token": 7})
	if response := client.response(id); response.Error == nil || response.Error.Code != rpcRequestCancelled {
		t.Fatalf("cancelled through the token = %#v", response)
	}
	startRun(client, runArguments(root, evidenceScenarioID))
	client.sync()
	client.notify("exit", nil)
	if code := client.waitExit(); code != 1 {
		t.Fatalf("exit during a run = %d", code)
	}
}
