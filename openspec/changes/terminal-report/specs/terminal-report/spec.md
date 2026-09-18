<!-- stele: spec v1 -->
## Purpose

Show people what a Stele run found, where, and what to do next, with live progress while it runs, without changing the machine-readable output that CI and tools consume.

## ADDED Requirements

### Requirement: Summarize a run in a human-readable report
Verification-ID: req.terminalreport.c2e0f6607fd6

Without `--json`, `stele validate`, `stele verify`, and `stele test` SHALL print a report to standard output with, in this order:

- a header naming the Stele version, the command, and the scope;
- an overview with one row per capability of the scope, in specification order, giving its scenario count, its test outcomes, its plan approval state, and a status mark;
- one check line per stage the command covers, each named for its stage: OpenSpec strict validation (`validate` only), specifications (Verification-IDs and annotations), plan approval, linkage (anchors), and test execution;
- the problems, the failed and stale tests, and a final one-line verdict.

The test execution line SHALL give the passed and total test count, the counts by evidence level, and, when tests ran in this command, the duration. `stele verify` SHALL show the execution line from the stored evidence, and `stele test` SHALL show only the test execution line. A problem SHALL fail only the check line of its own stage, so a plan approval problem is never reported as a linkage or implementation failure.

#### Scenario: Name the stage that failed
Verification-ID: scn.terminalreport.00fe379ba4b4

- **WHEN** `stele validate --specs` runs, every test passes, and the only diagnostics are `PLAN_UNAPPROVED` errors
- **THEN** the plan approval line fails with the number of unapproved entries, the specification, linkage, OpenSpec, and test execution lines pass, and the verdict line names plan approval as the reason the run failed

#### Scenario: Overview per capability
Verification-ID: scn.terminalreport.99cec4c2f435

- **WHEN** the scope has scenarios in two capabilities and a test of one of them failed
- **THEN** the overview lists both capabilities in specification order with their scenario counts, test outcomes, and approval state, and marks only the capability with the failed test as failed

#### Scenario: Count tests by level and time the run
Verification-ID: scn.terminalreport.5b46740a15da

- **WHEN** `stele validate` runs unit and e2e tests and the clock advances 42 seconds during the run
- **THEN** the test execution line gives the passed and total count, the unit and e2e counts, and the duration `42.0s`

### Requirement: Explain problems grouped by diagnostic code
Verification-ID: req.terminalreport.1183f56d16a2

The report SHALL list errors and then warnings, grouped by diagnostic code and ordered by code within each severity. Each group SHALL show the code, the number of findings, a one-line meaning, a line starting with `→ Fix:` that names a concrete next step for the scope, including the command to run when there is one, and its first five findings. Each finding SHALL show the evidence ID or identity, the requirement or scenario title when known, and `path:line` when known. When a group has more findings, it SHALL end with `… N more (--details)`. Every command SHALL show warnings, including `stele validate`. Every diagnostic code Stele can report SHALL have a stage, a meaning, and a fix step.

#### Scenario: Group and truncate one code
Verification-ID: scn.terminalreport.37651d67c39c

- **WHEN** the current specifications have 123 `PLAN_UNAPPROVED` findings
- **THEN** the report shows one `PLAN_UNAPPROVED` group with the count 123, its meaning, a fix step naming `stele approve --specs`, five findings with evidence ID, scenario title, and location, and the line `… 118 more (--details)`

#### Scenario: Show warnings in validation
Verification-ID: scn.terminalreport.a76da19c3313

- **WHEN** `stele validate --specs` passes and the combined plans produce `PLAN_V1_DEPRECATED` warnings
- **THEN** the report lists the warnings with a fix step naming `stele plan migrate`, the verdict says the run passed with warnings, and the exit code is `0`

#### Scenario: Give every diagnostic code guidance
Verification-ID: scn.terminalreport.78fda44eee93

- **WHEN** the catalogue of diagnostic guidance is compared with every diagnostic code that Stele can report
- **THEN** every code has a stage, a one-line meaning, and a fix step

#### Scenario: List failed and stale tests
Verification-ID: scn.terminalreport.4bf69bd755ef

- **WHEN** one test failed and the stored outcome of another test is stale
- **THEN** the report lists the failed test by name with its `path:line`, evidence ID, and failure reason, and lists the stale test separately with the command that runs it again

### Requirement: Show live progress on standard error
Verification-ID: req.terminalreport.a36aac068cc3

While a command runs, Stele SHALL write progress to standard error: each stage as it starts and, during test execution, the number of tests run out of the total, the current scenario and evidence level, the passed and failed counts so far, and the elapsed time. A failed test SHALL be reported as soon as it finishes. When standard error is a terminal and `TERM` is not `dumb`, progress SHALL update a single line in place and clear it before the report is printed. Otherwise progress SHALL be plain append-only lines without ANSI escape sequences: one line per stage, one line per test file when its tests finish, and one line per failed test. Progress SHALL never be written to standard output.

#### Scenario: Update one line on a terminal
Verification-ID: scn.terminalreport.a735c55107ce

- **WHEN** `stele validate` runs tests with standard error attached to a terminal
- **THEN** standard error shows one line that is updated in place with the test counter, the current scenario and level, the passed and failed counts, and the elapsed time, a failed test is printed on its own line as soon as it fails, and the progress line is cleared before the report

#### Scenario: Print plain lines without a terminal
Verification-ID: scn.terminalreport.3fc2e84a5b98

- **WHEN** `stele validate` runs with standard error redirected to a file
- **THEN** standard error contains append-only lines for each stage, each finished test file, and each failed test, and contains no ANSI escape sequences

#### Scenario: Keep standard output clean for JSON
Verification-ID: scn.terminalreport.9f3cb9c00f42

- **WHEN** `stele validate --json` runs tests
- **THEN** standard output contains exactly the JSON document, unchanged from earlier releases, and all progress appears on standard error

### Requirement: Control the detail and color of the output
Verification-ID: req.terminalreport.ebb070079783

`stele validate`, `stele verify`, `stele test`, and `stele check` SHALL accept `--details`, which lists every finding and every failed test without truncation, and `--quiet`, which prints only the final verdict line to standard output and no progress. Errors that stop the command SHALL still be written to standard error. They SHALL accept `--color=auto|always|never`. With `auto`, the default, a stream SHALL be colored only when it is a terminal, `NO_COLOR` is unset or empty, and `TERM` is not `dumb`. `always` and `never` SHALL override `NO_COLOR` and terminal detection. Any other value SHALL exit with code `2`. `--json` output SHALL be the same whatever `--details`, `--quiet`, and `--color` say.

#### Scenario: Show every finding with details
Verification-ID: scn.terminalreport.3c53b931fbf3

- **WHEN** `stele validate --specs --details` runs with 123 `PLAN_UNAPPROVED` findings
- **THEN** the group lists all 123 findings and no `more` line

#### Scenario: Print only the verdict when quiet
Verification-ID: scn.terminalreport.6678ddf5a27f

- **WHEN** `stele validate --quiet` runs and fails
- **THEN** standard output is the single verdict line, standard error has no progress, and the exit code is `1`

#### Scenario: Respect NO_COLOR
Verification-ID: scn.terminalreport.cf302ba34005

- **WHEN** `stele validate` runs on a terminal with `NO_COLOR=1`
- **THEN** its output contains no ANSI escape sequences, and with `--color=always` it does

#### Scenario: Reject an unknown color mode
Verification-ID: scn.terminalreport.14b3f3d729fb

- **WHEN** `stele validate --color=sometimes` runs
- **THEN** the command exits with code `2` before running anything and names the accepted values

### Requirement: Keep reports deterministic
Verification-ID: req.terminalreport.483a267b2c1a

For identical inputs and identical durations, the human-readable report SHALL be byte-identical. Durations SHALL be the only part of the report that depends on time. JSON output and the report and evidence files SHALL contain no durations, timestamps, or progress.

#### Scenario: Render identical reports for identical inputs
Verification-ID: scn.terminalreport.23e3fa620ea0

- **WHEN** the same verification result is rendered twice with the same durations
- **THEN** both renderings are byte-identical, whatever the order in which findings, capabilities, and tests were collected

#### Scenario: Leave timing out of machine output
Verification-ID: scn.terminalreport.334afe00983b

- **WHEN** `stele validate --json --report-file out/report.json` runs twice on identical inputs, at different speeds
- **THEN** both standard outputs are byte-identical, both report files are byte-identical, and neither contains a duration or progress text

### Requirement: Summarize every scope
Verification-ID: req.terminalreport.2cda6fd7e60c

With `--all`, the report of each scope SHALL follow a header naming the scope, and the output SHALL end with a summary listing every scope with its verdict and one final verdict line for the whole run. Progress SHALL name the scope being checked.

#### Scenario: Summarize three scopes
Verification-ID: scn.terminalreport.42e58de06dfa

- **WHEN** `stele validate --all` runs over the current specifications and two active changes, and one change fails
- **THEN** each scope's report follows its own header, the summary lists all three scopes with their verdicts, and the final line says one of three scopes failed and names it

### Requirement: Show the package version
Verification-ID: req.terminalreport.e78c864148cd

`stele --version`, the help text, the report header, and the `verifier.version` field of reports SHALL show the version of the npm package that contains the binary, such as `0.1.0-rc.4`. A binary built outside the package build SHALL show `0.0.0-dev`.

#### Scenario: Print the version of the installed package
Verification-ID: scn.terminalreport.cd40642c50e6

- **WHEN** the packed npm package is installed in a separate project and `stele --version` runs there
- **THEN** it prints the `version` field of the package's `package.json`

#### Scenario: Use one version everywhere
Verification-ID: scn.terminalreport.026c6192674a

- **WHEN** the binary's version is set
- **THEN** `stele --version`, the first line of the help text, the report header, and `verifier.version` in a verification report all show that version
