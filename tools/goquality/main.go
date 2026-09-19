// Command goquality provides repository-local quality gates for Go source.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const requiredCoverage = 100

// corePackage is the Go verifier core, which must be fully covered. The
// coverage run tests every Go package once, with the race detector, and
// counts only the core's statements.
const corePackage = "github.com/jensvansteen/stele/internal/stele/"

// coveragePackages are the packages the coverage run tests.
var coveragePackages = []string{"./cmd/...", "./internal/...", "./tools/..."}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 1 {
		if !writef(stderr, "usage: goquality <format|coverage>\n") {
			return 2
		}
		return 2
	}

	switch arguments[0] {
	case "format":
		return checkFormat(stderr)
	case "coverage":
		return checkCoverage(stdout, stderr)
	default:
		if !writef(stderr, "unknown quality check %q\n", arguments[0]) {
			return 2
		}
		return 2
	}
}

func checkFormat(stderr io.Writer) int {
	var paths bytes.Buffer
	command := exec.Command("gofmt", "-l", "cmd", "internal", "tests", "tools")
	command.Stdout = &paths
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return commandExitCode(err)
	}

	unformatted := strings.TrimSpace(paths.String())
	if unformatted == "" {
		return 0
	}
	if !writef(stderr, "Go files need formatting:\n%s\n", unformatted) {
		return 2
	}
	return 1
}

func checkCoverage(stdout, stderr io.Writer) int {
	temporaryDirectory, err := os.MkdirTemp("", "stele-coverage-")
	if err != nil {
		writef(stderr, "%v\n", err)
		return 2
	}
	defer func() {
		_ = os.RemoveAll(temporaryDirectory) // Cleanup is best-effort after the verdict is known.
	}()

	profile := filepath.Join(temporaryDirectory, "core.coverage")
	arguments := append([]string{"test", "-race", "-coverprofile=" + profile}, coveragePackages...)
	command := exec.Command("go", arguments...)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return commandExitCode(err)
	}

	covered, total, err := readCoverage(profile)
	if err != nil {
		writef(stderr, "%v\n", err)
		return 2
	}
	if total == 0 || covered != total {
		percentage := 0.0
		if total > 0 {
			percentage = float64(covered) / float64(total) * requiredCoverage
		}
		message := "Go verifier core statement coverage is %.2f%% (%d/%d); required: 100%%.\n"
		if !writef(stderr, message, percentage, covered, total) {
			return 2
		}
		return 1
	}

	if !writef(stdout, "Go verifier core statement coverage: 100.0%% (%d/%d).\n", covered, total) {
		return 2
	}
	return 0
}

func readCoverage(path string) (int, int, error) {
	profile, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open coverage profile: %w", err)
	}

	covered, total, parseErr := parseCoverage(profile, corePackage)
	closeErr := profile.Close()
	if parseErr != nil {
		return 0, 0, parseErr
	}
	if closeErr != nil {
		return 0, 0, fmt.Errorf("close coverage profile: %w", closeErr)
	}
	return covered, total, nil
}

// parseCoverage counts the covered and total statements of the files whose
// import path starts with prefix.
func parseCoverage(profile io.Reader, prefix string) (int, int, error) {
	scanner := bufio.NewScanner(profile)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode:") {
		return 0, 0, fmt.Errorf("coverage profile has no mode header")
	}

	covered := 0
	total := 0
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			return 0, 0, fmt.Errorf("invalid coverage record %q", scanner.Text())
		}
		if !strings.HasPrefix(fields[0], prefix) {
			continue
		}
		statements, err := strconv.Atoi(fields[len(fields)-2])
		if err != nil {
			return 0, 0, fmt.Errorf("parse statement count: %w", err)
		}
		executions, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			return 0, 0, fmt.Errorf("parse execution count: %w", err)
		}
		total += statements
		if executions > 0 {
			covered += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("read coverage profile: %w", err)
	}
	return covered, total, nil
}

func commandExitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 2
}

func writef(writer io.Writer, format string, arguments ...any) bool {
	_, err := fmt.Fprintf(writer, format, arguments...)
	return err == nil
}
