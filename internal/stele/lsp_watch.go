package stele

import (
	"encoding/json"
	"time"
)

// lspPollInterval is how often the server checks the files itself when the
// client cannot watch them.
const lspPollInterval = 2 * time.Second

// lspWatchGlobs are the files the client watches for the server: every input
// the snapshot keeps.
var lspWatchGlobs = []string{
	"**/*.{md,json,go,ts,tsx,mts,js,jsx,mjs,yaml,yml,toml,mod,sum}",
}

// watch asks the client to watch the project files when it supports dynamic
// registration, and otherwise starts the poller.
//
// @implements req.languageserver.ab7fbe14be03
func (server *lspServer) watch() {
	if server.client.Workspace.DidChangeWatchedFiles.DynamicRegistration {
		watchers := make([]map[string]string, 0, len(lspWatchGlobs))
		for _, glob := range lspWatchGlobs {
			watchers = append(watchers, map[string]string{"globPattern": glob})
		}
		server.sendRequest("client/registerCapability", map[string]any{"registrations": []map[string]any{{
			"id": "stele-watch", "method": "workspace/didChangeWatchedFiles",
			"registerOptions": map[string]any{"watchers": watchers},
		}}})
		return
	}
	server.poll = server.after(lspPollInterval)
}

// pollFiles compares every project's files with the disk, and rebuilds the
// projects that changed.
func (server *lspServer) pollFiles() {
	for _, project := range server.projects {
		if project.files != nil && project.files.refresh() {
			server.refresh = true
			project.dirty = true
		}
	}
	server.flush()
	server.poll = server.after(lspPollInterval)
}

// watchedFilesChanged re-reads the files the client reports as created,
// changed, or deleted, such as the evidence file a terminal run rewrote.
func (server *lspServer) watchedFilesChanged(raw json.RawMessage) {
	var params struct {
		Changes []struct {
			URI string `json:"uri"`
		} `json:"changes"`
	}
	_ = json.Unmarshal(raw, &params)
	for _, change := range params.Changes {
		path, local := uriPath(change.URI)
		if !local {
			continue
		}
		if project, relative, found := server.projectOf(path); found {
			project.files.update(relative)
			server.refresh = true
			server.changed(project)
		}
	}
}

// documentChanged keeps the text of open documents: opened and changed text
// overrides the saved file, a save re-reads the file from disk, and closing
// returns to the saved file.
func (server *lspServer) documentChanged(method string, raw json.RawMessage) {
	var params struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
			Text    string `json:"text"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	_ = json.Unmarshal(raw, &params)
	document := params.TextDocument
	path, local := uriPath(document.URI)
	if !local {
		return
	}
	switch method {
	case "textDocument/didOpen":
		server.documents[path] = &lspDocument{uri: document.URI, version: document.Version, text: document.Text}
	case "textDocument/didChange":
		if open := server.documents[path]; open != nil && len(params.ContentChanges) > 0 {
			open.version = document.Version
			open.text = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
	case "textDocument/didClose":
		delete(server.documents, path)
	}
	project, relative, found := server.projectOf(path)
	if !found || !trackedPath(relative) {
		return
	}
	if method == "textDocument/didSave" {
		project.files.update(relative)
		server.refresh = true
	}
	delete(project.files.overlays, relative)
	if open := server.documents[path]; open != nil {
		project.files.overlays[relative] = []byte(open.text)
	}
	server.changed(project)
}
