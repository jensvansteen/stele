package stele

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Server lifecycle states.
const (
	lspWaiting = iota
	lspRunning
	lspShutDown
)

// lspDebounce is how long the server waits after the last change before it
// rebuilds and republishes diagnostics.
const lspDebounce = 100 * time.Millisecond

// lspDocument is an open document with the text and version the client sent.
type lspDocument struct {
	uri     string
	version int
	text    string
}

// lspProject is one served Stele project: its snapshot and the latest build.
type lspProject struct {
	root  string
	files *lspFiles
	build *lspBuild
	dirty bool
}

// lspServer is the state of one `stele lsp` session. One goroutine runs it:
// messages, the debounce timer, and the poller are handled in turn, so the
// same messages always give the same output.
type lspServer struct {
	output    io.Writer
	logs      io.Writer
	after     func(time.Duration) <-chan time.Time
	state     int
	client    lspClientCapabilities
	encoding  string
	projects  []*lspProject
	documents map[string]*lspDocument
	published map[string]string
	requests  int
	debounce  <-chan time.Time
	poll      <-chan time.Time
	// refresh asks for a CodeLens refresh after the next rebuild.
	refresh bool
}

// lspInput is where `stele lsp` reads messages.
var lspInput io.Reader = os.Stdin

// lspCommand runs the language server on standard input and output.
func languageServerCommand(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) > 0 {
		if arguments[0] == "--help" || arguments[0] == "-h" {
			_, _ = fmt.Fprintln(stdout, "Usage: stele lsp\n\nRuns the Stele language server over standard input "+
				"and output. Editors start it; see the editor integration guide.")
			return 0
		}
		return writeCommandError(stderr, fmt.Errorf("unknown option: %s", arguments[0]))
	}
	return runLanguageServer(lspInput, stdout, stderr, time.After)
}

// runLanguageServer serves one session and returns the exit code: 0 on exit
// after shutdown, and 1 on exit, or the end of input, without shutdown.
//
// @implements req.languageserver.950acbe48c80
func runLanguageServer(input io.Reader, output, logs io.Writer, after func(time.Duration) <-chan time.Time) int {
	messages := make(chan lspIncoming)
	done := make(chan struct{})
	defer close(done)
	go readFrames(bufio.NewReader(input), messages, done)
	server := &lspServer{
		output: output, logs: logs, after: after, encoding: encodingUTF16,
		documents: map[string]*lspDocument{}, published: map[string]string{},
	}
	for {
		select {
		case incoming, open := <-messages:
			if !open {
				return server.exitCode()
			}
			if exit := server.handle(incoming); exit {
				return server.exitCode()
			}
		case <-server.debounce:
			server.debounce = nil
			server.flush()
		case <-server.poll:
			server.pollFiles()
		}
	}
}

func (server *lspServer) exitCode() int {
	if server.state == lspShutDown {
		return 0
	}
	return 1
}

// handle processes one frame and reports whether the session ends.
func (server *lspServer) handle(incoming lspIncoming) bool {
	if incoming.err != nil {
		server.respondError(nil, newRPCError(rpcParseError, "%s", incoming.err))
		return false
	}
	var message rpcMessage
	if err := json.Unmarshal(incoming.body, &message); err != nil {
		server.respondError(nil, newRPCError(rpcParseError, "parse error: %s", err))
		return false
	}
	switch {
	case message.Method == "":
		// A response to a request of the server; nothing waits for it.
		return false
	case len(message.ID) == 0:
		return server.notify(message)
	default:
		server.request(message)
		return false
	}
}

// notify handles a notification and reports whether the session ends.
func (server *lspServer) notify(message rpcMessage) bool {
	if message.Method == "exit" {
		return true
	}
	if server.state != lspRunning {
		return false
	}
	switch message.Method {
	case "initialized":
		server.initialized()
	case "textDocument/didOpen", "textDocument/didChange", "textDocument/didClose", "textDocument/didSave":
		server.documentChanged(message.Method, message.Params)
	case "workspace/didChangeWatchedFiles":
		server.watchedFilesChanged(message.Params)
	}
	return false
}

// request answers a request.
func (server *lspServer) request(message rpcMessage) {
	switch {
	case message.Method == "initialize" && server.state == lspWaiting:
		server.respond(message.ID, server.initialize(message.Params))
		return
	case server.state == lspWaiting:
		server.respondError(message.ID, newRPCError(rpcServerNotInitialized, "the server is not initialized"))
		return
	case server.state == lspShutDown || message.Method == "initialize":
		server.respondError(message.ID, newRPCError(rpcInvalidRequest, "%s is not accepted now", message.Method))
		return
	case message.Method == "shutdown":
		server.state = lspShutDown
		server.poll = nil
		server.respond(message.ID, nil)
		return
	}
	handler, known := lspHandlers[message.Method]
	if !known {
		server.respondError(message.ID, newRPCError(rpcMethodNotFound, "unknown method %s", message.Method))
		return
	}
	server.flush()
	result, err := handler(server, message.Params)
	var failure *rpcError
	switch {
	case errors.As(err, &failure):
		server.respondError(message.ID, failure)
	case err != nil:
		server.respondError(message.ID, newRPCError(rpcInvalidParams, "invalid params: %s", err))
	default:
		server.respondRaw(message.ID, result)
	}
}

// lspHandler answers a request with raw result bytes.
type lspHandler func(server *lspServer, params json.RawMessage) ([]byte, error)

// lspHandlers are the requests the server answers after initialization.
var lspHandlers = map[string]lspHandler{
	"textDocument/hover":       (*lspServer).hover,
	"textDocument/definition":  (*lspServer).definition,
	"textDocument/references":  (*lspServer).references,
	"textDocument/codeLens":    (*lspServer).codeLens,
	"textDocument/codeAction":  (*lspServer).codeAction,
	"workspace/executeCommand": (*lspServer).executeCommand,
	"stele/index":              (*lspServer).steleIndex,
}

// initialize records the client's capabilities, finds the projects of the
// workspace folders, and advertises only the features the server has.
func (server *lspServer) initialize(raw json.RawMessage) any {
	var params lspInitializeParams
	_ = json.Unmarshal(raw, &params)
	server.client = params.Capabilities
	for _, encoding := range server.client.General.PositionEncodings {
		if encoding == encodingUTF8 {
			server.encoding = encodingUTF8
		}
	}
	folders := make([]string, 0)
	for _, folder := range params.WorkspaceFolders {
		folders = append(folders, folder.URI)
	}
	if len(folders) == 0 && params.RootURI != "" {
		folders = append(folders, params.RootURI)
	}
	for _, folder := range folders {
		root, local := uriPath(folder)
		if local && isSteleProject(root) {
			server.projects = append(server.projects, &lspProject{root: root, dirty: true})
			continue
		}
		server.log("%s is not a Stele project; it has no openspec/ or stele.config.json", folder)
	}
	server.state = lspRunning
	codeActions := any(true)
	if len(server.client.TextDocument.CodeAction.CodeActionLiteralSupport) > 0 {
		codeActions = map[string]any{"codeActionKinds": []string{"quickfix"}}
	}
	return map[string]any{
		"capabilities": map[string]any{
			"positionEncoding":   server.encoding,
			"textDocumentSync":   map[string]any{"openClose": true, "change": 1, "save": true},
			"hoverProvider":      true,
			"definitionProvider": true,
			"referencesProvider": true,
			"codeLensProvider":   map[string]any{"resolveProvider": false},
			"codeActionProvider": codeActions,
			"executeCommandProvider": map[string]any{
				"commands": []string{commandAddAnnotation, commandShowSpecification, commandShowStatus},
			},
			"experimental": map[string]any{"stele": map[string]any{
				"diagnostics": "publish", "requests": []string{"stele/index"},
			}},
		},
		"serverInfo": map[string]any{"name": "stele", "version": Version},
	}
}

// isSteleProject reports whether a folder has openspec/ or stele.config.json.
func isSteleProject(root string) bool {
	info, err := os.Stat(filepath.Join(root, "openspec"))
	return (err == nil && info.IsDir()) || fileExists(filepath.Join(root, "stele.config.json"))
}

// initialized loads every project, publishes its diagnostics, and starts
// watching its files.
func (server *lspServer) initialized() {
	for _, project := range server.projects {
		project.files = newLSPFiles(project.root)
	}
	server.flush()
	server.watch()
}

// log writes a line to standard error; standard output carries only messages.
func (server *lspServer) log(format string, arguments ...any) {
	_, _ = fmt.Fprintf(server.logs, "stele lsp: "+format+"\n", arguments...)
}

func (server *lspServer) respond(id json.RawMessage, result any) {
	content, _ := json.Marshal(result)
	server.respondRaw(id, content)
}

func (server *lspServer) respondRaw(id json.RawMessage, result []byte) {
	writeFrame(server.output, responseBody(id, result))
}

func (server *lspServer) respondError(id json.RawMessage, err *rpcError) {
	writeFrame(server.output, errorBody(id, err))
}

func (server *lspServer) sendNotification(method string, params any) {
	writeFrame(server.output, messageBody("", method, params))
}

// sendRequest sends a request to the client. Its answer is not awaited.
func (server *lspServer) sendRequest(method string, params any) {
	server.requests++
	writeFrame(server.output, messageBody(fmt.Sprintf("stele-%d", server.requests), method, params))
}

// projectOf returns the project that contains a path, and the path relative
// to its root.
func (server *lspServer) projectOf(path string) (*lspProject, string, bool) {
	var best *lspProject
	relative := ""
	for _, project := range server.projects {
		if project.files == nil {
			continue
		}
		if inside, found := project.files.relative(path); found && inside != "" &&
			(best == nil || len(project.root) > len(best.root)) {
			best, relative = project, inside
		}
	}
	return best, relative, best != nil
}

// projectOfRoot returns the project a command's root URI names.
func (server *lspServer) projectOfRoot(uri string) (*lspProject, error) {
	root, _ := uriPath(uri)
	for _, project := range server.projects {
		if project.root == root && project.build != nil {
			return project, nil
		}
	}
	return nil, newRPCError(rpcInvalidParams, "%s is not a served Stele project", uri)
}

// flush rebuilds every changed project and publishes what changed.
func (server *lspServer) flush() {
	for _, project := range server.projects {
		if project.files == nil || !project.dirty {
			continue
		}
		project.dirty = false
		build, err := buildLSPProject(project.files)
		if err != nil {
			server.log("%s: %s; keeping the previous state", project.root, err)
			continue
		}
		project.build = build
		server.publish(project)
	}
	if server.refresh && server.client.Workspace.CodeLens.RefreshSupport {
		server.sendRequest("workspace/codeLens/refresh", nil)
	}
	server.refresh = false
}

// changed marks a project for rebuilding after the debounce delay.
func (server *lspServer) changed(project *lspProject) {
	project.dirty = true
	server.debounce = server.after(lspDebounce)
}

// text returns a file's text: an open document's, or the saved file's.
func (server *lspServer) text(project *lspProject, relative string) string {
	content, _ := project.files.view(true).readFile(filepath.Join(project.root, filepath.FromSlash(relative)))
	return string(content)
}

// publish sends the diagnostics of every file whose findings changed, and an
// empty list for a file whose problems are gone.
//
// @implements req.languageserver.c4e7f3210359
func (server *lspServer) publish(project *lspProject) {
	paths := make([]string, 0, len(project.build.findings))
	for relative := range project.build.findings {
		paths = append(paths, filepath.Join(project.root, filepath.FromSlash(relative)))
	}
	for path := range server.published {
		if relative, found := project.files.relative(path); found && project.build.findings[relative] == nil {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		relative, _ := project.files.relative(path)
		findings := project.build.findings[relative]
		params := lspPublishDiagnosticsParams{
			URI:         fileURI(path),
			Diagnostics: lspDiagnostics(findings, lspLines(server.text(project, relative)), server.encoding),
		}
		content, _ := json.Marshal(params)
		if server.published[path] == string(content) {
			continue
		}
		server.sendNotification("textDocument/publishDiagnostics", params)
		server.published[path] = string(content)
		if len(findings) == 0 {
			delete(server.published, path)
		}
	}
}

// document finds the project and text of a document URI.
func (server *lspServer) document(uri string) (*lspProject, string, []string, bool) {
	path, local := uriPath(uri)
	if !local {
		return nil, "", nil, false
	}
	project, relative, found := server.projectOf(path)
	if !found || project.build == nil {
		return nil, "", nil, false
	}
	return project, relative, lspLines(server.text(project, relative)), true
}

// locations turns places into locations: the span of the place's ID on its
// line, or the whole line.
func (server *lspServer) locations(project *lspProject, places []lspPlace) []lspLocation {
	locations := make([]lspLocation, 0, len(places))
	texts := make(map[string][]string)
	for _, place := range places {
		lines, read := texts[place.path]
		if !read {
			lines = lspLines(server.text(project, place.path))
			texts[place.path] = lines
		}
		location := lspLocation{
			URI:   fileURI(filepath.Join(project.root, filepath.FromSlash(place.path))),
			Range: lspLineRange(lines, place.line, server.encoding),
		}
		if place.line <= len(lines) {
			text := lines[place.line-1]
			for _, match := range anchorPattern.FindAllStringSubmatchIndex(text, -1) {
				if text[match[4]:match[5]] == place.id {
					location.Range = lspSpanRange(lines, place.line-1, match[4], match[7], server.encoding)
					break
				}
			}
		}
		locations = append(locations, location)
	}
	return locations
}

// steleIndex answers `stele/index` with the document `stele index --json
// --all` prints for the saved files, byte for byte.
//
// @implements req.languageserver.2bab83c5c10f
func (server *lspServer) steleIndex(raw json.RawMessage) ([]byte, error) {
	var params struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, err := server.projectOfRoot(params.Root)
	if err != nil {
		return nil, err
	}
	saved := project.files.view(false)
	config, err := readConfigFrom(saved, project.root)
	if err == nil {
		var backend specificationBackend
		if backend, err = resolveBackend(config.Adapter); err == nil {
			var index Index
			if index, err = savedIndex(project.root, backendWithFiles(backend, saved)); err == nil {
				return MarshalDeterministic(index)
			}
		}
	}
	return nil, newRPCError(rpcRequestFailed, "%s", err)
}

// savedIndex indexes every scope, like `stele index --all`.
func savedIndex(root string, backend specificationBackend) (Index, error) {
	scopes := everyScope(root, backend)
	if len(scopes) == 0 {
		return Index{}, errors.New("no current specifications and no active changes to index")
	}
	return loadIndex(root, scopes)
}
