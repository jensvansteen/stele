package stele

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed templates/openspec-extend.mjs
var openSpecExtendScript string

const defaultOpenSpecTools = "agents"

// openSpecSetup holds the init options that concern the OpenSpec backend.
type openSpecSetup struct {
	tools         string
	refreshSchema bool
}

type openSpecExtendResult struct {
	ConfigChanged bool     `json:"configChanged"`
	Notes         []string `json:"notes"`
}

var (
	setUpOpenSpec = installWorkflowSchema
	runNodeScript = runNode
)

// installWorkflowSchema prepares OpenSpec for Stele: it initializes OpenSpec with
// the bundled CLI when the project has none, forks spec-driven into the stele
// schema, and merges the Stele guidance. It returns notes for the user.
//
// @implements req.workflowschema.97dec0adc04d
func installWorkflowSchema(root string, setup openSpecSetup) ([]string, error) {
	notes := make([]string, 0)
	openSpecDir := filepath.Join(root, "openspec")
	if !directoryExists(openSpecDir) {
		if output, err := runBundledOpenSpec(root, "init", "--tools", setup.tools, "--no-animation", "."); err != nil {
			return nil, openSpecFailure("openspec init", output, err)
		}
		notes = append(notes, fmt.Sprintf("Initialized OpenSpec %s for %s.", OpenSpecVersion, setup.tools))
		if setup.tools == defaultOpenSpecTools {
			note, err := linkClaudeSkills(root)
			if err != nil {
				return nil, err
			}
			notes = append(notes, note)
		}
	}
	if !fileExists(filepath.Join(openSpecDir, "config.yaml")) {
		return append(notes, "openspec/config.yaml was not found; skipped the Stele schema and guidance."), nil
	}

	schemaDir := filepath.Join(openSpecDir, "schemas", "stele")
	fork := setup.refreshSchema || !directoryExists(schemaDir)
	if fork {
		arguments := []string{"schema", "fork", "spec-driven", "stele"}
		if setup.refreshSchema {
			arguments = append(arguments, "--force")
		}
		if output, err := runBundledOpenSpec(root, arguments...); err != nil {
			return nil, openSpecFailure("openspec schema fork", output, err)
		}
		notes = append(notes, "Installed the stele workflow schema in openspec/schemas/stele.")
	} else {
		notes = append(notes,
			"Kept the existing stele schema; run stele init --refresh-schema after changing the OpenSpec version.")
	}
	guidance, err := mergeOpenSpecGuidance(root, fork)
	if err != nil {
		return nil, err
	}
	return append(notes, guidance...), nil
}

// openSpecFailure explains a failed OpenSpec command with its output, or with the
// process error when the command printed nothing.
func openSpecFailure(action, output string, err error) error {
	if output == "" {
		return fmt.Errorf("%s failed: %w", action, err)
	}
	return fmt.Errorf("%s failed: %s", action, output)
}

// mergeOpenSpecGuidance patches a freshly forked schema and merges the Stele
// guidance into openspec/config.yaml without removing user configuration. The
// patch starts the schema's specification template with the Stele annotation,
// and the archive guidance names the annotation repair step.
//
// @implements req.workflowschema.db5e762d55ea
// @implements req.specannotation.0784be141698
// @implements req.specannotation.1707277552af
func mergeOpenSpecGuidance(root string, patchSchema bool) ([]string, error) {
	cli, err := openSpecCLI(root)
	if err != nil {
		return nil, err
	}
	input, _ := json.Marshal(map[string]any{
		"root":            root,
		"openspecPackage": filepath.Dir(filepath.Dir(cli)),
		"openspecVersion": OpenSpecVersion,
		"steleVersion":    Version,
		"patchSchema":     patchSchema,
	})
	output, err := runNodeScript(root, openSpecExtendScript, "STELE_OPENSPEC_EXTEND="+string(input))
	if err != nil {
		return nil, fmt.Errorf("updating the OpenSpec setup failed: %w", err)
	}
	var result openSpecExtendResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("updating the OpenSpec setup returned invalid output: %w", err)
	}
	notes := result.Notes
	if result.ConfigChanged {
		notes = append(notes, "Added Stele guidance to openspec/config.yaml.")
	}
	return notes, nil
}

// runNode runs an ES module from standard input and returns its standard output.
func runNode(root, script, environment string) ([]byte, error) {
	command := exec.Command("node", "--input-type=module", "-")
	command.Dir = root
	command.Env = append(os.Environ(), environment)
	command.Stdin = strings.NewReader(script)
	output, err := command.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return output, err
}

// linkClaudeSkills makes Claude Code read the tool-neutral .agents/skills
// directory, unless the project already has its own .claude/skills.
func linkClaudeSkills(root string) (string, error) {
	link := filepath.Join(root, ".claude", "skills")
	if _, err := os.Lstat(link); err == nil {
		return "Warning: .claude/skills already exists; left it unchanged. " +
			"Link it to ../.agents/skills to share the skills.", nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	if err := os.Symlink(filepath.Join("..", ".agents", "skills"), link); err != nil {
		return "", err
	}
	return "Linked .claude/skills to .agents/skills.", nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
