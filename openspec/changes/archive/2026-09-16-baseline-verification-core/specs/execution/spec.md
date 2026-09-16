## Purpose

Execute exactly the tests that scenarios name, one at a time, and record evidence that stays bound to the inputs it was produced from.

## ADDED Requirements

### Requirement: Execute each linked scenario test exactly
Verification-ID: req.execution.f9056cdc6fe6

The `stele test` command SHALL run every test that a scenario anchor resolves to on its own, count a scenario as passed only when that exact test ran and passed, and record a failure reason otherwise.

#### Scenario: Select one named TypeScript test
Verification-ID: scn.execution.bce9246e1444

- **WHEN** a scenario anchor resolves to a named test in a `.mts` or `.ts` file
- **THEN** the runner executes only that test and records the scenario as passed when it passes

#### Scenario: Fail a scenario whose test did not run
Verification-ID: scn.execution.8371d74b5134

- **WHEN** the test process succeeds but the named test was not executed
- **THEN** the scenario is recorded as failed with reason `test-not-executed`

#### Scenario: Reject unsupported test files
Verification-ID: scn.execution.a9dde959cf57

- **WHEN** a scenario anchor points into a test file type without a runner
- **THEN** the scenario is recorded as failed with reason `unsupported-test-extension`

### Requirement: Bind evidence to verified inputs
Verification-ID: req.execution.7e4755bd8f60

Execution evidence SHALL carry a digest of the repository inputs, and verification SHALL treat evidence whose digest no longer matches as stale.

#### Scenario: Mark changed-input evidence as stale
Verification-ID: scn.execution.a70f24b45cfe

- **WHEN** evidence was recorded for a different input digest than the current one
- **THEN** the report marks the affected scenarios and the execution stage as `stale`
