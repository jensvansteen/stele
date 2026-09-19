<!-- stele: spec v1 -->
## MODIFIED Requirements

### Requirement: Summarize a run in a human-readable report
Verification-ID: req.terminalreport.c2e0f6607fd6

Without `--json`, `stele validate`, `stele verify`, and `stele test` SHALL print a report to standard output with, in this order:

- a header naming the Stele version, the command, and the scope;
- an overview with one row per capability of the scope, in specification order, giving its scenario count, its test outcomes, its plan approval state, and a status mark;
- one check line per stage the command covers, each named for its stage: OpenSpec strict validation (`validate` only), specifications (Verification-IDs and annotations), plan approval, linkage (anchors), and test execution;
- when the scope has targeted specifications, a target matrix: one row per capability with, per target, the number of applicable scenarios and how many of them are passed, then one row per scenario that has a cell other than `passed` or `n/a`, with a mark per target (`✓ passed`, `✗ failed`, `✗ missing`, `! unapproved`, `~ stale`, `○ not run`, or `n/a`), at most ten such rows followed by `… N more (--details)`, and with `--details` every targeted scenario;
- the problems, the failed and stale tests, and a final one-line verdict.

With `--target`, the header SHALL name the selected targets, and the matrix SHALL show only their columns.

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

#### Scenario: Show the target matrix
Verification-ID: scn.terminalreport.29b8400fc1b1
- **WHEN** `stele verify --specs` runs for a capability with targets `ios` and `android` whose three scenarios all pass on `ios`, one of which has no `android` evidence and one of which is narrowed to `ios`
- **THEN** the report's target matrix shows the capability with `ios 3/3` and `android 1/2`, and one scenario row with `✓ passed` for `ios` and `✗ missing` for `android`, and no row for the other two scenarios

#### Scenario: Leave the matrix out without targets
Verification-ID: scn.terminalreport.33eebddcfa33
- **WHEN** `stele verify --specs` runs in a project without targets
- **THEN** the report has no target matrix and is otherwise unchanged
