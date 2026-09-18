## Why

Stele's CI runs `npm run verify:self`, which checks the current specifications in `openspec/specs/` with the published Stele (`stele-published`). The open changes of a pull request, under `openspec/changes/<change>/`, are never checked by Stele in CI:

- plan completeness and approval;
- anchors, and removed-behavior checks;
- Verification-IDs and annotations;
- planned evidence.

Their tests run through `npm test`, but nothing ties those tests to the approved plan. A pull request can therefore merge with an unapproved or incomplete plan. This happened locally: an approved plan was missing an e2e evidence entry, and only a manual `stele check` found it.

Stele users need the same gate, but the documentation gives no ready-made CI setup. It mentions CI only in passing: exit codes, `--json`, and the non-terminal output. When a check fails in GitHub Actions, the findings appear only in the job log, never on the pull request diff.

## What Changes

- **A second CI gate in this repository.** On Linux, for every pull request and every push to `main`, CI runs `npm run stele -- check --all`, which uses the Stele built from the same commit. It covers the current specifications and every active change, runs whatever paths changed, runs even when `verify:self` failed, and fails the job on any finding, including a `PLAN_UNAPPROVED` entry. `verify:self` stays unchanged as the gate that does not grade itself: the published Stele checks the archived behavior.
- **Merge a change complete.** Because the gate checks every active change at the implementation stage, a change merges into `main` approved, implemented, and passing. CONTRIBUTING drops "across one or more pull requests" and explains both gates.
- **GitHub Actions annotations.** `stele validate`, `verify`, `test`, and `check` get `--annotations=auto|github|never`. With `auto`, the default, Stele writes `::error` and `::warning` workflow commands to standard error when `GITHUB_ACTIONS` is `true`, so findings, failed tests, missing IDs, and missing annotations appear inline on the pull request diff. Annotations are presentation only, like progress: standard output, JSON, report and evidence files, and exit codes do not change.
- **A "Continuous integration" guide for Stele users**, `docs/guide/continuous-integration.md`:
  - a ready-to-copy GitHub Actions workflow: Node 24, `npm ci` with the npm cache, `npx stele check --all`, report upload;
  - notes on browsers for e2e evidence, such as Playwright's Chromium and its cache, and on the execution groups that the planned `fast-runs` change will add;
  - generic CI notes: exit codes `0`, `1`, and `2`, the non-terminal output, `NO_COLOR`, `--quiet`, `--json`, and annotations.

  Getting started, the CLI reference, and the sidebar link to it.
- **Documentation for contributors:** the CONTRIBUTING self-verification section describes both gates and how to run the second one locally.

Out of scope: the release workflow, the `fast-runs` change, a GitHub step summary (`GITHUB_STEP_SUMMARY`), and annotations for other CI systems.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `terminal-report`: adds GitHub Actions annotations and the `--annotations` option.
- `validate`: `stele check` passes `--annotations` through, annotates its ID and annotation steps, and the CI scenario for the installed command covers annotations.

`verification-scope` is unchanged: `--all` already covers the current specifications and every active change, and the gate uses it as it is. This repository's workflow and the documentation get no requirements (see design.md, Decision 6).

## Impact

- `internal/stele`: an annotation renderer over the existing report model, the `--annotations` option, workspace-relative paths, and annotation of the `check` ID and annotation steps.
- Tests that start the binary have to set or clear `GITHUB_ACTIONS` themselves. Otherwise, in GitHub Actions they inherit `true`, which changes their standard error and adds fake annotations from failing fixtures to this repository's pull requests.
- `.github/workflows/ci.yml` gets one step. A new repository test checks that both gates are declared.
- Documentation: a new guide page, links from Getting started, the CLI reference, and the sidebar, the CLI reference's option table and CI section, CONTRIBUTING, and the changelog.
- CI time: the Linux job runs the current specifications' tests twice, once per gate. That adds about 1–2 minutes until `fast-runs` lands, and about 15 s after it.
