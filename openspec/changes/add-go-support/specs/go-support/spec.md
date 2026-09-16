## Purpose

Let Stele trace requirements and scenarios to Go declarations and tests, and execute exact Go scenario tests as behavioral evidence.

## ADDED Requirements

### Requirement: Resolve Go anchors to declarations
Verification-ID: req.gosupport.4c353f2b171a

The verifier SHALL find `@implements` and `@verifies` annotations in comments of Go source files and bind each one to the Go declaration that directly follows it: a function, a method named as `Receiver.Method`, or a type for code files, and a top-level `Test` function for `_test.go` files.

#### Scenario: Bind a code anchor to a Go function, method, or type
Verification-ID: scn.gosupport.7798d2c7e562

- **WHEN** `@implements` comments directly precede a Go function, a method on a pointer receiver, and a type declaration
- **THEN** the anchors resolve to the function name, to `Receiver.Method`, and to the type name

#### Scenario: Bind a test anchor to a Go test function
Verification-ID: scn.gosupport.9bd4f6844031

- **WHEN** an `@verifies` comment directly precedes `func TestName(t *testing.T)` in a `_test.go` file
- **THEN** the anchor is a test anchor that resolves to `TestName`

#### Scenario: Ignore annotations outside comments
Verification-ID: scn.gosupport.351e919a25ad

- **WHEN** a Go file contains annotation text only inside a string literal
- **THEN** the verifier records no anchor for that text

#### Scenario: Leave an unattached Go anchor unresolved
Verification-ID: scn.gosupport.29a77eab096c

- **WHEN** an annotation comment is followed by a statement or another non-declaration instead of a declaration
- **THEN** the anchor has no selector and implementation verification reports `ANCHOR_TARGET_MISSING`

#### Scenario: Reject a Go file that cannot be parsed
Verification-ID: scn.gosupport.8954a51cba0f

- **WHEN** a scanned Go file contains a syntax error
- **THEN** verification exits with code `2` and names the file

### Requirement: Execute exact Go scenario tests
Verification-ID: req.gosupport.d8c058038daa

The `stele test` and `stele validate` commands SHALL run each Go test that a scenario anchor resolves to on its own, in its package, without cached results and with the build constraints of its file satisfied, and SHALL count the scenario as passed only when that exact test ran and passed.

#### Scenario: Pass a scenario whose Go test passes
Verification-ID: scn.gosupport.d195095fc292

- **WHEN** a scenario anchor resolves to a passing Go test
- **THEN** only that test runs and the scenario is recorded as passed

#### Scenario: Fail a scenario whose Go test fails
Verification-ID: scn.gosupport.208b6a95ea3f

- **WHEN** a scenario anchor resolves to a failing Go test
- **THEN** the scenario is recorded as failed with reason `test-process-failed`

#### Scenario: Fail a scenario whose Go test is skipped
Verification-ID: scn.gosupport.75d202e0ed36

- **WHEN** the linked Go test calls `t.Skip`
- **THEN** the scenario is recorded as failed with reason `test-skipped`

#### Scenario: Fail a scenario whose Go test did not run
Verification-ID: scn.gosupport.203331e4d990

- **WHEN** the Go test process succeeds but no test with the exact linked name ran
- **THEN** the scenario is recorded as failed with reason `test-not-executed`

#### Scenario: Honor build constraints of the test file
Verification-ID: scn.gosupport.4a0637d44b7a

- **WHEN** the linked test is in a file guarded by `//go:build integration`
- **THEN** the test runs with the `integration` build tag and its outcome is recorded

#### Scenario: Mix TypeScript and Go scenario tests
Verification-ID: scn.gosupport.2c88ed381429

- **WHEN** one change links scenarios to a named Node test and to a Go test, and both pass
- **THEN** `stele test` records both scenarios as passed and exits with `0`
