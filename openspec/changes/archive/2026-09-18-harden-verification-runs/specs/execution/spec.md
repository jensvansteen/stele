## ADDED Requirements

### Requirement: Isolate Node scenario tests from an enclosing test runner
Verification-ID: req.execution.c27e3b85223e

The Node test runner SHALL start each linked Node test without the enclosing process's `NODE_TEST_CONTEXT`, so that the test reports its own result to Stele even when Stele runs inside `node --test`.

#### Scenario: Run a linked Node test from inside node --test
Verification-ID: scn.execution.9cbf5cc6d03d

- **WHEN** Stele executes a passing linked Node test while `NODE_TEST_CONTEXT` is set in its environment
- **THEN** the test runs, reports its result, and the scenario is recorded as passed
