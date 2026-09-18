## Context

See proposal.md. The information an editor needs is spread over several places:

- **Spec text.** Stele parses spec headings (`specs.go`) but keeps only IDs and titles, not the text below them.
- **Anchors.** `anchors.go` and `goanchors.go` produce anchors. After `verification-strategy`, they also carry evidence IDs and levels.
- **Evidence.** `artifacts/test-results.json` holds a single whole-run `inputDigest`.
- **Verdict.** `Report.Verdict` comes only from diagnostics (`diagnosticSummary`). Execution appears in `stages.execution`, and only `validate` combines the two.

This change builds on `verification-strategy` and is implemented after it.

## Goals / Non-Goals

**Goals:**

- One command gives an editor everything it needs to render, navigate, and run spec-to-code-to-test links.
- A single behavior can be tested without running the whole scope.
- A report never looks green while execution failed.

**Non-Goals:**

- A language server or file watcher. Editors call `stele index` when they need fresh data.
- Parameterized, generated, or nested test names. The first version resolves only exact literal names.
- Showing design.md placement suggestions. They stay prose.

## Decisions

### Index document

`stele index [--change <id> | --specs] --json` prints (the `--json` flag is accepted for symmetry, and output is always JSON):

```json
{
  "schemaVersion": 1,
  "scopes": [{ "id": "todo-basics", "kind": "change" }, { "id": "specs", "kind": "specs" }],
  "requirements": [
    {
      "id": "req.todo.1a2b3c4d5e6f",
      "scope": "todo-basics",
      "title": "Create a todo",
      "text": "The application SHALL …",
      "source": { "path": "openspec/changes/todo-basics/specs/todo/spec.md", "line": 3 },
      "scenarios": ["scn.todo.0a1b2c3d4e5f"],
      "implementations": [{ "path": "src/todo.mts", "line": 7, "selector": "addTodo" }]
    }
  ],
  "scenarios": [
    {
      "id": "scn.todo.0a1b2c3d4e5f",
      "requirement": "req.todo.1a2b3c4d5e6f",
      "title": "Add non-empty todo",
      "text": "- **WHEN** …\n- **THEN** …",
      "steps": [{ "keyword": "WHEN", "text": "…" }, { "keyword": "THEN", "text": "…" }],
      "source": { "path": "…", "line": 8 },
      "evidence": [
        {
          "id": "scn.todo.0a1b2c3d4e5f.unit",
          "level": "unit",
          "approval": "approved",
          "tests": [{ "path": "tests/todo.test.mts", "line": 5, "selector": "adds a normalized todo" }],
          "execution": { "state": "executed", "outcome": "passed" }
        }
      ]
    }
  ],
  "anchors": [
    { "id": "req.todo.1a2b3c4d5e6f", "annotation": "implements", "path": "src/todo.mts", "line": 7, "selector": "addTodo", "status": "linked" }
  ]
}
```

- **`approval`** is `approved`, `unapproved`, `stale`, or `v1` (targets only).
- **Anchor `status`** is `linked`, `unresolved` (no selector), `other-scope` (declared by another change or spec), or `undeclared`.
- **Scopes:** when a change is selected, the index also includes the current specifications. Every requirement, scenario, and anchor carries a `scope`: the change ID or `specs`. A modified requirement appears once per scope with the same ID, so an editor can show the current and proposed text side by side. With `--specs`, only `specs` is present.
- **Ordering:** the selected change comes first, then the current specifications. Within a scope, requirements and scenarios follow spec order, and anchors are sorted by path and line.
- **No timestamps.**
- **Spec text, like doc comments.** An editor shows requirements and scenarios the way it shows JSDoc on hover, so the index carries their text:
  - A requirement's `text` is the Markdown under its heading, without the `Verification-ID` line and without its scenarios.
  - A scenario's `text` is its raw Markdown below the heading, without the `Verification-ID` line. Its `steps` list one `{keyword, text}` per `- **KEYWORD** …` bullet (`GIVEN`, `WHEN`, `THEN`, `AND`, …). A bullet that continues on following lines is joined into one step. A bullet without a bold keyword becomes a step with an empty keyword, and a scenario without text has empty `text` and no steps.
  - Line endings are normalized and surrounding blank lines trimmed. The approval digest keeps its own whitespace-collapsing normalization of the scenario, so approvals are unaffected.
  - The parser keeps this text on `Requirement` and `Scenario`, so every consumer of parsed specs, including a later language server, gets it without re-reading files.

### Evidence records and staleness

The evidence file moves to schema 3. Each execution records its `evidenceId`, `level`, `path`, `selector`, `outcome`, `reason`, and the `inputDigest` of its run.

- **Stale:** an execution whose `inputDigest` differs from the current digest reports `state: "stale"`.
- **Granularity:** in this version the digest stays whole-repository, so any input change marks every outcome stale. A follow-up change can add a digest per scenario, over its spec text, plan entries, and anchored files, for quieter editor feedback.
- **Rejected alternative:** a digest per test file. It misses transitive changes and gives a false sense of freshness.

### Running selected behavior

- **Positional targets, Jest-style:** `stele test [targets...]`, combined with `--change`, `--specs`, or nothing. A target starting with `req.` or `scn.` is an identity; a requirement ID selects its scenarios' evidence tests, a scenario ID that scenario's, and `scn.x.y.<level>[.<n>]` one entry. Any other target is the path of a specification file under `openspec/`, which selects every scenario in that file. Targets combine, and each test runs once.
- **Rejected alternative:** the planned `--scenario`, `--requirement`, and `--evidence <id>` flags. `--evidence` already names the evidence output file, and positional targets read better for the editor's "run this" commands.
- **Invalid targets:** an ID the scope does not declare, or a file that does not exist, exits with code `2` before any test runs, naming the target.
- **Test selection:** the runner uses the same anchor selection, filtered to the selected evidence IDs.
- **File targets:** a path is resolved against the repository root, must end in `spec.md` under `openspec/`, and matches the scope's parsed source paths.

### Checking every scope with `--all`

`--all` runs the command once per scope: the current specifications, then each active change in `openspec/changes` (archived changes excluded), in directory order. Each scope prints its own result line, JSON output becomes an array of per-scope documents, and the exit code is the worst of the scopes: `2` if any scope could not be checked, else `1` if any failed, else `0`. `--all` cannot be combined with `--change`, `--specs`, or test targets. A project without current specifications skips that scope instead of failing, because a repository before its first archive has none.

### Output file flags

`--evidence-file PATH` (test, validate) and `--report-file PATH` (verify, validate) name the output files; `stele index --output-file PATH` writes the index instead of printing it. `--evidence` and `--report` keep working until 0.2.0 and print `stele: warning: --report is deprecated; use --report-file` without changing the exit code. With `--all`, an explicit output path receives the scopes' documents in one file.
- **Merging:** results are merged into the evidence file by evidence ID and test target, replacing only matching executions.
- **Aggregate outcome:** recomputed over all scenarios of the scope, so a partial run never turns unexecuted scenarios into `passed`.

### Exact names only

- **TypeScript:** the pattern stops accepting template literals that contain `${`, so such anchors stay unresolved. `test.each` and `describe` already stay unresolved.
- **Go:** subtests stay out of scope.
- **Reporting:** unresolved test anchors show in the index as `unresolved`, and in implementation verification as `ANCHOR_TARGET_MISSING`.

### Separate verdicts

The report schema becomes `"2.1"` and adds `verdicts`:

- **`linkage`** is `pass` or `fail`, from diagnostics.
- **`execution`** is `passed`, `failed`, `stale`, or `not-run`, from the current evidence.
- **`overall`** is decided in order:
  1. In the proposal stage it equals `linkage`.
  2. It is `fail` if linkage failed, or if execution `failed`.
  3. It is `incomplete` if execution is `not-run` or `stale`.
  4. Otherwise it is `pass`.

The top-level `verdict` is redefined as `overall`. This is a breaking change while Stele is in 0.x, and the changelog announces it. Consumers that need the old meaning read `verdicts.linkage`.

- **`stele verify` exit codes** follow `linkage`, so a proposal gate or a verify-only CI step is unchanged. Its human output adds an execution line and names the overall result.
- **`stele validate`** already requires both parts. Its JSON output gains the same `verdicts`.

**Rejected alternative:** keeping `verdict` as a linkage alias. The name would stay misleading for every new consumer, including the editor.

### Looking ahead: `stele lsp`

A future step adds `stele lsp`, a language server that reuses the index core in memory to serve hover cards (requirement text and scenario steps, like JSDoc), go-to-definition, CodeLens run buttons, and diagnostics, with a thin VS Code extension. Nothing here builds it, but the index builder (`BuildIndex`) stays a pure in-process function over parsed specs, anchors, plans, and evidence, with loading kept separate, so the server can call it without spawning a process.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-18, before implementation started (via: agent-confirmed, chat review). The review confirmed positional targets, `--all`, the file flags, the redefined `verdict`, and the index scopes, and added spec text for hover cards with its own unit-level scenario. The scenarios changed by these decisions are marked "new" or "reworded" below.

Placement follows this repository's AGENTS.md: co-located Go tests in `internal/stele`, and executable tests in `tests/cli.test.mts`. Placement is advisory. This change is verified with the published v1 verifier, so its linkage plan records the ★ row.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Target | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|---|
| `scn.linkindex.30ab15bc8293` Index a change | unit ★ | `….30ab15bc8293.unit` | `internal/stele/index_test.go`, beside the new `index.go` | `internal/stele/index_test.go#TestIndexListsLinksForAChange` | Risk: the index misses or mislabels links. It is built from fixtures in process. |
| | e2e | `….30ab15bc8293.e2e` | `tests/cli.test.mts` (executable contract) | `tests/cli.test.mts#prints a link index for editors` | Distinct risk: the VS Code extension depends on the flags and JSON shape of the shipped binary. |
| `scn.linkindex.2bec33059b75` Keep requirement text and scenario steps (new) | unit ★ | `….2bec33059b75.unit` | `internal/stele/specs_test.go`, beside the spec parser tests | `internal/stele/specs_test.go#TestParseSpecsKeepsRequirementTextAndScenarioSteps` | Risk: hover cards show the wrong text, lose multi-line steps, or break on missing or malformed text. Parsing is pure over fixture files. |
| `scn.linkindex.48b3c24773d9` Include the current specifications | unit ★ | `….48b3c24773d9.unit` | `index_test.go` | `internal/stele/index_test.go#TestIndexIncludesCurrentSpecifications` | Risk: editors show only the change and lose the living behavior, or mix up scopes. Built from fixtures with archived specs. |
| `scn.linkindex.c0e515c9575a` Emit identical bytes for identical inputs | e2e ★ | `….c0e515c9575a.e2e` | `tests/cli.test.mts`, next to the existing determinism test | `tests/cli.test.mts#emits an identical link index for identical inputs` | Risk: map order or environment differences between processes. Only separate processes expose them, as for reports. |
| `scn.linkindex.331076ad0227` Mark stale execution outcomes | unit ★ | `….331076ad0227.unit` | `index_test.go` | `internal/stele/index_test.go#TestIndexMarksStaleOutcomes` | Risk: editors show green for outdated results. Digest comparison is pure. |
| `scn.linkindex.560aeb5e3970` Flag anchors that no specification declares | unit ★ | `….560aeb5e3970.unit` | `index_test.go` | `internal/stele/index_test.go#TestIndexFlagsUndeclaredAnchors` | Risk: typos in anchors stay invisible while editing. Pure check. |
| `scn.linkindex.352d7120ec6c` Run the tests of one scenario (reworded: positional target) | unit ★ | `….352d7120ec6c.unit` | `internal/stele/runner_test.go`, beside test selection | `internal/stele/runner_test.go#TestRunSelectedScenarioMergesEvidence` | Risk: extra tests run, or other outcomes are lost. A recording stub for the runner and a real evidence file show both. |
| `scn.linkindex.1f063f3c5e49` Run the tests of one evidence entry (reworded) | unit ★ | `….1f063f3c5e49.unit` | `runner_test.go` | `internal/stele/runner_test.go#TestRunSelectedEvidenceOnly` | Risk: the editor's "run this test" also runs other levels, or overwrites their outcomes. A recording stub for the runner. |
| `scn.linkindex.49a524bcf0ac` Run the tests of one requirement (reworded) | unit ★ | `….49a524bcf0ac.unit` | `runner_test.go` | `internal/stele/runner_test.go#TestRunSelectedRequirementScenarios` | Risk: a requirement's scenarios are missed. Selection logic with a recording stub. |
| `scn.linkindex.e0b21aa95623` Run the tests of one specification file (new) | unit ★ | `….e0b21aa95623.unit` | `runner_test.go` | `internal/stele/runner_test.go#TestRunSelectedSpecFileScenarios` | Risk: a file target selects the wrong scope or silently nothing. Path resolution against parsed sources, with a recording stub. |
| `scn.linkindex.6265c70bed70` Combine several targets (new) | unit ★ | `….6265c70bed70.unit` | `runner_test.go` | `internal/stele/runner_test.go#TestRunSelectedTargetsCombine` | Risk: combined targets drop tests or run one twice. Selection is pure over anchors. |
| `scn.linkindex.5c91d0c168f0` Reject an unknown target (reworded: IDs and files) | unit ★ | `….5c91d0c168f0.unit` | `internal/stele/cli_test.go`, beside argument handling | `internal/stele/cli_test.go#TestTestRejectsUnknownTargets` | Risk: a typo runs everything or nothing silently. `Run` returns the exit code in process, with a recording stub proving no test ran. |
| `scn.linkindex.7c1d78728036` Verify every scope separately (new) | unit ★ | `….7c1d78728036.unit` | `internal/stele/cli_test.go`, beside scope iteration | `internal/stele/cli_test.go#TestAllScopesReportSeparately` | Risk: one scope hides another's failure, or the combined exit code is wrong. In-process run over a fixture with two changes. |
| | e2e | `….7c1d78728036.e2e` | `tests/cli.test.mts` (executable contract) | `tests/cli.test.mts#checks every scope with --all` | Distinct risk: CI will use `--all` through the shipped binary, where output and exit code together decide the gate. |
| `scn.linkindex.bda4420352ef` Index every scope (new) | unit ★ | `….bda4420352ef.unit` | `internal/stele/index_test.go` | `internal/stele/index_test.go#TestIndexCoversEveryScope` | Risk: an editor sees only one change. Built from a fixture with two changes. |
| `scn.linkindex.4d6ba51a47bc` Reject conflicting scope options (new) | unit ★ | `….4d6ba51a47bc.unit` | `cli_test.go` | `internal/stele/cli_test.go#TestAllRejectsConflictingScopes` | Risk: `--all --change` silently ignores one of them. Option parsing in process. |
| `scn.linkindex.57dcee8c30a8` Leave parameterized test names unresolved | unit ★ | `….57dcee8c30a8.unit` | `internal/stele/anchors_test.go`, beside the TypeScript patterns | `internal/stele/anchors_test.go#TestScanAnchorsLeavesParameterizedTestsUnresolved` | Risk: a partial name pattern runs the wrong tests or none. Scanner fixtures. |
| `scn.verify.140b21cbc3f0` Separate a failed execution from passing linkage | unit ★ | `….140b21cbc3f0.unit` | `internal/stele/verify_test.go`, beside report building | `internal/stele/verify_test.go#TestReportSeparatesExecutionVerdict` | Risk: reports look green while tests fail, including through the redefined `verdict`. Report building is pure. |
| `scn.verify.a7ae8afade0a` Treat missing evidence as an incomplete overall verdict | unit ★ | `….a7ae8afade0a.unit` | `verify_test.go` | `internal/stele/verify_test.go#TestReportMarksMissingEvidenceIncomplete` | Risk: "not run" reads as passing through `verdict`. Pure. |
| `scn.verify.06f2be2af1e7` Write output files with the new flags (new) | unit ★ | `….06f2be2af1e7.unit` | `internal/stele/cli_test.go`, beside the output paths | `internal/stele/cli_test.go#TestOutputFileFlags` | Risk: the new flags do not reach the writers. `Run` in process with stubs, checking both files. |
| `scn.verify.70e950bf161c` Accept the deprecated file flags with a warning (new) | unit ★ | `….70e950bf161c.unit` | `cli_test.go` | `internal/stele/cli_test.go#TestDeprecatedOutputFlagsWarn` | Risk: the aliases break existing scripts, or warn without working. `Run` in process. |

Requirement implementation targets (v1 plan today):

| Requirement | Target |
|---|---|
| `req.linkindex.78a6abc9c59d` Export a deterministic link index | `internal/stele/index.go#BuildIndex` |
| `req.linkindex.860a4d91b9fe` Run the tests of selected behavior | `internal/stele/runner.go#selectBehaviorTests` |
| `req.linkindex.0c06109d8d29` Check every scope at once | `internal/stele/cli.go#runEveryScope` |
| `req.verify.3624e3449f6a` Name output files explicitly | `internal/stele/cli.go#registerOutputFlags` |
| `req.verify.27a52b8cfbd6` Report linkage and execution verdicts separately (`verdict` becomes overall) | `internal/stele/verify.go#reportVerdicts` |

## Risks / Trade-offs

- [The whole-repository digest marks everything stale after any edit] → This is honest but noisy. Editors can show "stale" softly. Finer digests are an open question.
- [Evidence schema 3 changes files that CI uploads] → The change is additive for readers of `outcome` and `scenarios`, and is documented in the changelog.
- [Rejecting interpolated test names can turn previously "resolved" anchors into unresolved ones] → Those anchors never selected the intended test, so the stricter result is correct. It is noted in the changelog.

## Decided questions

- **Behavior selection:** positional targets, Jest-style, instead of the planned `--scenario`, `--requirement`, and `--evidence <id>` flags.
- **Scopes:** `--all` checks the current specifications and every active change, separately, with one combined exit code.
- **Output files:** `--evidence-file`, `--report-file`, and `--output-file`, with the old flags as deprecated aliases until 0.2.0.
- **`verdict`:** redefined as the overall verdict (breaking in 0.x), with `verdicts.linkage` for the old meaning.
- **Staleness:** the input digest stays whole-repository in this version.
- **Index scopes:** the index includes the current specifications next to the selected change, labelled by scope.

- **Modified requirements:** a modified requirement appears once per scope, with the same ID.
- **Spec text:** requirements keep their body text, and scenarios keep their raw text and structured steps, for editor hover cards.

## Open Questions

None.
