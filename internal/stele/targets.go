package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// targetNamePattern is the form of a target name: a lowercase letter, then
// lowercase letters, digits, or hyphens.
var targetNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// TargetDefinition is one target of stele.config.json: where its code and
// tests live, and whether it only proves behavior, like an end-to-end suite.
type TargetDefinition struct {
	Paths        []string `json:"paths"`
	EvidenceOnly bool     `json:"evidenceOnly"`
}

// projectTargets is the validated target registry of a project, ordered by
// name. A nil registry means the project configures no targets.
type projectTargets struct {
	names       []string
	definitions map[string]TargetDefinition
	patterns    map[string][]*regexp.Regexp
}

// parseProjectTargets validates the targets of stele.config.json. Any problem
// names the target, so the command stops before verifying.
//
// @implements req.verificationtargets.fd65304d2bf7
func parseProjectTargets(raw map[string]json.RawMessage) (*projectTargets, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	targets := &projectTargets{
		definitions: make(map[string]TargetDefinition, len(raw)),
		patterns:    make(map[string][]*regexp.Regexp, len(raw)),
	}
	for name := range raw {
		targets.names = append(targets.names, name)
	}
	sort.Strings(targets.names)
	for _, name := range targets.names {
		problem := targetNameProblem(name)
		var definition TargetDefinition
		if problem == "" {
			definition, problem = decodeTargetDefinition(raw[name])
		}
		patterns := make([]*regexp.Regexp, 0, len(definition.Paths))
		for _, pattern := range definition.Paths {
			if problem != "" {
				break
			}
			compiled, err := compileTargetGlob(pattern)
			if err != nil {
				problem = err.Error()
			}
			patterns = append(patterns, compiled)
		}
		if problem != "" {
			return nil, fmt.Errorf("invalid target %q in stele.config.json: %s", name, problem)
		}
		targets.definitions[name] = definition
		targets.patterns[name] = patterns
	}
	return targets, nil
}

// targetNameProblem explains why a name cannot be a target, or returns "".
func targetNameProblem(name string) string {
	switch {
	case !targetNamePattern.MatchString(name):
		return "a target name is a lowercase letter followed by lowercase letters, digits, or hyphens"
	case slices.Contains(evidenceLevels, name):
		return "the evidence levels unit, integration, and e2e cannot be target names"
	}
	return ""
}

// decodeTargetDefinition reads one definition, rejecting unknown keys and a
// missing or empty paths array.
func decodeTargetDefinition(raw json.RawMessage) (TargetDefinition, string) {
	var definition TargetDefinition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return definition, strings.TrimPrefix(err.Error(), "json: ")
	}
	if len(definition.Paths) == 0 {
		return definition, "paths must list at least one glob pattern"
	}
	return definition, ""
}

// compileTargetGlob turns a path pattern into a regular expression: `*`
// matches within one path segment and `**` any number of segments.
func compileTargetGlob(pattern string) (*regexp.Regexp, error) {
	invalid := func(reason string) error {
		return fmt.Errorf("invalid path pattern %q: %s", pattern, reason)
	}
	switch {
	case strings.TrimSpace(pattern) == "":
		return nil, invalid("it is empty")
	case strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "\\"):
		return nil, invalid("use a slash-separated path relative to the project root")
	}
	var expression strings.Builder
	expression.WriteString("^")
	segments := strings.Split(pattern, "/")
	for index, segment := range segments {
		last := index == len(segments)-1
		switch {
		case segment == "" || segment == "." || segment == "..":
			return nil, invalid("it has an empty, `.`, or `..` segment")
		case segment == "**":
			expression.WriteString(choose(last, ".*", "(?:[^/]+/)*"))
			continue
		case strings.Contains(segment, "**"):
			return nil, invalid("`**` must be a whole path segment")
		}
		parts := strings.Split(segment, "*")
		for part, text := range parts {
			if part > 0 {
				expression.WriteString("[^/]*")
			}
			expression.WriteString(regexp.QuoteMeta(text))
		}
		if !last {
			expression.WriteString("/")
		}
	}
	expression.WriteString("$")
	return regexp.Compile(expression.String())
}

// configured reports whether the project configures a target.
func (targets *projectTargets) configured(name string) bool {
	if targets == nil {
		return false
	}
	_, found := targets.definitions[name]
	return found
}

// matches reports whether a repository path lies within a target's paths.
func (targets *projectTargets) matches(name, file string) bool {
	if targets == nil {
		return false
	}
	for _, pattern := range targets.patterns[name] {
		if pattern.MatchString(file) {
			return true
		}
	}
	return false
}

// evidenceOnly reports whether a target only proves behavior.
func (targets *projectTargets) evidenceOnly(name string) bool {
	return targets != nil && targets.definitions[name].EvidenceOnly
}

// paths lists a target's path patterns for messages.
func (targets *projectTargets) paths(name string) string {
	if targets == nil {
		return ""
	}
	return strings.Join(targets.definitions[name].Paths, ", ")
}

// list names the configured targets for messages.
func (targets *projectTargets) list() string {
	if targets == nil || len(targets.names) == 0 {
		return "none"
	}
	return strings.Join(targets.names, ", ")
}

// scanRoots returns the directories below which the targets' paths lie: the
// literal part of every pattern before its first wildcard segment.
func (targets *projectTargets) scanRoots() []string {
	if targets == nil {
		return nil
	}
	roots := make([]string, 0)
	for _, name := range targets.names {
		for _, pattern := range targets.definitions[name].Paths {
			literal := make([]string, 0)
			segments := strings.Split(pattern, "/")
			for index, segment := range segments {
				if strings.Contains(segment, "*") || index == len(segments)-1 {
					break
				}
				literal = append(literal, segment)
			}
			roots = append(roots, choose(len(literal) == 0, ".", path.Join(literal...)))
		}
	}
	sort.Strings(roots)
	return slices.Compact(roots)
}

// configuredTargetRoots reads the target scan roots of a project through its
// files. A project without targets, or with an unreadable configuration, has
// none; commands report configuration problems before they scan.
func configuredTargetRoots(repo repoFiles, root string) []string {
	config, err := readConfigFrom(repo, root)
	if err != nil {
		return nil
	}
	targets, err := parseProjectTargets(config.Targets)
	if err != nil {
		return nil
	}
	return targets.scanRoots()
}

// errNoTargets and errUnknownTargetSelection reject a --target that the
// project cannot satisfy.
var (
	errNoTargets = errors.New(
		"--target needs targets in stele.config.json, and this project configures none")
	errUnknownTargetSelection = errors.New("unknown target")
)

// validateTargetSelection checks every --target against the registry.
//
// @implements req.verificationtargets.366797f24e77
func validateTargetSelection(targets *projectTargets, selected []string) ([]string, error) {
	if len(selected) == 0 {
		return nil, nil
	}
	if targets == nil {
		return nil, errNoTargets
	}
	for _, name := range selected {
		if !targets.configured(name) {
			return nil, fmt.Errorf("%w %q; configured targets: %s", errUnknownTargetSelection, name, targets.list())
		}
	}
	result := append([]string{}, selected...)
	sort.Strings(result)
	return slices.Compact(result), nil
}

// targetSelection is the --target filter of a scope; an empty selection
// covers every target.
type targetSelection []string

// includes reports whether a target is selected. The empty target of an
// untargeted item is always included.
func (selection targetSelection) includes(target string) bool {
	return len(selection) == 0 || target == "" || slices.Contains(selection, target)
}

// active reports whether the command selects targets.
func (selection targetSelection) active() bool {
	return len(selection) > 0
}

// appliesTo reports whether an item with the given targets is in a
// selection. Untargeted items describe the whole project and are always in.
func (selection targetSelection) appliesTo(targeted bool, applicable []string) bool {
	if !targeted || !selection.active() {
		return true
	}
	return slices.ContainsFunc(applicable, selection.includes)
}

// parseTargetList reads a comma-separated target list. It returns the valid,
// distinct names in the written order and the problem of a malformed list.
func parseTargetList(value string) ([]string, string) {
	names := make([]string, 0)
	problems := make([]string, 0)
	for part := range strings.SplitSeq(value, ",") {
		name := strings.TrimSpace(part)
		switch {
		case name == "":
			problems = append(problems, "an empty name")
		case !targetNamePattern.MatchString(name) || slices.Contains(evidenceLevels, name):
			problems = append(problems, fmt.Sprintf("the invalid name %q", name))
		case slices.Contains(names, name):
			problems = append(problems, fmt.Sprintf("%q twice", name))
		default:
			names = append(names, name)
		}
	}
	if strings.TrimSpace(value) == "" {
		return names, "the list is empty"
	}
	if len(problems) > 0 {
		return names, "it has " + strings.Join(slices.Compact(problems), ", ")
	}
	return names, ""
}

// sortedTargets returns a sorted copy of a target list.
func sortedTargets(names []string) []string {
	result := append([]string{}, names...)
	sort.Strings(result)
	return result
}

// resolveSpecTargets gives every requirement and scenario of a targeted
// specification its applicable targets, and reports unknown, widening, and
// undeclared target lists. Narrowing follows the chain from the file's
// annotation through the requirement to the scenario.
//
// @implements req.verificationtargets.ab2f8aad3e78
// @implements req.verificationtargets.8d4a7d3bc335
func resolveSpecTargets(parsed *ParsedSpecs, targets *projectTargets) {
	files := make(map[string]SpecAnnotation, len(parsed.Annotations))
	for _, annotation := range parsed.Annotations {
		files[annotation.Path] = annotation
		for _, name := range annotation.Targets {
			if !targets.configured(name) {
				parsed.Diagnostics = append(parsed.Diagnostics,
					unknownTargetDiagnostic(annotation.Path, 1, name, targets))
			}
		}
	}
	for index := range parsed.Requirements {
		requirement := &parsed.Requirements[index]
		annotation := files[requirement.Source.Path]
		if !annotation.TargetsDeclared {
			reportUndeclared(parsed, requirement.targetsLine, requirement.Source.Path, requirement.ID)
			for scenarioIndex := range requirement.Scenarios {
				scenario := &requirement.Scenarios[scenarioIndex]
				reportUndeclared(parsed, scenario.targetsLine, scenario.Source.Path, scenario.ID)
			}
			continue
		}
		fileTargets := make([]string, 0, len(annotation.Targets))
		for _, name := range annotation.Targets {
			if targets.configured(name) {
				fileTargets = append(fileTargets, name)
			}
		}
		requirement.Targeted = true
		requirement.Targets = narrowTargets(parsed, targets, fileTargets, requirement.DeclaredTargets,
			requirement.targetsDeclared, requirement.Source.Path, requirement.targetsLine, requirement.ID)
		for scenarioIndex := range requirement.Scenarios {
			scenario := &requirement.Scenarios[scenarioIndex]
			scenario.Targeted = true
			scenario.Targets = narrowTargets(parsed, targets, requirement.Targets, scenario.DeclaredTargets,
				scenario.targetsDeclared, scenario.Source.Path, scenario.targetsLine, scenario.ID)
		}
	}
	sort.Slice(parsed.Diagnostics, func(i, j int) bool {
		return diagnosticKey(parsed.Diagnostics[i]) < diagnosticKey(parsed.Diagnostics[j])
	})
}

// narrowTargets returns an item's applicable targets: its parent's, or the
// declared names that its parent also applies to.
func narrowTargets(parsed *ParsedSpecs, targets *projectTargets, parent, declared []string, hasLine bool,
	file string, line int, identity string,
) []string {
	if !hasLine {
		return sortedTargets(parent)
	}
	applicable := make([]string, 0, len(declared))
	for _, name := range declared {
		switch {
		case !targets.configured(name):
			parsed.Diagnostics = append(parsed.Diagnostics, unknownTargetDiagnostic(file, line, name, targets))
		case !slices.Contains(parent, name):
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("SPEC_TARGETS_WIDENED", "error",
				fmt.Sprintf("Targets: names %s, which its parent does not apply to (%s); "+
					"a Targets: line may only narrow.",
					name, strings.Join(sortedTargets(parent), ", ")), file, line, identity))
		default:
			applicable = append(applicable, name)
		}
	}
	return sortedTargets(applicable)
}

func unknownTargetDiagnostic(file string, line int, name string, targets *projectTargets) Diagnostic {
	return diagnostic("SPEC_TARGET_UNKNOWN", "error",
		fmt.Sprintf("%s names target %s, which stele.config.json does not configure; configured targets: %s.",
			file, name, targets.list()), file, line, "")
}

func reportUndeclared(parsed *ParsedSpecs, line int, file, identity string) {
	if line == 0 {
		return
	}
	parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("SPEC_TARGETS_UNDECLARED", "error",
		fmt.Sprintf("%s has a Targets: line, but its first-line annotation declares no targets.", file),
		file, line, identity))
}

// compareDeltaTargets checks each delta spec of a change against the current
// specification of its capability: both declare targets or neither does, and
// a delta that changes the targets lists every current requirement whose
// applicable targets change.
//
// @implements req.verificationtargets.8d4a7d3bc335
func compareDeltaTargets(root string, scope verificationScope, parsed *ParsedSpecs) {
	if scope.currentSpecs {
		return
	}
	backend := scope.spec()
	for _, delta := range parsed.Annotations {
		current := backend.CurrentSpecFile(root, backend.Capability(scope, delta.Path))
		content, err := scope.files().readFile(current)
		if err != nil {
			continue
		}
		currentPath := repositoryPath(root, current)
		currentAnnotation := classifyAnnotation(currentPath, string(content)).result()
		switch {
		case delta.TargetsDeclared != currentAnnotation.TargetsDeclared:
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("SPEC_TARGETS_MISMATCH", "error",
				fmt.Sprintf("%s %s, but the current specification %s %s; copy its targets field, "+
					"or change targets deliberately.", delta.Path, declaresText(delta), currentPath,
					declaresText(currentAnnotation)), delta.Path, 1, ""))
		case delta.TargetsDeclared &&
			!slices.Equal(sortedTargets(delta.Targets), sortedTargets(currentAnnotation.Targets)):
			parsed.Diagnostics = append(parsed.Diagnostics,
				uncoveredTargetChanges(root, scope, parsed, delta, currentAnnotation, current)...)
		}
	}
}

func declaresText(annotation SpecAnnotation) string {
	if !annotation.TargetsDeclared {
		return "declares no targets"
	}
	return "declares the targets " + strings.Join(annotation.Targets, ", ")
}

// uncoveredTargetChanges names the current requirements whose applicable
// targets a delta's new target list changes and that the delta does not list.
func uncoveredTargetChanges(root string, scope verificationScope, parsed *ParsedSpecs, delta,
	currentAnnotation SpecAnnotation, current string,
) []Diagnostic {
	listed := make(map[string]bool)
	for _, requirement := range parsed.Requirements {
		if requirement.Source.Path == delta.Path {
			listed[requirement.Title] = true
		}
	}
	for _, removed := range slices.Concat(parsed.Removed, parsed.Renamed) {
		if removed.Source.Path == delta.Path {
			listed[removed.Name] = true
		}
	}
	// The file was just read, so a failing second read only leaves fewer
	// requirements to compare.
	currentParsed, _ := parseSpecFiles(scope.files(), root, []string{current}, "")
	diagnostics := make([]Diagnostic, 0)
	for _, requirement := range currentParsed.Requirements {
		before, after := sortedTargets(currentAnnotation.Targets), sortedTargets(delta.Targets)
		if requirement.targetsDeclared {
			before = intersectTargets(requirement.DeclaredTargets, before)
			after = intersectTargets(requirement.DeclaredTargets, after)
		}
		if slices.Equal(before, after) || listed[requirement.Title] {
			continue
		}
		diagnostics = append(diagnostics, diagnostic("SPEC_TARGETS_CHANGE_UNCOVERED", "error",
			fmt.Sprintf("%s changes the targets of its capability, which changes requirement %q (%s) from %s to %s; "+
				"list it as a modified requirement.", delta.Path, requirement.Title, requirement.ID,
				targetListText(before), targetListText(after)), delta.Path, 1, requirement.ID))
	}
	return diagnostics
}

func intersectTargets(declared, file []string) []string {
	result := make([]string, 0)
	for _, name := range declared {
		if slices.Contains(file, name) {
			result = append(result, name)
		}
	}
	return sortedTargets(result)
}

func targetListText(names []string) string {
	if len(names) == 0 {
		return "no targets"
	}
	return strings.Join(names, ", ")
}

// anchorTargetDiagnostics reports `@verifies` anchors of the scope whose
// evidence ID names a target that the anchor's file does not lie in.
//
// @implements req.verificationtargets.d1ac12f01077
func anchorTargetDiagnostics(input linkageValidationInput, known map[string]bool) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, anchor := range input.Anchors {
		if anchor.Target == "" || !known[anchor.ID] || !input.Targets.configured(anchor.Target) ||
			!input.Selection.includes(anchor.Target) || input.Targets.matches(anchor.Target, anchor.Path) {
			continue
		}
		outside := diagnostic("ANCHOR_TARGET_OUTSIDE_PATHS", "error",
			fmt.Sprintf("@verifies %s is evidence for target %s, but %s is outside its paths (%s).",
				anchor.EvidenceID, anchor.Target, anchor.Path, input.Targets.paths(anchor.Target)),
			anchor.Path, anchor.Line, anchor.EvidenceID)
		outside.target = anchor.Target
		diagnostics = append(diagnostics, outside)
	}
	return diagnostics
}

// targetImplementationDiagnostics requires, in the implementation stage, an
// `@implements` anchor within the paths of every target a requirement applies
// to, except targets that only prove behavior.
//
// @implements req.verificationtargets.d1ac12f01077
func targetImplementationDiagnostics(input linkageValidationInput, requirement Requirement) []Diagnostic {
	if input.Mode != "implementation" || !requirement.Targeted {
		return nil
	}
	anchors := anchorsFor(input.Anchors, requirement.ID, "code")
	diagnostics := make([]Diagnostic, 0)
	for _, target := range requirement.Targets {
		if input.Targets.evidenceOnly(target) || !input.Selection.includes(target) {
			continue
		}
		implemented := false
		for _, anchor := range anchors {
			implemented = implemented || input.Targets.matches(target, anchor.Path)
		}
		if implemented {
			continue
		}
		missing := diagnostic("LINK_TARGET_IMPLEMENTATION_MISSING", "error",
			fmt.Sprintf("No @implements %s lies within the paths of target %s (%s).", requirement.ID, target,
				input.Targets.paths(target)),
			requirement.Source.Path, requirement.Source.Line, requirement.ID)
		missing.target = target
		diagnostics = append(diagnostics, missing)
	}
	return diagnostics
}
