## Context

See proposal.md. `parseScopeSpecs` returns zero requirements when the scope's spec directory is missing or empty, and `verifyScope` then reports a pass. `runScopeTests` already fails with an empty outcome, but only as a policy failure (exit `1`). `executeNodeTest` starts `node --test` with the full parent environment, including `NODE_TEST_CONTEXT`.

## Goals / Non-Goals

**Goals:**

- A verification gate never goes green for a scope with nothing in it.
- Node scenario tests report to Stele no matter where Stele runs.

**Non-Goals:**

- Rejecting a scope whose specifications contain no requirements, such as a spec file with only a Purpose section. That is OpenSpec's validation job.
- Changing Go test execution. `go test` does not read `NODE_TEST_CONTEXT`.

## Decisions

### Check the scope before any other work

`resolveScope` stays as is. A new `requireScopeSpecs(root, scope)` runs at the start of `verifyScope` and `runScopeTests`, and therefore of `validate`, before tests or OpenSpec start. It returns an error when the scope's spec directory has no Markdown files. The CLI already maps such errors to exit code `2`. The messages are "change <name> has no delta specs" and "no current specifications in openspec/specs". Alternative considered: a verification diagnostic with exit `1`. An empty scope is a wrong invocation, not a failed policy, so exit `2` matches the existing contract.

### Build the Node child environment explicitly

A `nodeTestEnvironment()` helper copies `os.Environ()` without `NODE_TEST_CONTEXT` and appends `STELE_CHILD_TEST=1`. `executeNodeTest` uses it. Alternative considered: setting `NODE_TEST_CONTEXT` to an empty value. Node treats presence, not content, as the signal, so the variable has to be removed.

### Remove the end-to-end workaround

`tests/cli.test.mts` stops filtering the environment in its `cli()` helper. The mixed and archive end-to-end tests then prove the fix, because they run `stele test` and `stele validate` from inside `node --test`.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-16, before implementation started (plan commit ebdac6e).

Levels, defined by what the test reaches: **unit** calls code directly, in process, possibly with temporary files or stubbed dependencies. **integration** exercises our code together with one real outside tool, such as Node, OpenSpec, or the Go toolchain. **e2e** uses the real product through its user-facing entry point: the installed executable or packed package.

Evidence IDs extend the scenario's behavior ID with the level (`<scenario>.<level>[.<n>]`). The v1 linkage plan records the ★ row.

| Scenario | Level | Evidence ID | Boundary | Target | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|---|
| `scn.verificationscope.0246717fcb77` Reject a change that does not exist | unit ★ | `scn.verificationscope.0246717fcb77.unit` | CLI `Run` on a temporary root | `internal/stele/scope_test.go#TestRunRejectsMissingChange` | Risk: a typo or an archived default change passes a gate. `Run` returns the exit code and message in process. |
| `scn.verificationscope.2af83d65e804` Reject a change without delta specs | unit ★ | `scn.verificationscope.2af83d65e804.unit` | CLI `Run` for verify, test, and validate | `internal/stele/scope_test.go#TestRunRejectsChangeWithoutSpecs` | Risk: one of the three commands keeps the silent pass. The check must run before tests and OpenSpec, which stubs confirm cheaply. |
| `scn.verificationscope.6f3a1ce24f05` Reject empty current specifications | unit ★ | `scn.verificationscope.6f3a1ce24f05.unit` | CLI `Run --specs` on a temporary root | `internal/stele/scope_test.go#TestRunRejectsEmptyCurrentSpecs` | Risk: a repository without archived changes passes `--specs` gates vacuously. Pure file check. |
| `scn.execution.9cbf5cc6d03d` Run a linked Node test from inside node --test | integration ★ | `scn.execution.9cbf5cc6d03d.integration` | real `node --test` with `NODE_TEST_CONTEXT=child-v8` set | `internal/stele/runner_test.go#TestExecuteNodeTestIgnoresEnclosingTestRunner` | Risk: the nested runner prints no TAP and the scenario reads as not executed. Only real Node shows that behavior. |
| | e2e | `scn.execution.9cbf5cc6d03d.e2e` | built `dist/stele` started by `node --test` | `tests/cli.test.mts#runs TypeScript and Go scenario tests together` | Second level for a distinct risk: a real enclosing `node:test` runner may set more than the variable the integration test simulates. The existing mixed end-to-end test covers it once its workaround is removed. |

Requirement implementation targets:

| Requirement | Target |
|---|---|
| `req.verificationscope.270b822fff6b` Reject an empty verification scope | `internal/stele/scope.go#requireScopeSpecs` |
| `req.execution.c27e3b85223e` Isolate Node scenario tests from an enclosing test runner | `internal/stele/runner.go#nodeTestEnvironment` |

## Risks / Trade-offs

- [A repository that runs `stele validate --specs` before archiving anything now fails with exit `2`] → This is intended. The CLI reference documents it, and such repositories use `--change` until their first archive.
- [Fixture-based tests elsewhere may rely on empty scopes passing] → They are updated in the same change, and the 100% coverage gate exposes any missed branch.
