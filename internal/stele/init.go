package stele

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/*.md
var skillTemplates embed.FS

var readSkillTemplate = skillTemplates.ReadFile

var marshalConfig = json.MarshalIndent

// steleSkills lists the lifecycle entry points first, then the reference skills.
var steleSkills = []string{"stele-propose", "stele-apply", "stele-archive", "stele-plan", "stele-verify"}

// Initialize writes the Stele configuration, installs the Stele skills, and
// ensures the artifacts directory exists. An empty changeID leaves the
// configuration without a default change.
//
// @implements req.init.eed35c447821
// @implements req.verificationstrategy.ace39f5009c6
func Initialize(root, changeID string) ([]string, error) {
	created, _, err := initializeBackend(root, changeID, specificationBackends[defaultAdapter])
	return created, err
}

// initializeBackend returns the files it created or annotated, and warnings
// about specification files whose annotation a person has to fix.
func initializeBackend(root, changeID string, backend specificationBackend) ([]string, []string, error) {
	created := make([]string, 0)
	configPath := filepath.Join(root, "stele.config.json")
	if !fileExists(configPath) {
		config := Config{SchemaVersion: 1, Adapter: backend.Name(), Change: changeID}
		content, err := marshalConfig(config, "", "  ")
		if err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(configPath, append(content, '\n'), 0o644); err != nil {
			return nil, nil, err
		}
		created = append(created, "stele.config.json")
	}
	installed, err := installSkills(root)
	if err != nil {
		return nil, nil, err
	}
	created = append(created, installed...)
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		return nil, nil, err
	}
	annotated, warnings, err := annotateInitialSpecs(root, backend)
	if err != nil {
		return nil, nil, err
	}
	return append(created, annotated...), warnings, nil
}

// annotateInitialSpecs annotates the current specifications and the delta
// specs of every active change, never archived changes. It returns the
// annotated files and a warning for each file it cannot annotate.
//
// @implements req.specannotation.0784be141698
func annotateInitialSpecs(root string, backend specificationBackend) ([]string, []string, error) {
	result, err := annotateScopes(root, everyScope(root, backend), false)
	if err != nil {
		return nil, nil, err
	}
	annotated := make([]string, 0)
	warnings := make([]string, 0)
	for _, file := range result.Files {
		switch {
		case file.Changed:
			annotated = append(annotated, file.Path)
		case file.State != annotationAnnotated:
			warnings = append(warnings, fmt.Sprintf("%s %s", file.Path, annotationProblem[file.State]))
		}
	}
	return annotated, warnings, nil
}

// installSkills installs every missing Stele skill and leaves existing skill
// files, including project skills, unchanged.
//
// @implements req.lifecycle.4c9232a6cf44
// @implements req.specannotation.1707277552af
func installSkills(root string) ([]string, error) {
	created := make([]string, 0)
	for _, skill := range steleSkills {
		destination := filepath.Join(root, ".agents", "skills", skill, "SKILL.md")
		if fileExists(destination) {
			continue
		}
		content, err := readSkillTemplate(fmt.Sprintf("templates/%s.md", skill))
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(destination, content, 0o644); err != nil {
			return nil, err
		}
		created = append(created, filepath.ToSlash(filepath.Join(".agents", "skills", skill, "SKILL.md")))
	}
	return created, nil
}
