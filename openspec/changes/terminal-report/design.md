## Context

See proposal.md for the problem. The current state that shapes the approach:

- **Rendering is spread over the commands.** `cli.go` prints one summary line per stage from `renderValidation`, `renderVerification`, and `renderScenarioExecution`. `validate` prints no diagnostics, and `verify` prints each diagnostic as one raw line.
- **Stages are merged.** `Report.Verdicts.Linkage` comes from every diagnostic, so identity (`ID_*`), plan (`PLAN_*`), and anchor (`LINK_*`, `ANCHOR_*`) problems all appear as "implementation verification fail".
- **The data is already there.** Reports carry requirements and scenarios with titles and source locations, planned evidence with approval states, and executions with evidence IDs. Diagnostics carry a code, a severity, an identity, and a source. The capability is the directory name in each source path.
- **Tests run one at a time.** `executeTestGroups` runs one process per test group (path and selector), so a full run of this repository takes about a minute, with no output until the end.
- **Seams.** The package swaps collaborators through package-level function variables (`currentWorkingDirectory`, `runProjectScenarios`, `verifyProject`, `validateProjectOpenSpec`). New seams follow that style.
- **Version.** `Version` is the constant `0.1.0` in `cli.go`. It appears in `--version`, the help text, `verifier.version` in reports, and the header of the forked workflow schema.
- **Archived plans.** `loadArchivedPlans` combines archived plans in path order. OpenSpec 1.13 names an archive `YYYY-MM-DD-<change>` from the local date (`formatLocalDate()` in `core/archive.js`) and records nothing finer: `.openspec.yaml` holds only `schema` and a `created` date. Changes archived on the same day therefore sort by name. In the rc.3 self-verification, `verification-strategy` sorts after `link-index` although it was applied first.
- **Scenario renames.** OpenSpec's `findScenarioLossIssues` rejects a MODIFIED requirement that omits a scenario name the current specification has, and a requirement cannot be both REMOVED and ADDED under the same name. A REMOVED requirement plus an ADDED requirement under a new name validates and archives correctly (tried against a copy of the rc.3 specifications).
- **Branches that land first.** `chore/self-verify-rc3` archives five changes into `openspec/specs` and moves self-verification to rc.3. `feat/spec-annotation` adds `stele annotate` and the `SPEC_ANNOTATION_*` codes. The delta specs here are written against the specifications as they will be after `chore/self-verify-rc3`, and the change is rebased after both branches merge.

## Goals / Non-Goals

**Goals:**

- A person reading one report knows which stage failed, why, where, and the next command to run.
- Long runs show progress without polluting standard output or CI logs.
- One command gates CI.
- The machine interface (`--json`, report and evidence files) stays unchanged and deterministic.

**Non-Goals:**

- Running tests in parallel. That would shorten the minute, but it changes execution semantics and needs its own change.
- A full-screen or interactive user interface.
- Localizing messages, or a machine-readable diagnostic catalogue (`stele explain CODE`). The catalogue could support it later.
- Fence-aware annotation detection. By the user's decision it becomes a separate follow-up change (see Decision 13).
- Lifecycle skills ending with `stele check`. By the user's decision that is a later change.

## Decisions

### 1. A report model between results and text

A new `report.go` builds a `humanReport` from what the command already has (`Report`, `Evidence`, the OpenSpec result, the scope, the stage durations). A pure `renderHumanReport(writer, model, style)` then prints it. The commands only choose which sections apply:

| Command | Header | Overview | Check lines | Problems | Tests | Verdict |
|---|---|---|---|---|---|---|
| `validate` | yes | yes | OpenSpec, specifications, plan approval, linkage, test execution | yes | failed and stale | yes |
| `verify` | yes | yes | specifications, plan approval, linkage, test execution (stored evidence, no duration) | yes | failed and stale | yes |
| `test` | yes | yes | test execution | no | failed and stale | yes |

The model is sorted when it is built: capabilities in specification order, groups by severity and then code, findings by path, line, and ID, and tests by path and selector. Rendering never depends on map order.

*Alternative:* keep printing in each command and add diagnostics to `renderValidation`. It was rejected because the three commands would drift, and the rendering could not be tested for determinism in one place.

### 2. Stages come from a diagnostic catalogue

A new `diagnostics.go` holds one entry per code: its stage, a one-line meaning, and a fix template that gets the scope (`--change <id>` or `--specs`):

| Stage (check line) | Codes |
|---|---|
| Specifications | `ID_*`, `SCENARIO_MISSING`, `SPEC_ANNOTATION_*` (after rebase), and the new `SPEC_REMOVED_UNMATCHED` |
| Plan approval | `PLAN_*`, including the new `PLAN_ARCHIVE_ORDER_AMBIGUOUS` and `PLAN_REMOVED_BEHAVIOR_PLANNED` |
| Linkage (anchors) | `LINK_*`, `ANCHOR_*`, including the new `LINK_REMOVED_BEHAVIOR_ANCHORED` |

Examples:

- `PLAN_UNAPPROVED` means "planned evidence has no human approval yet". Its fix is: review the levels in design.md, then run `stele approve --specs`; agents run `… --confirmed-in-chat` after an explicit yes.
- `PLAN_APPROVAL_STALE` means "the scenario or entry changed after approval". Its fix is to approve again with the same command.
- `PLAN_V1_DEPRECATED` means "version 1 plan, supported until 0.2.0". Its fix is `stele plan migrate --specs`, then review and approve.
- `LINK_EVIDENCE_MISSING` means "no test has `@verifies <evidence-id>`". Its fix is to add the anchor above the test named in the plan's placement.
- `LINK_CODE_MISSING` means "no code has `@implements <req-id>`". Its fix is to add the anchor above the implementing declaration.
- `LINK_REMOVED_BEHAVIOR_ANCHORED` means "code or a test is still anchored to behavior this change removes (or, with `--specs`, that an archived change removed)". Its fix is to delete the code or test, or, if it now serves other behavior, remove the anchor and anchor it to that behavior's ID.
- `PLAN_REMOVED_BEHAVIOR_PLANNED` means "the plan still lists evidence for removed behavior". Its fix is to delete the entry from `linkage-plan.json` (`openspec-update-change`), then run the proposal check again.
- `SPEC_REMOVED_UNMATCHED` means "a REMOVED requirement name matches no requirement in the current specification, so nothing is checked". Its fix is to copy the exact name from `openspec/specs/<capability>/spec.md` or remove the entry.

The existing `verdicts.linkage` keeps its meaning in JSON (any error). Only the human check lines are split by stage.

An unknown code, for example one added without a catalogue entry, renders under linkage with the text "see docs/reference/cli.md#diagnostics". A unit test prevents that from shipping: it scans the package's non-test sources for diagnostic code literals and requires a catalogue entry for each. The check has a small allowlist for non-diagnostic constants such as `NODE_TEST_CONTEXT`.

### 3. Findings: ID, title, location

A finding shows `IdentityID`. For an evidence ID, the title is looked up by stripping the level suffix. The title comes from the report's requirements and scenarios. The location is the diagnostic's `Source`, or else the scenario's or requirement's source. Groups show five findings. `--details` removes the limit, and the `… N more (--details)` line appears only when findings were hidden.

### 4. The report layout

The overview is a fixed-width table. Column widths are computed from the content, so the output is still deterministic. The status mark is `✓`, `✗`, or `!` (warnings only).

**On a terminal**, `stele validate --all`, final report on standard output:

```
stele 0.1.0-rc.4 · validate · current specifications

  Capability             Scenarios  Tests               Plan                Status
  backend                        4  4 passed            4/4 approved        ✓
  execution                      7  7 passed            7/7 approved        ✓
  init                           4  5 passed            1/5 approved        ✗
  verify                        21  24 passed, 1 failed 25/25 approved      ✗
  ⋮

  ✓ OpenSpec strict validation   passed
  ✓ Specifications               36 requirements, 114 scenarios; every ID present
  ✗ Plan approval                123 of 139 evidence entries unapproved
  ✓ Linkage (anchors)            36 requirements implemented, 139 tests anchored
  ✗ Test execution               138/139 passed · unit 121 · integration 6 · e2e 12 · 58.3s

Errors
  PLAN_UNAPPROVED ×123  Planned evidence has no human approval yet.
    → Fix: review the levels in each change's design.md, then run `stele approve --specs`
           (agents: after an explicit yes in chat, `stele approve --specs --confirmed-in-chat`)
    scn.backend.17bc5c526b79.unit   Use OpenSpec by default                openspec/specs/backend/spec.md:14
    scn.init.cbf5781012fa.unit      Create configuration, skills, and …   openspec/specs/init/spec.md:20
    scn.init.cbf5781012fa.e2e       Create configuration, skills, and …   openspec/specs/init/spec.md:20
    scn.init.e841b29256e0.unit      Preserve existing files on a repeat…  openspec/specs/init/spec.md:26
    scn.init.e841b29256e0.unit.2    Preserve existing files on a repeat…  openspec/specs/init/spec.md:26
    … 118 more (--details)

Warnings
  PLAN_V1_DEPRECATED ×1  Linkage plan uses schema version 1, supported until 0.2.0.
    → Fix: run `stele plan migrate --specs`, review the migrated levels, and approve them
    openspec/changes/archive/2026-09-16-add-go-support/linkage-plan.json:1

Failed tests
  ✗ internal/stele/cli_test.go:314  TestValidateNamesFailingCheck
      scn.validate.d9553f1a4c1c.unit · Pass only when every check passes · exit status 1

✗ FAILED  current specifications — plan approval (123 unapproved), test execution (1 failed) · 1 warning
```

`⋮` marks rows left out of this mockup; the report lists every capability.

With `--all`, each scope follows its own `== <scope> ==` header, and the output ends with:

```
Scopes
  ✗ current specifications   plan approval, test execution
  ✓ change spec-annotation   passed
  ✗ change terminal-report   plan approval
✗ FAILED  2 of 3 scopes failed: current specifications, change terminal-report
```

**Live progress on a terminal** (standard error): one line updated in place with `\r` and cleared with `\r\x1b[2K`, cut to the terminal width (`COLUMNS`, else 80). Each failure is printed above it as soon as it happens:

```
✓ OpenSpec strict validation (2.1s)
✗ Plan approval: 123 unapproved (listed in the report)
✗ internal/stele/cli_test.go:314 TestValidateNamesFailingCheck (scn.validate.d9553f1a4c1c.unit)
⠸ current specifications · tests 37/139 · verify › Separate a failed execution from passing linkage (unit) · 36 ✓ 1 ✗ · 0:21
```

**Progress without a terminal** (CI, standard error, append-only, no escapes, one line per finished test file):

```
stele 0.1.0-rc.4 validate --all: 3 scopes
[current specifications] OpenSpec strict validation: passed (2.1s)
[current specifications] plan approval: 123 unapproved; linkage: passed
[current specifications] test execution: 139 tests in 22 files
[current specifications]   ✓ internal/stele/adapter_test.go  8/8 (3.2s) [8/139]
[current specifications]   FAILED internal/stele/cli_test.go:314 TestValidateNamesFailingCheck (scn.validate.d9553f1a4c1c.unit)
[current specifications]   ✗ internal/stele/cli_test.go  17/18 (12.9s) [26/139]
…
[current specifications] test execution: 138/139 passed (58.3s)
[change spec-annotation] …
```

The scope prefix appears only with `--all`. The standard output that follows is the same report as on a terminal, without color unless `--color=always` is given.

`--quiet` prints only the final line, for example `✗ FAILED  current specifications — plan approval (123 unapproved), test execution (1 failed) · 1 warning`.

### 5. Stage order in `validate`

`validate` now runs the fast stages first: OpenSpec strict validation, then a first verification without evidence. Plan, specification, and linkage problems then appear in the progress output before the minute of tests. After the tests, the report is assembled from a second verification with the evidence, as today. Both verifications are deterministic and take milliseconds. The files written and the exit code do not change.

*Alternative:* keep the order and only add progress. It was rejected because the most common failure, an unapproved plan, would still show only after every test.

### 6. Progress: observer, clock, and terminal

- The runner gets an optional `testObserver` in `testRequest` (`planned(total, files)`, `started(group)`, `finished(execution)`). With no observer it does nothing, and it never affects evidence.
- The observer implementations are `ttyProgress` (one line updated in place) and `lineProgress` (append-only). `--quiet` selects none.
- **Clock:** `var now = time.Now`, a seam in the package style. Durations are measured only for progress and the human report.
- **Terminal:** `var isTerminal = func(file io.Writer) bool` checks whether the writer is an `*os.File` whose mode has `os.ModeCharDevice`. This uses only the standard library, so the verifier stays dependency-light. `TERM=dumb` counts as not a terminal for in-place updates. Environment lookups go through `var lookupEnv = os.LookupEnv`.

*Alternative:* `golang.org/x/term`. It was rejected because the only things needed are character-device detection and a width, and a new dependency is not justified for them.

### 7. Color and flags

`--color=auto|always|never` is decided per stream. In `auto`, a stream is colored only when it is a terminal, `NO_COLOR` is empty or unset, and `TERM` is not `dumb`. An explicit `always` or `never` overrides `NO_COLOR`, as no-color.org allows for explicit flags. Colors are used only for marks and the verdict: green `✓`, red `✗`, yellow `!`, and dim locations. The text is complete without color.

- `--details` affects only the human report.
- `--quiet` affects the human report and progress.
- `--json` ignores `--details` and `--color`. `--quiet` still silences progress under `--json`.
- Invalid values exit with code `2` during option parsing.
- Errors that end a command with code `2` are always written to standard error.

### 8. Determinism

- The human report is a pure function of the model and its durations. Unit tests use a fixed clock and a fake terminal, and compare golden output built in process. No golden files are added: the expected strings are small and built in the test.
- JSON and files gain no timing fields: durations live only in the `humanReport` model.
- One e2e test runs `validate --json --report-file` twice in separate processes and compares the bytes, as the existing index determinism test does.

### 9. `stele check`

`check` is a command in `cli.go` routed to `check.go`. It runs three steps, in this order:

1. **IDs:** `assignScopeIdentities(root, scope, check=true)` for every scope, including the current specifications. That internal call already works for them, so `stele ids` gets no new flags.
2. **Annotations:** the annotation check from `spec-annotation`, for the same scopes.
3. **Validation:** `validate`, or `runEveryScope("validate")` with `--all`, with the pass-through flags.

Every step runs. The exit code is the worst one (`2` over `1` over `0`), the same rule `--all` uses. The human output is a step summary (`✓ Verification-IDs`, `✓ Annotations`, `✗ Validation`), then the validation report, then one verdict line that names the failed steps. The `--json` document is `{schemaVersion: 1, verdict, exitCode, steps: [{step, exitCode, result}]}`, where each `result` is the step's own JSON (`IdentityResult`s per scope, the `annotate --json` document, and the `validate --json` document or `--all` array).

The `stele-verify` skill and the CLI reference recommend `stele check --change <change>` during work and `stele check --specs` after archiving. The lifecycle skills keep naming `stele validate`: the `lifecycle` specification names it, and changing that is a separate decision (see Open Questions). `verify:self` runs the published package (`stele-published`), so it switches to `stele check --specs` only after a release that contains `check`.

### 10. The version from `package.json`

`Version` becomes `var Version = "0.0.0-dev"` in a new `version.go`, because `-X` works only on variables. `scripts/build-go.mts` reads `package.json` and passes `-ldflags "-s -w -X github.com/jensvansteen/stele/internal/stele.Version=<version>"` to every target. `prepack` and `release.yml` already go through `npm run build`, so released binaries always carry the package version. Go tests keep the default, and unit tests set the variable when they need a specific value.

*Alternatives:*

- `go:embed` of `package.json` was rejected: embed cannot reach outside the package directory.
- A checked-in constant plus a test that compares it with `package.json` was rejected: it adds a manual release step that the test only catches afterwards.
- A generated `version.go` was rejected: it would be generated output in git.

The forked workflow schema's header comment will name the real version. Nothing compares it, so existing schema forks do not drift.

### 11. Combining archived plans by content, then date

For `--specs`, Stele parses each archived change's delta specs once and records a normalized text digest per identity. It uses the same normalization as the approval digest: scenario title and block, or requirement text. For each identity in the current specifications:

1. The candidates are the archived plans that list the identity.
2. The preferred ones are candidates whose archived delta declares the identity with the current text.
3. The winner is the last of the preferred ones (or of all candidates when none match) by date prefix, then change name.
4. When the winner and the runner-up share a date prefix and plan different entries (evidence IDs and levels, or v1 targets), Stele adds a `PLAN_ARCHIVE_ORDER_AMBIGUOUS` warning that names both plans.

In practice, the last change that modified a scenario is the one whose text matches. This fixes the same-day case without new metadata.

*Alternatives:*

- A Stele-recorded archive sequence, for example a field written by `stele-archive`, was rejected for now. It adds state that archives made directly through OpenSpec would lack.
- Ordering by git history was rejected: it breaks in shallow CI clones and in package tarballs.

### 12. Renaming scenario `scn.init.754fd262e114`

A MODIFIED requirement cannot rename a scenario in OpenSpec 1.13, so the init delta:

- removes "Initialize a consumer project", written in the bullet form ``- `### Requirement: …` `` that OpenSpec accepts for REMOVED;
- adds "Initialize a project" with the same text, IDs, and scenarios, where `scn.init.754fd262e114` becomes "Initialize without a default change".

Stele IDs are explicit lines, so identity is kept. The title change makes the scenario's existing approval stale, and this plan lists it again.

The bullet form is kept, although Decision 14 makes the header form work too: both forms are valid OpenSpec, and the bullet form lets this change pass its proposal check with the released binary before Decision 14 is implemented.

### 13. Fence-aware annotation detection: a separate follow-up

The user decided on 2026-09-18 that fence-aware annotation detection becomes its own follow-up change. It stays out of this change and out of `spec-annotation`. Until then, an annotation line inside a fenced code block is reported as misplaced, and the specification format page documents that (task 7.4).

### 14. REMOVED requirements: parse them correctly, then check that removed behavior is gone

**Parsing.** While planning this change, Stele's spec parser (`specs.go`) read `### Requirement:` under `## REMOVED Requirements` as an active requirement. It then reported `ID_REQUIREMENT_MISSING` and `SCENARIO_MISSING`, although `stele ids` already skips REMOVED sections. The parser will track the delta section the same way `ids.go` does (`deltaSectionPattern`) and skip requirement headers and their bodies inside REMOVED. The bullet form was never parsed as a heading, and it now also feeds the list of removed names.

**Resolution.** For a change scope, each REMOVED name is resolved against `openspec/specs/<capability>/spec.md`, where the capability is the delta's directory, using exact matching after trimming, as OpenSpec's `normalizeRequirementName` does:

- The removed identities are the living requirement's ID and its scenarios' IDs, minus every identity that the change declares again. The change's own init delta (Decision 12) is exactly that case: its IDs move to "Initialize a project" and must not be flagged.
- A living requirement without a Verification-ID contributes nothing. `--specs` verification already reports that ID as missing.

**Diagnostics:**

| Code | Stage | Severity | When |
|---|---|---|---|
| `LINK_REMOVED_BEHAVIOR_ANCHORED` | Linkage | error | Implementation stage: an `@implements` or `@verifies` anchor names a removed identity. The evidence ID `<scenario>.<level>[.<n>]` is reduced to its scenario first. The finding shows the anchor's `path:line`. |
| `PLAN_REMOVED_BEHAVIOR_PLANNED` | Plan approval | error | Both stages: the change's plan lists a removed scenario (v2) or a removed requirement or scenario (v1). It replaces `PLAN_UNKNOWN_ID` for that ID, so each problem is reported once. |
| `SPEC_REMOVED_UNMATCHED` | Specifications | error | Both stages: a REMOVED name matches nothing, so nothing would be checked. OpenSpec also rejects this at archive time; Stele reports it earlier, in its own report. |

Anchors are checked only in the implementation stage. While a change is being proposed, the code of the removed behavior legitimately still exists.

**After archiving.** The request assumed that `ANCHOR_DANGLING` would cover leftovers once the change is archived. It does not. `declaredIdentities` counts archived changes, and the archived delta that once added the requirement still declares its IDs, so a leftover anchor is neither dangling nor in scope, and passes silently. The `verification-scope` delta therefore modifies "Scope anchors to declared identities":

- In `--specs` verification, an anchor whose identity (or evidence ID's scenario) only archived changes declare is behavior removed by an archived change. It is reported as `LINK_REMOVED_BEHAVIOR_ANCHORED`.
- An identity that no specification declares stays `ANCHOR_DANGLING`, as before.
- Identities of active changes stay out of the `--specs` verdict.

This keeps the check continuous: `--change` catches leftovers while the change is open, and `--specs` catches them after it is archived.

**RENAMED** requirements keep their Verification-IDs, because the ID line moves with the heading. They need no removed-behavior check, and the requirement says so explicitly.

*Alternative:* a warning instead of an error for leftover anchors. It was rejected because a test anchored to removed behavior is evidence for a claim the specification no longer makes, which is exactly what Stele exists to prevent.

### Verification strategy

**Status: proposed, awaiting approval.** No entry is approved.

Placement follows AGENTS.md: co-located Go tests in `internal/stele`, CLI contract tests in `tests/cli.test.mts`, and packed-package tests in `tests/package_install_test.go`. Placement is advisory. Unit tests call `Run` or the renderer in process, with the seams from Decision 6 (clock, terminal, environment) and the existing command stubs. No e2e is planned for the removed-behavior checks: name resolution, anchor matching, and plan checks run the same code in process as in the binary, so the shipped binary adds no distinct risk. e2e is used only where the shipped binary adds a risk of its own: real terminal detection on real file descriptors, the process-level determinism of files, the version wired in by the build script, and the installed CI gate.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|
| `scn.terminalreport.00fe379ba4b4` Name the stage that failed | unit | `….00fe379ba4b4.unit` | `internal/stele/report_test.go`, beside the new `report.go` | Risk: plan problems still read as implementation failures (the reported bug). Stage mapping and rendering are pure over a built report. |
| `scn.terminalreport.99cec4c2f435` Overview per capability | unit | `….99cec4c2f435.unit` | `report_test.go` | Risk: capabilities are mis-grouped or mis-marked. Pure grouping over report sources. |
| `scn.terminalreport.5b46740a15da` Count tests by level and time the run | unit | `….5b46740a15da.unit` | `report_test.go` | Risk: wrong level counts, or a duration taken from the wrong clock. A fixed clock makes 42.0s exact. |
| `scn.terminalreport.37651d67c39c` Group and truncate one code | unit | `….37651d67c39c.unit` | `report_test.go` | Risk: groups are split, the more-count is off by one, or titles are missing for evidence IDs. Pure over 123 synthetic findings. |
| `scn.terminalreport.a76da19c3313` Show warnings in validation | unit | `….a76da19c3313.unit` | `internal/stele/cli_test.go`, beside the validate command tests | Risk: `validate` keeps hiding warnings, or warnings change the exit code. `Run` in process with stubbed tests and OpenSpec. |
| `scn.terminalreport.78fda44eee93` Give every diagnostic code guidance | unit | `….78fda44eee93.unit` | `internal/stele/diagnostics_test.go`, beside the new catalogue | Risk: a new code (for example `SPEC_ANNOTATION_*` after rebase) ships without meaning or fix. Scanning the package sources is in process. |
| `scn.terminalreport.4bf69bd755ef` List failed and stale tests | unit | `….4bf69bd755ef.unit` | `report_test.go` | Risk: stale tests are shown as failed or hidden, or locations are missing. Pure over evidence. |
| `scn.terminalreport.a735c55107ce` Update one line on a terminal | unit | `….a735c55107ce.unit` | `internal/stele/progress_test.go`, beside the new `progress.go` | Risk: the line is not cleared, failures are overwritten, or the width is exceeded. A fake terminal and clock capture the exact bytes. |
| | e2e | `….a735c55107ce.e2e` | `tests/cli.test.mts`, run under a pseudo-terminal with `script` (per-OS arguments) | Distinct risk: terminal detection on the real standard error file descriptor of the shipped binary, which no in-process writer exercises. |
| `scn.terminalreport.3fc2e84a5b98` Print plain lines without a terminal | unit | `….3fc2e84a5b98.unit` | `progress_test.go` | Risk: in-place control bytes leak into logs, or the line granularity is wrong. The fake writer captures the bytes. |
| | e2e | `….3fc2e84a5b98.e2e` | `tests/cli.test.mts` (pipes, as CI has) | Distinct risk: the real binary with redirected standard error, which is exactly CI. It proves that no ESC byte appears. |
| `scn.terminalreport.9f3cb9c00f42` Keep standard output clean for JSON | unit | `….9f3cb9c00f42.unit` | `cli_test.go` | Risk: progress is written to standard output, or JSON changes. `Run` gets separate writers in process, and the output is compared with the JSON of the current format. |
| `scn.terminalreport.3c53b931fbf3` Show every finding with details | unit | `….3c53b931fbf3.unit` | `report_test.go` | Risk: `--details` still truncates. Pure. |
| `scn.terminalreport.6678ddf5a27f` Print only the verdict when quiet | unit | `….6678ddf5a27f.unit` | `cli_test.go` | Risk: quiet mode still prints progress or extra lines, or loses the exit code. `Run` in process. |
| `scn.terminalreport.cf302ba34005` Respect NO_COLOR | unit | `….cf302ba34005.unit` | `internal/stele/report_test.go` or `style_test.go` | Risk: color is forced in CI or ignores `NO_COLOR`. Environment and terminal are seams. |
| `scn.terminalreport.14b3f3d729fb` Reject an unknown color mode | unit | `….14b3f3d729fb.unit` | `cli_test.go`, beside option parsing | Risk: a typo is silently accepted. Option parsing in process. |
| `scn.terminalreport.23e3fa620ea0` Render identical reports for identical inputs | unit | `….23e3fa620ea0.unit` | `report_test.go` | Risk: map-order or collection-order nondeterminism in the human report. The same model is rendered from shuffled inputs. |
| `scn.terminalreport.334afe00983b` Leave timing out of machine output | e2e | `….334afe00983b.e2e` | `tests/cli.test.mts`, next to the existing determinism tests | Risk: durations or progress leak into JSON or files. Only separate real processes, with a real clock, show it. |
| `scn.terminalreport.42e58de06dfa` Summarize three scopes | unit | `….42e58de06dfa.unit` | `cli_test.go`, beside `TestAllScopesReportSeparately` | Risk: the summary hides a scope or misnames the failure. In-process run over a fixture with two changes. |
| `scn.terminalreport.cd40642c50e6` Print the version of the installed package | e2e | `….cd40642c50e6.e2e` | `tests/package_install_test.go` (packed-package smoke test) | Risk: the build script does not pass the linker flag, or the packed binary is a stale build. Only the installed package proves it. |
| `scn.terminalreport.026c6192674a` Use one version everywhere | unit | `….026c6192674a.unit` | `cli_test.go`, extending the existing help and version test | Risk: one output keeps a separate constant. The test sets `Version` and checks help, `--version`, the header, and `verifier.version`. |
| `scn.validate.ed99aac59b49` Run every step and report each one | unit | `….ed99aac59b49.unit` | `internal/stele/check_test.go`, beside the new `check.go` | Risk: an early failure skips later steps, or the summary misreports. `Run` in process over a fixture change. |
| `scn.validate.d768594936b1` Exit with the worst code | unit | `….d768594936b1.unit` | `check_test.go` | Risk: code `1` masks a tool failure. Stubbed OpenSpec failure in process. |
| `scn.validate.704b1662c309` Check every scope | unit | `….704b1662c309.unit` | `check_test.go` | Risk: the current specifications or a change are skipped by a step. Fixture with specs and two changes. |
| `scn.validate.ddff25a243df` Gate CI with the installed command | e2e | `….ddff25a243df.e2e` | `tests/package_install_test.go` (consumer project) | Distinct risk: CI and `verify:self` run the installed binary without a terminal, where the exit code, streams, and bundled OpenSpec decide together. |
| `scn.verificationscope.8b577e9712e7` Verify archived behavior (unchanged) | unit | `….8b577e9712e7.unit` (existing) | `internal/stele/scope_test.go` (existing) | Risk: combined plans no longer verify archived behavior after the ordering change. Fixture archive in process. |
| `scn.verificationscope.70c3ee060846` Prefer the most recent archived plan (unchanged) | unit | `….70c3ee060846.unit` (existing) | `scope_test.go` (existing) | Risk: the date order regresses when both texts match. Fixture archives on two dates. |
| `scn.verificationscope.4ad1b478a310` Prefer the plan whose specification matches on the same day | unit | `….4ad1b478a310.unit` | `scope_test.go`, beside the existing ordering test | Risk: the reported rc.3 bug, where alphabetical order wins over the actual last modifier. Fixture with two same-day archives. |
| `scn.verificationscope.1c6a44335f91` Warn when the archive order cannot be decided | unit | `….1c6a44335f91.unit` | `scope_test.go` | Risk: an undecidable order passes silently. Fixture, pure. |
| `scn.verificationscope.f8ab6e2e1812` Run OpenSpec validation for the current specifications (unchanged) | integration | `….f8ab6e2e1812.integration` (existing) | `scope_test.go` (existing) | Risk: the real OpenSpec CLI is not invoked for `--specs`. It needs the bundled OpenSpec. |
| `scn.verificationscope.79a83cf82aba` Reject a conflicting selection (unchanged) | unit | `….79a83cf82aba.unit` (existing) | `scope_test.go` (existing) | Risk: option conflicts are accepted. Parsing in process. |
| `scn.verificationscope.ee5d7b261d62` Ignore anchors of another active change (unchanged) | unit | `….ee5d7b261d62.unit` (existing) | `scope_test.go` (existing) | Risk: the new archive-only rule starts failing scopes on active changes' anchors. Fixture with two changes. |
| `scn.verificationscope.101e06b07f39` Report an anchor that no specification declares (unchanged) | unit | `….101e06b07f39.unit` (existing) | `scope_test.go` (existing) | Risk: `ANCHOR_DANGLING` is lost to the new code. Pure fixture. |
| `scn.verificationscope.9c3ef2025b56` Report anchors to behavior removed by an archived change | unit | `….9c3ef2025b56.unit` | `scope_test.go`, beside the anchor scoping tests | Risk: leftovers pass silently after archiving, because archived deltas still declare the IDs. Fixture archive with a removal, `--specs` in process. |
| `scn.verify.6c22483ab3c3` Preserve requirement and scenario relationships (unchanged) | unit | `….6c22483ab3c3.unit` (existing) | `internal/stele/specs_test.go` (existing) | Risk: section tracking breaks ordinary parsing. Pure fixture. |
| `scn.verify.c5fd3656da59` Report identity shape errors (unchanged) | unit | `….c5fd3656da59.unit` (existing) | `specs_test.go` (existing) | Risk: shape errors stop being reported outside REMOVED. Pure fixture. |
| `scn.verify.9fb98ac252a0` Ignore removed requirements in both forms | unit | `….9fb98ac252a0.unit` | `specs_test.go`, beside identity parsing | Risk: the header or bullet form is parsed as an active requirement (the bug found while planning). Pure parsing. |
| `scn.verify.5172c64aec19` Report code and tests still anchored to removed behavior | unit | `….5172c64aec19.unit` | `internal/stele/verify_test.go` (or `removed_test.go` beside a new `removed.go`) | Risk: leftover code or tests, including evidence-suffixed IDs, go unnoticed. Fixture current spec, change, and anchors, in process. |
| `scn.verify.c03b05c1c11e` Pass when removed behavior has no anchors left | unit | `….c03b05c1c11e.unit` | same file | Risk: false positives once cleanup is done. Same fixture without anchors. |
| `scn.verify.42bc49e1f29f` Report plan entries for removed behavior | unit | `….42bc49e1f29f.unit` | `internal/stele/plan_test.go` | Risk: the finding is reported only as `PLAN_UNKNOWN_ID`, or twice. Pure plan checks. |
| `scn.verify.70eaaa57724a` Reject a removed name that matches nothing | unit | `….70eaaa57724a.unit` | `verify_test.go` | Risk: a misspelled name silently checks nothing. Pure resolution. |
| `scn.verify.ec83e3f24b0a` Accept behavior that moves to a new requirement | unit | `….ec83e3f24b0a.unit` | `verify_test.go` | Risk: moved behavior, like this change's init delta, is flagged as removed. Fixture with REMOVED plus ADDED sharing IDs. |
| `scn.init.cbf5781012fa` Create configuration, skills, and artifacts directory (moved) | unit | `….cbf5781012fa.unit` (existing) | `cli_test.go` (existing) | Risk: init misses a file. Temporary directory in process. |
| | e2e | `….cbf5781012fa.e2e` (existing) | `tests/package_install_test.go` (existing) | Distinct risk: templates embedded in the shipped binary. |
| `scn.init.e841b29256e0` Preserve existing files on a repeated run (moved) | unit | `….e841b29256e0.unit` (existing) | `init_test.go` (existing) | Risk: a second run overwrites user files. |
| | unit | `….e841b29256e0.unit.2` (existing) | `cli_test.go` (existing) | Risk: the CLI message for an initialized project is wrong. A separate in-process path. |
| `scn.init.754fd262e114` Initialize without a default change (renamed) | unit | `….754fd262e114.unit` (existing) | `cli_test.go` (existing) | Risk: init demands a change again, or a bare verify passes. `Run` in process with the OpenSpec setup stubbed. |
| `scn.init.c8813c799047` Print the default workflow (moved) | unit | `….c8813c799047.unit` (existing) | `cli_test.go` (existing) | Risk: the printed workflow drifts from the skills. The output is checked in process. |

Requirements are anchored with `@implements` during implementation. The expected homes are:

- `req.terminalreport.c2e0f6607fd6`: `report.go`, `renderHumanReport`.
- `req.terminalreport.1183f56d16a2`: `diagnostics.go`, the catalogue.
- `req.terminalreport.a36aac068cc3`: `progress.go`.
- `req.terminalreport.ebb070079783`: the output option parsing.
- `req.terminalreport.483a267b2c1a`: the report model builder.
- `req.terminalreport.2cda6fd7e60c`: `runEveryScope`'s summary.
- `req.terminalreport.e78c864148cd`: `version.go`.
- `req.validate.70d1435b3ff3`: `check.go`.
- `req.verify.9dbf2146c01f`: the spec parser (existing anchor).
- `req.verify.511302d1be0b`: a new `removed.go`, the removed-behavior resolution and checks.
- `req.verificationscope.9c81618619df`: the anchor scoping (existing anchor).
- `req.verificationscope.bee89d6750ed`: `loadArchivedPlans`.
- `req.init.eed35c447821`: `initCommand` (existing anchor).

## Risks / Trade-offs

- **[Human output changes completely, and scripts may parse it.]** The CLI reference already says not to. The changelog names the change, and `--json` is unchanged.
- **[The pseudo-terminal e2e test depends on `script`, whose arguments differ on macOS and Linux.]** The test chooses the arguments by platform and skips with a visible message when `script` is missing. Unit tests still cover the rendering.
- **[Content matching for archived plans parses every archived delta on each `--specs` run.]** The cost is milliseconds for tens of archives. If it grows, the digests can be cached in memory per run.
- **[`LINK_REMOVED_BEHAVIOR_ANCHORED` under `--specs` can fail repositories that archived removals and kept tests.]** Those tests prove behavior the specification no longer claims. The changelog names the new error and its fix.
- **[`PLAN_ARCHIVE_ORDER_AMBIGUOUS` could appear in existing repositories.]** It is a warning and never fails a run, and the report gives a fix: approve the intended entries with `stele approve --specs`.
- **[Stage reordering in `validate` runs verification twice.]** Both runs are pure and fast, and only the second, with evidence, is written.
- **[The rebase touches `cli.go`, `README`/docs, and the CHANGELOG, which `feat/spec-annotation` also edits.]** The work is planned after both branches merge, and task 0.3 rebases before implementation starts.

## Migration Plan

- Ship in 0.1.0-rc.4 with a changelog entry: the new human output, `--details`, `--quiet`, `--color`, `stele check`, the version fix, archive ordering, and `PLAN_ARCHIVE_ORDER_AMBIGUOUS`.
- After rc.4 is published, bump `stele-published` and switch `verify:self` to `stele check --specs`.
- Rollback: human output is presentation only, so reverting the release restores the old output without any data migration.

## Decided questions

Decided by the user in chat review on 2026-09-18. The verification levels are not yet approved.

- **Fence-aware annotation detection:** a separate follow-up change, not this one and not `spec-annotation`. Until then, task 7.4 documents the limitation.
- **REMOVED parsing:** fixed in this change, in both the header and the bullet form (Decision 14).
- **Removed behavior:** verified as gone in this change, with `LINK_REMOVED_BEHAVIOR_ANCHORED`, `PLAN_REMOVED_BEHAVIOR_PLANNED`, and `SPEC_REMOVED_UNMATCHED` (Decision 14).
- **Lifecycle skills ending with `stele check`:** a later change.
- **The `verify:self` switch to `stele check`:** after rc.4 is published (task 8.2).
- **The `script`-based pseudo-terminal e2e test:** it may skip with a message when `script` is unavailable.

## Open Questions

- **Leftover anchors after archiving:** the request expected `ANCHOR_DANGLING` to cover them, but archived deltas still declare the IDs. This plan adds the `--specs` archive-only rule to `verification-scope` (Decision 14). Confirm it, or drop that scenario and document the gap instead.
