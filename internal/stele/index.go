package stele

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

// indexSchemaVersion is the version of the link index document.
const indexSchemaVersion = 1

// Index is the deterministic link index an editor reads: every requirement,
// scenario, anchor, planned evidence entry, and last execution outcome of the
// selected scopes.
type Index struct {
	SchemaVersion int                `json:"schemaVersion"`
	Scopes        []IndexScope       `json:"scopes"`
	Requirements  []IndexRequirement `json:"requirements"`
	Scenarios     []IndexScenario    `json:"scenarios"`
	Anchors       []IndexAnchor      `json:"anchors"`
}

// IndexScope names one scope of the index: a change or the current specifications.
type IndexScope struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// IndexLocation is a code or test declaration linked to an identity.
type IndexLocation struct {
	Path     string  `json:"path"`
	Line     int     `json:"line"`
	Selector *string `json:"selector"`
}

// IndexRequirement is one requirement of one scope.
type IndexRequirement struct {
	ID              string          `json:"id"`
	Scope           string          `json:"scope"`
	Title           string          `json:"title"`
	Text            string          `json:"text"`
	Source          Source          `json:"source"`
	Scenarios       []string        `json:"scenarios"`
	Implementations []IndexLocation `json:"implementations"`
}

// IndexScenario is one scenario of one scope with its evidence.
type IndexScenario struct {
	ID          string          `json:"id"`
	Scope       string          `json:"scope"`
	Requirement string          `json:"requirement"`
	Title       string          `json:"title"`
	Text        string          `json:"text"`
	Steps       []ScenarioStep  `json:"steps"`
	Source      Source          `json:"source"`
	Evidence    []IndexEvidence `json:"evidence"`
}

// IndexEvidence is one planned or anchored piece of evidence of a scenario.
// Approval is approved, unapproved, or stale for a version 2 plan entry, v1
// for a scenario with a version 1 target, and unplanned otherwise.
type IndexEvidence struct {
	ID        string          `json:"id"`
	Level     string          `json:"level,omitempty"`
	Approval  string          `json:"approval"`
	Placement string          `json:"placement,omitempty"`
	Target    string          `json:"target,omitempty"`
	Tests     []IndexLocation `json:"tests"`
	Execution ExecutionState  `json:"execution"`
}

// IndexAnchor is one anchor, listed once per scope that declares its
// identity. Status is linked, unresolved (no selector), other-scope (declared
// only outside the indexed scopes), or undeclared.
type IndexAnchor struct {
	ID         string  `json:"id"`
	Scope      string  `json:"scope,omitempty"`
	Annotation string  `json:"annotation"`
	Kind       string  `json:"kind"`
	Path       string  `json:"path"`
	Line       int     `json:"line"`
	Selector   *string `json:"selector"`
	EvidenceID string  `json:"evidenceId,omitempty"`
	Level      string  `json:"level,omitempty"`
	Status     string  `json:"status"`
}

// IndexScopeInput is the parsed specifications and plan of one scope.
type IndexScopeInput struct {
	Scope  IndexScope
	Parsed ParsedSpecs
	Plan   LinkagePlan
}

// IndexInput is everything the index is built from. Building does no I/O, so
// a language server can call BuildIndex in process with data it keeps loaded.
type IndexInput struct {
	Scopes      []IndexScopeInput
	Anchors     []Anchor
	Declared    map[string]bool
	Evidence    *Evidence
	InputDigest string
}

// BuildIndex builds the link index from parsed inputs. Scopes keep their
// order; requirements and scenarios follow spec order within a scope, and
// anchors are sorted by scope, path, and line.
//
// @implements req.linkindex.78a6abc9c59d
func BuildIndex(input IndexInput) Index {
	index := Index{
		SchemaVersion: indexSchemaVersion,
		Scopes:        []IndexScope{},
		Requirements:  []IndexRequirement{},
		Scenarios:     []IndexScenario{},
		Anchors:       []IndexAnchor{},
	}
	executions := indexExecutions(input.Evidence)
	for _, scope := range input.Scopes {
		index.Scopes = append(index.Scopes, scope.Scope)
		for _, requirement := range scope.Parsed.Requirements {
			entry := IndexRequirement{
				ID:              requirement.ID,
				Scope:           scope.Scope.ID,
				Title:           requirement.Title,
				Text:            requirement.Text,
				Source:          requirement.Source,
				Scenarios:       []string{},
				Implementations: indexLocations(anchorsFor(input.Anchors, requirement.ID, "code")),
			}
			for _, scenario := range requirement.Scenarios {
				entry.Scenarios = append(entry.Scenarios, scenario.ID)
				index.Scenarios = append(index.Scenarios, IndexScenario{
					ID:          scenario.ID,
					Scope:       scope.Scope.ID,
					Requirement: requirement.ID,
					Title:       scenario.Title,
					Text:        scenario.Body,
					Steps:       append([]ScenarioStep{}, scenario.Steps...),
					Source:      scenario.Source,
					Evidence:    indexEvidence(scope.Plan, scenario, input.Anchors, executions, input.InputDigest),
				})
			}
			index.Requirements = append(index.Requirements, entry)
		}
	}
	index.Anchors = indexAnchors(input)
	return index
}

func indexLocations(anchors []Anchor) []IndexLocation {
	locations := make([]IndexLocation, 0, len(anchors))
	for _, anchor := range anchors {
		locations = append(locations, IndexLocation{Path: anchor.Path, Line: anchor.Line, Selector: anchor.Selector})
	}
	sort.SliceStable(locations, func(i, j int) bool {
		return fmt.Sprintf("%s:%09d", locations[i].Path, locations[i].Line) <
			fmt.Sprintf("%s:%09d", locations[j].Path, locations[j].Line)
	})
	return locations
}

// indexEvidence lists a scenario's evidence: the entries of a version 2 plan,
// or otherwise its test anchors grouped by evidence ID.
func indexEvidence(
	plan LinkagePlan,
	scenario Scenario,
	anchors []Anchor,
	executions map[testGroupKey]TestExecution,
	inputDigest string,
) []IndexEvidence {
	tests := anchorsFor(anchors, scenario.ID, "test")
	result := make([]IndexEvidence, 0)
	if plan.usesEvidence(scenario.ID) {
		for _, entry := range plan.Evidence[scenario.ID] {
			entryTests := make([]Anchor, 0)
			for _, anchor := range tests {
				if anchor.EvidenceID == entry.ID {
					entryTests = append(entryTests, anchor)
				}
			}
			result = append(result, IndexEvidence{
				ID:        entry.ID,
				Level:     entry.Level,
				Approval:  approvalState(scenario, entry),
				Placement: entry.Placement,
				Tests:     indexLocations(entryTests),
				Execution: evidenceExecution(entryTests, executions, inputDigest),
			})
		}
		return result
	}

	target := plan.Scenarios[scenario.ID]
	approval := choose(target != "", "v1", "unplanned")
	groups := make(map[string][]Anchor)
	identities := make([]string, 0)
	for _, anchor := range tests {
		identity := anchor.EvidenceID
		if identity == "" {
			identity = scenario.ID
		}
		if _, seen := groups[identity]; !seen {
			identities = append(identities, identity)
		}
		groups[identity] = append(groups[identity], anchor)
	}
	sort.Strings(identities)
	for _, identity := range identities {
		group := groups[identity]
		result = append(result, IndexEvidence{
			ID:        identity,
			Level:     group[0].Level,
			Approval:  approval,
			Target:    target,
			Tests:     indexLocations(group),
			Execution: evidenceExecution(group, executions, inputDigest),
		})
	}
	if len(result) == 0 && target != "" {
		result = append(result, IndexEvidence{
			ID:        scenario.ID,
			Approval:  approval,
			Target:    target,
			Tests:     []IndexLocation{},
			Execution: ExecutionState{State: "not-run", Outcome: "not-run"},
		})
	}
	return result
}

// indexExecutions maps each stored execution to its test, filling in the
// file's digest for executions recorded before evidence schema 3.
func indexExecutions(evidence *Evidence) map[testGroupKey]TestExecution {
	executions := make(map[testGroupKey]TestExecution)
	if evidence == nil {
		return executions
	}
	for _, execution := range evidence.Executions {
		if execution.InputDigest == "" {
			execution.InputDigest = evidence.InputDigest
		}
		executions[executionKey(execution)] = execution
	}
	return executions
}

// evidenceExecution returns the last known outcome of an evidence entry's
// tests. It has not run while any test lacks an execution, fails when any
// test failed, and is stale when any execution used other inputs.
func evidenceExecution(
	tests []Anchor,
	executions map[testGroupKey]TestExecution,
	inputDigest string,
) ExecutionState {
	if len(tests) == 0 {
		return ExecutionState{State: "not-run", Outcome: "not-run"}
	}
	state := ExecutionState{State: "executed", Outcome: "passed"}
	for _, anchor := range tests {
		execution, found := executions[testGroupKey{Path: anchor.Path, Selector: pointerValue(anchor.Selector)}]
		if !found {
			return ExecutionState{State: "not-run", Outcome: "not-run"}
		}
		if execution.Outcome != "passed" {
			state.Outcome = "failed"
		}
		if execution.InputDigest != inputDigest {
			state.State = "stale"
		}
	}
	return state
}

// indexAnchors lists every anchor once per indexed scope that declares its
// identity, and once without a scope when no indexed scope declares it.
func indexAnchors(input IndexInput) []IndexAnchor {
	declaring := make(map[string][]string)
	order := make(map[string]int, len(input.Scopes))
	for position, scope := range input.Scopes {
		order[scope.Scope.ID] = position
		for identity := range knownIdentities(scope.Parsed) {
			declaring[identity] = append(declaring[identity], scope.Scope.ID)
		}
	}
	anchors := make([]IndexAnchor, 0, len(input.Anchors))
	for _, anchor := range input.Anchors {
		entry := IndexAnchor{
			ID:         anchor.ID,
			Annotation: anchor.Annotation,
			Kind:       anchor.Kind,
			Path:       anchor.Path,
			Line:       anchor.Line,
			Selector:   anchor.Selector,
			EvidenceID: anchor.EvidenceID,
			Level:      anchor.Level,
		}
		scopes := declaring[anchor.ID]
		if len(scopes) == 0 {
			entry.Status = choose(input.Declared[anchor.ID], "other-scope", "undeclared")
			anchors = append(anchors, entry)
			continue
		}
		entry.Status = choose(anchor.Selector == nil, "unresolved", "linked")
		for _, scope := range scopes {
			entry.Scope = scope
			anchors = append(anchors, entry)
		}
	}
	key := func(anchor IndexAnchor) string {
		position := len(input.Scopes)
		if anchor.Scope != "" {
			position = order[anchor.Scope]
		}
		return fmt.Sprintf("%09d:%s:%09d:%s:%s", position, anchor.Path, anchor.Line, anchor.ID, anchor.EvidenceID)
	}
	sort.SliceStable(anchors, func(i, j int) bool { return key(anchors[i]) < key(anchors[j]) })
	return anchors
}

// indexScopes returns the scopes an index command covers: every scope with
// --all, the current specifications with --specs, and otherwise the selected
// change followed by the current specifications when there are any.
func indexScopes(parsed options) ([]verificationScope, error) {
	if parsed.allScopes {
		scopes := everyScope(parsed.root, parsed.backend)
		if len(scopes) == 0 {
			return nil, errors.New("no current specifications and no active changes to index")
		}
		return scopes, nil
	}
	selected := resolveScope(parsed)
	if err := requireScopeSpecs(parsed.root, selected); err != nil {
		return nil, err
	}
	scopes := []verificationScope{selected}
	specs := verificationScope{currentSpecs: true, backend: parsed.backend}
	if !selected.currentSpecs && len(specs.spec().SpecFiles(parsed.root, specs)) > 0 {
		scopes = append(scopes, specs)
	}
	return scopes, nil
}

// LoadIndex reads the specifications, plans, anchors, and stored evidence of
// the scopes and builds their index.
func loadIndex(root string, scopes []verificationScope) (Index, error) {
	input := IndexInput{Scopes: make([]IndexScopeInput, 0, len(scopes))}
	for _, scope := range scopes {
		parsed, err := parseScopeSpecs(root, scope)
		if err != nil {
			return Index{}, err
		}
		plan, _ := loadScopePlan(root, scope)
		name, kind, _ := scopeName(scope)
		input.Scopes = append(input.Scopes, IndexScopeInput{
			Scope:  IndexScope{ID: name, Kind: kind},
			Parsed: parsed,
			Plan:   plan,
		})
	}
	var err error
	if input.Anchors, err = ScanAnchors(root); err != nil {
		return Index{}, err
	}
	if input.Declared, err = scopes[0].spec().DeclaredIdentities(root); err != nil {
		return Index{}, err
	}
	if input.InputDigest, err = ComputeInputDigest(root); err != nil {
		return Index{}, err
	}
	var evidence Evidence
	if readJSON(filepath.Join(root, defaultEvidencePath), &evidence) {
		input.Evidence = &evidence
	}
	return BuildIndex(input), nil
}

// indexCommand prints the link index, or writes it with --output-file.
func indexCommand(parsed options, stdout, stderr io.Writer) int {
	scopes, err := indexScopes(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	index, err := loadIndex(parsed.root, scopes)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if parsed.outputPath == "" {
		writeMachineJSON(stdout, index)
		return 0
	}
	if err := writeJSON(resolveWithin(parsed.root, parsed.outputPath), index); err != nil {
		return writeCommandError(stderr, fmt.Errorf("writing the index: %w", err))
	}
	return 0
}
