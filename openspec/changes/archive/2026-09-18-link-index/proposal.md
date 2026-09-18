## Why

Stele's links between specification, code, and tests should help people review and navigate, not only gate CI. An editor should be able to:

- start from a scenario and run its tests, or jump to the implementation;
- start from code, hover an ID, and see the specification case it covers.

That needs one deterministic, machine-readable view of every link, and a way to run the tests of a single behavior. The VS Code extension is the first consumer; a custom UI may follow.

The verification report also has a misleading field. `verdict` reflects only linkage diagnostics, so a report can say `pass` while execution failed. Execution failures show up only in `stages.execution` and in `validate`'s exit code.

## What Changes

- **`stele index [--change <id> | --specs] --json`** prints a deterministic link index. It contains:
  - every requirement and scenario in the scope, with its ID, title, specification text, and source location, and the relation between requirements and scenarios. Scenarios also carry structured `WHEN`/`THEN`/`AND` steps, so an editor can show a hover card like a doc comment;
  - every code and test anchor, with its location, selector, and evidence level;
  - the planned evidence and approval state;
  - the last known execution outcome for each piece of evidence, marked `stale` when the inputs have changed since.
  When a change is selected, the current specifications are included too, and every item is labelled with its scope. Anchors whose IDs no specification declares are included and flagged.
- **`stele test [targets...]`** runs only the anchored tests of the targets, Jest-style. A target is a requirement, scenario, or evidence ID, or the path of a specification file; several targets combine. The results are merged into the stored evidence without discarding other outcomes. Without targets the whole scope runs, as before. Version 1 supports exact test names only: a test whose name is parameterized or generated stays unresolved.
- **`--all`** on `test`, `verify`, `validate`, and `index` checks the current specifications and every active change, each as its own scope, reported separately with one combined exit code.
- **Explicit file flags.** `--evidence-file PATH` and `--report-file PATH` name the output files, and `stele index` writes with `--output-file PATH`. The existing `--evidence` and `--report` flags keep working as deprecated aliases with a warning until 0.2.0.
- **Separate verdicts in the report.** Linkage, execution, and overall verdicts become separate fields. **BREAKING (0.x):** the top-level `verdict` now means the overall verdict, and consumers that need the old meaning read `verdicts.linkage`.

## Capabilities

### New Capabilities

- `link-index`: the machine-readable link index and running the tests of one behavior.

### Modified Capabilities

- `verify`: reports gain separate linkage, execution, and overall verdicts.

## Impact

- `internal/stele`: a new index builder usable in process, positional target selection, `--all` scope iteration, evidence merging, the new file flags, report verdict fields, and stricter handling of TypeScript test names.
- The evidence schema records per-execution input digests, so staleness can be decided for each entry.
- The documentation covers the CLI reference, the index schema for editor authors, and the changelog.
- It builds on `verification-strategy` for evidence IDs, levels, and approvals, and ships after it.
