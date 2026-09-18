package stele

import (
	"encoding/json"
	"slices"
	"strings"
)

// annotationActionTitle names the annotation quick fix and its command.
const annotationActionTitle = "Add Stele annotation"

// annotationEdit returns the edit that adds the Stele annotation to a
// specification's text: the bytes `stele annotate` inserts, at the start of
// the file or after its byte order mark. It reports false unless the text has
// no annotation at all; broken annotations are left to a person.
//
// @implements req.languageserver.c37c196d0a2e
func annotationEdit(path, text, encoding string) (lspTextEdit, bool) {
	if classifyAnnotation(path, text).result().State != annotationMissing {
		return lspTextEdit{}, false
	}
	body := strings.TrimPrefix(text, byteOrderMark)
	bom := len(text) - len(body)
	annotated := insertAnnotation(text)
	inserted := annotated[bom : len(annotated)-len(body)]
	first, _, _ := strings.Cut(text, "\n")
	position := lspPosition{Character: lspColumn(first, bom, encoding)}
	return lspTextEdit{Range: lspRange{Start: position, End: position}, NewText: inserted}, true
}

// annotationWorkspaceEdit wraps the annotation edit for the client: with the
// document's version when the client accepts document changes, so a changed
// buffer rejects it, and as plain changes otherwise.
func annotationWorkspaceEdit(uri string, version *int, edit lspTextEdit, documentChanges bool) lspWorkspaceEdit {
	if documentChanges {
		return lspWorkspaceEdit{DocumentChanges: []lspTextDocumentEdit{{
			TextDocument: lspVersionedDocument{URI: uri, Version: version},
			Edits:        []lspTextEdit{edit},
		}}}
	}
	return lspWorkspaceEdit{Changes: map[string][]lspTextEdit{uri: {edit}}}
}

// wantsQuickFix reports whether a code action request accepts quick fixes.
func wantsQuickFix(only []string) bool {
	if len(only) == 0 {
		return true
	}
	return slices.Contains(only, "quickfix")
}

// codeAction answers textDocument/codeAction with the "Add Stele annotation"
// quick fix for a SPEC_ANNOTATION_MISSING diagnostic in the requested range:
// a code action with its edit, or for clients without code action literals
// the stele.addAnnotation command.
func (server *lspServer) codeAction(raw json.RawMessage) ([]byte, error) {
	var params lspCodeActionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, relative, lines, found := server.document(params.TextDocument.URI)
	if !found || !wantsQuickFix(params.Context.Only) {
		return []byte("[]"), nil
	}
	edit, missing := annotationEdit(relative, server.text(project, relative), server.encoding)
	for _, diagnostic := range lspDiagnostics(project.build.findings[relative], lines, server.encoding) {
		if !missing || diagnostic.Code != "SPEC_ANNOTATION_MISSING" ||
			params.Range.Start.Line > diagnostic.Range.End.Line || params.Range.End.Line < diagnostic.Range.Start.Line {
			continue
		}
		if len(server.client.TextDocument.CodeAction.CodeActionLiteralSupport) == 0 {
			return json.Marshal([]lspCommand{{
				Title: annotationActionTitle, Command: commandAddAnnotation,
				Arguments: []any{lspCommandArguments{URI: params.TextDocument.URI}},
			}})
		}
		return json.Marshal([]lspCodeAction{{
			Title: annotationActionTitle, Kind: "quickfix", Diagnostics: []lspDiagnostic{diagnostic},
			IsPreferred: true, Edit: server.annotationWorkspaceEdit(params.TextDocument.URI, edit),
		}})
	}
	return []byte("[]"), nil
}

// annotationWorkspaceEdit wraps an annotation edit of a document.
func (server *lspServer) annotationWorkspaceEdit(uri string, edit lspTextEdit) lspWorkspaceEdit {
	var version *int
	if path, local := uriPath(uri); local && server.documents[path] != nil {
		value := server.documents[path].version
		version = &value
	}
	return annotationWorkspaceEdit(uri, version, edit, server.client.Workspace.WorkspaceEdit.DocumentChanges)
}

// addAnnotation runs the stele.addAnnotation command: it asks the client to
// apply the annotation edit to the file.
//
// @implements req.languageserver.c37c196d0a2e
func (server *lspServer) addAnnotation(uri string) ([]byte, error) {
	project, relative, _, found := server.document(uri)
	if !found || !slices.ContainsFunc(project.build.index.SpecFiles, func(file IndexSpecFile) bool {
		return file.Path == relative
	}) {
		return nil, newRPCError(rpcInvalidParams, "%s is not a specification of a served Stele project", uri)
	}
	edit, missing := annotationEdit(relative, server.text(project, relative), server.encoding)
	if !missing {
		return nil, newRPCError(rpcRequestFailed, "%s does not lack a Stele annotation", relative)
	}
	server.sendRequest("workspace/applyEdit", map[string]any{
		"label": annotationActionTitle, "edit": server.annotationWorkspaceEdit(uri, edit),
	})
	return []byte("null"), nil
}
