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
	created := make([]string, 0)
	configPath := filepath.Join(root, "stele.config.json")
	if !fileExists(configPath) {
		config := Config{SchemaVersion: 1, Adapter: "openspec", Change: changeID}
		content, err := marshalConfig(config, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, append(content, '\n'), 0o644); err != nil {
			return nil, err
		}
		created = append(created, "stele.config.json")
	}
	installed, err := installSkills(root)
	if err != nil {
		return nil, err
	}
	created = append(created, installed...)
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		return nil, err
	}
	return created, nil
}

// installSkills installs every missing Stele skill and leaves existing skill
// files, including project skills, unchanged.
//
// @implements req.lifecycle.4c9232a6cf44
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
