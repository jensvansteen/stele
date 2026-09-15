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
	for _, skill := range []string{"stele-plan", "stele-verify"} {
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
		relative, _ := filepath.Rel(root, destination)
		created = append(created, filepath.ToSlash(relative))
	}
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		return nil, err
	}
	return created, nil
}
