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
	// scenarioText collects the lines of the open scenario block.
	scenarioText *string
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
func parseScopeSpecs(root string, scope verificationScope) (ParsedSpecs, error) {
	return scope.spec().ParseSpecs(root, scope)
}

// parseSpecFiles parses OpenSpec requirement and scenario blocks.
func parseSpecFiles(root string, files []string) (ParsedSpecs, error) {
	parsed := ParsedSpecs{Requirements: []Requirement{}, Diagnostics: []Diagnostic{}, Files: []string{}}

	for _, file := range files {
		relative, err := relativePath(root, file)
		if err != nil {
			return parsed, err
		}
		relative = filepath.ToSlash(relative)
		parsed.Files = append(parsed.Files, relative)

		requirements, diagnostics, err := parseSpecFile(file, relative)
		parsed.Requirements = append(parsed.Requirements, requirements...)
		parsed.Diagnostics = append(parsed.Diagnostics, diagnostics...)
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

func parseSpecFile(path, relativePath string) ([]Requirement, []Diagnostic, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = handle.Close() }()

	parser := specFileParser{path: relativePath, target: noIdentityTarget}
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		parser.line++
		parser.parseLine(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return parser.requirements, parser.diagnostics, err
	}
	return parser.requirements, parser.diagnostics, nil
}

func (parser *specFileParser) parseLine(line string) {
	if strings.HasPrefix(line, "#") {
		// Any heading ends the text of the current scenario.
		parser.scenarioText = nil
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
}

func (parser *specFileParser) beginRequirement(title string) {
	parser.requirements = append(parser.requirements, Requirement{
		Title:     strings.TrimSpace(title),
		Source:    Source{Path: parser.path, Line: parser.line},
		Scenarios: []Scenario{},
	})
	parser.currentRequirement = &parser.requirements[len(parser.requirements)-1]
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
