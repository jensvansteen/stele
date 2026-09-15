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

func ParseSpecs(root, changeID string) (ParsedSpecs, error) {
	specsRoot := filepath.Join(root, "openspec", "changes", changeID, "specs")
	files := walkFiles(specsRoot, func(path string) bool { return strings.EqualFold(filepath.Ext(path), ".md") })
	parsed := ParsedSpecs{Requirements: []Requirement{}, Diagnostics: []Diagnostic{}, Files: []string{}}

	for _, file := range files {
		relative, err := relativePath(root, file)
		if err != nil {
			return parsed, err
		}
		relative = filepath.ToSlash(relative)
		parsed.Files = append(parsed.Files, relative)
		handle, err := os.Open(file)
		if err != nil {
			return parsed, err
		}
		scanner := bufio.NewScanner(handle)
		lineNumber := 0
		requirementIndex := -1
		scenarioIndex := -1
		target := ""
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if match := requirementPattern.FindStringSubmatch(line); match != nil {
				parsed.Requirements = append(parsed.Requirements, Requirement{Title: strings.TrimSpace(match[1]), Source: Source{Path: relative, Line: lineNumber}, Scenarios: []Scenario{}})
				requirementIndex = len(parsed.Requirements) - 1
				scenarioIndex = -1
				target = "requirement"
				continue
			}
			if match := scenarioPattern.FindStringSubmatch(line); match != nil && requirementIndex >= 0 {
				requirement := &parsed.Requirements[requirementIndex]
				requirement.Scenarios = append(requirement.Scenarios, Scenario{Title: strings.TrimSpace(match[1]), Source: Source{Path: relative, Line: lineNumber}})
				scenarioIndex = len(requirement.Scenarios) - 1
				target = "scenario"
				continue
			}
			match := verificationIDPattern.FindStringSubmatch(line)
			if match == nil || target == "" {
				continue
			}
			identity := match[1]
			var existing string
			if target == "scenario" {
				existing = parsed.Requirements[requirementIndex].Scenarios[scenarioIndex].ID
			} else {
				existing = parsed.Requirements[requirementIndex].ID
			}
			if existing != "" {
				title := parsed.Requirements[requirementIndex].Title
				if target == "scenario" {
					title = parsed.Requirements[requirementIndex].Scenarios[scenarioIndex].Title
				}
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_MULTIPLE", "error", fmt.Sprintf("Multiple IDs declared for %s.", title), relative, lineNumber, ""))
				continue
			}
			validKind := (target == "scenario") == strings.HasPrefix(identity, "scn.")
			if !identityPattern.MatchString(identity) || !validKind {
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_FORMAT", "error", fmt.Sprintf("Invalid %s ID: %s.", target, identity), relative, lineNumber, ""))
			}
			if target == "scenario" {
				parsed.Requirements[requirementIndex].Scenarios[scenarioIndex].ID = identity
			} else {
				parsed.Requirements[requirementIndex].ID = identity
			}
		}
		if err := scanner.Err(); err != nil {
			_ = handle.Close()
			return parsed, err
		}
		_ = handle.Close()
	}

	identities := make(map[string]Source)
	for requirementIndex := range parsed.Requirements {
		requirement := &parsed.Requirements[requirementIndex]
		if requirement.ID == "" {
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_REQUIREMENT_MISSING", "error", fmt.Sprintf("Requirement %q has no Verification-ID.", requirement.Title), requirement.Source.Path, requirement.Source.Line, ""))
		}
		if len(requirement.Scenarios) == 0 {
			name := requirement.ID
			if name == "" {
				name = requirement.Title
			}
			parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("SCENARIO_MISSING", "error", fmt.Sprintf("Requirement %s has no scenarios.", name), requirement.Source.Path, requirement.Source.Line, ""))
		}
		if requirement.ID != "" {
			if _, exists := identities[requirement.ID]; exists {
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_DUPLICATE", "error", fmt.Sprintf("Identity %s is declared more than once.", requirement.ID), requirement.Source.Path, requirement.Source.Line, requirement.ID))
			}
			identities[requirement.ID] = requirement.Source
		}
		for scenarioIndex := range requirement.Scenarios {
			scenario := &requirement.Scenarios[scenarioIndex]
			if scenario.ID == "" {
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_SCENARIO_MISSING", "error", fmt.Sprintf("Scenario %q has no Verification-ID.", scenario.Title), scenario.Source.Path, scenario.Source.Line, ""))
				continue
			}
			if _, exists := identities[scenario.ID]; exists {
				parsed.Diagnostics = append(parsed.Diagnostics, diagnostic("ID_DUPLICATE", "error", fmt.Sprintf("Identity %s is declared more than once.", scenario.ID), scenario.Source.Path, scenario.Source.Line, scenario.ID))
			}
			identities[scenario.ID] = scenario.Source
		}
	}
	sort.Slice(parsed.Diagnostics, func(i, j int) bool {
		return diagnosticKey(parsed.Diagnostics[i]) < diagnosticKey(parsed.Diagnostics[j])
	})
	return parsed, nil
}

func diagnosticKey(value Diagnostic) string {
	path := ""
	line := 0
	if value.Source != nil {
		path = value.Source.Path
		line = value.Source.Line
	}
	return fmt.Sprintf("%s:%s:%09d", value.Code, path, line)
}
