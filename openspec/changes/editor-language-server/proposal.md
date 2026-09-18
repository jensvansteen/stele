## Why

Stele aims to make a codebase readable as plain English: every behavior in the specification links to the code that implements it and to the tests that prove it. Those links serve two equal purposes, verification and review. Today they only serve verification, and only in a terminal or CI.

People read and change code in an editor. There they should be able to:

- start from a scenario in a `spec.md` and run its tests, or jump to its implementation;
- start from code, hover an `@implements req.…` or `@verifies scn.….<level>` anchor, and read the specification text and its status, the way a doc comment shows up on hover;
- ask "where is this requirement used?" and get every implementation and test;
- see broken links, specification problems, and failed or stale results while typing, with the same meaning and fix that `stele check` prints, not at the next CI run.

Stele 0.1.0-rc.4 has everything this needs in process: the `link-index` builder (`BuildIndex`) with requirement text, scenario steps, `specFiles`, and `specVersion`; positional `stele test` targets; `--all`; the `spec-annotation` checks; and the `terminal-report` diagnostic catalogue, verdicts, and concurrency-ready progress observer. A language server is the editor-neutral way to put that in front of people: one server inside the Stele binary serves VS Code, Zed, JetBrains IDEs, Neovim, Helix, and any other LSP client, and the editor plugins stay thin.

## What Changes

- **`stele lsp`**: a new subcommand that runs a Language Server Protocol server over standard input and output, in the same binary and npm package as the rest of Stele. It serves every scope of the workspace, like `--all`: the current specifications and every active change. `serverInfo` carries the package version that `stele --version` prints.
- **Hover cards**: on an `@implements` or `@verifies` anchor, a doc-comment-like card with the requirement body or the scenario's `WHEN`/`THEN`/`AND` steps, its scope, and, for evidence, the level, approval, and last outcome. On a requirement or scenario heading in a `spec.md`, where the behavior is implemented and tested, with status.
- **Go to definition, both ways**: from an anchor to its specification heading; from a requirement heading to its implementations; from a scenario heading to its tests.
- **Find references** (new): from a requirement, a scenario, or an anchor ID, every implementation and test anchor of that behavior, and optionally its headings.
- **CodeLens**: on requirement and scenario headings, run buttons (run all, and one per evidence level) and a status line such as `unit ✓ · e2e ✗ · stale`, combined with the same rule as the execution verdict. On code and test anchors, a summary lens with the behavior's title and its first `WHEN` and `THEN` steps that opens the specification, and on test anchors a run button for their own evidence entry.
- **Running tests**: run buttons execute through the same runner as `stele test <targets...>`, including the batching, `--jobs` default, execution groups, and setup and teardown that the planned `fast-runs` change adds. Progress is reported with `$/progress`; cancelling behaves like interrupting `stele test`. The result names each outcome and the scope's `verdicts` (`linkage`, `execution`, `overall`).
- **Diagnostics**: the findings `stele verify` reports for each served scope, with the same codes and severities, including `SPEC_ANNOTATION_*`, `LINK_REMOVED_BEHAVIOR_ANCHORED`, `PLAN_REMOVED_BEHAVIOR_PLANNED`, and `SPEC_REMOVED_UNMATCHED`, each explained with the diagnostic catalogue's meaning and fix. `PLAN_UNAPPROVED` stays out of the editor. Two execution codes, `EXECUTION_FAILED` and `EXECUTION_STALE`, join the catalogue.
- **Quick fix** (new): "Add Stele annotation" on specification files that lack `<!-- stele: spec v1 -->`, writing exactly the bytes `stele annotate` writes.
- **Keeping current**: the index is rebuilt incrementally when files are edited (including unsaved buffers), saved, created, or deleted, and when a terminal run of `stele test` rewrites the evidence file.
- **Deterministic and editor-neutral**: identical workspace state and requests produce identical responses. The server adapts only to capabilities the client declares, never to which editor it is. A custom `stele/index` request returns the same document as `stele index --json --all`.

Editor clients (VS Code, Zed, JetBrains) are planned separately in the `stele-editors` repository and contain no Stele logic.

## Capabilities

### New Capabilities

- `language-server`: the `stele lsp` language server: transport and lifecycle, hover cards, navigation and references, CodeLens, running tests with progress, diagnostics, the annotation quick fix, change tracking, and determinism.

### Modified Capabilities

None. The server reuses `link-index`, `spec-annotation`, `terminal-report`, and `execution` behavior without changing what the CLI does. Adding the two `EXECUTION_*` codes to the diagnostic catalogue leaves the catalogue requirement ("every code has a stage, a meaning, and a fix") unchanged.

## Impact

- `internal/stele`: a new language-server component (JSON-RPC framing, dispatch, document store, position mapping, feature handlers, file watching) that calls `BuildIndex`, the verification checks, the diagnostic catalogue, the annotation insertion, and the test runner in process. Three changes without effect on CLI output make that possible: verification is split into loading and a pure check over loaded inputs, as `loadIndex` and `BuildIndex` already are; the evidence file is written atomically (temporary file, then rename), so the server never reads a half-written file; and the catalogue gains an execution stage for the two new codes. `cmd/stele/main.go` stays a composition root.
- No new Go module dependency: the design keeps `go.mod` dependency-free (see design.md).
- Documentation: a CLI reference entry for `stele lsp`, an "Editor integration" guide with the protocol surface for client authors, the two new codes in the diagnostics reference, and the changelog.
- Depends on the planned `fast-runs` change for the run feature (a cancellable run context, the scheduler, and the extended observer events). The other features depend only on merged work and can be built first.
