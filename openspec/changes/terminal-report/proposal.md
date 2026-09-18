## Why

`stele validate --all` tells a person that something failed, but not what, where, or what to do next:

```
== current specifications ==
✓ OpenSpec strict validation passed
✓ scenario execution passed: 114/114 passed
✗ implementation verification fail: 36 requirements, 114 scenarios, 123 errors
✗ deterministic validation failed
✗ 1 of 1 scopes failed: current specifications
```

- All 123 errors are `PLAN_UNAPPROVED`, yet the line says "implementation verification fail". Plan approval, linkage, and specification problems share one misleading line.
- `validate` prints no diagnostics and hides warnings such as `PLAN_V1_DEPRECATED`; only `verify` lists them, one raw line each.
- A full run is silent for about a minute while 114 tests run one by one.
- The rc.3 self-verification also found smaller defects: `stele --version` prints `0.1.0` instead of the package version, archived plans from the same day combine in alphabetical order instead of archive order, a living scenario title contradicts its text, and CI needs three commands (`ids --check`, `annotate --check`, `validate`) to gate a project.

## What Changes

- **A human-readable terminal report** for `validate`, `verify`, and `test`, and for each scope of `--all`:
  - a header with the Stele version, the command, and the scope;
  - a per-capability overview: scenarios, test outcomes, plan approval, and status;
  - one check line per stage, each named for what it checks: OpenSpec strict validation, specifications (IDs and annotations), plan approval, linkage (anchors), and test execution with counts by level and the duration;
  - problems grouped by diagnostic code, each with a count, a one-line meaning, a concrete `→ Fix:` step, and the first five items (ID, title, `file:line`), then `… N more (--details)`;
  - warnings in every command, including `validate`;
  - failed and stale tests by name and location;
  - a final one-line verdict.
- **Live progress on standard error.** The current stage; during tests a counter, the current scenario and level, pass and fail counts, and elapsed time, with failures shown immediately. A terminal gets one updating line; anything else gets plain, append-only lines without ANSI escapes. Standard output stays clean for `--json`.
- **Output flags.** `--details` removes truncation, `--quiet` prints only the verdict line and no progress, and `--color=auto|always|never` controls color; `auto` respects `NO_COLOR` and `TERM=dumb`. `--json` output is unchanged.
- **Determinism.** The report is byte-identical for identical inputs apart from durations, which come from an injected clock. JSON output and report and evidence files never contain timing or progress.
- **`stele check`**, one CI gate that runs the ID check, the annotation check, and `validate` for the selected scope, and prints one combined summary with one exit code (`2` over `1` over `0`). The `stele-verify` skill and the documentation recommend it, and `verify:self` switches to it once a published release contains it.
- **Fixes from the rc.3 self-verification:**
  - `stele --version`, help, and reports name the npm package version, set at build time from `package.json`.
  - Combining archived plans for `--specs` prefers the archived change whose specification text matches the current specification, and falls back to archive date and name, with a warning when that fallback decides between differing plans.
  - Stele stops reading requirements under `## REMOVED Requirements` as active requirements, in both OpenSpec forms; the header form reported `ID_REQUIREMENT_MISSING` and `SCENARIO_MISSING`.
  - The living scenario `scn.init.754fd262e114` gets a title that matches its text, "Initialize without a default change". OpenSpec cannot rename a scenario inside a MODIFIED requirement, so the requirement is removed and added again under a new name, keeping every Verification-ID.

- **Verify that removed behavior is gone.** For a change, each REMOVED requirement is resolved to its IDs in the current specification. Stele reports:
  - anchors that still name them (`LINK_REMOVED_BEHAVIOR_ANCHORED`);
  - plan entries that still list them (`PLAN_REMOVED_BEHAVIOR_PLANNED`);
  - removed names that match nothing (`SPEC_REMOVED_UNMATCHED`).

  After archiving, `--specs` verification reports anchors to identities that only archived changes declare.

Out of scope, by the user's decision: fence-aware annotation detection (a separate follow-up change) and lifecycle skills ending with `stele check` (a later change).

## Capabilities

### New Capabilities

- `terminal-report`: the human-readable report, live progress, output flags, determinism of human output, the per-scope summary of `--all`, and the version shown by the CLI.

### Modified Capabilities

- `validate`: adds `stele check`, the combined CI gate.
- `verification-scope`: combining archived plans uses the specification text, then archive date and name, instead of path order alone; `--specs` reports anchors to behavior that only archived changes declare.
- `verify`: identity parsing ignores REMOVED requirements, and a new requirement verifies that removed behavior is gone.
- `init`: the requirement "Initialize a consumer project" is replaced by "Initialize a project" so that scenario `scn.init.754fd262e114` can be renamed; its behavior and IDs are unchanged.

## Impact

- `internal/stele`: a report model and renderer separated from the commands, a diagnostic catalogue (stage, meaning, fix), a progress reporter with injected clock and terminal, a `check` command, the version variable, archive-aware plan combination, REMOVED-aware spec parsing, and removed-behavior checks.
- `scripts/build-go.mts` passes the package version to the Go linker.
- Human output changes completely. The CLI reference has always said not to parse it; JSON stays the machine interface.
- The documentation covers the CLI reference (output format, flags, CI behavior, `stele check`), Getting started, and the changelog, and the `stele-verify` skill template recommends `stele check`.
- It builds on `chore/self-verify-rc3` (archived specs, rc.3 self-verification) and `feat/spec-annotation` (`stele annotate`, `SPEC_ANNOTATION_*`), which land first; this change is rebased after them.
