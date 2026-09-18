<!-- stele: spec v1 -->
## MODIFIED Requirements

### Requirement: Execute exact Go scenario tests
Verification-ID: req.gosupport.d8c058038daa

The `stele test` and `stele validate` commands SHALL run each Go test that a scenario anchor resolves to, and no Go test that no selected anchor resolves to, in its package, without cached results and with the build constraints of its file satisfied, and SHALL count the scenario as passed only when that exact test ran and passed. A Go test SHALL share its `go test` process only with selected tests of the same package and build tags.

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
