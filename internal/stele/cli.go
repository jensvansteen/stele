package stele

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	Version         = "0.1.0"
	OpenSpecVersion = "1.13.0"
)

const helpBody = ` — deterministic OpenSpec implementation verification

Usage:
  stele init [--change ID] [--root PATH]
  stele verify [--stage proposal|implementation] [--change ID | --specs] [--root PATH] [--report PATH] [--json]
  stele test [--change ID | --specs] [--root PATH] [--evidence PATH] [--json]
  stele validate [--change ID | --specs] [--root PATH] [--report PATH] [--evidence PATH] [--json]

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
	specs        bool
}

type validationResult struct {
	SchemaVersion int    `json:"schemaVersion"`
	Verdict       string `json:"verdict"`
	OpenSpec      string `json:"openspec"`
	Execution     string `json:"execution"`
	Verification  string `json:"verification"`
}

var (
	currentWorkingDirectory = os.Getwd
	absolutePath            = filepath.Abs
	verifyProject           = verifyScope
	runProjectScenarios     = runScopeTests
	validateProjectOpenSpec = runOpenSpec
)

// @implements req.verify.999a5d082295
func Run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		writeHelp(stdout)
		return 0
	}

	command := arguments[0]
	switch command {
	case "help", "--help", "-h":
		writeHelp(stdout)
		return 0
	case "version", "--version", "-v":
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	case "init", "verify", "test", "validate":
		break
	default:
		_, _ = fmt.Fprintf(stderr, "stele: unknown command: %s\n", command)
		return 2
	}

	parsed, err := parseOptions(command, arguments[1:])
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if command == "init" {
		return initCommand(parsed, stdout, stderr)
	}

	parsed, err = withConfig(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	return routeCommand(command, parsed, stdout, stderr)
}

func writeHelp(stdout io.Writer) {
	_, _ = fmt.Fprintf(stdout, "stele %s%s", Version, helpBody)
}

func writeCommandError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
	return 2
}

func routeCommand(command string, parsed options, stdout, stderr io.Writer) int {
	switch command {
	case "verify":
		return verifyCommand(parsed, stdout, stderr)
	case "test":
		return testCommand(parsed, stdout, stderr)
	default:
		return validateCommand(parsed, stdout, stderr)
	}
}

func initCommand(parsed options, stdout, stderr io.Writer) int {
	if parsed.changeID == "" {
		return writeCommandError(stderr, errors.New("--change is required for init"))
	}

	created, err := Initialize(parsed.root, parsed.changeID)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if len(created) == 0 {
		_, _ = fmt.Fprintln(stdout, "Stele is already initialized.")
		return 0
	}

	_, _ = fmt.Fprintf(stdout, "Initialized Stele: %s\n", strings.Join(created, ", "))
	return 0
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
	if command != "init" {
		flags.BoolVar(&parsed.specs, "specs", false, "verify the current specifications")
	}
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
	if parsed.specs && parsed.changeID != "" {
		return options{}, errConflictingScope
	}
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
	if parsed.specs {
		return parsed, nil
	}
	if parsed.changeID == "" {
		return options{}, errors.New("no OpenSpec change selected; pass --change or run stele init")
	}
	return parsed, nil
}

func verifyCommand(parsed options, stdout, stderr io.Writer) int {
	report, err := verifyProject(parsed.root, resolveScope(parsed), parsed.stage, parsed.reportPath)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if parsed.json {
		writeMachineJSON(stdout, report)
	} else {
		renderVerification(stdout, report)
	}
	return resultExitCode(report.Verdict == "pass")
}

func renderVerification(stdout io.Writer, report Report) {
	mark := choose(report.Verdict == "pass", "✓", "✗")
	_, _ = fmt.Fprintf(
		stdout,
		"%s %s verification %s: %d requirements, %d scenarios, %d errors\n",
		mark,
		report.Mode,
		report.Verdict,
		report.Summary.Requirements,
		report.Summary.Scenarios,
		report.Summary.Errors,
	)
	for _, diagnostic := range report.Diagnostics {
		_, _ = fmt.Fprintf(
			stdout,
			"  %s %s: %s\n",
			strings.ToUpper(diagnostic.Severity),
			diagnostic.Code,
			diagnostic.Message,
		)
	}
}

func testCommand(parsed options, stdout, stderr io.Writer) int {
	evidence, err := runProjectScenarios(parsed.root, resolveScope(parsed), parsed.evidencePath)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if parsed.json {
		writeMachineJSON(stdout, evidence)
	} else {
		renderScenarioExecution(stdout, evidence)
	}
	return resultExitCode(evidence.Outcome == "passed")
}

func renderScenarioExecution(stdout io.Writer, evidence Evidence) {
	passed := countPassed(evidence)
	mark := choose(evidence.Outcome == "passed", "✓", "✗")
	_, _ = fmt.Fprintf(
		stdout,
		"%s scenario execution %s: %d/%d passed\n",
		mark,
		evidence.Outcome,
		passed,
		len(evidence.Scenarios),
	)
}

// @implements req.validate.56cc774dc871
func validateCommand(parsed options, stdout, stderr io.Writer) int {
	setDefaultOutputPaths(&parsed)
	evidence, err := runProjectScenarios(parsed.root, resolveScope(parsed), parsed.evidencePath)
	if err != nil {
		return writeCommandError(stderr, err)
	}

	openSpecPassed, openSpecErr := validateProjectOpenSpec(parsed.root, resolveScope(parsed))
	if openSpecErr != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", openSpecErr)
	}

	report, err := verifyProject(parsed.root, resolveScope(parsed), "implementation", parsed.reportPath)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	passed := evidence.Outcome == "passed" && openSpecPassed && report.Verdict == "pass"
	if parsed.json {
		writeMachineJSON(stdout, newValidationResult(evidence, report, openSpecPassed, passed))
	} else {
		renderValidation(stdout, evidence, report, openSpecPassed, passed)
	}
	return resultExitCode(passed)
}

func setDefaultOutputPaths(parsed *options) {
	if parsed.evidencePath == "" {
		parsed.evidencePath = "artifacts/test-results.json"
	}
	if parsed.reportPath == "" {
		parsed.reportPath = "artifacts/verification-report.json"
	}
}

func newValidationResult(
	evidence Evidence,
	report Report,
	openSpecPassed bool,
	passed bool,
) validationResult {
	return validationResult{
		SchemaVersion: 1,
		Verdict:       choose(passed, "pass", "fail"),
		OpenSpec:      choose(openSpecPassed, "pass", "fail"),
		Execution:     evidence.Outcome,
		Verification:  report.Verdict,
	}
}

func renderValidation(
	stdout io.Writer,
	evidence Evidence,
	report Report,
	openSpecPassed bool,
	passed bool,
) {
	_, _ = fmt.Fprintf(
		stdout,
		"%s OpenSpec strict validation %s\n",
		choose(openSpecPassed, "✓", "✗"),
		choose(openSpecPassed, "passed", "failed"),
	)
	_, _ = fmt.Fprintf(
		stdout,
		"%s scenario execution %s: %d/%d passed\n",
		choose(evidence.Outcome == "passed", "✓", "✗"),
		evidence.Outcome,
		countPassed(evidence),
		len(evidence.Scenarios),
	)
	_, _ = fmt.Fprintf(
		stdout,
		"%s implementation verification %s: %d requirements, %d scenarios, %d errors\n",
		choose(report.Verdict == "pass", "✓", "✗"),
		report.Verdict,
		report.Summary.Requirements,
		report.Summary.Scenarios,
		report.Summary.Errors,
	)
	_, _ = fmt.Fprintf(
		stdout,
		"%s deterministic validation %s\n",
		choose(passed, "✓", "✗"),
		choose(passed, "passed", "failed"),
	)
}

func resultExitCode(passed bool) int {
	if passed {
		return 0
	}
	return 1
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
