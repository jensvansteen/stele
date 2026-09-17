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

// openSpecCLI locates the OpenSpec CLI bundled with the Stele package, falling
// back to the consumer project's own pinned installation.
func openSpecCLI(root string) (string, error) {
	executable, err := executablePath()
	if err != nil {
		return "", err
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
		return "", errors.New("OpenSpec CLI was not found in the Stele package or consumer project")
	}
	return cli, nil
}

// runBundledOpenSpec runs the pinned OpenSpec CLI in root and returns its
// combined output.
func runBundledOpenSpec(root string, arguments ...string) (string, error) {
	cli, err := openSpecCLI(root)
	if err != nil {
		return "", err
	}
	command := exec.Command("node", append([]string{cli}, arguments...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "OPENSPEC_TELEMETRY=0")
	output, runErr := command.CombinedOutput()
	return strings.TrimSpace(string(output)), runErr
}

func runOpenSpec(root string, scope verificationScope) (bool, error) {
	arguments := []string{"validate", scope.changeID, "--strict", "--no-interactive"}
	if scope.currentSpecs {
		arguments = []string{"validate", "--specs", "--strict", "--no-interactive"}
	}
	output, err := runBundledOpenSpec(root, arguments...)
	if err != nil {
		if output == "" {
			return false, err
		}
		return false, fmt.Errorf("OpenSpec validation failed: %s", output)
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
