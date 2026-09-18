# CLI reference

The npm package exposes the compiled Go executable directly as the `stele` command. Node remains part of the consumer toolchain because npm installs the package, OpenSpec runs on Node, and Stele executes exact named `.ts` and `.mts` tests through Node. `.tsx` declarations can carry anchors, but TSX test execution needs a future configurable runner adapter. Go consumers also need the Go toolchain: Stele resolves Go anchors itself and runs exact Go tests through `go test`.

## Global commands

```text
stele init [--change ID] [--tools TOOLS] [--refresh-schema] [--strict-versions] [--root PATH]
stele ids [--change ID] [--check] [--root PATH] [--json]
stele verify [--stage proposal|implementation] [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--json] [--strict-versions]
stele test [targets...] [--change ID | --specs | --all] [--root PATH] [--evidence-file PATH] [--json]
stele validate [--change ID | --specs | --all] [--root PATH] [--report-file PATH] [--evidence-file PATH] [--json] [--strict-versions]
stele index [--change ID | --specs | --all] [--root PATH] [--output-file PATH] [--json]
stele approve [--change ID | --specs] [--evidence ID]... [--scenario ID]... [--all --yes | --confirmed-in-chat] [--by NAME] [--root PATH]
stele plan migrate [--change ID | --specs] [--root PATH]
stele help
stele version
```

## `stele init`

Prepares a project for Stele. Existing files are preserved.

1. When the project has no `openspec/` directory, runs the bundled `openspec init` for the tools in `--tools` and, for the default `agents` target, links `.claude/skills` to `.agents/skills` unless `.claude/skills` exists.
2. Forks OpenSpec's `spec-driven` schema into `openspec/schemas/stele`, unless that schema exists. The fork adds a `verification` artifact that generates `linkage-plan.json`, requires it before `tasks` and apply, and extends the apply instruction.
3. Selects `stele` as the default schema when `openspec/config.yaml` selects `spec-driven`, and merges Stele guidance into `operations.apply.guidance`, `operations.archive.guidance`, and `rules.verification`. Existing values and comments are kept; the file is not rewritten when nothing is missing. Existing changes keep their schema.
4. Writes `stele.config.json`, installs the `stele-propose`, `stele-apply`, `stele-archive`, `stele-plan`, and `stele-verify` skills in `.agents/skills/`, and ensures `artifacts/` exists.
5. Prints the default workflow and the Stele commands to run around OpenSpec's own skills.

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
| `--json` | Print the inserted or missing IDs with their kind, title, file, and line |

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

The exit code follows the linkage verdict, so a proposal gate or a verify-only CI step keeps its meaning. In the implementation stage the human output adds an execution line, names failed scenarios, and names the overall verdict:

```text
✓ implementation verification pass: 3 requirements, 7 scenarios, 0 errors
✗ execution failed: 6/7 scenarios passed
  FAILED scn.todo.20d9cd2785a4 Non-empty title
✗ overall fail
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

Runs scenario tests, OpenSpec strict validation, and implementation verification as one gate. Default outputs are `artifacts/test-results.json` and `artifacts/verification-report.json`; `--evidence-file` and `--report-file` choose others. The implementation verification uses the evidence of this run. Its JSON output has the same `verdicts` as the report.

With `--specs`, OpenSpec validates all specifications with `openspec validate --specs --strict`.

## `stele index`

Prints a deterministic JSON link index for editors and review tools: every requirement and scenario with its text, structured steps, and source location; every code and test anchor with its status; the planned evidence with its approval state; and the last execution outcome of each piece of evidence, marked `stale` when inputs changed since. It writes to standard output, or with `--output-file PATH` to that file. `--json` is accepted for symmetry; the output is always JSON. Identical inputs give identical bytes, without timestamps.

With `--change`, the index also includes the current specifications, and every item is labelled with its scope. With `--specs`, only the current specifications are included, and with `--all`, the current specifications and every active change. See [Link index](/reference/link-index) for the document format.

## Every scope

`--all` on `verify`, `test`, `validate`, and `index` covers the current specifications and then every active change in `openspec/changes/` (archived changes excluded), in directory order. When there are no current specifications, that scope is skipped. `--all` cannot be combined with `--change`, `--specs`, or test targets; such a command exits with code `2` and checks nothing.

`verify`, `test`, and `validate` check each scope separately and print its result under a heading, then a summary that names the failing scopes:

```text
== current specifications ==
✓ implementation verification pass: 12 requirements, 30 scenarios, 0 errors
== change todo-basics ==
✗ implementation verification fail: 1 requirements, 2 scenarios, 1 errors
  ERROR LINK_CODE_MISSING: No code anchor resolves for req.todo.22b616c90f42.
✗ 1 of 2 scopes failed: change todo-basics
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
| Current specifications | `openspec/specs/` | Every `linkage-plan.json` under `openspec/changes/archive/`, combined in archive order; a later archive wins for the same ID, whatever its version |

A plan whose `changeId` names a different change fails with `PLAN_CHANGE_MISMATCH`. Anchors for IDs declared elsewhere under `openspec/`, in another change or in the current specifications, do not affect the selected scope. An anchor fails with `ANCHOR_DANGLING` only when no specification declares its ID.

A scope must contain specifications. When the selected change has no delta specs, or `--specs` finds none in `openspec/specs/`, `verify`, `test`, and `validate` stop with exit code `2` before running tests or OpenSpec. A repository without archived changes therefore verifies with `--change` until its first `openspec archive`.

## Common options

`--root PATH` sets the consumer repository root for all filesystem resolution. CLI options override `stele.config.json`; omitted values fall back to configuration.

`stele.config.json` selects the specification backend with `adapter`. `openspec` is the only supported adapter and the default when the field or file is missing; any other value makes every command exit with code `2`.

`init`, `verify`, and `validate` warn on standard error when the project's OpenSpec skills or `stele` schema were generated by another OpenSpec version, and `init` also when the `openspec` on `PATH` reports another version. `--strict-versions` turns these warnings into errors with exit code `1`. See [Versions](/reference/versions).

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | All selected checks passed |
| `1` | A deterministic policy, linkage, OpenSpec, or selected-test check failed |
| `2` | The invocation was invalid or a required tool could not run |

Use the exit code for gates and JSON for explanation. Do not parse the human-readable summary.
