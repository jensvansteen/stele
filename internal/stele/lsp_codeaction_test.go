package stele

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// applyTextEdit applies one single-line edit to a text.
func applyTextEdit(text string, edit lspTextEdit, encoding string) string {
	lines := strings.SplitAfter(text, "\n")
	offset := 0
	for _, line := range lines[:edit.Range.Start.Line] {
		offset += len(line)
	}
	current := strings.TrimRight(lines[edit.Range.Start.Line], "\r\n")
	start := offset + lspOffset(current, edit.Range.Start.Character, encoding)
	end := offset + lspOffset(current, edit.Range.End.Character, encoding)
	return text[:start] + edit.NewText + text[end:]
}

// unannotatedSpec is the change's delta spec without its annotation.
var unannotatedSpec = strings.TrimPrefix(evidenceSpec, "<!-- stele: spec v1 -->\n")

// openDocument opens a document in a session.
func openDocument(client *lspTestClient, root, relative, text string, version int) string {
	uri := fileURI(filepath.Join(root, relative))
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "markdown", "version": version, "text": text,
	}})
	return uri
}

// codeActionsAt requests the code actions of a line.
func codeActionsAt(client *lspTestClient, uri string, line int, only []string) lspTestMessage {
	return client.request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"range": map[string]any{
			"start": map[string]int{"line": line, "character": 0}, "end": map[string]int{"line": line, "character": 3},
		},
		"context": map[string]any{"diagnostics": []any{}, "only": only},
	})
}

// @verifies scn.languageserver.4edc5f1a06b4.unit
func TestLSPOffersTheAnnotationQuickFix(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, lspChangeSpec, unannotatedSpec)
	client := openLSP(t, root, lspCapabilities())
	uri := openDocument(client, root, lspChangeSpec, unannotatedSpec, 7)
	var actions []lspCodeAction
	client.result(codeActionsAt(client, uri, 0, nil), &actions)
	if len(actions) != 1 {
		t.Fatalf("actions = %#v", actions)
	}
	action := actions[0]
	changes := action.Edit.DocumentChanges
	if action.Title != "Add Stele annotation" || action.Kind != "quickfix" || !action.IsPreferred ||
		len(action.Diagnostics) != 1 || action.Diagnostics[0].Code != "SPEC_ANNOTATION_MISSING" ||
		len(changes) != 1 || changes[0].TextDocument.URI != uri || *changes[0].TextDocument.Version != 7 ||
		changes[0].Edits[0].Range != (lspRange{}) || changes[0].Edits[0].NewText != annotationCanonical+"\n" {
		t.Fatalf("action = %#v", action)
	}
	for _, empty := range []lspTestMessage{
		codeActionsAt(client, uri, 5, nil), codeActionsAt(client, uri, 0, []string{"refactor"}),
		client.request("textDocument/codeAction", map[string]any{
			"textDocument": map[string]string{"uri": fileURI(filepath.Join(root, "elsewhere.md"))},
		}),
	} {
		if string(empty.Result) != "[]" {
			t.Fatalf("other code actions = %s", empty.Result)
		}
	}
	if response := client.request("textDocument/codeAction", "invalid"); response.Error == nil {
		t.Fatalf("invalid code action params = %#v", response)
	}
	plain := openLSP(t, root, map[string]any{"textDocument": map[string]any{
		"codeAction": map[string]any{"codeActionLiteralSupport": map[string]any{}},
	}})
	client.result(codeActionsAt(plain, fileURI(filepath.Join(root, lspChangeSpec)), 0, []string{"quickfix"}), &actions)
	if len(actions) != 1 || actions[0].Edit.Changes[fileURI(filepath.Join(root, lspChangeSpec))] == nil {
		t.Fatalf("plain changes = %#v", actions)
	}
}

// @verifies scn.languageserver.fe68c635aa85.unit
func TestLSPAnnotationQuickFixWritesTheBytesOfAnnotate(t *testing.T) {
	for _, text := range []string{
		strings.ReplaceAll(strings.TrimSuffix(unannotatedSpec, "\n"), "\n", "\r\n"),
		byteOrderMark + unannotatedSpec,
		byteOrderMark + "é",
		"",
	} {
		for _, encoding := range []string{encodingUTF16, encodingUTF8} {
			edit, missing := annotationEdit("spec.md", text, encoding)
			if !missing || applyTextEdit(text, edit, encoding) != insertAnnotation(text) {
				t.Fatalf("%s edit of %q = %#v", encoding, text, edit)
			}
		}
	}
	if edit, _ := annotationEdit("spec.md", byteOrderMark+"x", encodingUTF16); edit.Range.Start.Character != 1 {
		t.Fatalf("UTF-16 edit after a byte order mark = %#v", edit)
	}
	if edit, _ := annotationEdit("spec.md", byteOrderMark+"x", encodingUTF8); edit.Range.Start.Character != 3 {
		t.Fatalf("UTF-8 edit after a byte order mark = %#v", edit)
	}
	if edit, _ := annotationEdit("spec.md", "a\r\nb", encodingUTF16); edit.NewText != annotationCanonical+"\r\n" {
		t.Fatalf("CRLF edit = %#v", edit)
	}
}

// @verifies scn.languageserver.4b2f05995999.unit
func TestLSPLeavesBrokenAnnotationsToAPerson(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	for index, text := range []string{
		"<!-- stele: spec -->\n" + unannotatedSpec,
		"<!-- stele: spec v2 -->\n" + unannotatedSpec,
		"## ADDED Requirements\n\n<!-- stele: spec v1 -->\n" +
			strings.TrimPrefix(unannotatedSpec, "## ADDED Requirements\n"),
	} {
		if _, offered := annotationEdit("spec.md", text, encodingUTF16); offered {
			t.Fatalf("an annotation edit was offered for %q", text)
		}
		uri := openDocument(client, root, lspChangeSpec, text, index+1)
		for line := range 4 {
			if response := codeActionsAt(client, uri, line, nil); string(response.Result) != "[]" {
				t.Fatalf("code actions for %q line %d = %s", text, line, response.Result)
			}
		}
	}
}

// @verifies scn.languageserver.53c81eb42bf4.unit
func TestLSPAppliesTheAnnotationFixThroughACommand(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, lspChangeSpec, unannotatedSpec)
	client := openLSP(t, root, map[string]any{})
	uri := fileURI(filepath.Join(root, lspChangeSpec))
	var commands []lspCommand
	client.result(codeActionsAt(client, uri, 0, nil), &commands)
	if len(commands) != 1 || commands[0].Command != "stele.addAnnotation" ||
		commands[0].Title != "Add Stele annotation" {
		t.Fatalf("commands = %#v", commands)
	}
	arguments, _ := json.Marshal(commands[0].Arguments)
	response := client.request("workspace/executeCommand", map[string]any{
		"command": commands[0].Command, "arguments": json.RawMessage(arguments),
	})
	edits := client.take("workspace/applyEdit")
	if response.Error != nil || len(edits) != 1 {
		t.Fatalf("executeCommand = %#v, %#v", response, client.received)
	}
	var applied struct {
		Label string           `json:"label"`
		Edit  lspWorkspaceEdit `json:"edit"`
	}
	if err := json.Unmarshal(edits[0].Params, &applied); err != nil {
		t.Fatal(err)
	}
	edit := applied.Edit.Changes[uri]
	if applied.Label != "Add Stele annotation" || len(edit) != 1 ||
		applyTextEdit(unannotatedSpec, edit[0], encodingUTF16) != insertAnnotation(unannotatedSpec) {
		t.Fatalf("applyEdit = %s", edits[0].Params)
	}
	writeFixture(t, root, lspChangeSpec, evidenceSpec)
	watched(client, root, lspChangeSpec)
	for _, target := range []string{fileURI(filepath.Join(root, "src/demo.mts")), "untitled:1", uri} {
		failed := client.request("workspace/executeCommand", map[string]any{
			"command": "stele.addAnnotation", "arguments": []map[string]string{{"uri": target}},
		})
		if failed.Error == nil {
			t.Fatalf("addAnnotation %s = %#v", target, failed)
		}
	}
}
