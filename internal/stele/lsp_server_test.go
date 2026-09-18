package stele

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// lspTestMessage is one message the server sent.
type lspTestMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// lspTestClient drives an in-process server session over pipes.
type lspTestClient struct {
	t        *testing.T
	input    *io.PipeWriter
	messages chan lspTestMessage
	exit     chan int
	logs     *bytes.Buffer
	// raw records every frame the server wrote, and received the messages
	// other than responses.
	raw      *bytes.Buffer
	received []lspTestMessage
	next     int
}

// neverFires is a timer source whose timers never fire, so a test decides
// when the server rebuilds.
func neverFires(time.Duration) <-chan time.Time {
	return nil
}

// startLSP starts a session with a timer source.
func startLSP(t *testing.T, after func(time.Duration) <-chan time.Time) *lspTestClient {
	t.Helper()
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	client := &lspTestClient{
		t: t, input: inputWriter, messages: make(chan lspTestMessage, 4096), exit: make(chan int, 1),
		logs: &bytes.Buffer{}, raw: &bytes.Buffer{},
	}
	logs := &bytes.Buffer{}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		code := runLanguageServer(inputReader, outputWriter, logs, after)
		_ = outputWriter.Close()
		client.logs.Write(logs.Bytes())
		client.exit <- code
	}()
	go func() {
		reader := bufio.NewReader(outputReader)
		for {
			body, err := readFrame(reader)
			if err != nil {
				close(client.messages)
				return
			}
			client.raw.Write(body)
			client.raw.WriteByte('\n')
			var message lspTestMessage
			if err := json.Unmarshal(body, &message); err != nil {
				t.Errorf("server wrote invalid JSON %q", body)
			}
			client.messages <- message
		}
	}()
	t.Cleanup(func() {
		// Ending the input ends the session, which stops its runs; wait for it,
		// so no run outlives its test.
		_ = inputWriter.Close()
		<-finished
	})
	return client
}

// sendRaw writes one frame with the given body.
func (client *lspTestClient) sendRaw(body string) {
	client.t.Helper()
	writeFrame(client.input, []byte(body))
}

// notify sends a notification.
func (client *lspTestClient) notify(method string, params any) {
	client.t.Helper()
	writeFrame(client.input, messageBody("", method, params))
}

// request sends a request and returns its response, keeping the messages that
// arrive before it.
func (client *lspTestClient) request(method string, params any) lspTestMessage {
	client.t.Helper()
	client.next++
	content, _ := json.Marshal(params)
	id := client.next
	client.sendRaw(`{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"` + method + `","params":` + string(content) + `}`)
	return client.response(itoa(id))
}

// response waits for the response with an ID.
func (client *lspTestClient) response(id string) lspTestMessage {
	client.t.Helper()
	for {
		select {
		case message, open := <-client.messages:
			if !open {
				client.t.Fatalf("the session ended before response %s", id)
			}
			if message.Method == "" && string(message.ID) == id {
				return message
			}
			client.received = append(client.received, message)
		case <-time.After(30 * time.Second):
			client.t.Fatalf("no response %s", id)
		}
	}
}

// result decodes a response's result.
func (client *lspTestClient) result(response lspTestMessage, target any) {
	client.t.Helper()
	if response.Error != nil {
		client.t.Fatalf("error response %#v", response.Error)
	}
	if err := json.Unmarshal(response.Result, target); err != nil {
		client.t.Fatalf("result %s: %v", response.Result, err)
	}
}

// sync waits until the server handled everything sent so far.
func (client *lspTestClient) sync() {
	client.t.Helper()
	client.request("stele/unknown-sync", nil)
}

// take returns and forgets the received messages of a method.
func (client *lspTestClient) take(method string) []lspTestMessage {
	found := make([]lspTestMessage, 0)
	kept := make([]lspTestMessage, 0)
	for _, message := range client.received {
		if message.Method == method {
			found = append(found, message)
		} else {
			kept = append(kept, message)
		}
	}
	client.received = kept
	return found
}

// waitExit closes nothing and waits for the session's exit code.
func (client *lspTestClient) waitExit() int {
	client.t.Helper()
	select {
	case code := <-client.exit:
		return code
	case <-time.After(30 * time.Second):
		client.t.Fatal("the session did not end")
		return -1
	}
}

func itoa(value int) string {
	content, _ := json.Marshal(value)
	return string(content)
}

// lspCapabilities are client capabilities for tests: Markdown hovers, code
// action literals, document changes, CodeLens refresh, showDocument, and
// dynamic file watching.
func lspCapabilities() map[string]any {
	return map[string]any{
		"textDocument": map[string]any{
			"hover":      map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
			"codeAction": map[string]any{"codeActionLiteralSupport": map[string]any{}},
		},
		"workspace": map[string]any{
			"workspaceEdit":         map[string]any{"documentChanges": true},
			"codeLens":              map[string]any{"refreshSupport": true},
			"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true},
		},
		"window": map[string]any{"showDocument": map[string]any{"support": true}},
	}
}

// initializeLSP initializes a session for a project root.
func (client *lspTestClient) initializeLSP(root string, capabilities map[string]any) lspTestMessage {
	client.t.Helper()
	response := client.request("initialize", map[string]any{
		"processId": nil, "capabilities": capabilities,
		"workspaceFolders": []map[string]string{{"uri": fileURI(root), "name": "project"}},
	})
	client.notify("initialized", map[string]any{})
	return response
}

// openLSP starts and initializes a session.
func openLSP(t *testing.T, root string, capabilities map[string]any) *lspTestClient {
	t.Helper()
	client := startLSP(t, neverFires)
	client.initializeLSP(root, capabilities)
	return client
}

// position is a request's document position.
func position(root, relative string, line, character int) map[string]any {
	return map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, relative))},
		"position":     map[string]int{"line": line, "character": character},
	}
}

// @verifies scn.languageserver.be03d984234b.unit
func TestLSPRefusesRequestsBeforeInitialization(t *testing.T) {
	root := lspFixture(t)
	client := startLSP(t, neverFires)
	client.notify("initialized", map[string]any{})
	response := client.request("textDocument/hover", position(root, "src/demo.mts", 0, 20))
	if response.Error == nil || response.Error.Code != rpcServerNotInitialized || len(response.Result) != 0 {
		t.Fatalf("hover before initialize = %#v", response)
	}
	if len(client.received) != 0 {
		t.Fatalf("the server sent data before initialize: %#v", client.received)
	}
}

// @verifies scn.languageserver.e311dc0148f1.unit
func TestLSPExitWithoutShutdown(t *testing.T) {
	client := openLSP(t, lspFixture(t), lspCapabilities())
	client.notify("exit", nil)
	if code := client.waitExit(); code != 1 {
		t.Fatalf("exit without shutdown = %d", code)
	}
}

func TestLSPLifecycle(t *testing.T) {
	root := lspFixture(t)
	client := startLSP(t, neverFires)
	other := t.TempDir()
	response := client.request("initialize", map[string]any{
		"capabilities": map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-16", "utf-8"}}},
		"workspaceFolders": []map[string]string{
			{"uri": fileURI(root)}, {"uri": fileURI(other)}, {"uri": "untitled:x"},
		},
	})
	var initialized struct {
		Capabilities struct {
			PositionEncoding   string `json:"positionEncoding"`
			CodeActionProvider bool   `json:"codeActionProvider"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	client.result(response, &initialized)
	if initialized.Capabilities.PositionEncoding != "utf-8" || !initialized.Capabilities.CodeActionProvider ||
		initialized.ServerInfo.Name != "stele" || initialized.ServerInfo.Version != Version {
		t.Fatalf("initialize = %s", response.Result)
	}
	if again := client.request("initialize", map[string]any{}); again.Error == nil ||
		again.Error.Code != rpcInvalidRequest {
		t.Fatalf("second initialize = %#v", again)
	}
	client.notify("initialized", map[string]any{})
	client.sendRaw(`{"jsonrpc":"2.0","id":"stele-1","result":null}`)
	if unknown := client.request("textDocument/rename", map[string]any{}); unknown.Error == nil ||
		unknown.Error.Code != rpcMethodNotFound {
		t.Fatalf("unknown method = %#v", unknown)
	}
	if invalid := client.request("textDocument/hover", "not params"); invalid.Error == nil ||
		invalid.Error.Code != rpcInvalidParams {
		t.Fatalf("invalid params = %#v", invalid)
	}
	client.request("shutdown", nil)
	if late := client.request("textDocument/hover", position(root, "src/demo.mts", 0, 0)); late.Error == nil ||
		late.Error.Code != rpcInvalidRequest {
		t.Fatalf("request after shutdown = %#v", late)
	}
	client.notify("textDocument/didOpen", map[string]any{})
	client.notify("exit", nil)
	if code := client.waitExit(); code != 0 {
		t.Fatalf("exit after shutdown = %d", code)
	}
	if logs := client.logs.String(); !strings.Contains(logs, "is not a Stele project") ||
		strings.Count(logs, "\n") != 2 {
		t.Fatalf("logs = %q", logs)
	}
}

func TestLSPEndOfInput(t *testing.T) {
	client := openLSP(t, lspFixture(t), map[string]any{})
	_ = client.input.Close()
	if code := client.waitExit(); code != 1 {
		t.Fatalf("end of input = %d", code)
	}
	shutDown := openLSP(t, lspFixture(t), map[string]any{})
	shutDown.request("shutdown", nil)
	_ = shutDown.input.Close()
	if code := shutDown.waitExit(); code != 0 {
		t.Fatalf("end of input after shutdown = %d", code)
	}
}

func TestLSPCommand(t *testing.T) {
	code, stdout, _ := runCommand(t, "lsp", "--help")
	if code != 0 || !strings.Contains(stdout, "Usage: stele lsp") {
		t.Fatalf("lsp --help = %d %q", code, stdout)
	}
	if code, _, stderr := runCommand(t, "lsp", "--stdio"); code != 2 || !strings.Contains(stderr, "--stdio") {
		t.Fatalf("lsp --stdio = %d %q", code, stderr)
	}
	original := lspInput
	t.Cleanup(func() { lspInput = original })
	lspInput = strings.NewReader("Content-Length: 33\r\n\r\n" + `{"jsonrpc":"2.0","method":"exit"}`)
	if code, stdout, _ := runCommand(t, "lsp"); code != 1 || stdout != "" {
		t.Fatalf("lsp with exit = %d %q", code, stdout)
	}
}

// lspSessionOutput runs a fixed sequence of requests and returns every frame
// the server wrote.
func lspSessionOutput(t *testing.T, root, clientName string) string {
	t.Helper()
	client := startLSP(t, neverFires)
	client.request("initialize", map[string]any{
		"capabilities": lspCapabilities(), "clientInfo": map[string]string{"name": clientName},
		"workspaceFolders": []map[string]string{{"uri": fileURI(root)}},
	})
	client.notify("initialized", map[string]any{})
	client.request("textDocument/hover", position(root, "src/demo.mts", 0, 20))
	client.request("textDocument/hover", position(root, "tests/value.test.mts", 1, 20))
	client.request("textDocument/definition", position(root, lspChangeSpec, 3, 0))
	references := position(root, lspChangeSpec, 3, 0)
	references["context"] = map[string]bool{"includeDeclaration": true}
	client.request("textDocument/references", references)
	client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, lspChangeSpec))},
	})
	client.request("textDocument/codeLens", map[string]any{
		"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, "tests/value.test.mts"))},
	})
	client.request("stele/index", map[string]string{"root": fileURI(root)})
	client.request("shutdown", nil)
	client.notify("exit", nil)
	client.waitExit()
	return client.raw.String()
}

// @verifies scn.languageserver.8d66c5680555.unit
func TestLSPAnswersIdenticallyForIdenticalState(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "src/extra.mts", "// @implements "+"req.demo.000000000000\nexport function extra() {}\n")
	first := lspSessionOutput(t, root, "editor")
	second := lspSessionOutput(t, root, "editor")
	if first != second {
		t.Fatalf("sessions differ:\n%s\n---\n%s", first, second)
	}
	for _, want := range []string{"publishDiagnostics", "Return value", "stele.showSpecification", "schemaVersion"} {
		if !strings.Contains(first, want) {
			t.Fatalf("session output lacks %q:\n%s", want, first)
		}
	}
}

// @verifies scn.languageserver.300e9d84bbbd.unit
func TestLSPIgnoresClientIdentity(t *testing.T) {
	root := lspFixture(t)
	if lspSessionOutput(t, root, "Visual Studio Code") != lspSessionOutput(t, root, "Neovim") {
		t.Fatal("responses depend on the client's name")
	}
}

func TestLSPIndexRequestErrors(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	response := client.request("stele/index", map[string]string{"root": fileURI(t.TempDir())})
	if response.Error == nil || response.Error.Code != rpcInvalidParams {
		t.Fatalf("index of another root = %#v", response)
	}
	if response := client.request("stele/index", "root"); response.Error == nil {
		t.Fatalf("index with invalid params = %#v", response)
	}
	writeFixture(t, root, "stele.config.json", `{"adapter":"other"}`)
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{
		{"uri": fileURI(filepath.Join(root, "stele.config.json")), "type": 2},
	}})
	if response := client.request("stele/index", map[string]string{"root": fileURI(root)}); response.Error == nil ||
		!strings.Contains(response.Error.Message, "unsupported adapter") {
		t.Fatalf("index with an invalid config = %#v", response)
	}
	writeFixture(t, root, "stele.config.json", `{`)
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{
		{"uri": fileURI(filepath.Join(root, "stele.config.json")), "type": 2},
	}})
	if response := client.request("stele/index", map[string]string{"root": fileURI(root)}); response.Error == nil ||
		response.Error.Code != rpcRequestFailed {
		t.Fatalf("index with a malformed config = %#v", response)
	}
	if !strings.Contains(logsAfterExit(client), "keeping the previous state") {
		t.Fatal("a failed rebuild was not logged")
	}
}

// logsAfterExit ends a session and returns its logs.
func logsAfterExit(client *lspTestClient) string {
	client.notify("exit", nil)
	client.waitExit()
	return client.logs.String()
}

func TestLSPIndexWithoutScopes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := openLSP(t, root, map[string]any{})
	response := client.request("stele/index", map[string]string{"root": fileURI(root)})
	if response.Error == nil || !strings.Contains(response.Error.Message, "no current specifications") {
		t.Fatalf("index without scopes = %#v", response)
	}
}

// lspChangeSpec is the delta spec of the fixture's change "example".
const lspChangeSpec = "openspec/changes/example/specs/demo/spec.md"

// lspFixture returns change "example" with an approved unit and e2e entry
// for scenario bbbb, each with a test, an unapproved entry for scenario cccc,
// and an implementation anchor on line 1 of src/demo.mts.
func lspFixture(t *testing.T) string {
	t.Helper()
	return linkedChangeFixture(t)
}

func TestLSPProjectStates(t *testing.T) {
	root := lspFixture(t)
	early := startLSP(t, neverFires)
	early.request("initialize", map[string]any{"rootUri": fileURI(root), "capabilities": map[string]any{}})
	if _, text := hoverAt(t, early, root, "src/demo.mts", 0, 20); text != "" {
		t.Fatalf("hover before initialized = %q", text)
	}
	early.notify("initialized", map[string]any{})
	if _, text := hoverAt(t, early, root, "src/demo.mts", 0, 20); !strings.Contains(text, "Return value") {
		t.Fatalf("hover of a rootUri project = %q", text)
	}
	broken := lspFixture(t)
	writeFixture(t, broken, "stele.config.json", "{")
	client := openLSP(t, broken, lspCapabilities())
	if _, text := hoverAt(t, client, broken, "src/demo.mts", 0, 20); text != "" {
		t.Fatalf("hover without a build = %q", text)
	}
	if backend := backendWithFiles(memoryBackend{calls: &[]string{}}, diskFiles{}); backend.Name() != "memory" {
		t.Fatalf("backend = %#v", backend)
	}
}
