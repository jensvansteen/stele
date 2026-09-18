# CLI reference

The npm package exposes the compiled Go executable directly as the `stele` command. Node remains part of the consumer toolchain because npm installs the package, OpenSpec runs on Node, and Stele executes exact named `.ts` and `.mts` tests through Node. `.tsx` declarations can carry anchors, but TSX test execution needs a future configurable runner adapter. Go consumers also need the Go toolchain: Stele resolves Go anchors itself and runs exact Go tests through `go test`.

## Global commands

```text
stele init [--change ID] [--tools TOOLS] [--refresh-schema] [--strict-versions] [--root PATH]
stele ids [--change ID] [--check] [--root PATH] [--json]
stele annotate [--change ID | --specs | --all] [--check] [--root PATH] [--json]
stele check [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--evidence-file PATH] [--json] [--strict-versions] [OUTPUT]
stele verify [--stage proposal|implementation] [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--json] [--strict-versions] [OUTPUT]
stele test [targets...] [--change ID | --specs | --all] [--root PATH] [--evidence-file PATH] [--json] [OUTPUT]
stele validate [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--evidence-file PATH] [--json] [--strict-versions] [OUTPUT]
stele index [--change ID | --specs | --all] [--root PATH] [--output-file PATH] [--json]
stele approve [--change ID | --specs] [--evidence ID]... [--scenario ID]... [--all --yes | --confirmed-in-chat] [--by NAME] [--root PATH]
stele plan migrate [--change ID | --specs] [--root PATH]
stele help
stele version
```

`OUTPUT` is `[--details] [--quiet] [--color auto|always|never] [--annotations auto|github|never]`; see [Terminal output](#terminal-output). `stele --version` prints the version of the npm package that contains the binary, such as `0.1.0-rc.4`; a binary built outside the package build prints `0.0.0-dev`.

## `stele init`

Prepares a project for Stele. Existing files are preserved.

1. When the project has no `openspec/` directory, runs the bundled `openspec init` for the tools in `--tools` and, for the default `agents` target, links `.claude/skills` to `.agents/skills` unless `.claude/skills` exists.
2. Forks OpenSpec's `spec-driven` schema into `openspec/schemas/stele`, unless that schema exists. The fork adds a `verification` artifact that generates `linkage-plan.json`, requires it before `tasks` and apply, and extends the apply instruction.
3. Selects `stele` as the default schema when `openspec/config.yaml` selects `spec-driven`, and merges Stele guidance into `operations.apply.guidance`, `operations.archive.guidance`, and `rules.verification`. Existing values and comments are kept; the file is not rewritten when nothing is missing. Existing changes keep their schema.
4. Writes `stele.config.json`, installs the `stele-propose`, `stele-apply`, `stele-archive`, `stele-plan`, and `stele-verify` skills in `.agents/skills/`, and ensures `artifacts/` exists.
5. Adds the [Stele annotation](/concepts/spec-format) to every current specification and every delta spec of an active change that lacks one, following the rules of [`stele annotate`](#stele-annotate), and lists each annotated file. Archived changes are not touched. A file with a misplaced, malformed, or unsupported annotation is left unchanged with a warning; it does not fail `init`.
6. Prints the default workflow and the Stele commands to run around OpenSpec's own skills.

When `init` forks or refreshes the `stele` schema, the schema's specification template starts with the annotation, so new delta specs start annotated.

| Option | Meaning |
|---|---|
| `--change ID` | Default OpenSpec change written to new configuration; optional |
| `--tools TOOLS` | Tools passed to `openspec init` when OpenSpec is missing; defaults to `agents` |
| `--refresh-schema` | Fork and patch the `stele` schema again, for example after an OpenSpec upgrade |
| `--strict-versions` | Fail with exit code `1` on [OpenSpec version drift](/reference/versions#drift-warnings) instead of warning |
| `--root PATH` | Consumer project root; defaults to the current directory |

## `stele ids`

Inserts a `Verification-ID` line below every requirement and scenario heading of the change's delta specs that has none. Existing IDs and every other byte, including line endings and a missing final newline, are preserved. Headings in `REMOVED` and `RENAMED` sections are skipped, and `MODIFIED` headings reuse their ID from `openspec/specs/<capability>/spec.md`.

| Option | Meaning |
|---|---|
| `--change ID` | Override the configured change |
| `--check` | Write nothing; exit with code `1` while an ID is missing |
| `--json` | Print the inserted or missing IDs with their kind, title, file, and line, and each delta spec's annotation in `annotations` |

`stele ids` also adds the annotation to every delta spec that lacks one, in the same write, and reports ID line numbers as they are in the written file. A delta spec with a malformed or unsupported annotation is left unchanged, is reported, and makes the command exit with code `1`, because a newer format may have other ID rules. With `--check`, a missing annotation is listed next to missing IDs and fails the check only when `unannotatedSpecs` is `error`. Each `annotations` entry has `path`, `state`, `version`, and `changed`, as in [`stele annotate`](#stele-annotate).

## `stele annotate`

Adds `<!-- stele: spec v1 -->` as the first line of every specification file in the scope that lacks a valid [annotation](/concepts/spec-format). Without a scope option it uses the configured change, and exits with code `2` when there is none. `--all` covers the current specifications and the delta specs of every active change, never archived changes.

- The annotation goes at byte 0, or after a byte order mark. It ends with the file's first line ending, or with a line feed when the file has none. Every other byte is preserved.
- A file that already starts with a valid annotation is not changed, so a second run changes nothing.
- A file with a misplaced, malformed, or unsupported annotation is never edited. It is named in the output and the command exits with code `1`, but the other files are still annotated.
- Every file of the scope is read before any file is written.

| Option | Meaning |
|---|---|
| `--change ID`, `--specs`, `--all` | The scope to annotate |
| `--check` | Write nothing; exit with code `1` while any file lacks a valid annotation |
| `--json` | Print the files of the scope with their state |

```json
{
  "schemaVersion": 1,
  "mode": "check",
  "verdict": "fail",
  "files": [
    { "scope": "specs", "path": "openspec/specs/todo/spec.md", "state": "missing", "version": null, "changed": false }
  ]
}
```

`state` is the state Stele found: `annotated`, `missing`, `misplaced`, `malformed`, or `unsupported`. `version` is the declared version of an annotated or unsupported file, and `changed` is `true` for a file the command annotated. Files follow scope order, then path.

## `stele verify`

Parses the selected OpenSpec change and validates identities, the linkage plan, and anchors. It reads the stored evidence in `artifacts/test-results.json` to report execution.

| Option | Meaning |
|---|---|
| `--stage proposal` | Allow planned files and declarations that do not exist yet |
| `--stage implementation` | Require real anchors that match planned declarations; default |
| `--change ID` | Override the configured OpenSpec change |
| `--specs` | Verify the current specifications in `openspec/specs/` instead of a change |
| `--all` | Verify every scope; see [Every scope](#every-scope) |
| `--report-file PATH` | Write the deterministic verification report to this path |
| `--json` | Print canonical machine-readable JSON to standard output |

The exit code follows the linkage verdict, so a proposal gate or a verify-only CI step keeps its meaning. The [human report](#terminal-output) shows the specification, plan approval, and linkage check lines; in the implementation stage it adds a test execution line from the stored evidence, lists failed and stale tests, and names the overall verdict on the final line:

```text
  ✓ Linkage (anchors)           3/3 requirements and 7/7 scenarios linked
  ✗ Test execution              6/7 passed · 1 failed · unit 6 · e2e 1

Failed tests
  ✗ tests/todo.test.mts:14  rejects an empty title
      scn.todo.20d9cd2785a4.unit · Non-empty title · test-process-failed

✓ PASSED  change todo-basics · overall fail (test execution failed)
```

### Report verdicts

Reports have schema version `2.1` and keep three verdicts apart in `verdicts`:

| Field | Values | Meaning |
|---|---|---|
| `linkage` | `pass`, `fail` | Diagnostics: identities, plan, and anchors |
| `execution` | `passed`, `failed`, `stale`, `not-run` | The current evidence of the scope's scenarios |
| `overall` | `pass`, `fail`, `incomplete` | In the proposal stage, `linkage`. Otherwise `fail` when linkage failed or execution `failed`, `incomplete` when execution is `not-run` or `stale`, and `pass` only when both pass |

The top-level `verdict` equals `verdicts.overall`. In reports before schema version 2.1 it meant linkage only; read `verdicts.linkage` for that meaning.

## `stele test`

Finds scenario test anchors, selects every named test independently, and records execution evidence.

```bash
npx stele test --change todo-basics                          # every test of the change
npx stele test scn.todo.20d9cd2785a4                          # one scenario
npx stele test scn.todo.20d9cd2785a4.e2e                      # one evidence entry
npx stele test req.todo.22b616c90f42                          # every scenario of a requirement
npx stele test openspec/changes/todo-basics/specs/todo/spec.md  # every scenario in a file
```

Positional targets select what runs, Jest-style:

- An argument that starts with `req.` or `scn.` is an ID: a requirement selects the tests of all its scenarios, a scenario its tests, and an evidence ID (`<scenario>.<level>[.<n>]`) the tests of that entry.
- Any other argument is a `spec.md` file under `openspec/`, resolved against the repository root, and selects every scenario the scope parsed from it.
- Targets combine as a union, and each test runs once, even when several targets or scenarios share it.
- An ID the scope does not declare, or a file that does not exist or is not part of the scope, exits with code `2` before any test runs, naming the target.

With targets, Stele merges the outcomes into the stored evidence, `artifacts/test-results.json` unless `--evidence-file` names another file. Only executions of the tests that ran are replaced; other outcomes are kept. Each scenario's outcome is recomputed over all its tests, so a scenario whose other tests never ran is `not-run`, and one whose tests ran with older inputs is `stale`. The exit code follows the tests that ran: `0` when all passed, otherwise `1`. Without targets the whole scope runs as before, and evidence is written only with `--evidence-file`.

Only exact test names are selectable. A test whose name is a template literal with an interpolation, a `test.each` table, or a `describe` block leaves its anchor without a selector; the test is reported as not selectable (`target-not-resolved`), never run by a partial name.

| Option | Meaning |
|---|---|
| `--change ID` | Override the configured change |
| `--specs` | Run the tests linked to the current specifications |
| `--all` | Run the tests of every scope |
| `--evidence-file PATH` | Write test evidence to this path |
| `--json` | Print the scope's evidence document to standard output |

### Evidence schema 3

Evidence files have schema version `3`. Every execution records the `inputDigest` of the run that produced it, next to its `path`, `selector`, `scenarioIds`, `evidenceIds`, `outcome`, and `reason`, so an execution is stale when its digest differs from the current one. The digest covers the whole repository, so any input change marks every outcome stale. Readers of `outcome` and `scenarios` are unaffected; schema 2 files are still read, with the file's digest for every execution.

## `stele validate`

Runs OpenSpec strict validation, scenario tests, and implementation verification as one gate. Default outputs are `artifacts/test-results.json` and `artifacts/verification-report.json`; `--evidence-file` and `--report-file` choose others. The implementation verification uses the evidence of this run. Its JSON output has the same `verdicts` as the report.

The fast stages run first: OpenSpec strict validation, then a check of the specifications, plan approval, and linkage without evidence, whose result appears in the progress output, so an unapproved plan shows before the tests start. The tests run next, and the final verification uses their evidence. The files written and the exit code do not depend on this order. The report lists warnings as well as errors.

## `stele check`

One CI gate for a scope. It runs three steps and always runs all of them, even when an earlier one fails:

1. **Verification-IDs**, as `stele ids --check` does, for every selected scope, including the current specifications.
2. **Annotations**, as `stele annotate --check` does.
3. **Validation**, as `stele validate` does, with `--all`, `--report-file`, `--evidence-file`, `--strict-versions`, and the output options passed through.

```bash
npx stele check --change todo-basics   # while a change is in progress
npx stele check --specs                # after archiving
npx stele check --all                  # CI: current specifications and every active change
```

The output starts with one line per step, lists missing IDs and annotations below their step, prints the validation report, and ends with one verdict line:

```text
stele 0.1.0-rc.4 · check · change todo-basics

  ✗ Verification-IDs    1 heading without a Verification-ID; run `stele ids`
      openspec/changes/todo-basics/specs/todo/spec.md:18 scenario "Trim whitespace"
  ✓ Annotations         1 file annotated
  ✓ Validation          passed
  …
✗ FAILED  check · change todo-basics — verification-ids
```

The exit code is the worst of the steps: `2` when a step could not run, else `1` when a step failed, else `0`. With `--json`, standard output is `{schemaVersion, verdict, exitCode, steps}`, where each step has `step` (`ids`, `annotate`, `validate`), `exitCode`, `result` (the step's own JSON: one `stele ids --json` document per scope, the `stele annotate --json` document, and the `stele validate --json` document or `--all` array), and `error` when it could not run.

With `--specs`, OpenSpec validates all specifications with `openspec validate --specs --strict`.

In GitHub Actions, `check` also annotates each missing Verification-ID at its heading and each specification file without a valid annotation; see [GitHub Actions annotations](#github-actions-annotations). [Continuous integration](/guide/continuous-integration) has a workflow that runs `stele check --all`.

## `stele index`

Prints a deterministic JSON link index for editors and review tools: every requirement and scenario with its text, structured steps, and source location; every code and test anchor with its status; the planned evidence with its approval state; and the last execution outcome of each piece of evidence, marked `stale` when inputs changed since. It writes to standard output, or with `--output-file PATH` to that file. `--json` is accepted for symmetry; the output is always JSON. Identical inputs give identical bytes, without timestamps.

With `--change`, the index also includes the current specifications, and every item is labelled with its scope. With `--specs`, only the current specifications are included, and with `--all`, the current specifications and every active change. See [Link index](/reference/link-index) for the document format.

## Every scope

`--all` on `verify`, `test`, `validate`, and `index` covers the current specifications and then every active change in `openspec/changes/` (archived changes excluded), in directory order. When there are no current specifications, that scope is skipped. `--all` cannot be combined with `--change`, `--specs`, or test targets; such a command exits with code `2` and checks nothing.

`verify`, `test`, and `validate` check each scope separately and print its report under a heading, then a summary with every scope's verdict and one final line that names the failing scopes. Progress lines name the scope being checked.

```text
== current specifications ==
stele 0.1.0-rc.4 · validate · current specifications
…
✓ PASSED  current specifications

== change todo-basics ==
stele 0.1.0-rc.4 · validate · change todo-basics
…
✗ FAILED  change todo-basics — linkage (anchors) (1 unimplemented requirement)

Scopes
  ✓ PASSED  current specifications
  ✗ FAILED  change todo-basics — linkage (anchors) (1 unimplemented requirement)
✗ FAILED  1 of 2 scopes failed: change todo-basics
```

The exit code is the worst of the scopes: `2` when a scope could not be checked, else `1` when a scope failed, else `0`. With `--json`, standard output is an array with one `{scope, kind, exitCode, result}` entry per scope. An output file receives every scope in one file: `--report-file` an array of reports, and `--evidence-file` one evidence document that merges the scopes. `stele index --all` prints one index with every scope.

## Output file flags

`--evidence-file PATH` (`test`, `validate`) and `--report-file PATH` (`verify`, `validate`) name the output files. The earlier `--evidence PATH` and `--report PATH` keep working until 0.2.0 and print `stele: warning: --report is deprecated and will be removed in 0.2.0; use --report-file` to standard error without changing the exit code.

## `stele approve`

Records approvals for the pending and stale evidence entries of a version 2 plan. Each approval stores the approver, the date, a digest of the entry and the scenario's whitespace-normalized wording, `via`, and the current revision when there is one.

| Option | Meaning |
|---|---|
| (none) | In an interactive terminal, show each entry (scenario text, level, rationale, advisory placement) and ask approve, reject, or skip. Approvals use `via: cli`. |
| `--all --yes` | Approve every selected entry without prompts, with `via: cli` |
| `--confirmed-in-chat` | Record a confirmation that a human gave in an agent conversation, with `via: agent-confirmed`. An agent may pass it only after an explicit yes. The output names each approved entry. |
| `--evidence ID`, `--scenario ID` | Limit the selection; repeat the option or separate IDs with commas |
| `--by NAME` | Approver name; defaults to `git config user.name` |
| `--specs` | Approve entries in the version 2 plans of archived changes |

Without a terminal, `--all --yes`, or `--confirmed-in-chat`, the command approves nothing and exits with code `2`. It also exits with code `2` when no approver name is known or the plan is still version 1. Rejected and skipped entries stay unapproved and are offered again next time.

## `stele plan migrate`

Converts a version 1 plan to version 2 in place. Each scenario gets one unapproved entry per level its test anchors already name, such as `.unit` or `.e2e.2`, and otherwise one unapproved `unit` entry whose rationale asks you to choose the level. Targets and requirement entries are dropped. With `--specs`, every version 1 plan of an archived change is converted.

## Linkage plan versions

A **version 2** plan records decisions a person approves, not locations:

```json
{
  "schemaVersion": 2,
  "changeId": "todo-basics",
  "scenarios": {
    "scn.todo.20d9cd2785a4": {
      "evidence": [
        {
          "id": "scn.todo.20d9cd2785a4.unit",
          "level": "unit",
          "rationale": "Pure logic; no I/O is needed.",
          "placement": "tests/todo.test.ts",
          "approval": {
            "approver": "Jens",
            "date": "2026-09-17",
            "digest": "sha256:…",
            "via": "agent-confirmed",
            "revision": "16f57cf…"
          }
        }
      ]
    }
  }
}
```

| Stage | Version 2 rules |
|---|---|
| Both | Every scenario lists at least one entry (`PLAN_EVIDENCE_MISSING`). Entries have a known level, an ID that matches their scenario and level, a rationale, and a unique ID (`PLAN_EVIDENCE_INVALID`). The plan lists only declared scenarios (`PLAN_UNKNOWN_ID`). Every entry is approved (`PLAN_UNAPPROVED`) and unchanged since approval (`PLAN_APPROVAL_STALE`). |
| Implementation | Every approved entry has a `@verifies <evidence-id>` anchor on a named test (`LINK_EVIDENCE_MISSING`); no test claims an unplanned evidence ID or a bare scenario ID (`ANCHOR_EVIDENCE_UNPLANNED`); every requirement has an `@implements` anchor (`LINK_CODE_MISSING`). Locations are never compared. |

A **version 1** plan maps each requirement and scenario to one `path#selector` target. It keeps its earlier rules until Stele 0.2.0 and adds a `PLAN_V1_DEPRECATED` warning, which does not change the verdict.

Reports list each version 2 scenario's planned `evidence` with its approval state, and links carry `evidenceId` and `level`. Test evidence lists the `evidenceIds` and `inputDigest` of each execution and the `failedEvidence` of a failed scenario.

## Scopes and linkage plans

Each run checks one scope: a change, selected with `--change` or the configured default, or the current specifications, selected with `--specs`. The two options cannot be combined. `--all` checks [every scope](#every-scope) separately.

| Scope | Specifications | Linkage plan |
|---|---|---|
| Change | `openspec/changes/<change>/specs/` | `openspec/changes/<change>/linkage-plan.json`, or `artifacts/linkage-plan.json` when the change has none |
| Current specifications | `openspec/specs/` | Every `linkage-plan.json` under `openspec/changes/archive/`, combined per ID; see below |

For the current specifications, each ID takes its entries from the archived change whose archived delta spec declares the ID with the same text as the current specification: in practice, the change that last modified it. When several archived changes qualify, or none does, the one archived last wins, ordered by the archive folder's date prefix and then by change name, whatever the plan's version. OpenSpec records only the date of an archive, so when two changes archived on the same date still compete with different entries, the one whose name sorts last wins and `PLAN_ARCHIVE_ORDER_AMBIGUOUS` warns, naming both plans.

A plan whose `changeId` names a different change fails with `PLAN_CHANGE_MISMATCH`. Anchors for IDs declared elsewhere under `openspec/`, in another change or in the current specifications, do not affect the selected scope. An anchor fails with `ANCHOR_DANGLING` only when no specification declares its ID.

### Removed requirements

A delta spec can remove a requirement under `## REMOVED Requirements`, as a `### Requirement:` header or as a bullet such as ``- `### Requirement: Export todos` ``. Neither form is an active requirement of the change, so it needs no Verification-ID and no scenarios. Stele checks that the removed behavior is actually gone:

- Each removed name is resolved against the current specification of the same capability, matching the name exactly after trimming, as OpenSpec does. A name that matches nothing fails with `SPEC_REMOVED_UNMATCHED`, because nothing would be checked.
- The removed IDs are the matched requirement's ID and its scenarios' IDs, except IDs the change declares again in another requirement: that behavior moved, as when a requirement is removed and added under a new name to rename a scenario.
- In the implementation stage, every `@implements` or `@verifies` anchor that still names a removed ID, including evidence IDs such as `<scenario>.unit.2`, fails with `LINK_REMOVED_BEHAVIOR_ANCHORED` at the anchor's location.
- In both stages, a plan entry that still names a removed ID fails with `PLAN_REMOVED_BEHAVIOR_PLANNED`, instead of `PLAN_UNKNOWN_ID`.

After the change is archived, `--specs` continues the check: an anchor whose ID only archived changes declare names behavior an archived change removed, and fails with `LINK_REMOVED_BEHAVIOR_ANCHORED`. `ANCHOR_DANGLING` would not catch it, because the archived change that once added the requirement still declares its ID. Requirements under `## RENAMED Requirements` keep their Verification-IDs and are not checked as removed.

A scope must contain specifications. When the selected change has no delta specs, or `--specs` finds none in `openspec/specs/`, `verify`, `test`, and `validate` stop with exit code `2` before running tests or OpenSpec. A repository without archived changes therefore verifies with `--change` until its first `openspec archive`.

## Terminal output

Without `--json`, `validate`, `verify`, `test`, and `check` print a report for people to standard output. Its layout, top to bottom:

- **Header**: the Stele version, the command, and the scope.
- **Overview**: one row per capability in specification order, with its scenario count, its test outcomes, its plan approval, and a status mark (`✓` passed, `✗` failed, `!` warnings, stale, or not run).
- **Check lines**: one per stage, each named for what it checks: OpenSpec strict validation (`validate`), Specifications (IDs and annotations), Plan approval, Linkage (anchors), and Test execution with passed and total tests, counts by level, and the duration. A problem fails only the line of its own stage, so an unapproved plan never reads as a linkage failure. `test` shows only the test execution line.
- **Errors, then Warnings**: grouped by diagnostic code, with the number of findings, the code's meaning, a `→ Fix:` line with the next step for this scope, and the first five findings with their ID, title, and `file:line`; `… N more (--details)` counts the rest.
- **Failed tests and Stale tests**: by name and location, with their evidence IDs and reason; a stale test names the `stele test` command that runs it again.
- **Verdict**: `✓ PASSED` or `✗ FAILED`, the scope, the failing stages, and the number of warnings.

```text
stele 0.1.0-rc.4 · validate · current specifications

  Capability  Scenarios  Tests     Plan          Status
  todo                3  3 passed  1/3 approved  ✗

  ✓ OpenSpec strict validation  passed
  ✓ Specifications              1 requirement, 3 scenarios
  ✗ Plan approval               1/3 evidence entries approved; 2 unapproved
  ✓ Linkage (anchors)           1/1 requirements and 3/3 scenarios linked
  ✓ Test execution              3/3 passed · unit 2 · e2e 1 · 1.8s

Errors
  PLAN_UNAPPROVED ×2  Planned evidence has no human approval yet.
    → Fix: review the levels in design.md, then run `stele approve --specs` (agents: after an explicit yes in chat, `stele approve --specs --confirmed-in-chat`)
    scn.todo.591a3b429cf0.e2e   Save entered text  openspec/specs/todo/spec.md:12
    scn.todo.7c2d91e0b4a5.unit  Reject empty text  openspec/specs/todo/spec.md:18

✗ FAILED  current specifications — plan approval (2 unapproved)
```

| Option | Meaning |
|---|---|
| `--details` | List every finding and every failed and stale test, without truncation |
| `--quiet` | Print only the verdict line, and no progress; errors that stop the command still go to standard error |
| `--color auto` | Default. Color a stream only when it is a terminal, `NO_COLOR` is unset or empty, and `TERM` is not `dumb` |
| `--color always`, `--color never` | Force color on or off, whatever `NO_COLOR` and the terminal say |
| `--annotations auto` | Default. Write GitHub Actions annotations to standard error only when `GITHUB_ACTIONS` is `true` |
| `--annotations github`, `--annotations never` | Force annotations on or off, whatever `GITHUB_ACTIONS` says |

Any other `--color` or `--annotations` value exits with code `2`. `--json` output is the same whatever these options say.

### Progress

While a command runs, progress goes to standard error, never to standard output:

- **On a terminal**, one line is updated in place: the stage, or during tests the test counter, the current scenario and evidence level, the passed and failed counts, and the elapsed time. A failed test is printed on its own line as soon as it fails, and the progress line is cleared before the report.
- **Otherwise**, as in CI, progress is plain append-only lines without escape sequences: one per stage, one per test file when its tests finish, and one per failed test.

```text
OpenSpec strict validation: passed (0.2s)
specifications: passed; plan approval: 2 unapproved; linkage (anchors): passed (0.0s)
test execution: 3 tests in 2 files
  ✓ tests/todo.test.mts  2/2 (0.8s) [2/3]
  ✓ tests/cli.test.mts  1/1 (0.9s) [3/3]
test execution: 3/3 passed (1.8s)
```

`TERM=dumb` gets plain lines. `--quiet` turns progress off.

### GitHub Actions annotations

When annotations are on, `validate`, `verify`, `test`, and `check` write one [workflow command](https://docs.github.com/actions/reference/workflows-and-actions/workflow-commands) per finding the report shows and per failed test to standard error, after the run:

```text
::error file=openspec/specs/todo/spec.md,line=12,title=PLAN_UNAPPROVED::Planned evidence has no human approval yet. scn.todo.591a3b429cf0.e2e · Save entered text
::error file=tests/todo.test.mts,line=20,title=Test failed::saves entered text · scn.todo.591a3b429cf0.e2e · Save entered text
::error title=PLAN_UNAPPROVED::118 more findings not shown; run with --details to list every one
```

- Errors and failed tests are `::error`, and warnings `::warning`. `title` is the diagnostic code, or `Test failed`. The message is the code's meaning, then the ID and title.
- A group the report truncates adds one annotation without a location that counts the rest; `--details` annotates every finding.
- `check` also annotates missing Verification-IDs (`ID_REQUIREMENT_MISSING`, `ID_SCENARIO_MISSING`) at their heading, and files without a valid annotation (`SPEC_ANNOTATION_*`).
- Paths are relative to `GITHUB_WORKSPACE` when the project root lies inside it, so `--root app` gives `file=app/src/todo.ts`, and relative to the root otherwise. Values are escaped as GitHub requires, and identical lines are written once.
- Standard output, JSON output, report and evidence files, and exit codes do not change. `--quiet` and `--json` do not turn annotations off.

### Determinism in CI

For identical inputs, the report is byte-identical apart from its durations, and annotations are byte-identical. JSON output, report files, and evidence files never contain durations or progress. Gate on the exit code and read JSON for automation; the human report may change between releases. See [Continuous integration](/guide/continuous-integration) for a GitHub Actions workflow.

### Diagnostics

Every code belongs to one stage, and the report prints its meaning and fix:

| Code | Stage | Meaning |
|---|---|---|
| `ID_REQUIREMENT_MISSING`, `ID_SCENARIO_MISSING` | Specifications | A requirement or scenario has no Verification-ID; run `stele ids` |
| `ID_FORMAT`, `ID_MULTIPLE`, `ID_DUPLICATE` | Specifications | A malformed, repeated, or duplicated Verification-ID |
| `SCENARIO_MISSING` | Specifications | A requirement has no scenarios |
| `SPEC_ANNOTATION_MISSING`, `SPEC_ANNOTATION_MISPLACED`, `SPEC_ANNOTATION_MALFORMED`, `SPEC_ANNOTATION_UNSUPPORTED`, `SPEC_ANNOTATION_FIELD_IGNORED` | Specifications | See [Specification format](/concepts/spec-format) |
| `SPEC_REMOVED_UNMATCHED` | Specifications | A removed requirement name matches no current requirement |
| `PLAN_UNAPPROVED`, `PLAN_APPROVAL_STALE` | Plan approval | An entry is not approved, or changed since approval; run `stele approve` |
| `PLAN_EVIDENCE_MISSING`, `PLAN_EVIDENCE_INVALID`, `PLAN_UNKNOWN_ID`, `PLAN_CHANGE_MISMATCH` | Plan approval | The plan misses, misstates, or does not belong to its scenarios |
| `PLAN_CODE_MISSING`, `PLAN_TEST_MISSING`, `PLAN_V1_DEPRECATED` | Plan approval | Version 1 plans; run `stele plan migrate` |
| `PLAN_ARCHIVE_ORDER_AMBIGUOUS` | Plan approval | Two same-day archives plan an ID differently (warning) |
| `PLAN_REMOVED_BEHAVIOR_PLANNED` | Plan approval | The plan still lists removed behavior |
| `LINK_CODE_MISSING`, `LINK_TEST_MISSING`, `LINK_EVIDENCE_MISSING`, `LINK_TARGET_MISMATCH` | Linkage | A requirement, scenario, or evidence entry has no matching anchor |
| `LINK_REMOVED_BEHAVIOR_ANCHORED` | Linkage | Code or a test is still anchored to removed behavior |
| `ANCHOR_DANGLING`, `ANCHOR_KIND`, `ANCHOR_TARGET_MISSING`, `ANCHOR_EVIDENCE_UNPLANNED` | Linkage | An anchor names an undeclared ID, has the wrong kind, is not above a declaration, or claims unplanned evidence |

## Common options

`--root PATH` sets the consumer repository root for all filesystem resolution. CLI options override `stele.config.json`; omitted values fall back to configuration.

`stele.config.json` selects the specification backend with `adapter`. `openspec` is the only supported adapter and the default when the field or file is missing; any other value makes every command exit with code `2`.

`unannotatedSpecs` sets the severity of `SPEC_ANNOTATION_MISSING` and `SPEC_ANNOTATION_MISPLACED`: `warn` or `error`. Without it, Stele 0.1.x warns; **from 0.2.0 the default is `error`**. Any other value makes every command exit with code `2`. See [Specification format](/concepts/spec-format#the-unannotatedspecs-policy).

`init`, `verify`, and `validate` warn on standard error when the project's OpenSpec skills or `stele` schema were generated by another OpenSpec version, and `init` also when the `openspec` on `PATH` reports another version. `--strict-versions` turns these warnings into errors with exit code `1`. See [Versions](/reference/versions).

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | All selected checks passed |
| `1` | A deterministic policy, linkage, OpenSpec, or selected-test check failed |
| `2` | The invocation was invalid or a required tool could not run |

Use the exit code for gates and JSON for explanation. Do not parse the human-readable summary.
