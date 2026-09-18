<!-- stele: spec v1 -->
# execution Specification

## Purpose
Execute exactly the tests that scenarios name, one at a time, and record evidence that stays bound to the inputs it was produced from.

## Requirements

### Requirement: Execute each linked scenario test exactly
Verification-ID: req.execution.f9056cdc6fe6

The `stele test` command SHALL run every test that a scenario anchor resolves to and no test that no selected anchor resolves to, count a scenario as passed only when that exact test ran and passed, and record a failure reason otherwise. A test MAY share its test process with other selected tests, as the requirement "Batch selected tests per package and file" describes, and its outcome SHALL still be decided from that test's own result.

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

### Requirement: Isolate Node scenario tests from an enclosing test runner
Verification-ID: req.execution.c27e3b85223e

The Node test runner SHALL start each linked Node test without the enclosing process's `NODE_TEST_CONTEXT`, so that the test reports its own result to Stele even when Stele runs inside `node --test`.

#### Scenario: Run a linked Node test from inside node --test
Verification-ID: scn.execution.9cbf5cc6d03d

- **WHEN** Stele executes a passing linked Node test while `NODE_TEST_CONTEXT` is set in its environment
- **THEN** the test runs, reports its result, and the scenario is recorded as passed

### Requirement: Batch selected tests per package and file
Verification-ID: req.execution.5aa38806b077

The runner SHALL start one test process per batch. A Go batch SHALL hold the selected tests of one package that need the same set of build tags, run as `go test -json -count=1 -run '^(A|B|…)$'` with those tags. A Node batch SHALL hold the selected tests of one test file, run as `node --test --test-reporter=tap --test-name-pattern='^(a|b|…)$'`. Batches SHALL run one at a time, in order of their file path and build tags. Each test's outcome SHALL come from its own result in the batch output: passed; failed with `test-process-failed`; failed with `test-skipped` when it was skipped (Go `t.Skip`, Node `# SKIP`); or failed with `test-not-executed` when a batch that completed reported no result for it.

A batch completes when its test process reports the end of its run: for Go, the test binary's final `PASS` or `FAIL` line; for Node, the TAP summary with no failure of the test file itself. When a batch does not complete, because the process crashed, exited early, failed to build, or was killed, each test with a final result SHALL keep it, and every other test of the batch SHALL be recorded as failed with reason `test-process-failed`. The progress output and the human report SHALL name a batch that did not complete and its exit status. Progress SHALL report each test's result as soon as the batch output reveals it, so a failed test is shown before its batch ends and the line of a test file appears when its last test has a result. Evidence SHALL contain no process output. For tests that neither crash nor depend on sharing their process, the evidence, report files, `--json` output, and final report SHALL be identical to those of running each test in its own process.

#### Scenario: Batch the tests of one Go package
Verification-ID: scn.execution.500afe3889f6

- **WHEN** scenarios link four names in one Go package: a passing test, a failing test, a test that calls `t.Skip`, and a name no test has
- **THEN** one `go test` process runs, and the scenarios are recorded as passed, failed with `test-process-failed`, failed with `test-skipped`, and failed with `test-not-executed`

#### Scenario: Batch the tests of one Node test file
Verification-ID: scn.execution.8653331d296a

- **WHEN** scenarios link four names in one `.mts` file: a passing test, a failing test, a skipped test, and a name no test has
- **THEN** one `node --test` process runs, and the scenarios are recorded as passed, failed with `test-process-failed`, failed with `test-skipped`, and failed with `test-not-executed`

#### Scenario: Keep different build tag sets apart
Verification-ID: scn.execution.0aa78cf92f34

- **WHEN** one Go package has selected tests in a plain file and in a file guarded by `//go:build integration`
- **THEN** the runner starts two processes for the package, only one of them with `-tags=integration`, and records every test's outcome

#### Scenario: Mark the unreported tests of a crashed batch
Verification-ID: scn.execution.aba48bf93edd

- **WHEN** a Go batch of three tests runs a test that calls `os.Exit(3)` after the first test passed, and a Node batch runs a test that calls `process.exit(3)`
- **THEN** the Go test that passed stays passed, every other test of both batches is recorded as failed with `test-process-failed`, and the report names each batch and its exit status

#### Scenario: Match the outcomes of one process per test
Verification-ID: scn.execution.e49b224babb8

- **WHEN** the same Go and Node fixtures of passing, failing, skipped, and missing tests run once in batches and once with one process per test
- **THEN** both runs record the same outcome and reason for every test, and write byte-identical evidence files and `--json` output

#### Scenario: Show failures before the batch ends
Verification-ID: scn.execution.93ab1bbe751c

- **WHEN** a Go batch of three tests runs with standard error redirected to a file, and its first test fails while the others are still running
- **THEN** the failed test's line appears on standard error before the batch ends, and the line of the test file follows once all three tests have results
