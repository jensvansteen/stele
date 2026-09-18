## Context

See proposal.md for the problem. The current state that shapes the approach:

- **CI.** The `verify` job of `.github/workflows/ci.yml` runs on `ubuntu-latest` and `macos-14`, for pull requests and for pushes to `main`. It runs `npm ci`, `npm test` (which builds `dist/stele`), the documentation build, and `npm pack --dry-run`. On Linux only, it then runs `npm run verify:self`, which is `npm run build --silent && stele check --specs`. `stele` there resolves to the `stele-published` devDependency (`stele-spec@0.1.0-rc.4`).
- **`npm run stele`** already exists: `npm run build --silent && ./dist/stele`. CONTRIBUTING uses it for checks that need the local build.
- **`stele check --all`** runs the ID check, the annotation check, and `stele validate --all` over the current specifications and every active change, in the implementation stage. It exits `1` on any finding, including `PLAN_UNAPPROVED`. With no active change it checks the current specifications only.
- **Anchors** are read only from `.ts`, `.tsx`, `.mts`, and `.go` files (`anchors.go`). A requirement needs an `@implements` anchor on a code declaration, or linkage reports `LINK_CODE_MISSING`.
- **The report model.** `humanReport` already holds the grouped findings (code, meaning, count, first five items with identity, title, and `path:line`), the failed tests with their locations, and the truncation counts. The environment and the terminal are seams (`lookupEnv`, `isTerminal`).
- **Changes so far** merged with their plan and implementation in one pull request (`feat: spec annotation` #18, `feat: terminal report` #20). They were archived in a later pull request, after the release that contained them (#22).

## Goals / Non-Goals

**Goals:**

- No pull request merges an active change that Stele would reject: unapproved or incomplete plan, missing IDs or annotations, broken anchors, anchored removed behavior, or failing evidence.
- Keep a gate that does not depend on the build under review.
- Findings are visible where reviewers look, on the pull request diff, without changing any machine output.
- Stele users can copy one workflow and know what each exit code and output mode means in CI.

**Non-Goals:**

- The release workflow.
- `fast-runs` (batching and parallel test execution), and the execution groups it adds.
- A GitHub step summary, `::group::` folding, or annotation formats of other CI systems.
- Checking a change at the proposal stage in CI (see Decision 7).

## Decisions

### 1. Two gates, because each misses what the other catches

| Gate | Stele | Scope | Catches | Cannot catch |
|---|---|---|---|---|
| `verify:self` (unchanged) | published `stele-published` | `--specs` | A pull request that breaks archived behavior, judged by a verifier the pull request cannot change | Active changes, which may need features the published version lacks |
| New step | `dist/stele` built from the same commit | `--all` | Everything about active changes: plan completeness and approval, IDs, annotations, anchors, removed-behavior checks, evidence | A bug in the build under review that hides its own findings: Stele grades itself here |

Neither gate alone is enough. Only the published Stele can judge a pull request that changes Stele itself without trusting that change. For example, a pull request that broke `PLAN_UNAPPROVED` detection would pass its own gate. Only the local build understands a change that uses features the release lacks, such as a new plan field or a new diagnostic. Once the change is released and archived, its behavior moves under `verify:self`. The self-graded window is therefore bounded by the next release.

### 2. `check --all`, not `--change` per active change

The step runs `npm run stele -- check --all`.

- It is the command the user guide recommends. This repository runs what it tells users to run.
- It also checks the current specifications with the local build. A pull request that tightens a rule so that archived behavior no longer passes then fails immediately. Without this, that failure would surface only when `stele-published` is bumped after the next release, when it is expensive to fix.
- It needs no shell loop over `openspec/changes/*` that has to skip `archive/`, and no change to `stele`.

*Cost:* the current specifications' tests run twice on Linux, about 1–2 minutes more, and about 15 s once `fast-runs` lands.

*Alternative:* a `--change` loop over the active changes only. It saves that time but loses the second point, and adds shell logic that duplicates `everyScope`.

### 3. Always run; no path filter

Anchors live in code and tests, not only in `openspec/`. A pull request that touches only `internal/` can break an active change's anchors or tests. A `paths:` filter or a `git diff` condition would skip exactly those cases. A skipped required check also leaves the pull request waiting, so it needs workaround jobs. The cost is bounded (Decision 2). The step runs on every pull request and every push to `main`.

### 4. Linux only

- The verdict is platform-independent by design: IDs, annotations, plan, and anchors are pure file analysis, and the report is deterministic.
- Platform-specific execution is already covered: `npm test` runs every Go and Node test, including the packed-package test, on both `ubuntu-latest` and `macos-14`.
- macOS runners cost ten times the minutes of Linux runners.
- `verify:self` is Linux-only for the same reasons, and both gates stay side by side.

*Risk:* a scenario test that passes on Linux and fails only on macOS is still caught by `npm test` on macOS, but not reported as Stele evidence. That is acceptable, because Stele's evidence is not a platform matrix.

### 5. The step

The step goes into the existing `verify` job, on the Linux leg, after `verify:self`:

```yaml
      - name: Verify Stele's archived behavior with the published Stele
        if: matrix.os == 'ubuntu-latest'
        run: npm run verify:self
      - name: Check active changes and current specifications with the Stele built from this commit
        if: ${{ !cancelled() && matrix.os == 'ubuntu-latest' }}
        run: npm run stele -- check --all
```

- **`!cancelled()`** runs it even when `verify:self` or an earlier step failed, so one CI run reports both gates. Without it, a failing `verify:self` would hide the state of the active changes.
- **`npm run stele`** rebuilds `dist/stele` if `npm test` did not get that far. The Go build is cached, so a rebuild is cheap.
- **No `continue-on-error`**: any finding fails the job, and `PLAN_UNAPPROVED` fails it with exit code `1`. That enforces the rule that unapproved plan entries fail CI.
- The job is already a required check, so no new required check is needed. A separate parallel job was rejected: it would repeat the setup of Go, Node, and `npm ci` for no gain in total CI time.

### 6. What evidence is honest for the workflow file and the docs

The workflow step and the guide get **no requirements and no linkage-plan entries**. Every Stele requirement needs an `@implements` anchor on a `.ts`, `.mts`, or `.go` declaration. A requirement "CI runs the second gate" would be implemented by YAML, which Stele cannot anchor. It would fail with `LINK_CODE_MISSING` for good, or would need a script created only to carry an anchor. The project rule is to never shape code around anchors.

What protects them instead, in increasing strength:

1. **A repository test** (task 5.2) reads `.github/workflows/ci.yml` and asserts:
   - the Linux leg of `verify` runs `npm run verify:self`;
   - the Linux leg of `verify` runs `npm run stele -- check --all` under `!cancelled()`, without `continue-on-error`;
   - the workflow triggers on `pull_request` and on pushes to `main`;
   - `package.json` defines `stele` as the local build and `verify:self` as `stele check --specs`.

   A second test reads the guide and asserts that its workflow block runs `npx stele check --all` with `node-version: 24`. These tests catch an accidental removal in a later edit, which is the likely failure. They cannot prove that GitHub runs the step. They are ordinary tests without `@verifies`. The repository has no YAML library and stays dependency-free, so the tests read the fixed structure of this one file line by line. This is acceptable because the file is ours and the test fails loudly when the structure changes.
2. **The behavior behind the step is already Stele evidence.** `scn.validate.ddff25a243df` runs the installed `stele check --all` in a consumer project with an unapproved plan and expects exit code `1`. It is extended here to GitHub Actions (Decision 8).
3. **Observation on the implementing pull request** (task 6.2): its CI log shows the new step running `check --all` over `ci-gates`. The step passes only after this change's plan is approved and implemented.

An e2e run of the workflow, for example with `act` and Docker, was rejected. It would test GitHub's runner emulation rather than this repository, and add a heavy dependency for one file.

### 7. Active changes are checked at the implementation stage, so changes merge complete

`check --all` checks every active change at the implementation stage. A plan-only pull request, or one that implements part of a change, fails the gate: requirements have no anchors yet. Once such a change is on `main`, every later pull request fails too. With this gate, a change reaches `main` approved, implemented, and passing, as #18 and #20 did. Plans are reviewed on their branch or in a draft pull request, and implementation continues on the same branch. CONTRIBUTING step 2 changes from "across one or more pull requests" to this rule.

*Alternatives, not chosen:*

- **`stele check --stage proposal` for changes with open tasks.** Stele would have to read `tasks.md` checkboxes, which belong to OpenSpec. A forgotten checkbox would silently lower a change's gate.
- **Proposal stage for all active changes.** This drops evidence, the problem this change fixes.
- **A pull request label or a list of in-progress changes in configuration.** This adds a manual override to a gate meant to be mechanical.

This is the main open question for the reviewer.

### 8. GitHub Actions annotations: auto-detected, with an override

- **Mechanism.** GitHub Actions turns lines such as `::error file=F,line=L,title=T::message` in a step's output into annotations on the pull request diff. Gitea and Forgejo Actions set `GITHUB_ACTIONS=true` and understand the same commands.
- **Why not `--ci`.** The non-terminal output is already right for CI (plain lines, no color), and `--quiet` and `--json` exist. A `--ci` flag would bundle unrelated behaviors.
- **Why not detection alone.** Detection alone gives no way to turn annotations off, for example when a wrapper parses standard error. It also gives no way to test annotations outside Actions.
- **So:** `--annotations=auto|github|never`, modeled on `--color`. `auto` follows `GITHUB_ACTIONS`, and any other value exits `2`.
- **Presentation only, like progress.**
  - Annotations go to standard error, after the run and before the command exits. The runner reads workflow commands from both streams, so standard output stays byte-identical, and `--json` stays clean.
  - Exit codes, JSON, report files, and evidence files do not change.
  - `--quiet` and `--json` do not suppress annotations: CI may use either and still wants them inline. `--annotations=never` turns them off.
- **Content.** The annotations are rendered from the same `humanReport`, so they cannot disagree with the report:
  - one annotation per shown finding, and one per failed test;
  - `title` is the diagnostic code, or `Test failed`;
  - the message is the code's meaning, then the identity or evidence ID and its title;
  - a truncated group adds one location-free annotation, such as "118 more findings; run with `--details`".
  - GitHub shows at most 10 error and 10 warning annotations per step, and 50 per job. Mirroring the report's truncation keeps the log readable, and the most useful findings come first.
- **`stele check`** adds annotations for missing Verification-IDs (heading `file:line`) and for files without a valid annotation (file only). The annotations of `--all` scopes are written in scope order, and identical lines are written once.
- **Paths.** Stele's paths are relative to `--root`, but GitHub resolves paths from the repository checkout (`GITHUB_WORKSPACE`). When the root lies inside the workspace, the path is prefixed with the root's relative location. Otherwise the root-relative path is used.
- **Escaping**, as GitHub's toolkit does it:
  - in messages, `%` becomes `%25`, CR `%0D`, and LF `%0A`;
  - in properties, also `:` becomes `%3A` and `,` becomes `%2C`.
- **Determinism.** Annotations contain no durations, so they are byte-identical for identical inputs.
- **Placement.** A new `annotations.go` beside `report.go` renders from `humanReport` and the check step summaries. The command paths call it where they finish progress.

### 9. Test environment hygiene

This repository's own CI sets `GITHUB_ACTIONS=true`, and every test that starts the binary inherits it:

- `tests/cli.test.mts`;
- `tests/package_install_test.go`;
- in-process `Run` tests that use the real `lookupEnv`.

Two things would go wrong:

- assertions on standard error, such as the plain-progress checks, would change;
- fixtures that fail on purpose would print `::error` lines into the job log, and GitHub would show them as fake annotations on this repository's pull requests.

The test helpers therefore remove `GITHUB_ACTIONS` from the environment they pass, unless a test sets it explicitly. In-process tests stub `lookupEnv`. Stele's own scenario runner leaves the environment of the tests it runs unchanged, because consumers' tests may rely on it.

### 10. The user guide's workflow

`docs/guide/continuous-integration.md` offers this workflow to copy:

```yaml
name: Stele

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  stele:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
      # Go projects: Stele runs Go evidence tests with the Go toolchain.
      # - uses: actions/setup-go@v6
      #   with:
      #     go-version-file: go.mod
      #     cache: true
      - run: npm ci
      # Projects with browser e2e evidence, such as Playwright:
      # - uses: actions/cache@v4
      #   with:
      #     path: ~/.cache/ms-playwright
      #     key: playwright-${{ runner.os }}-${{ hashFiles('package-lock.json') }}
      # - run: npx playwright install --with-deps chromium
      - name: Check specifications, plans, anchors, and evidence
        run: npx stele check --all
      - name: Keep the verification report and evidence
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v4
        with:
          name: stele-report
          path: |
            artifacts/verification-report.json
            artifacts/test-results.json
```

The page explains:

- **Exit codes:** `0` passed; `1` a check failed, such as a finding, a failed test, or an unapproved entry; `2` the command could not run, such as a bad option, an empty scope, or OpenSpec failing to start.
- **Output:** without a terminal, output is plain and uncolored, and `NO_COLOR` is respected. Progress goes to standard error. `--quiet` prints only the verdict line. `--json` is the machine interface, and nobody should parse the human report. Annotations appear inline in GitHub Actions, with `--annotations=never` to opt out.
- **Pinning:** `stele-spec` is pinned in `package-lock.json`, so `npx stele` runs the locked version.
- **The merge rule:** `--all` gates every active change at the implementation stage (Decision 7).
- **Performance:** the planned `fast-runs` change will add parallel execution and `execution.groups` in `stele.config.json`. The page links to the CLI reference for it once released. Until then it says only that the page will cover it.

Getting started links to the guide where it mentions CI. The CLI reference links to it from `stele check` and from "Determinism in CI". The sidebar lists the guide after "Build a verified change".

## Risks / Trade-offs

- **Self-grading in the second gate.** A defect in the build under review can hide its own findings. Mitigation: `verify:self` (Decision 1) and review. The window closes at the next release.
- **The merge rule blocks plan-only merges** (Decision 7). This is a workflow change for the maintainer.
- **Longer Linux job**, 1–2 minutes until `fast-runs` lands.
- **Duplicate annotations later.** Once `stele-published` contains this feature, `verify:self` also annotates the current specifications, and both gates would annotate the same findings. This only happens when both fail on the current specifications. Accept it, or pass `--annotations=never` to `verify:self` then (open question).
- **The annotation limit.** GitHub shows only the first 10 errors per step inline. The full list stays in the log and the report.
- **The line-based `ci.yml` test** breaks when the workflow is restructured. It fails loudly, which is the point.

## Verification strategy

**Status: proposed, awaiting approval.** No entry is approved. `stele approve` is the maintainer's action.

Placement follows AGENTS.md: co-located Go tests in `internal/stele`, CLI contract tests in `tests/cli.test.mts`, and packed-package tests in `tests/package_install_test.go`. Placement is advisory. Unit tests call the annotation renderer or `Run` in process, with `lookupEnv` stubbed.

No new e2e entry is planned for the annotation scenarios. The renderer and the flag run the same code in process as in the binary, and the environment is read through the `lookupEnv` seam, whose production value is `os.LookupEnv`. The one risk only the shipped binary shows is annotations from the installed package in a real GitHub-like environment, with real streams, bundled OpenSpec, and no terminal. The existing e2e `scn.validate.ddff25a243df` covers it, now reworded for GitHub Actions. The workflow file and the guide are not in this table (Decision 6).

| Scenario | Level | Evidence ID | Advisory placement (reason) | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|
| `scn.terminalreport.d9386a6ce84d` Annotate a finding at its location | unit | `scn.terminalreport.d9386a6ce84d.unit` | `internal/stele/cli_test.go`, beside the validate command tests (runs `Run` twice with a stubbed environment) | Risk: the annotation lacks or misplaces `file` or `line`, or turning annotations on changes standard output or the exit code. Running the command in process twice, with a stubbed `lookupEnv`, compares both runs exactly. |
| `scn.terminalreport.447957e12de7` Annotate a failed test | unit | `scn.terminalreport.447957e12de7.unit` | `internal/stele/annotations_test.go`, beside the new `annotations.go` | Risk: failed tests are left out, since they are not diagnostics, or annotated without their location or evidence ID. Rendering from a built report with one failed execution is pure. |
| `scn.terminalreport.e045d1b2d55c` Annotate only in GitHub Actions unless asked | unit | `scn.terminalreport.e045d1b2d55c.unit` | `internal/stele/cli_test.go`, beside the `--color` and `NO_COLOR` tests | Risk: annotations leak into local terminals and other CI systems, or `never` does not win over the environment. Mode resolution is option parsing plus the environment seam, in process. |
| `scn.terminalreport.50fd0b8c9244` Reject an unknown annotation mode | unit | `scn.terminalreport.50fd0b8c9244.unit` | `internal/stele/cli_test.go`, beside the unknown color mode test | Risk: a typo is accepted and annotations silently stay off. Option parsing in process. |
| `scn.terminalreport.be5c922ef009` Follow the truncation of the report | unit | `scn.terminalreport.be5c922ef009.unit` | `internal/stele/annotations_test.go` | Risk: 123 annotations flood the log past GitHub's limit, or the remainder count is off by one, or `--details` is ignored. Pure over 123 synthetic findings, as the report's own truncation test does. |
| `scn.terminalreport.fbadf694cd04` Locate files from the workspace root | unit | `scn.terminalreport.fbadf694cd04.unit` | `internal/stele/annotations_test.go` | Risk: with `--root`, annotations point at wrong or missing files on the diff. Path mapping with temporary directories is in process. |
| `scn.terminalreport.f466c27970e9` Escape workflow command values | unit | `scn.terminalreport.f466c27970e9.unit` | `internal/stele/annotations_test.go` | Risk: a message with `%` or a line break splits into a broken command, or a comma cuts a property. The encoder is pure. |
| `scn.validate.ed99aac59b49` Run every step and report each one | unit | `scn.validate.ed99aac59b49.unit` | `internal/stele/check_test.go` (existing test) | Unchanged scenario in the modified requirement. An early failure could skip later steps, or the summary misreport them. Run in process over a fixture change. |
| `scn.validate.d768594936b1` Exit with the worst code | unit | `scn.validate.d768594936b1.unit` | `internal/stele/check_test.go` (existing test) | Unchanged scenario. Exit code `1` could mask a tool failure. A stubbed OpenSpec failure in process shows the combined code. |
| `scn.validate.704b1662c309` Check every scope | unit | `scn.validate.704b1662c309.unit` | `internal/stele/check_test.go` (existing test) | Unchanged scenario. A step could skip the current specifications or a change, which is what this repository's second gate relies on. A fixture with specifications and two changes runs in process. |
| `scn.validate.ddff25a243df` Gate CI with the installed command | e2e | `scn.validate.ddff25a243df.e2e` | `tests/package_install_test.go`, in the consumer project (existing test, now with `GITHUB_ACTIONS=true`) | Distinct risk: CI runs the installed binary without a terminal, where the exit code, both streams, annotation detection from the real process environment, and bundled OpenSpec decide together. Only the packed package shows this. |
| `scn.validate.0c6103aa40e7` Annotate a missing Verification-ID in GitHub Actions | unit | `scn.validate.0c6103aa40e7.unit` | `internal/stele/check_test.go`, beside the ID-step test | Risk: `check`'s own steps, which are not validation diagnostics, are left out of annotations, or the heading line is wrong. Run in process over a fixture change with a stubbed environment. |
