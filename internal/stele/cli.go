package stele

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	Version         = "0.3.0"
	OpenSpecVersion = "1.13.0"
)

const help = `stele 0.3.0 — deterministic OpenSpec implementation verification

Usage:
  stele init [--change ID] [--root PATH]
  stele verify [--stage proposal|implementation] [--change ID] [--root PATH] [--report PATH] [--json]
  stele test [--change ID] [--root PATH] [--evidence PATH] [--json]
  stele validate [--change ID] [--root PATH] [--report PATH] [--evidence PATH] [--json]

Exit codes:
  0  selected checks passed
  1  deterministic policy or test failure
  2  invalid invocation or tool failure
`

type options struct {
	root         string
	changeID     string
	stage        string
	reportPath   string
	evidencePath string
	json         bool
}

var (
	currentWorkingDirectory = os.Getwd
	absolutePath            = filepath.Abs
	executablePath          = os.Executable
	verifyProject           = RunVerification
	runProjectScenarios     = RunScenarioTests
	validateProjectOpenSpec = runOpenSpec
)

func Run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 || arguments[0] == "help" || arguments[0] == "--help" || arguments[0] == "-h" {
		_, _ = io.WriteString(stdout, help)
		return 0
	}
	if arguments[0] == "version" || arguments[0] == "--version" || arguments[0] == "-v" {
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	}
	command := arguments[0]
	if !contains([]string{"init", "verify", "test", "validate"}, command) {
		_, _ = fmt.Fprintf(stderr, "stele: unknown command: %s\n", command)
		return 2
	}
	parsed, err := parseOptions(command, arguments[1:])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	if command == "init" {
		if parsed.changeID == "" {
			_, _ = fmt.Fprintln(stderr, "stele: --change is required for init")
			return 2
		}
		created, initErr := Initialize(parsed.root, parsed.changeID)
		if initErr != nil {
			_, _ = fmt.Fprintf(stderr, "stele: %s\n", initErr)
			return 2
		}
		if len(created) == 0 {
			_, _ = fmt.Fprintln(stdout, "Stele is already initialized.")
		} else {
			_, _ = fmt.Fprintf(stdout, "Initialized Stele: %s\n", strings.Join(created, ", "))
		}
		return 0
	}
	parsed, err = withConfig(parsed)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	switch command {
	case "verify":
		return verifyCommand(parsed, stdout, stderr)
	case "test":
		return testCommand(parsed, stdout, stderr)
	}
	return validateCommand(parsed, stdout, stderr)
}

func parseOptions(command string, arguments []string) (options, error) {
	root, err := currentWorkingDirectory()
	if err != nil {
		return options{}, err
	}
	parsed := options{root: root, stage: "implementation"}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&parsed.root, "root", parsed.root, "project root")
	flags.StringVar(&parsed.changeID, "change", "", "OpenSpec change")
	flags.StringVar(&parsed.stage, "stage", parsed.stage, "verification stage")
	flags.StringVar(&parsed.reportPath, "report", "", "report path")
	flags.StringVar(&parsed.evidencePath, "evidence", "", "evidence path")
	flags.BoolVar(&parsed.json, "json", false, "JSON output")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() > 0 {
		return options{}, fmt.Errorf("unknown option: %s", flags.Arg(0))
	}
	absolute, err := absolutePath(parsed.root)
	if err != nil {
		return options{}, err
	}
	parsed.root = absolute
	if parsed.stage != "proposal" && parsed.stage != "implementation" {
		return options{}, fmt.Errorf("unknown stage: %s", parsed.stage)
	}
	return parsed, nil
}

func withConfig(parsed options) (options, error) {
	configPath := filepath.Join(parsed.root, "stele.config.json")
	content, err := os.ReadFile(configPath)
	if err == nil {
		var config Config
		if err := json.Unmarshal(content, &config); err != nil {
			return options{}, fmt.Errorf("invalid stele.config.json: %w", err)
		}
		if parsed.changeID == "" {
			parsed.changeID = config.Change
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return options{}, err
	}
	if parsed.changeID == "" {
		return options{}, errors.New("no OpenSpec change selected; pass --change or run stele init")
	}
	return parsed, nil
}

func verifyCommand(parsed options, stdout, stderr io.Writer) int {
	report, err := verifyProject(parsed.root, parsed.changeID, parsed.stage, parsed.reportPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	if parsed.json {
		writeMachineJSON(stdout, report)
	} else {
		mark := "✓"
		if report.Verdict != "pass" {
			mark = "✗"
		}
		_, _ = fmt.Fprintf(stdout, "%s %s verification %s: %d requirements, %d scenarios, %d errors\n", mark, report.Mode, report.Verdict, report.Summary.Requirements, report.Summary.Scenarios, report.Summary.Errors)
		for _, diagnostic := range report.Diagnostics {
			_, _ = fmt.Fprintf(stdout, "  %s %s: %s\n", strings.ToUpper(diagnostic.Severity), diagnostic.Code, diagnostic.Message)
		}
	}
	if report.Verdict == "pass" {
		return 0
	}
	return 1
}

func testCommand(parsed options, stdout, stderr io.Writer) int {
	evidence, err := runProjectScenarios(parsed.root, parsed.changeID, parsed.evidencePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	if parsed.json {
		writeMachineJSON(stdout, evidence)
	} else {
		passed := 0
		for _, scenario := range evidence.Scenarios {
			if scenario.Outcome == "passed" {
				passed++
			}
		}
		mark := "✓"
		if evidence.Outcome != "passed" {
			mark = "✗"
		}
		_, _ = fmt.Fprintf(stdout, "%s scenario execution %s: %d/%d passed\n", mark, evidence.Outcome, passed, len(evidence.Scenarios))
	}
	if evidence.Outcome == "passed" {
		return 0
	}
	return 1
}

func validateCommand(parsed options, stdout, stderr io.Writer) int {
	evidencePath := parsed.evidencePath
	if evidencePath == "" {
		evidencePath = "artifacts/test-results.json"
	}
	reportPath := parsed.reportPath
	if reportPath == "" {
		reportPath = "artifacts/verification-report.json"
	}
	evidence, err := runProjectScenarios(parsed.root, parsed.changeID, evidencePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	openSpecPassed, openSpecErr := validateProjectOpenSpec(parsed.root, parsed.changeID)
	if openSpecErr != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", openSpecErr)
	}
	report, err := verifyProject(parsed.root, parsed.changeID, "implementation", reportPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
		return 2
	}
	passed := evidence.Outcome == "passed" && openSpecPassed && report.Verdict == "pass"
	if parsed.json {
		payload := struct {
			SchemaVersion int    `json:"schemaVersion"`
			Verdict       string `json:"verdict"`
			OpenSpec      string `json:"openspec"`
			Execution     string `json:"execution"`
			Verification  string `json:"verification"`
		}{1, choose(passed, "pass", "fail"), choose(openSpecPassed, "pass", "fail"), evidence.Outcome, report.Verdict}
		writeMachineJSON(stdout, payload)
	} else {
		_, _ = fmt.Fprintf(stdout, "%s OpenSpec strict validation %s\n", choose(openSpecPassed, "✓", "✗"), choose(openSpecPassed, "passed", "failed"))
		_, _ = fmt.Fprintf(stdout, "%s scenario execution %s: %d/%d passed\n", choose(evidence.Outcome == "passed", "✓", "✗"), evidence.Outcome, countPassed(evidence), len(evidence.Scenarios))
		_, _ = fmt.Fprintf(stdout, "%s implementation verification %s: %d requirements, %d scenarios, %d errors\n", choose(report.Verdict == "pass", "✓", "✗"), report.Verdict, report.Summary.Requirements, report.Summary.Scenarios, report.Summary.Errors)
		_, _ = fmt.Fprintf(stdout, "%s deterministic validation %s\n", choose(passed, "✓", "✗"), choose(passed, "passed", "failed"))
	}
	if passed {
		return 0
	}
	return 1
}

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
	var cli string
	for _, candidate := range candidates {
		if fileExists(candidate) {
			cli = candidate
			break
		}
	}
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

func writeMachineJSON(writer io.Writer, value any) {
	content, err := MarshalDeterministic(value)
	if err == nil {
		_, _ = writer.Write(content)
	}
}

func choose(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func countPassed(evidence Evidence) int {
	count := 0
	for _, scenario := range evidence.Scenarios {
		if scenario.Outcome == "passed" {
			count++
		}
	}
	return count
}
