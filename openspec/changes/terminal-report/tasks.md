## 0. Approval and baseline

- [ ] 0.1 Get the reviewer's explicit approval of the verification levels in design.md (proposed, awaiting approval), the decisions, and the open questions; only then run `stele approve --change terminal-report --confirmed-in-chat`, mark the design table approved, and verify `stele verify --stage proposal --change terminal-report` reports no `PLAN_UNAPPROVED`. Start no task in sections 1-8 before this.
- [ ] 0.2 Confirm `chore/self-verify-rc3` and `feat/spec-annotation` are merged on main; verify with `git log origin/main` and that `stele annotate --check` exists in a local build
- [ ] 0.3 Rebase this branch on main, re-run `stele ids --change terminal-report` and `npx openspec validate --all --strict`, and verify the init and verification-scope deltas still copy the current requirement text exactly (re-approve any entry that `stele verify --stage proposal` reports as `PLAN_APPROVAL_STALE`)
- [ ] 0.4 Resolve the open question on fence-aware annotations with the owner of `spec-annotation`; record the outcome in design.md

## 1. Version

- [ ] 1.1 Make `Version` a variable defaulting to `0.0.0-dev` in `internal/stele/version.go` and pass `-X …/internal/stele.Version=<package.json version>` from `scripts/build-go.mts`; verify `./dist/stele --version` prints `0.1.0-rc.4` after the version bump and the unit test `TestVersionIsUsedEverywhere` passes
- [ ] 1.2 Assert the installed package's `stele --version` equals its `package.json` version in `tests/package_install_test.go`; verify `npm run test:package` passes

## 2. Diagnostic catalogue and report model

- [ ] 2.1 Add `diagnostics.go` with stage, meaning, and fix template for every code, including `SPEC_ANNOTATION_*` and `PLAN_ARCHIVE_ORDER_AMBIGUOUS`; verify the source-scanning catalogue test passes
- [ ] 2.2 Build the `humanReport` model (header, capability overview, stage check lines, grouped findings with titles and locations, failed and stale tests, verdict) sorted at build time; verify the report unit tests for stage naming, overview, grouping and truncation, `--details`, failed and stale tests, and shuffled-input determinism pass
- [ ] 2.3 Add color and style resolution (`--color`, `NO_COLOR`, `TERM=dumb`, per stream) with injected environment and terminal; verify the NO_COLOR and invalid-mode tests pass

## 3. Commands use the report

- [ ] 3.1 Render `validate`, `verify`, and `test` through the report, show warnings in `validate`, and add `--details`, `--quiet`, and `--color` to their option parsing; verify the in-process command tests (warnings, quiet, JSON unchanged) pass and existing `cli_test.go` expectations are updated only for human output
- [ ] 3.2 Run `validate` stages in the order OpenSpec, pre-check verification, tests, final verification, with identical files and exit codes; verify existing validate tests and `npm run test:node` pass unchanged for JSON
- [ ] 3.3 Add the per-scope summary to `--all`; verify the three-scope summary test passes

## 4. Progress

- [ ] 4.1 Add the runner observer and the `now`, `isTerminal`, and `lookupEnv` seams without changing evidence; verify runner tests pass unchanged
- [ ] 4.2 Implement terminal progress (in-place line, immediate failures, cleared line, width limit) and plain progress (stage lines, one line per test file, failure lines, scope prefix under `--all`); verify the progress unit tests pass
- [ ] 4.3 Add the CLI tests for plain progress through pipes, for a pseudo-terminal through `script` (skipped with a message when unavailable), and for JSON determinism across two runs; verify `npm run test:node` passes on macOS and in CI on Linux

## 5. Archived plan ordering

- [ ] 5.1 Combine archived plans by current-text match, then date prefix and name, and report `PLAN_ARCHIVE_ORDER_AMBIGUOUS` for undecidable same-day conflicts; verify the new and existing `scope_test.go` ordering tests pass and `stele verify --specs` on this repository reports no new errors

## 6. `stele check`

- [ ] 6.1 Add `stele check [--change | --specs | --all]` running the ID check (including current specifications), the annotation check, and validation, with every step run, a combined summary, worst exit code, and a JSON document; verify the `check_test.go` tests pass
- [ ] 6.2 Add the installed-package CI gate test; verify `npm run test:package` passes

## 7. Documentation and skills

- [ ] 7.1 Update `docs/reference/cli.md`: the report layout, check lines and stages, the diagnostic catalogue (a table generated from or checked against `diagnostics.go`), progress on standard error, `--details`, `--quiet`, `--color`, `NO_COLOR`, CI behavior, and `stele check`; verify `npm run docs:build` passes
- [ ] 7.2 Update `docs/guide/getting-started.md` with an example report and `stele check`, and the `stele-verify` skill template and `.agents/skills/stele-verify/SKILL.md` to recommend `stele check`; verify the init skill tests pass
- [ ] 7.3 Add the 0.1.0-rc.4 changelog entry (new human output, flags, progress, `stele check`, version fix, archive ordering and its warning, renamed init requirement and scenario); verify `npm run docs:build` passes
- [ ] 7.4 If fence-aware detection is not folded into `spec-annotation`, document that an annotation line inside a fenced code block is reported as misplaced in `docs/concepts/spec-format.md`; verify `npm run docs:build` passes

## 8. Gate

- [ ] 8.1 Add `@implements` and `@verifies` anchors for this change and run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- check --change terminal-report` passes with the local build
- [ ] 8.2 After a release containing this change: bump `stele-published`, switch `verify:self` to `stele check --specs`, archive this change, and verify `npm run verify:self` passes
