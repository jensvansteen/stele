## Context

This change records behavior that already ships in `stele-spec` 0.1.0-rc.1 (see proposal.md, Why). No product code changes. The design explains how this repository verifies itself and which evidence proves each identity.

Constraints from the published verifier:

- It resolves anchors only in `.ts`, `.tsx`, and `.mts` files and executes only named Node tests. Every Go target below stays unresolved until `add-go-support` ships in a published release.
- It resolves a test anchor only when the next declaration starts with `test(` or `it(`. The Node CLI tests use `void test(...)`, which the strict typed ESLint rules require for floating promises, so the published verifier cannot select them yet.
- It matches annotation text anywhere on a line, including string literals.
- The v1 linkage plan holds one target per identity.

## Goals / Non-Goals

**Goals:**

- Declare stable identities for the existing `init`, `verify`, `execution`, and `validate` behavior, keeping the four `scn.verify.*` identities already anchored in `tests/cli.test.mts`.
- Choose, for every scenario, the lowest test level that proves it, and name the existing test that does so.
- Pass `stele verify --stage proposal` with the published package.

**Non-Goals:**

- Adding `@implements` and `@verifies` comments to Go files. That follows `add-go-support`, when those comments start to count.
- Passing implementation verification or `stele validate` for this change with the published package.
- Encoding more than one target per identity in the linkage plan. The `verification-strategy` change covers that.

## Decisions

### Verify with the published package

`package.json` pins `stele-published`, an npm alias of `stele-spec@0.1.0-rc.1`, and the `stele:published` script runs its binary: `npm run stele:published -- verify --stage proposal`. npm refuses to install a package under a root package with the same name, which is why the alias exists. `npx stele` is avoided because inside this repository it resolves to the checkout's own `dist/stele`. Alternative considered: the local build. It would let unreleased verifier code grade itself.

### Keep specs as an unarchived change

Stele verifies the delta specs of one change. Archiving this baseline would move its requirements into `openspec/specs/`, where the published verifier no longer reads them. The baseline therefore stays in `openspec/changes/` until Stele can verify archived specs. Alternative considered: archiving now and verifying nothing.

### Track the linkage plan, ignore generated artifacts

`.gitignore` keeps `artifacts/` ignored except `artifacts/linkage-plan.json`, which is a hand-reviewed planning input rather than generated output.

### Stop spelling anchors in test fixture literals

`tests/cli.test.mts` writes a consumer fixture whose file contents contain `@implements` and `@verifies` text. The published scanner reports those strings as `ANCHOR_DANGLING`. The fixture now assembles the annotation from two string parts, so the file contents stay the same. `add-go-support` makes the Go scanner ignore non-comment text.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-16, before implementation started.

Each row is one piece of evidence. A scenario gets a second row only when that row covers a risk the first one cannot. No scenario needs every level. The primary row, marked ★, is the one the v1 linkage plan records. "Checkable now" says whether the published 0.1.0-rc.1 verifier can resolve and execute that target.

Levels, defined by what the test reaches: **unit** calls code directly, in process, possibly with temporary files or stubbed dependencies. **integration** exercises our code together with one real outside tool, such as Node, OpenSpec, or the Go toolchain. **e2e** uses the real product through its user-facing entry point: the UI for applications, and the installed executable or packed package for command-line tools such as Stele.

Evidence IDs extend the scenario's single behavior ID with the level, for example `scn.todo.591a3b429cf0.e2e`. A second entry at the same level adds a number, as in `.e2e.2`. The spec keeps only the behavior ID. Anchors name the evidence ID, as in `// @verifies scn.todo.591a3b429cf0.e2e`. The published verifier reads such an anchor as the plain scenario ID, and the v1 linkage plan stays keyed by that ID until `verification-strategy` adds per-entry plans.

| Scenario | Level | Evidence ID | Boundary | Target | Risk and why this level is the lowest convincing one | Checkable now |
|---|---|---|---|---|---|---|
| `scn.init.cbf5781012fa` Create configuration, skills, and artifacts directory | unit ★ | `scn.init.cbf5781012fa.unit` | `Initialize` on a temporary root | `internal/stele/cli_test.go#TestInitializeWritesConfigAndSkills` | Risk: a file is missing or the config names the wrong change. File creation is deterministic, so an in-process test on a temporary directory proves it. | No, Go |
| | e2e | `scn.init.cbf5781012fa.e2e` | packed package in a separate consumer | `tests/package_install_test.go#TestPackedPackageInitializesAndValidatesSeparateConsumer` | Second level for a distinct risk: the embedded skill templates are missing from the published binary or tarball. Only a real install shows that. | No, Go |
| `scn.init.e841b29256e0` Preserve existing files on a repeated run | unit ★ | `scn.init.e841b29256e0.unit` | `Initialize` twice | `internal/stele/init_test.go#TestInitializeIsIdempotent` | Risk: a re-run overwrites user edits. Idempotency is filesystem logic that a unit test covers. | No, Go |
| | unit | `scn.init.e841b29256e0.unit.2` | CLI `Run` | `internal/stele/cli_test.go#TestRunHandlesHelpVersionAndInitialization` | Same level, covering the other half of the scenario: the "already initialized" message. | No, Go |
| `scn.init.754fd262e114` Require a change for initialization | unit ★ | `scn.init.754fd262e114.unit` | CLI `Run` | `internal/stele/cli_test.go#TestRunHandlesHelpVersionAndInitialization` | Risk: init writes a config without a change. Argument validation happens in process before any I/O. | No, Go |
| `scn.verify.6c22483ab3c3` Preserve requirement and scenario relationships | unit ★ | `scn.verify.6c22483ab3c3.unit` | `ParseSpecs` | `internal/stele/specs_test.go#TestParseSpecsPreservesRelationships` | Risk: an ID attaches to the wrong heading. This is pure parsing on fixtures. | No, Go |
| `scn.verify.c5fd3656da59` Report identity shape errors | unit ★ | `scn.verify.c5fd3656da59.unit` | `ParseSpecs` | `internal/stele/specs_test.go#TestParseSpecsReportsAllShapeDiagnostics` | Risk: a malformed spec passes silently. One parser fixture covers every diagnostic code. | No, Go |
| `scn.verify.36f0ef1f337f` Bind anchors to adjacent TypeScript declarations | unit ★ | `scn.verify.36f0ef1f337f.unit` | `ScanAnchors` | `internal/stele/anchors_test.go#TestScanAnchorsResolvesTypeScriptDeclarations` | Risk: an anchor binds to the wrong declaration. The scanner is pure text analysis on fixtures. | No, Go |
| `scn.verify.0d43596abe4a` Ignore unsupported consumer languages | unit ★ | `scn.verify.0d43596abe4a.unit` | `ScanAnchors` | `internal/stele/anchors_test.go#TestScanAnchorsIgnoresUnsupportedConsumerLanguages` | Risk: anchors in unsupported files produce false links. Scanner fixtures prove it. The `.go` fixture case changes with `add-go-support`. | No, Go |
| `scn.verify.14c6b4fe39bc` Accept planned targets in the proposal stage | unit ★ | `scn.verify.14c6b4fe39bc.unit` | `RunVerification` | `internal/stele/verify_test.go#TestRunVerificationProposalPlans` | Risk: proposal review is blocked by files that cannot exist yet. Stage policy is deterministic over fixtures. | No, Go |
| `scn.verify.c4db6a432869` Reject a mismatched implementation target | unit ★ | `scn.verify.c4db6a432869.unit` | `RunVerification` | `internal/stele/verify_test.go#TestRunVerificationRejectsMismatchedTarget` | Risk: a stale or misplaced anchor counts as linked. Stage policy is deterministic over fixtures. | No, Go |
| `scn.verify.5e9a130cd7b4` Verify a passing change from the command line | e2e ★ | `scn.verify.5e9a130cd7b4.e2e` | built `dist/stele` | `tests/cli.test.mts#exposes an executable verify command` | Risk: the shipped executable, its flags, or its JSON differ from the in-process code. That is only observable by running the binary. | Resolves after the `void test` fix |
| `scn.verify.a2c7e48b610f` Return stable failure exit codes | e2e ★ | `scn.verify.a2c7e48b610f.e2e` | built `dist/stele` | `tests/cli.test.mts#returns stable failure exit codes` | Risk: CI gates misread results. Exit codes are a process contract, so only a real process proves them. | Resolves after the `void test` fix |
| `scn.verify.d6f8012b3ea5` Emit identical JSON for identical inputs | e2e ★ | `scn.verify.d6f8012b3ea5.e2e` | built `dist/stele` | `tests/cli.test.mts#emits identical default JSON for identical inputs` | Risk: nondeterminism across runs, such as map order or environment. Only separate processes expose it. | Resolves after the `void test` fix |
| `scn.verify.3a70c9d1ef24` Omit volatile metadata | e2e ★ | `scn.verify.3a70c9d1ef24.e2e` | built `dist/stele` | `tests/cli.test.mts#omits volatile metadata from the deterministic payload` | Risk: timestamps leak into the emitted payload. The check reads what users receive. | Resolves after the `void test` fix |
| `scn.execution.bce9246e1444` Select one named TypeScript test | integration ★ | `scn.execution.bce9246e1444.integration` | real `node --test` process | `internal/stele/runner_test.go#TestExecuteExactTypeScriptTest` | Risk: Node's name filter selects more or fewer tests than intended. A mock cannot prove real Node behavior; the binary is not needed. | No, Go |
| `scn.execution.8371d74b5134` Fail a scenario whose test did not run | integration ★ | `scn.execution.8371d74b5134.integration` | real `node --test` process | `internal/stele/runner_test.go#TestExecuteNodeTestDetectsNoMatchingExecution` | Risk: a successful process with zero matching tests counts as a pass. Needs Node's real output. | No, Go |
| `scn.execution.a9dde959cf57` Reject unsupported test files | unit ★ | `scn.execution.a9dde959cf57.unit` | `executeExactTest` | `internal/stele/runner_test.go#TestExecuteExactTestRejectsUnsupportedExtensions` | Risk: an unrunnable file is reported as passed. Rejection happens before any process starts. | No, Go |
| `scn.execution.a70f24b45cfe` Mark changed-input evidence as stale | unit ★ | `scn.execution.a70f24b45cfe.unit` | `BuildReport` | `internal/stele/verify_test.go#TestBuildReportTracksEvidenceAndDiagnostics` | Risk: old evidence vouches for new code. Digest comparison is pure logic. | No, Go |
| `scn.validate.d9553f1a4c1c` Pass only when every check passes | unit ★ | `scn.validate.d9553f1a4c1c.unit` | `validateCommand` with stubbed checks | `internal/stele/cli_test.go#TestValidateCommand` | Risk: one failing check is masked by the others. Stubs make every combination cheap. Gap: the test asserts exit codes but not the naming of the failing check. | No, Go |
| `scn.validate.10388560c2dc` Validate a separately installed package | e2e ★ | `scn.validate.10388560c2dc.e2e` | packed package, separate consumer, `integration` build tag | `tests/package_install_test.go#TestPackedPackageInitializesAndValidatesSeparateConsumer` | Risk: the package works only inside this checkout. Only a real install proves the distribution boundary. | No, Go |

Requirement implementation targets:

| Requirement | Target |
|---|---|
| `req.init.eed35c447821` Initialize a consumer project | `internal/stele/init.go#Initialize` |
| `req.verify.9dbf2146c01f` Parse OpenSpec identities | `internal/stele/specs.go#ParseSpecs` |
| `req.verify.43d1d0d9f883` Resolve anchors to declarations | `internal/stele/anchors.go#ScanAnchors` |
| `req.verify.1d6031f2d3dd` Verify proposal and implementation stages | `internal/stele/verify.go#RunVerification` |
| `req.verify.999a5d082295` Expose verification through the command line | `internal/stele/cli.go#Run` |
| `req.verify.aa9f017c4cb4` Produce deterministic reports | `internal/stele/verify.go#MarshalDeterministic` |
| `req.execution.f9056cdc6fe6` Execute each linked scenario test exactly | `internal/stele/runner.go#RunScenarioTests` |
| `req.execution.7e4755bd8f60` Bind evidence to verified inputs | `internal/stele/files.go#ComputeInputDigest` |
| `req.validate.56cc774dc871` Run the complete deterministic gate | `internal/stele/cli.go#validateCommand` |

## Risks / Trade-offs

- [No executable evidence with the published verifier: every Go target is unresolvable, and the four Node tests use `void test(...)`] → Proposal verification is the gate for now. Implementation verification waits for `add-go-support` and a fix for `void test(...)` selection in a published release.
- [Unarchived baseline and future changes both describe the same capabilities] → `add-go-support` adds a separate capability. Archiving waits until Stele can verify archived specs.
- [`scn.validate.d9553f1a4c1c` names the failing check in the spec, but its test checks only the exit code] → Recorded as a test gap to close when the test is anchored.

- [The published verifier reads one `artifacts/linkage-plan.json` for every change and ignores its `changeId`] → The plan lists the identities of both active changes, and its `changeId` names the default change.
- [Anchors for this change count as `ANCHOR_DANGLING` when another change is selected] → Other changes check their proposals on an isolated copy until Stele can verify several active changes together.

## Open Questions

- How should Stele verify behavior after its change is archived? This decides when the baseline can be archived and does not change the targets above.
