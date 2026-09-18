package stele

import (
	"encoding/json"
	"fmt"
	"sort"
)

// lspTarget is what the cursor is on: the ID of an anchor, or a requirement or
// scenario heading or its Verification-ID line in a specification.
type lspTarget struct {
	// id is the requirement or scenario identity, and evidence the full
	// evidence ID of an anchor that names one.
	id       string
	evidence string
	// scope is the scope of a heading's specification file; anchors have none.
	scope string
	line  int
	span  lspRange
}

// heading reports whether the target is a heading or Verification-ID line.
func (target lspTarget) heading() bool {
	return target.scope != ""
}

// lspTargetAt finds the target at a position of a document. An anchor counts
// only where the index has it, so an ID in a string is not a target.
func lspTargetAt(index Index, path string, lines []string, position lspPosition, encoding string) (lspTarget,
	bool,
) {
	if position.Line < 0 || position.Line >= len(lines) {
		return lspTarget{}, false
	}
	text := lines[position.Line]
	offset := lspOffset(text, position.Character, encoding)
	for _, match := range anchorPattern.FindAllStringSubmatchIndex(text, -1) {
		if offset < match[0] || offset > match[7] {
			continue
		}
		id, evidence := text[match[4]:match[5]], text[match[4]:match[7]]
		if !indexHasAnchor(index, path, position.Line+1, id) {
			return lspTarget{}, false
		}
		return lspTarget{
			id: id, evidence: choose(evidence == id, "", evidence), line: position.Line + 1,
			span: lspSpanRange(lines, position.Line, match[4], match[7], encoding),
		}, true
	}
	return specTargetAt(index, path, lines, position.Line, encoding)
}

// specTargetAt finds a heading or Verification-ID line of a specification.
func specTargetAt(index Index, path string, lines []string, line int, encoding string) (lspTarget, bool) {
	text := lines[line]
	declared := ""
	if match := verificationIDPattern.FindStringSubmatch(text); match != nil {
		declared = match[1]
	}
	matches := func(id string, source Source) bool {
		if source.Path != path {
			return false
		}
		return source.Line == line+1 || id == declared
	}
	target := lspTarget{line: line + 1, span: lspLineRange(lines, line+1, encoding)}
	for _, requirement := range index.Requirements {
		if requirement.ID != "" && matches(requirement.ID, requirement.Source) {
			target.id, target.scope = requirement.ID, requirement.Scope
			return target, true
		}
	}
	for _, scenario := range index.Scenarios {
		if scenario.ID != "" && matches(scenario.ID, scenario.Source) {
			target.id, target.scope = scenario.ID, scenario.Scope
			return target, true
		}
	}
	return lspTarget{}, false
}

func indexHasAnchor(index Index, path string, line int, id string) bool {
	for _, anchor := range index.Anchors {
		if anchor.Path == path && anchor.Line == line && anchor.ID == id {
			return true
		}
	}
	return false
}

// lspPlace is a one-based line of a repository file, with the ID whose span
// the location covers, or none for the whole line.
type lspPlace struct {
	path string
	line int
	id   string
}

// sortPlaces orders places by path and line and keeps each place once.
func sortPlaces(places []lspPlace) []lspPlace {
	sort.SliceStable(places, func(i, j int) bool {
		return fmt.Sprintf("%s\x00%09d", places[i].path, places[i].line) <
			fmt.Sprintf("%s\x00%09d", places[j].path, places[j].line)
	})
	unique := make([]lspPlace, 0, len(places))
	for _, place := range places {
		if len(unique) > 0 && unique[len(unique)-1].path == place.path && unique[len(unique)-1].line == place.line {
			continue
		}
		unique = append(unique, place)
	}
	return unique
}

// declarationPlaces lists the headings that declare an identity, one per scope.
func declarationPlaces(index Index, id string) []lspPlace {
	places := make([]lspPlace, 0)
	for _, requirement := range index.Requirements {
		if requirement.ID == id {
			places = append(places, lspPlace{path: requirement.Source.Path, line: requirement.Source.Line})
		}
	}
	for _, scenario := range index.Scenarios {
		if scenario.ID == id {
			places = append(places, lspPlace{path: scenario.Source.Path, line: scenario.Source.Line})
		}
	}
	return places
}

// anchorPlaces lists the anchors of the given annotation that name an identity.
func anchorPlaces(index Index, id, annotation string) []lspPlace {
	places := make([]lspPlace, 0)
	for _, anchor := range index.Anchors {
		if anchor.ID == id && anchor.Annotation == annotation {
			places = append(places, lspPlace{path: anchor.Path, line: anchor.Line, id: anchor.ID})
		}
	}
	return places
}

// lspDefinition returns the direct counterpart of a target: from an anchor,
// the headings that declare its ID; from a requirement, its @implements
// anchors; and from a scenario, the @verifies anchors of its evidence.
//
// @implements req.languageserver.488754628741
func lspDefinition(index Index, target lspTarget) []lspPlace {
	switch {
	case !target.heading():
		return sortPlaces(declarationPlaces(index, target.id))
	case isRequirementID(target.id):
		return sortPlaces(anchorPlaces(index, target.id, "implements"))
	default:
		return sortPlaces(anchorPlaces(index, target.id, "verifies"))
	}
}

// lspReferences returns every use of a target's behavior: for a requirement
// its @implements anchors and the @verifies anchors of its scenarios, for a
// scenario or evidence ID the @verifies anchors of the scenario, and with the
// declaration the headings that declare the ID in every scope.
//
// @implements req.languageserver.70e72541715e
func lspReferences(index Index, target lspTarget, declaration bool) []lspPlace {
	places := make([]lspPlace, 0)
	if declaration {
		places = append(places, declarationPlaces(index, target.id)...)
	}
	if !isRequirementID(target.id) {
		return sortPlaces(append(places, anchorPlaces(index, target.id, "verifies")...))
	}
	places = append(places, anchorPlaces(index, target.id, "implements")...)
	scenarios := make(map[string]bool)
	for _, requirement := range index.Requirements {
		if requirement.ID != target.id {
			continue
		}
		for _, scenario := range requirement.Scenarios {
			if !scenarios[scenario] {
				scenarios[scenario] = true
				places = append(places, anchorPlaces(index, scenario, "verifies")...)
			}
		}
	}
	return sortPlaces(places)
}

func isRequirementID(id string) bool {
	return len(id) > 4 && id[:4] == "req."
}

// definition answers textDocument/definition.
func (server *lspServer) definition(raw json.RawMessage) ([]byte, error) {
	var params lspTextDocumentPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, relative, lines, found := server.document(params.TextDocument.URI)
	if !found {
		return []byte("[]"), nil
	}
	target, found := lspTargetAt(project.build.index, relative, lines, params.Position, server.encoding)
	if !found {
		return []byte("[]"), nil
	}
	return json.Marshal(server.locations(project, lspDefinition(project.build.index, target)))
}

// references answers textDocument/references.
func (server *lspServer) references(raw json.RawMessage) ([]byte, error) {
	var params lspReferenceParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, relative, lines, found := server.document(params.TextDocument.URI)
	if !found {
		return []byte("[]"), nil
	}
	target, found := lspTargetAt(project.build.index, relative, lines, params.Position, server.encoding)
	if !found {
		return []byte("[]"), nil
	}
	places := lspReferences(project.build.index, target, params.Context.IncludeDeclaration)
	return json.Marshal(server.locations(project, places))
}
