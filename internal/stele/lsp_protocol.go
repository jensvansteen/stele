package stele

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// The protocol types of the Language Server Protocol 3.17 subset that
// `stele lsp` serves. Fields are explicit and in a fixed order, so responses
// are deterministic.

// lspPosition is a zero-based line and a column in the session's position
// encoding.
type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type lspTextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type lspTextDocumentPositionParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     lspPosition               `json:"position"`
}

type lspReferenceParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Position     lspPosition               `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type lspMarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspHover struct {
	Contents lspMarkupContent `json:"contents"`
	Range    lspRange         `json:"range"`
}

type lspCommand struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type lspCodeLens struct {
	Range   lspRange   `json:"range"`
	Command lspCommand `json:"command"`
}

// lspDiagnostic is a published diagnostic; Code is the Stele code.
type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Code     string   `json:"code"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}

type lspPublishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type lspTextEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type lspVersionedDocument struct {
	URI     string `json:"uri"`
	Version *int   `json:"version"`
}

type lspTextDocumentEdit struct {
	TextDocument lspVersionedDocument `json:"textDocument"`
	Edits        []lspTextEdit        `json:"edits"`
}

// lspWorkspaceEdit carries either documentChanges or changes.
type lspWorkspaceEdit struct {
	Changes         map[string][]lspTextEdit `json:"changes,omitempty"`
	DocumentChanges []lspTextDocumentEdit    `json:"documentChanges,omitempty"`
}

type lspCodeAction struct {
	Title       string           `json:"title"`
	Kind        string           `json:"kind"`
	Diagnostics []lspDiagnostic  `json:"diagnostics"`
	IsPreferred bool             `json:"isPreferred"`
	Edit        lspWorkspaceEdit `json:"edit"`
}

type lspCodeActionParams struct {
	TextDocument lspTextDocumentIdentifier `json:"textDocument"`
	Range        lspRange                  `json:"range"`
	Context      struct {
		Only []string `json:"only"`
	} `json:"context"`
}

// lspCommandArguments are the arguments of the server's commands.
type lspCommandArguments struct {
	Root    string   `json:"root,omitempty"`
	ID      string   `json:"id,omitempty"`
	URI     string   `json:"uri,omitempty"`
	Targets []string `json:"targets,omitempty"`
	Scope   string   `json:"scope,omitempty"`
}

type lspExecuteCommandParams struct {
	Command       string                `json:"command"`
	Arguments     []lspCommandArguments `json:"arguments"`
	WorkDoneToken json.RawMessage       `json:"workDoneToken"`
}

// lspClientCapabilities is the part of the client's capabilities the server
// adapts to. Nothing else, and never the client's name, changes behavior.
type lspClientCapabilities struct {
	General struct {
		PositionEncodings []string `json:"positionEncodings"`
	} `json:"general"`
	TextDocument struct {
		Hover struct {
			ContentFormat []string `json:"contentFormat"`
		} `json:"hover"`
		CodeAction struct {
			CodeActionLiteralSupport json.RawMessage `json:"codeActionLiteralSupport"`
		} `json:"codeAction"`
	} `json:"textDocument"`
	Workspace struct {
		DidChangeWatchedFiles struct {
			DynamicRegistration bool `json:"dynamicRegistration"`
		} `json:"didChangeWatchedFiles"`
		CodeLens struct {
			RefreshSupport bool `json:"refreshSupport"`
		} `json:"codeLens"`
		WorkspaceEdit struct {
			DocumentChanges bool `json:"documentChanges"`
		} `json:"workspaceEdit"`
	} `json:"workspace"`
	Window struct {
		ShowDocument struct {
			Support bool `json:"support"`
		} `json:"showDocument"`
		WorkDoneProgress bool `json:"workDoneProgress"`
	} `json:"window"`
}

type lspInitializeParams struct {
	RootURI          string                `json:"rootUri"`
	Capabilities     lspClientCapabilities `json:"capabilities"`
	WorkspaceFolders []struct {
		URI string `json:"uri"`
	} `json:"workspaceFolders"`
}

// Position encodings.
const (
	encodingUTF16 = "utf-16"
	encodingUTF8  = "utf-8"
)

// lspColumn converts a byte offset within a line to a column in the encoding.
//
// @implements req.languageserver.488754628741
func lspColumn(line string, offset int, encoding string) int {
	offset = min(max(offset, 0), len(line))
	if encoding == encodingUTF8 {
		return offset
	}
	units := 0
	for _, character := range line[:offset] {
		units++
		if character >= 0x10000 {
			units++
		}
	}
	return units
}

// lspOffset converts a column in the encoding to a byte offset within a line.
// A column past the end of the line is the line's end.
func lspOffset(line string, column int, encoding string) int {
	if encoding == encodingUTF8 {
		return min(max(column, 0), len(line))
	}
	units := 0
	for offset, character := range line {
		if units >= column {
			return offset
		}
		units++
		if character >= 0x10000 {
			units++
		}
	}
	return len(line)
}

// lspLines splits a document into lines without their terminators.
func lspLines(text string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// lspLineRange is the range of a whole one-based line of a document.
func lspLineRange(lines []string, line int, encoding string) lspRange {
	text := ""
	if line >= 1 && line <= len(lines) {
		text = lines[line-1]
	}
	return lspRange{
		Start: lspPosition{Line: max(line-1, 0)},
		End:   lspPosition{Line: max(line-1, 0), Character: lspColumn(text, len(text), encoding)},
	}
}

// lspSpanRange is the range of the bytes start to end of a zero-based line.
func lspSpanRange(lines []string, line, start, end int, encoding string) lspRange {
	text := lines[line]
	return lspRange{
		Start: lspPosition{Line: line, Character: lspColumn(text, start, encoding)},
		End:   lspPosition{Line: line, Character: lspColumn(text, end, encoding)},
	}
}

// fileURI returns the file URI of an absolute path.
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// uriPath returns the absolute path of a file URI, or false for another URI.
func uriPath(uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || parsed.Path == "" {
		return "", false
	}
	return filepath.Clean(filepath.FromSlash(parsed.Path)), true
}

// truncateRunes cuts text to at most limit characters on a rune boundary,
// ending a cut text with an ellipsis.
func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit-1]) + "…"
}
