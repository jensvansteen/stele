## 0. Approval and baseline

- [x] 0.1 Get the maintainer's explicit approval of the verification levels in design.md (proposed, awaiting approval), the decisions (especially Decision 7, "changes merge complete"), and the open questions. Only then does the maintainer run `stele approve --change ci-gates` (an agent runs it only after an explicit yes in chat, with `--confirmed-in-chat`), and the design table is marked approved. Verify that `stele verify --stage proposal --change ci-gates` reports no `PLAN_UNAPPROVED`. Start no task in sections 1-6 before this.
- [x] 0.2 Rebase on `main`, run `stele ids --change ci-gates`, `npx openspec validate --all --strict`, and `stele verify --stage proposal --change ci-gates`, and check that the `validate` delta still copies the current text of "Check a project with one command". Any entry that the proposal check reports as `PLAN_APPROVAL_STALE` is re-approved by the maintainer.

## 1. Annotation mode

- [x] 1.1 Add `--annotations=auto|github|never` to `validate`, `verify`, `test`, and `check`, resolved through the `lookupEnv` seam (`GITHUB_ACTIONS=true`), and reject other values with exit code `2`. Verify that the mode and unknown-value tests in `cli_test.go` pass.

## 2. Annotation rendering

- [x] 2.1 Add `annotations.go`, which renders the shown findings, the truncation remainders, and the failed tests from `humanReport` as `::error` and `::warning` commands, with the code or `Test failed` as the title, escaped properties and messages, and no duplicate lines. Verify that the rendering, truncation, and escaping tests in `annotations_test.go` pass.
- [x] 2.2 Map paths from `--root` to `GITHUB_WORKSPACE` when the root lies inside it. Verify that the workspace path test passes.
- [x] 2.3 Write annotations to standard error after the run in `validate`, `verify`, `test`, and every `--all` scope, whatever `--quiet` and `--json` say. Verify that standard output, JSON, report and evidence files, and exit codes are unchanged, that the in-process comparison test passes, and that `npm run test:node` passes.
- [x] 2.4 Annotate the missing Verification-IDs and annotations of `check`'s ID and annotation steps, and pass the option through to validation. Verify that the `check_test.go` tests pass.

## 3. Test environment hygiene

- [x] 3.1 Remove `GITHUB_ACTIONS` from the environment of every binary started by `tests/cli.test.mts` and `tests/package_install_test.go`, unless a test sets it, and stub `lookupEnv` in in-process tests. Verify with `GITHUB_ACTIONS=true npm test`: every test passes, and the output contains no line starting with `::`.
- [x] 3.2 Extend the consumer CI gate test (`scn.validate.ddff25a243df`) to run with `GITHUB_ACTIONS=true` and expect `::error` lines titled `PLAN_UNAPPROVED`, with no ANSI escapes and exit code `1`. Verify that `npm run test:package` passes.

## 4. Documentation

- [x] 4.1 Add `docs/guide/continuous-integration.md` with the workflow from design.md Decision 10, and the notes on exit codes, the non-terminal output, `NO_COLOR`, `--quiet`, `--json`, annotations, pinning, browser e2e, and the merge rule, without `execution.groups` (open question 4). Add it to the sidebar after "Build a verified change". Verify that `npm run docs:build` passes.
- [x] 4.2 Link the guide from `docs/guide/getting-started.md` and from `stele check` and "Determinism in CI" in `docs/reference/cli.md`. Add `--annotations` to the output options table and the `OUTPUT` synopsis. Verify that `npm run docs:build` passes, with no dead links.
- [x] 4.3 Add a changelog entry for annotations and the CI guide. Verify that `npm run docs:build` passes.

## 5. This repository's CI

- [x] 5.1 Add the step from design.md Decision 5 to `.github/workflows/ci.yml`, after `verify:self`, with `if: ${{ !cancelled() && matrix.os == 'ubuntu-latest' }}`, running `npm run stele -- check --all`. Verify that `npm run stele -- check --all` passes locally on this branch once sections 1-4 are done.
- [x] 5.2 Add a repository test that reads `.github/workflows/ci.yml` and `package.json` and asserts both gates as Decision 6 lists them. Add another that asserts the guide's workflow block runs `npx stele check --all` with `node-version: 24`. Both are ordinary tests, with no `@verifies`. Verify that each test fails when its step is removed, then passes again when it is restored.
- [x] 5.3 Update the CONTRIBUTING self-verification section: the two gates and why both exist, step 2 "merge a change complete" (Decision 7), and `npm run stele -- check --all` as the local equivalent of the second gate. Verify that `npm run docs:build` passes and that the text matches ci.yml.

## 6. Gate

- [x] 6.1 Add `@implements` and `@verifies` anchors for this change and run `npm run verify`. Verify that it passes with exactly 100% core coverage, and that `npm run stele -- check --all` passes with the local build.
- [ ] 6.2 Open the implementing pull request. Verify in its CI log that the new step ran `check --all` over the current specifications and `ci-gates` and passed, that `verify:self` also passed, and that no fake `::error` annotations from test fixtures appear on the pull request.
- [ ] 6.3 After a release containing this change: bump `stele-published`, pass `--annotations=never` to `verify:self` (open question 3, decided), archive this change with the `stele-archive` skill, and verify that `npm run verify:self` passes.
