package stele

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var executablePath = os.Executable

func runOpenSpec(root, changeID string) (bool, error) {
	executable, err := executablePath()
	if err != nil {
		return false, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}

	packageRoot := filepath.Dir(filepath.Dir(executable))
	candidates := []string{
		filepath.Join(packageRoot, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js"),
		filepath.Join(root, "node_modules", "@fission-ai", "openspec", "bin", "openspec.js"),
	}
	cli := firstExistingPath(candidates)
	if cli == "" {
		return false, errors.New("OpenSpec CLI was not found in the Stele package or consumer project")
	}

	command := exec.Command("node", cli, "validate", changeID, "--strict", "--no-interactive")
	command.Dir = root
	command.Env = append(os.Environ(), "OPENSPEC_TELEMETRY=0")
	output, runErr := command.CombinedOutput()
	if runErr != nil {
		return false, fmt.Errorf("OpenSpec validation failed: %s", strings.TrimSpace(string(output)))
	}
	return true, nil
}

func firstExistingPath(candidates []string) string {
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}
