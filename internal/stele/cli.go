package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// OpenSpecVersion is the pinned OpenSpec release.
const OpenSpecVersion = "1.13.0"

const helpBody = ` — deterministic OpenSpec implementation verification

Usage:
  stele init [--change ID] [--tools TOOLS] [--refresh-schema] [--strict-versions] [--root PATH]
  stele ids [--change ID] [--root PATH] [--check] [--json]
  stele annotate [--change ID | --specs | --all] [--root PATH] [--check] [--json]
  stele check [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--evidence-file PATH]
              [--json] [--strict-versions] [OUTPUT]
  stele verify [--stage proposal|implementation] [--change ID | --specs | --all] [--root PATH]
               [--report-file PATH] [--json] [--strict-versions] [OUTPUT]
  stele test [targets...] [--change ID | --specs | --all] [--root PATH] [--evidence-file PATH] [--json]
             [OUTPUT]
  stele validate [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--evidence-file PATH]
                 [--json] [--strict-versions] [OUTPUT]
  stele index [--change ID | --specs | --all] [--root PATH] [--output-file PATH] [--json]
  stele approve [--change ID | --specs] [--evidence ID]... [--scenario ID]... [--all --yes | --confirmed-in-chat]
                [--by NAME] [--root PATH]
  stele plan migrate [--change ID | --specs] [--root PATH]

OUTPUT is [--details] [--quiet] [--color auto|always|never] [--annotations auto|github|never]: --details
lists every finding, --quiet prints only the verdict line, --color auto colors terminals unless NO_COLOR
is set, and --annotations auto writes GitHub Actions annotations when GITHUB_ACTIONS is true. Progress and
annotations go to standard error; --json output on standard output is unchanged.
stele check runs stele ids --check, stele annotate --check, and stele validate, and exits with the worst code.
Test targets are requirement, scenario, or evidence IDs, or spec.md files under openspec/.
--report and --evidence are deprecated aliases of --report-file and --evidence-file until 0.2.0.

Exit codes:
  0  selected checks passed
  1  deterministic policy or test failure
  2  invalid invocation or tool failure
`

type options struct {
	root          string
	changeID      string
	stage         string
	reportPath    string
	evidencePath  string
	json          bool
	specs         bool
	check         bool
	tools         string
	refreshSchema bool
	// approve options
	evidenceIDs     []string
	scenarioIDs     []string
	all             bool
	yes             bool
	confirmedInChat bool
	approver        string
	// backend is the specification backend selected by the adapter setting.
	backend        specificationBackend
	strictVersions bool
	// targets are the positional test targets; allScopes selects every scope.
	targets    []string
	allScopes  bool
	outputPath string
	// deprecated lists the deprecated flags that were used.
	deprecated []deprecatedFlag
	// every is set while one scope of an --all run executes.
	every *everyScopeRun
	// unannotated is the unannotatedSpecs policy from stele.config.json.
	unannotated string
	// details, quiet, and color control the human report and progress.
	details bool
	quiet   bool
	color   string
	// annotationMode is --annotations, and annotations collects the GitHub
	// Actions annotations of the run when they are on.
	annotationMode string
	annotations    *annotationSink
}

type validationResult struct {
	SchemaVersion int            `json:"schemaVersion"`
	Verdict       string         `json:"verdict"`
	OpenSpec      string         `json:"openspec"`
	Execution     string         `json:"execution"`
	Verification  string         `json:"verification"`
	Verdicts      ReportVerdicts `json:"verdicts"`
}

var (
	currentWorkingDirectory = os.Getwd
	absolutePath            = filepath.Abs
	verifyProject           = verifyScope
	runProjectScenarios     = runScopeTests
	validateProjectOpenSpec = validateScopeSpecs
	checkScopeSpecs         = requireScopeSpecs
)

// validateScopeSpecs runs the backend's strict validation for a scope.
func validateScopeSpecs(root string, scope verificationScope) (bool, error) {
	return scope.spec().Validate(root, scope)
}

// reportVersionDrift prints backend version drift. Without strict versions the
// findings are warnings; with them they are errors and the result is false.
func reportVersionDrift(parsed options, lookPath bool, stderr io.Writer) bool {
	findings := resolveScope(parsed).spec().VersionDrift(parsed.root, lookPath)
	for _, finding := range findings {
		_, _ = fmt.Fprintf(stderr, "stele: %s: %s\n", choose(parsed.strictVersions, "error", "warning"), finding)
	}
	return !parsed.strictVersions || len(findings) == 0
}

// withVersionCheck turns a passing exit code into a policy failure when strict
// versions are requested and the backend drifted.
func withVersionCheck(code int, parsed options, stderr io.Writer) int {
	if !reportVersionDrift(parsed, false, stderr) && code == 0 {
		return 1
	}
	return code
}

// @implements req.verify.999a5d082295
func Run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		writeHelp(stdout)
		return 0
	}

	command, rest := arguments[0], arguments[1:]
	switch command {
	case "help", "--help", "-h":
		writeHelp(stdout)
		return 0
	case "version", "--version", "-v":
		_, _ = fmt.Fprintln(stdout, Version)
		return 0
	case "plan":
		if len(rest) == 0 || rest[0] != "migrate" {
			return writeCommandError(stderr, errUnknownPlanCommand)
		}
		command, rest = "plan migrate", rest[1:]
	case "init", "ids", "annotate", "verify", "test", "validate", "check", "approve", "index":
	default:
		_, _ = fmt.Fprintf(stderr, "stele: unknown command: %s\n", command)
		return 2
	}

	parsed, err := parseOptions(command, rest)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	for _, used := range parsed.deprecated {
		_, _ = fmt.Fprintf(stderr, "stele: warning: --%s is deprecated and will be removed in %s; use --%s\n",
			used.name, v1RemovalRelease, used.replacement)
	}
	if command == "init" {
		return initCommand(parsed, stdout, stderr)
	}

	parsed, err = withConfig(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	parsed.annotations = newAnnotationSink(parsed.annotationMode, parsed.root)
	code := routeCommand(command, parsed, stdout, stderr)
	parsed.annotations.flush(stderr)
	return code
}

// writeHelp prints the help text, headed by the package version that the
// build sets in Version; --version, reports, and the human report use it too.
//
// @implements req.terminalreport.e78c864148cd
func writeHelp(stdout io.Writer) {
	_, _ = fmt.Fprintf(stdout, "stele %s%s", Version, helpBody)
}

func writeCommandError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "stele: %s\n", err)
	return 2
}

func routeCommand(command string, parsed options, stdout, stderr io.Writer) int {
	if parsed.allScopes && command != "index" && command != "annotate" && command != "check" {
		return runEveryScope(command, parsed, stdout, stderr)
	}
	switch command {
	case "index":
		return indexCommand(parsed, stdout, stderr)
	case "annotate":
		return annotateCommand(parsed, stdout, stderr)
	case "check":
		return checkCommand(parsed, stdout, stderr)
	case "verify":
		return verifyCommand(parsed, stdout, stderr)
	case "ids":
		return identitiesCommand(parsed, stdout, stderr)
	case "approve":
		return approveCommand(parsed, stdout, stderr)
	case "plan migrate":
		return migrateCommand(parsed, stdout, stderr)
	case "test":
		return testCommand(parsed, stdout, stderr)
	default:
		return validateCommand(parsed, stdout, stderr)
	}
}

const initWorkflow = `
Next steps:
  Use the stele-propose, stele-apply, and stele-archive skills to plan,
  implement, and archive changes.

Using OpenSpec skills directly? The stele schema adds the planning step to
new changes. Run these Stele commands around the OpenSpec steps:
  after writing specs: stele ids, then stele verify --stage proposal
  after applying:      stele validate --change <change>
  after archiving:     stele annotate --specs, then stele validate --specs
Enable OpenSpec's verify-change skill with: npx openspec config profile
`

func initCommand(parsed options, stdout, stderr io.Writer) int {
	backend, err := configuredBackend(parsed.root)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	parsed.backend = backend
	notes, err := backend.Install(parsed.root, openSpecSetup{tools: parsed.tools, refreshSchema: parsed.refreshSchema})
	if err != nil {
		return writeCommandError(stderr, err)
	}
	created, warnings, err := initializeBackend(parsed.root, parsed.changeID, backend)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(stderr, "stele: warning: %s\n", warning)
	}
	if len(created) == 0 {
		_, _ = fmt.Fprintln(stdout, "Stele is already initialized.")
	} else {
		_, _ = fmt.Fprintf(stdout, "Initialized Stele: %s\n", strings.Join(created, ", "))
	}
	for _, note := range notes {
		_, _ = fmt.Fprintln(stdout, note)
	}
	_, _ = fmt.Fprint(stdout, initWorkflow)
	return resultExitCode(reportVersionDrift(parsed, true, stderr))
}

func identitiesCommand(parsed options, stdout, stderr io.Writer) int {
	result, err := assignScopeIdentities(parsed.root, resolveScope(parsed), parsed.check)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if parsed.json {
		writeMachineJSON(stdout, result)
	} else {
		renderIdentities(stdout, result)
	}
	return resultExitCode(result.Verdict == "pass")
}

func renderIdentities(stdout io.Writer, result IdentityResult) {
	renderIdentityInsertions(stdout, result)
	renderAnnotationFiles(stdout, result.Annotations)
}

func renderIdentityInsertions(stdout io.Writer, result IdentityResult) {
	if len(result.Insertions) == 0 {
		_, _ = fmt.Fprintln(stdout, "✓ every requirement and scenario has a Verification-ID")
		return
	}
	if result.Mode == "check" {
		_, _ = fmt.Fprintf(stdout, "✗ %d headings lack a Verification-ID\n", len(result.Insertions))
	} else {
		_, _ = fmt.Fprintf(stdout, "✓ inserted %d Verification-IDs\n", len(result.Insertions))
	}
	for _, insertion := range result.Insertions {
		_, _ = fmt.Fprintf(
			stdout,
			"  %s:%d %s %s %q\n",
			insertion.Path,
			insertion.Line,
			insertion.ID,
			insertion.Kind,
			insertion.Title,
		)
	}
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
	flags.BoolVar(&parsed.json, "json", false, "JSON output")
	registerCommandFlags(flags, command, &parsed)
	registerOutputFlags(flags, command, &parsed)
	if err := parseArguments(flags, command, arguments, &parsed); err != nil {
		return options{}, err
	}

	absolute, err := absolutePath(parsed.root)
	if err != nil {
		return options{}, err
	}
	parsed.root = absolute
	if parsed.specs && parsed.changeID != "" {
		return options{}, errConflictingScope
	}
	if parsed.allScopes && (parsed.specs || parsed.changeID != "" || len(parsed.targets) > 0) {
		return options{}, errConflictingAll
	}
	if parsed.stage != "proposal" && parsed.stage != "implementation" {
		return options{}, fmt.Errorf("unknown stage: %s", parsed.stage)
	}
	if parsed.color != "" && !colorModes[parsed.color] {
		return options{}, fmt.Errorf("unknown --color value %q; accepted values: auto, always, never", parsed.color)
	}
	if parsed.annotationMode != "" && !annotationModes[parsed.annotationMode] {
		return options{}, fmt.Errorf("unknown --annotations value %q; accepted values: auto, github, never",
			parsed.annotationMode)
	}
	return parsed, nil
}

var errConflictingAll = errors.New("--all cannot be combined with --change, --specs, or test targets")

// parseArguments parses flags anywhere on the command line. Only `stele test`
// accepts positional arguments, its targets.
func parseArguments(flags *flag.FlagSet, command string, arguments []string, parsed *options) error {
	for {
		if err := flags.Parse(arguments); err != nil {
			return err
		}
		if flags.NArg() == 0 {
			return nil
		}
		if command != "test" {
			return fmt.Errorf("unknown option: %s", flags.Arg(0))
		}
		parsed.targets = append(parsed.targets, flags.Arg(0))
		arguments = flags.Args()[1:]
	}
}

// deprecatedFlag names a deprecated flag and the flag that replaces it.
type deprecatedFlag struct {
	name        string
	replacement string
}

// deprecatedPathFlag sets the path of its replacement flag and records its use.
type deprecatedPathFlag struct {
	target *string
	flag   deprecatedFlag
	used   *[]deprecatedFlag
}

func (value deprecatedPathFlag) String() string {
	if value.target == nil {
		return ""
	}
	return *value.target
}

func (value deprecatedPathFlag) Set(path string) error {
	*value.target = path
	*value.used = append(*value.used, value.flag)
	return nil
}

// registerOutputFlags adds the output file flags: --evidence-file for test and
// validate, --report-file for verify and validate, and --output-file for
// index. --evidence and --report remain deprecated aliases until 0.2.0.
//
// @implements req.verify.3624e3449f6a
func registerOutputFlags(flags *flag.FlagSet, command string, parsed *options) {
	register := func(name, deprecated string, target *string) {
		flags.StringVar(target, name, "", name+" path")
		flags.Var(deprecatedPathFlag{
			target: target,
			flag:   deprecatedFlag{name: deprecated, replacement: name},
			used:   &parsed.deprecated,
		}, deprecated, "deprecated alias of --"+name)
	}
	switch command {
	case "verify":
		register("report-file", "report", &parsed.reportPath)
	case "test":
		register("evidence-file", "evidence", &parsed.evidencePath)
	case "validate", "check":
		register("report-file", "report", &parsed.reportPath)
		register("evidence-file", "evidence", &parsed.evidencePath)
	case "index":
		flags.StringVar(&parsed.outputPath, "output-file", "", "index path")
	}
}

// registerCommandFlags adds the options that only some commands accept.
func registerCommandFlags(flags *flag.FlagSet, command string, parsed *options) {
	switch command {
	case "init":
		flags.StringVar(&parsed.tools, "tools", defaultOpenSpecTools, "OpenSpec tools to initialize")
		flags.BoolVar(&parsed.refreshSchema, "refresh-schema", false, "re-fork the stele workflow schema")
		flags.BoolVar(&parsed.strictVersions, "strict-versions", false, "fail on backend version drift")
		return
	case "ids":
		flags.BoolVar(&parsed.check, "check", false, "report missing identities without writing")
		return
	case "annotate":
		flags.BoolVar(&parsed.check, "check", false, "report missing annotations without writing")
		flags.BoolVar(&parsed.allScopes, "all", false, "annotate the current specifications and every active change")
	case "approve":
		flags.Var(listFlag{&parsed.evidenceIDs}, "evidence", "evidence IDs to approve")
		flags.Var(listFlag{&parsed.scenarioIDs}, "scenario", "scenario IDs to approve")
		flags.BoolVar(&parsed.all, "all", false, "select every pending entry")
		flags.BoolVar(&parsed.yes, "yes", false, "approve without prompts")
		flags.BoolVar(&parsed.confirmedInChat, "confirmed-in-chat", false, "record a confirmation given in chat")
		flags.StringVar(&parsed.approver, "by", "", "approver name")
	case "plan migrate":
	case "verify", "validate", "check":
		flags.BoolVar(&parsed.strictVersions, "strict-versions", false, "fail on backend version drift")
		fallthrough
	case "test":
		flags.BoolVar(&parsed.details, "details", false, "list every finding without truncation")
		flags.BoolVar(&parsed.quiet, "quiet", false, "print only the verdict line and no progress")
		flags.StringVar(&parsed.color, "color", "auto", "color: auto, always, or never")
		flags.StringVar(&parsed.annotationMode, "annotations", "auto",
			"GitHub Actions annotations: auto, github, or never")
		fallthrough
	default:
		flags.BoolVar(&parsed.allScopes, "all", false, "check the current specifications and every active change")
	}
	flags.BoolVar(&parsed.specs, "specs", false, "use the current specifications")
}

// listFlag collects repeated or comma-separated values.
type listFlag struct {
	values *[]string
}

func (list listFlag) String() string {
	if list.values == nil {
		return ""
	}
	return strings.Join(*list.values, ",")
}

func (list listFlag) Set(value string) error {
	for part := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			*list.values = append(*list.values, trimmed)
		}
	}
	return nil
}

func withConfig(parsed options) (options, error) {
	config, err := readConfig(parsed.root)
	if err != nil {
		return options{}, err
	}
	if parsed.changeID == "" {
		parsed.changeID = config.Change
	}
	parsed.unannotated = config.UnannotatedSpecs
	parsed.backend, err = resolveBackend(config.Adapter)
	if err != nil {
		return options{}, err
	}
	if parsed.specs || parsed.allScopes {
		return parsed, nil
	}
	if parsed.changeID == "" {
		return options{}, errors.New("no OpenSpec change selected; pass --change or run stele init")
	}
	return parsed, nil
}

// verifyCommand exits according to the linkage verdict, so proposal gates and
// verify-only CI steps keep their meaning; the report also names execution
// and the overall verdict.
func verifyCommand(parsed options, stdout, stderr io.Writer) int {
	progress := parsed.progress(stderr)
	progress.stage("Verification")
	scope := resolveScope(parsed)
	report, err := verifyProject(verifyRequest{
		root:       parsed.root,
		scope:      scope,
		mode:       parsed.stage,
		reportPath: parsed.every.reportPath(parsed.reportPath),
	})
	progress.done()
	if err != nil {
		return writeCommandError(stderr, err)
	}
	parsed.every.collect(report)
	passed := report.Verdicts.Linkage == "pass"
	input := humanReportInput{command: "verify", scope: scope, report: &report, passed: passed}
	if report.Mode == "implementation" {
		input.testsRelevant = true
		input.executions = storedExecutions(parsed.root, report)
		input.currentDigest = report.Repository.InputDigest
		if report.Verdicts.Overall != "pass" {
			input.notes = append(input.notes, "overall "+report.Verdicts.Overall+
				" (test execution "+strings.ReplaceAll(report.Verdicts.Execution, "-", " ")+")")
		}
	}
	if parsed.json {
		writeMachineJSON(stdout, report)
	} else {
		parsed.writeReport(stdout, input)
	}
	parsed.annotate(input)
	return parsed.every.versionCheck(resultExitCode(passed), parsed, stderr)
}

// storedExecutions returns the stored test executions of a report's scenarios.
func storedExecutions(root string, report Report) []TestExecution {
	var evidence Evidence
	if !readJSON(filepath.Join(root, defaultEvidencePath), &evidence) {
		return nil
	}
	scenarios := make(map[string]bool)
	for _, requirement := range report.Requirements {
		for _, scenario := range requirement.Scenarios {
			scenarios[scenario.ID] = true
		}
	}
	executions := make([]TestExecution, 0)
	for _, execution := range evidence.Executions {
		if execution.InputDigest == "" {
			execution.InputDigest = evidence.InputDigest
		}
		for _, id := range execution.ScenarioIDs {
			if scenarios[id] {
				executions = append(executions, execution)
				break
			}
		}
	}
	return executions
}

// testCommand runs the scope's tests, or with targets only the selected ones,
// merging their outcomes into the stored evidence. A targeted run exits
// according to the tests it ran.
func testCommand(parsed options, stdout, stderr io.Writer) int {
	if len(parsed.targets) > 0 && parsed.evidencePath == "" {
		parsed.evidencePath = defaultEvidencePath
	}
	progress := parsed.progress(stderr)
	started := now()
	scope := resolveScope(parsed)
	run, err := runProjectScenarios(testRequest{
		root:         parsed.root,
		scope:        scope,
		evidencePath: parsed.evidencePath,
		targets:      parsed.targets,
		merge:        parsed.every.mergesEvidence(),
		observer:     progress,
	})
	duration := now().Sub(started)
	progress.done()
	if err != nil {
		return writeCommandError(stderr, err)
	}
	passed := run.evidence.Outcome == "passed"
	executions := run.evidence.Executions
	if run.targeted {
		passed = selectedPassed(run.selected)
		executions = append(append([]TestExecution{}, run.selected...), staleExecutions(run)...)
	}
	input := humanReportInput{
		command: "test", scope: scope, specs: run.parsed.Requirements,
		executions: executions, currentDigest: run.evidence.InputDigest, testsRelevant: true,
		duration: &duration, passed: passed, incomplete: run.incomplete,
	}
	if run.targeted {
		input.notes = []string{fmt.Sprintf("scope evidence: %d/%d scenarios passed",
			countPassed(run.evidence), len(run.evidence.Scenarios))}
	}
	if parsed.json {
		writeMachineJSON(stdout, run.evidence)
	} else {
		parsed.writeReport(stdout, input)
	}
	parsed.annotate(input)
	return resultExitCode(passed)
}

// staleExecutions returns the stored executions of a targeted run's scope that
// ran with other inputs.
func staleExecutions(run testRun) []TestExecution {
	stale := make([]TestExecution, 0)
	for _, execution := range run.evidence.Executions {
		if execution.InputDigest != run.evidence.InputDigest {
			stale = append(stale, execution)
		}
	}
	return stale
}

func selectedPassed(executions []TestExecution) bool {
	for _, execution := range executions {
		if execution.Outcome != "passed" {
			return false
		}
	}
	return len(executions) > 0
}

// validateCommand runs the fast stages first, OpenSpec strict validation and
// a check of the plan and linkage, then the tests, then the verification that
// uses their evidence.
//
// @implements req.validate.56cc774dc871
func validateCommand(parsed options, stdout, stderr io.Writer) int {
	setDefaultOutputPaths(&parsed)
	scope := resolveScope(parsed)
	if err := checkScopeSpecs(parsed.root, scope); err != nil {
		return writeCommandError(stderr, err)
	}
	progress := parsed.progress(stderr)
	progress.stage("OpenSpec strict validation")
	openSpecPassed, openSpecErr := validateProjectOpenSpec(parsed.root, scope)
	if openSpecErr != nil {
		_, _ = fmt.Fprintf(stderr, "stele: %s\n", openSpecErr)
	}
	progress.stageDone(choose(openSpecPassed, markPass, markFail),
		"OpenSpec strict validation: "+choose(openSpecPassed, "passed", "failed"))
	if _, silent := progress.(silentProgress); !silent {
		precheck(parsed, scope, progress)
	}
	started := now()
	run, err := runProjectScenarios(testRequest{
		root:         parsed.root,
		scope:        scope,
		evidencePath: parsed.evidencePath,
		merge:        parsed.every.mergesEvidence(),
		observer:     progress,
	})
	duration := now().Sub(started)
	if err != nil {
		progress.done()
		return writeCommandError(stderr, err)
	}
	evidence := run.evidence
	report, err := verifyProject(verifyRequest{
		root:       parsed.root,
		scope:      scope,
		mode:       "implementation",
		reportPath: parsed.every.reportPath(parsed.reportPath),
		evidence:   &evidence,
	})
	progress.done()
	if err != nil {
		return writeCommandError(stderr, err)
	}
	parsed.every.collect(report)
	passed := evidence.Outcome == "passed" && openSpecPassed && report.Verdicts.Linkage == "pass"
	input := humanReportInput{
		command: "validate", scope: scope, report: &report,
		executions: evidence.Executions, currentDigest: evidence.InputDigest, testsRelevant: true,
		openSpec: &openSpecPassed, duration: &duration, passed: passed, incomplete: run.incomplete,
	}
	if parsed.json {
		writeMachineJSON(stdout, newValidationResult(evidence, report, openSpecPassed, passed))
	} else {
		parsed.writeReport(stdout, input)
	}
	parsed.annotate(input)
	return parsed.every.versionCheck(resultExitCode(passed), parsed, stderr)
}

// precheck verifies the plan and linkage before the tests run, so their
// problems show in the progress output a minute earlier. Nothing is written.
func precheck(parsed options, scope verificationScope, progress progressReporter) {
	progress.stage("Plan and linkage")
	report, err := verifyProject(verifyRequest{
		root: parsed.root, scope: scope, mode: "implementation",
		evidence: &Evidence{},
	})
	if err != nil {
		progress.stageDone(markWarn, "plan and linkage: checked after the tests")
		return
	}
	parts := make([]string, 0, 3)
	failed := false
	for _, stage := range []string{stageSpecifications, stagePlan, stageLinkage} {
		errors, _ := stageCounts(stage, report.Diagnostics)
		failed = failed || len(errors) > 0
		parts = append(parts, strings.ToLower(stage)+": "+choose(len(errors) == 0, "passed",
			strings.Join(errors, ", ")))
	}
	progress.stageDone(choose(failed, markFail, markPass), strings.Join(parts, "; "))
}

func setDefaultOutputPaths(parsed *options) {
	if parsed.evidencePath == "" {
		parsed.evidencePath = defaultEvidencePath
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
		Verdicts:      report.Verdicts,
	}
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

// everyScopeRun carries the state of an --all run into each scope's command:
// reports are collected into one file, evidence after the first scope merges
// into the same file, and version drift is checked once.
type everyScopeRun struct {
	first    bool
	reports  *[]Report
	verdicts *[]humanReport
}

func (run *everyScopeRun) reportPath(path string) string {
	if run != nil {
		return ""
	}
	return path
}

func (run *everyScopeRun) collect(report Report) {
	if run != nil {
		*run.reports = append(*run.reports, report)
	}
}

func (run *everyScopeRun) mergesEvidence() bool {
	return run != nil && !run.first
}

func (run *everyScopeRun) versionCheck(code int, parsed options, stderr io.Writer) int {
	if run != nil {
		return code
	}
	return withVersionCheck(code, parsed, stderr)
}

// scopeResult is one scope's entry in the JSON output of an --all run.
type scopeResult struct {
	Scope    string          `json:"scope"`
	Kind     string          `json:"kind"`
	ExitCode int             `json:"exitCode"`
	Result   json.RawMessage `json:"result"`
}

// everyScope lists the current specifications, when there are any, and then
// every active change in directory order.
func everyScope(root string, backend specificationBackend) []verificationScope {
	scopes := make([]verificationScope, 0)
	specs := verificationScope{currentSpecs: true, backend: backend}
	if len(specs.spec().SpecFiles(root, specs)) > 0 {
		scopes = append(scopes, specs)
	}
	for _, change := range specs.spec().Changes(root) {
		scopes = append(scopes, verificationScope{changeID: change, backend: backend})
	}
	return scopes
}

// scopeName returns a scope's name, kind, and human label.
func scopeName(scope verificationScope) (string, string, string) {
	if scope.currentSpecs {
		return "specs", "specs", "current specifications"
	}
	return scope.changeID, "change", "change " + scope.changeID
}

// runOneScope runs a command for one scope of an --all run. It returns the
// scope's JSON entry and its human output under a heading, and records a
// verdict for a scope that could not be checked.
func runOneScope(
	command string,
	parsed options,
	scope verificationScope,
	run *everyScopeRun,
	stdout, stderr io.Writer,
) (scopeResult, []byte) {
	name, kind, label := scopeName(scope)
	scoped := parsed
	scoped.allScopes, scoped.specs, scoped.changeID = false, scope.currentSpecs, scope.changeID
	// Each scope renders into a buffer, so color follows the real output.
	scoped.color = choose(colorEnabled(parsed.color, stdout), "always", "never")
	scoped.every = run
	var output bytes.Buffer
	recorded := len(*run.verdicts)
	code := routeCommand(command, scoped, &output, stderr)
	if len(*run.verdicts) == recorded {
		*run.verdicts = append(*run.verdicts, humanReport{verdict: "FAILED  " + label + " — could not be checked"})
	}
	result := scopeResult{Scope: name, Kind: kind, ExitCode: code, Result: rawJSON(output.Bytes())}
	return result, []byte(fmt.Sprintf("== %s ==\n%s\n", label, output.String()))
}

// runEveryScope runs a command once per scope, reports each scope separately,
// and exits with the worst code: 2 when a scope could not be checked, else 1
// when a scope failed, else 0.
//
// @implements req.linkindex.0c06109d8d29
func runEveryScope(command string, parsed options, stdout, stderr io.Writer) int {
	scopes := everyScope(parsed.root, parsed.backend)
	if len(scopes) == 0 {
		return writeCommandError(stderr, errors.New("no current specifications and no active changes to check"))
	}
	if command == "validate" {
		setDefaultOutputPaths(&parsed)
	}
	reports := make([]Report, 0, len(scopes))
	verdicts := make([]humanReport, 0, len(scopes))
	results := make([]scopeResult, 0, len(scopes))
	failed := make([]string, 0)
	worst := 0
	for index, scope := range scopes {
		run := &everyScopeRun{first: index == 0, reports: &reports, verdicts: &verdicts}
		result, output := runOneScope(command, parsed, scope, run, stdout, stderr)
		worst = max(worst, result.ExitCode)
		if result.ExitCode != 0 {
			_, _, label := scopeName(scope)
			failed = append(failed, label)
		}
		results = append(results, result)
		if !parsed.json && !parsed.quiet {
			_, _ = stdout.Write(output)
		}
	}
	if parsed.reportPath != "" && command != "test" {
		if err := writeJSON(resolveWithin(parsed.root, parsed.reportPath), reports); err != nil {
			return writeCommandError(stderr, err)
		}
	}
	if parsed.json {
		writeMachineJSON(stdout, results)
	} else {
		renderScopeSummary(stdout, verdicts, failed, len(scopes), parsed)
	}
	if command == "test" {
		return worst
	}
	return withVersionCheck(worst, parsed, stderr)
}
