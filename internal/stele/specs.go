package stele

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	identityPattern       = regexp.MustCompile(`^(req|scn)\.[a-z0-9]+\.[a-f0-9]{12}$`)
	requirementPattern    = regexp.MustCompile(`^### Requirement:\s*(.+)$`)
	scenarioPattern       = regexp.MustCompile(`^#### Scenario:\s*(.+)$`)
	verificationIDPattern = regexp.MustCompile(`^Verification-ID:\s*(\S+)\s*$`)
	stepBulletPattern     = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	stepKeywordPattern    = regexp.MustCompile(`^\*\*([A-Za-z][A-Za-z ]*?)\*\*:?\s*(.*)$`)
)

type identityTarget uint8

const (
	noIdentityTarget identityTarget = iota
	requirementIdentityTarget
	scenarioIdentityTarget
)

type specFileParser struct {
	path               string
	line               int
	target             identityTarget
	requirements       []Requirement
	diagnostics        []Diagnostic
	currentRequirement *Requirement
	currentScenario    *Scenario
	// scenarioText collects the lines of the open scenario block, and
	// requirementText those of the open requirement body.
	scenarioText    *string
	requirementText *string
}

func diagnostic(code, severity, message, path string, line int, identity string) Diagnostic {
	var source *Source
	if path != "" {
		source = &Source{Path: filepath.ToSlash(path), Line: line}
	}
	var identityID *string
	if identity != "" {
		value := identity
		identityID = &value
	}
	return Diagnostic{Code: code, Severity: severity, Message: message, IdentityID: identityID, Source: source}
}

// @implements req.verify.9dbf2146c01f
func ParseSpecs(root, changeID string) (ParsedSpecs, error) {
	return parseScopeSpecs(root, changeScope(changeID))
}

// parseScopeSpecs reads a scope's specifications through its backend.
// It applies the scope's policy to missing and misplaced annotations.
func parseScopeSpecs(root string, scope verificationScope) (ParsedSpecs, error) {
	parsed, err := scope.spec().ParseSpecs(root, scope)
	applyAnnotationPolicy(parsed.Diagnostics, scope.unannotated)
	return parsed, err
}

// parseSpecFiles parses OpenSpec requirement and scenario blocks and each
// file's annotation. fix names the command that adds missing annotations.
func parseSpecFiles(root string, files []string, fix string) (ParsedSpecs, error) {
	parsed := ParsedSpecs{
		Requirements: []Requirement{},
		Diagnostics:  []Diagnostic{},
		Files:        []string{},
		Annotations:  []SpecAnnotation{},
	}

	for _, file := range files {
		relative, err := relativePath(root, file)
		if err != nil {
			return parsed, err
		}
		relative = filepath.ToSlash(relative)
		parsed.Files = append(parsed.Files, relative)

		requirements, diagnostics, annotation, err := parseSpecFile(file, relative, fix)
		parsed.Requirements = append(parsed.Requirements, requirements...)
		parsed.Diagnostics = append(parsed.Diagnostics, diagnostics...)
		parsed.Annotations = append(parsed.Annotations, annotation)
		if err != nil {
			return parsed, err
		}
	}

	validateSpecSet(&parsed)
	sort.Slice(parsed.Diagnostics, func(i, j int) bool {
		return diagnosticKey(parsed.Diagnostics[i]) < diagnosticKey(parsed.Diagnostics[j])
	})
	return parsed, nil
}

func parseSpecFile(path, relativePath, fix string) ([]Requirement, []Diagnostic, SpecAnnotation, error) {
	classifier := newAnnotationClassifier(relativePath)
	handle, err := os.Open(path)
	if err != nil {
		return nil, nil, classifier.result(), err
	}
	defer func() { _ = handle.Close() }()

	parser := specFileParser{path: relativePath, target: noIdentityTarget}
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		parser.line++
		classifier.line(parser.line, scanner.Text())
		parser.parseLine(scanner.Text())
	}
	finishSpecText(parser.requirements)
	parser.diagnostics = append(parser.diagnostics, classifier.diagnostics(fix)...)
	return parser.requirements, parser.diagnostics, classifier.result(), scanner.Err()
}

// finishSpecText trims the collected requirement text and derives each
// scenario's body and steps.
func finishSpecText(requirements []Requirement) {
	for index := range requirements {
		requirement := &requirements[index]
		requirement.Text = strings.TrimSpace(requirement.Text)
		for scenarioIndex := range requirement.Scenarios {
			scenario := &requirement.Scenarios[scenarioIndex]
			_, body, _ := strings.Cut(scenario.Text, "\n")
			scenario.Body = strings.TrimSpace(body)
			scenario.Steps = scenarioSteps(scenario.Body)
		}
	}
}

// scenarioSteps returns one step per bullet of a scenario body. A bullet's
// continuation lines join its text; text outside bullets stays only in the
// body, and a bullet without a bold keyword has an empty keyword.
//
// @implements req.linkindex.78a6abc9c59d
func scenarioSteps(body string) []ScenarioStep {
	steps := make([]ScenarioStep, 0)
	var current *ScenarioStep
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := stepBulletPattern.FindStringSubmatch(trimmed); match != nil {
			step := ScenarioStep{Text: match[1]}
			if keyword := stepKeywordPattern.FindStringSubmatch(match[1]); keyword != nil {
				step = ScenarioStep{Keyword: strings.ToUpper(keyword[1]), Text: keyword[2]}
			}
			steps = append(steps, step)
			current = &steps[len(steps)-1]
			continue
		}
		if trimmed == "" || current == nil {
			current = nil
			continue
		}
		current.Text = strings.TrimSpace(current.Text + " " + trimmed)
	}
	return steps
}

func (parser *specFileParser) parseLine(line string) {
	if strings.HasPrefix(line, "#") {
		// Any heading ends the text of the current requirement or scenario.
		parser.scenarioText = nil
		parser.requirementText = nil
	}
	if match := requirementPattern.FindStringSubmatch(line); match != nil {
		parser.beginRequirement(match[1])
		return
	}
	if match := scenarioPattern.FindStringSubmatch(line); match != nil {
		parser.beginScenario(match[1])
		return
	}
	if match := verificationIDPattern.FindStringSubmatch(line); match != nil {
		parser.assignIdentity(match[1])
		return
	}
	if parser.scenarioText != nil {
		*parser.scenarioText += "\n" + line
	}
	if parser.requirementText != nil {
		*parser.requirementText += "\n" + line
	}
}

func (parser *specFileParser) beginRequirement(title string) {
	parser.requirements = append(parser.requirements, Requirement{
		Title:     strings.TrimSpace(title),
		Source:    Source{Path: parser.path, Line: parser.line},
		Scenarios: []Scenario{},
	})
	parser.currentRequirement = &parser.requirements[len(parser.requirements)-1]
	parser.requirementText = &parser.currentRequirement.Text
	parser.currentScenario = nil
	parser.target = requirementIdentityTarget
}

func (parser *specFileParser) beginScenario(title string) {
	if parser.currentRequirement == nil {
		return
	}

	requirement := parser.currentRequirement
	requirement.Scenarios = append(requirement.Scenarios, Scenario{
		Title:  strings.TrimSpace(title),
		Source: Source{Path: parser.path, Line: parser.line},
	})
	parser.currentScenario = &requirement.Scenarios[len(requirement.Scenarios)-1]
	parser.currentScenario.Text = strings.TrimSpace(title)
	parser.scenarioText = &parser.currentScenario.Text
	parser.target = scenarioIdentityTarget
}

func (parser *specFileParser) assignIdentity(identity string) {
	var targetName string
	var title string
	var targetID *string

	switch parser.target {
	case requirementIdentityTarget:
		targetName = "requirement"
		title = parser.currentRequirement.Title
		targetID = &parser.currentRequirement.ID
	case scenarioIdentityTarget:
		targetName = "scenario"
		title = parser.currentScenario.Title
		targetID = &parser.currentScenario.ID
	default:
		return
	}

	if *targetID != "" {
		parser.addDiagnostic(
			"ID_MULTIPLE",
			fmt.Sprintf("Multiple IDs declared for %s.", title),
			"",
		)
		return
	}

	isScenario := parser.target == scenarioIdentityTarget
	validKind := isScenario == strings.HasPrefix(identity, "scn.")
	if !identityPattern.MatchString(identity) || !validKind {
		parser.addDiagnostic(
			"ID_FORMAT",
			fmt.Sprintf("Invalid %s ID: %s.", targetName, identity),
			"",
		)
	}
	*targetID = identity
}

func (parser *specFileParser) addDiagnostic(code, message, identity string) {
	parser.diagnostics = append(
		parser.diagnostics,
		diagnostic(code, "error", message, parser.path, parser.line, identity),
	)
}

func validateSpecSet(parsed *ParsedSpecs) {
	identities := make(map[string]struct{})
	for requirementIndex := range parsed.Requirements {
		requirement := &parsed.Requirements[requirementIndex]
		if requirement.ID == "" {
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic(
				"ID_REQUIREMENT_MISSING",
				"error",
				fmt.Sprintf("Requirement %q has no Verification-ID.", requirement.Title),
				requirement.Source.Path,
				requirement.Source.Line,
				"",
			))
		}
		if len(requirement.Scenarios) == 0 {
			name := requirement.ID
			if name == "" {
				name = requirement.Title
			}
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic(
				"SCENARIO_MISSING",
				"error",
				fmt.Sprintf("Requirement %s has no scenarios.", name),
				requirement.Source.Path,
				requirement.Source.Line,
				"",
			))
		}
		if requirement.ID != "" {
			recordIdentity(parsed, identities, requirement.ID, requirement.Source)
		}
		for scenarioIndex := range requirement.Scenarios {
			scenario := &requirement.Scenarios[scenarioIndex]
			if scenario.ID == "" {
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic(
					"ID_SCENARIO_MISSING",
					"error",
					fmt.Sprintf("Scenario %q has no Verification-ID.", scenario.Title),
					scenario.Source.Path,
					scenario.Source.Line,
					"",
				))
				continue
			}
			recordIdentity(parsed, identities, scenario.ID, scenario.Source)
		}
	}
}

func recordIdentity(parsed *ParsedSpecs, identities map[string]struct{}, identity string, source Source) {
	if _, exists := identities[identity]; exists {
		parsed.Diagnostics = append(parsed.Diagnostics, diagnostic(
			"ID_DUPLICATE",
			"error",
			fmt.Sprintf("Identity %s is declared more than once.", identity),
			source.Path,
			source.Line,
			identity,
		))
	}
	identities[identity] = struct{}{}
}

func diagnosticKey(value Diagnostic) string {
	path := ""
	line := 0
	if value.Source != nil {
		path = value.Source.Path
		line = value.Source.Line
	}
	identity := ""
	if value.IdentityID != nil {
		identity = *value.IdentityID
	}
	return fmt.Sprintf("%s:%s:%09d:%s:%s", value.Code, path, line, identity, value.Message)
}
