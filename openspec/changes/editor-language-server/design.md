## Context

See proposal.md for the motivation. This change adds `stele lsp` to the existing binary. It was first planned before `link-index` merged; this revision plans against `main` at 0.1.0-rc.4, where `link-index`, `spec-annotation`, and `terminal-report` are merged and archived, and against the planned `fast-runs` change for running tests.

What the server builds on:

- **Index core (`link-index`, merged).** `BuildIndex(IndexInput) Index` in `index.go` is pure; `loadIndex` does the I/O. The document (schema 1) lists `scopes`, `specFiles`, `requirements` (with `text`, `source`, `specVersion`, `scenarios`, `implementations`), `scenarios` (with raw `text`, `steps` of `{keyword, text}`, `source`, `specVersion`, and `evidence` with `approval`, `tests`, and `execution {state, outcome}`), and `anchors` (with `status` `linked`, `unresolved`, `other-scope`, or `undeclared`, plus `evidenceId` and `level`). `stele index --json --all` prints one document over every scope.
- **Behavior runner (`link-index`, merged).** `stele test <targets...>` accepts `req.…`, `scn.…`, `scn.….<level>[.<n>]`, or a `spec.md` path; unknown targets exit `2` before any test runs; outcomes are merged by evidence ID and test target. Internally, `runScopeTests(testRequest{root, scope, evidencePath, targets, merge, observer})`.
- **Annotations (`spec-annotation`, merged).** `classifyAnnotation(path, content)` and `insertAnnotation(content)` in `annotation.go` are pure over the file text. Findings: `SPEC_ANNOTATION_MISSING`, `_MISPLACED` (severity from `unannotatedSpecs`, default `warn`), `_MALFORMED`, `_UNSUPPORTED` (errors), `_FIELD_IGNORED` (warning). The `spec-annotation` design names the diagnostics and an "Add Stele annotation" code action as follow-ups for this plan.
- **Report and catalogue (`terminal-report`, merged).** `diagnostics.go` holds `diagnosticGuides`: per code, a stage (`Specifications`, `Plan approval`, `Linkage (anchors)`), a one-line meaning, a short label, and a fix template with `{scope}`. A unit test requires a catalogue entry for every code literal in the package. `reportVerdicts` defines `verdicts {linkage, execution, overall}`, with the top-level `verdict` equal to `overall`; `aggregateExecution` combines outcomes (`passed` only if all passed, else `failed`, else `stale`, else `not-run`). `removed.go` adds `LINK_REMOVED_BEHAVIOR_ANCHORED`, `PLAN_REMOVED_BEHAVIOR_PLANNED`, and `SPEC_REMOVED_UNMATCHED`. `progress.go` defines `testObserver` (`planned`, `started`, `finished`), safe for concurrent use, with events keyed by test identity and carrying a group and worker slot. `Version` is set from `package.json` at build time. `stele check` combines the ID check, the annotation check, and `validate`.
- **Verification reads disk.** `verifyScope(verifyRequest)` parses specs, scans anchors, and reads plans and evidence itself, then calls the pure `validateLinkage` and `buildReport`. The server needs the same checks over its in-memory state (Decision 5).
- **Dependencies.** `go.mod` has no requirements; AGENTS.md and CLAUDE.md ask to keep the verifier dependency-light, private under `internal/stele`, with `cmd/stele/main.go` as a minimal composition root, co-located `*_test.go` files, and an exact 100% core coverage gate.

### Dependencies

| From | State | Used for |
|---|---|---|
| `link-index`: `BuildIndex`, `IndexInput`, the index document | merged (rc.3) | Hover, definition, references, CodeLens, the `stele/index` request |
| `link-index`: requirement `text`, scenario `text` and `steps` | merged | Hover cards and summary lenses |
| `link-index`: positional targets, evidence merging, `--all` scope iteration | merged | Run commands; the server's workspace scope |
| `link-index`: per-execution input digests and `stale` | merged | Status, stale diagnostics |
| `spec-annotation`: `classifyAnnotation`, `insertAnnotation`, `specFiles`, `specVersion`, the `SPEC_ANNOTATION_*` findings and policy | merged (rc.4) | Annotation diagnostics and the quick fix |
| `terminal-report`: `diagnosticGuides`, `reportVerdicts`, `aggregateExecution`, removed-behavior checks, `testObserver`, `Version` | merged (rc.4) | Diagnostic messages, run results, status, progress, `serverInfo` |
| `fast-runs`: cancellable run context, scheduler (`--jobs` resolution, `execution.groups`, setup and teardown, process groups), observer events `batchEnded`, `groupSetup`, `groupTeardown`, and "write no files when interrupted" | **planned, not built** (`openspec/changes/fast-runs` on `chore/plan-fast-runs`) | `stele.runTests`, its progress and its cancellation |

**The run feature is blocked on `fast-runs`.** Everything else depends only on merged code, so task sections 1 to 5 can start before `fast-runs` merges, and section 6 starts after it (task 0.3). The server consumes `fast-runs`' interfaces and does not redefine them. If `fast-runs` changes any of them before it merges (for example open question 4, the interruption exit code, or the observer's event set), this design is updated before its levels are approved. Without `fast-runs` the current runner has no context: it cannot be cancelled, and its sequential per-test processes would make the "Run all" lens slow on this repository (about 70 seconds today).

## Goals / Non-Goals

**Goals:**

- One editor-neutral server that every LSP client can use, with thin editor clients.
- No second implementation of Stele rules: every answer comes from the same core as the CLI, including the catalogue's wording.
- Responses fast enough for hover and typing on repositories the size of this one.

**Non-Goals:**

- Editor UI beyond standard LSP features (panels, tree views, test explorers). Clients may build those from the `stele/index` request, without Stele logic.
- Completion, rename, semantic tokens, and code actions other than "Add Stele annotation". Fixes for other codes (for example `stele ids` for `ID_*_MISSING`) are follow-ups (open question 3).
- Editing specifications or approving evidence from the editor. Approval stays in `stele approve` or the agent conversation.
- Other transports than stdio (sockets, TCP).
- Per-scenario input digests. Staleness stays whole-repository, as in `link-index`; it is honest but noisy, and a follow-up can refine it.
- References from linkage plans. `references` returns anchors and headings; plan entries are not located in the index.

## Decisions

### 1. Same binary, same core, in process

`stele lsp` is a subcommand of the existing binary and npm package, so editor clients never pin a second tool and the server and CLI can never drift in version. The server holds the parsed state in memory and calls `BuildIndex`, the verification checks, the catalogue, the annotation functions, and the runner directly, not by spawning `stele`.

**Alternative rejected:** a server that shells out to `stele index --json --all` on every change. It is simpler, but it re-parses the whole repository for every keystroke, cannot see unsaved buffers, and duplicates process management.

### 2. LSP library: hand-rolled JSON-RPC and protocol types

The server needs a small subset of LSP 3.17: `initialize`/`initialized`/`shutdown`/`exit`, `textDocument/didOpen|didChange|didSave|didClose`, `workspace/didChangeWatchedFiles`, `client/registerCapability`, `textDocument/hover`, `textDocument/definition`, `textDocument/references`, `textDocument/codeLens`, `textDocument/codeAction`, `workspace/executeCommand`, `workspace/applyEdit`, `window/showDocument`, `textDocument/publishDiagnostics`, `workspace/codeLens/refresh`, `window/workDoneProgress/create`, `window/workDoneProgress/cancel`, `$/progress`, `$/cancelRequest`, `window/showMessage`, and the custom `stele/index`.

| Option | Dependencies (checked 2026-09-18 on GitHub) | Assessment |
|---|---|---|
| `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2` | `go-json-experiment/json`, `go-cmp`, `go.lsp.dev/jsonrpc2`, `go.lsp.dev/uri`; module also lists a large lint toolchain as indirect requirements | Full generated types, but a pre-release JSON library and a broad `go.sum` for a subset of about 25 messages. |
| `github.com/tliron/glsp` | `gorilla/websocket`, `pkg/errors`, `sourcegraph/jsonrpc2`, `tliron/commonlog`, plus terminal and crypto indirects; last push 2025-06 | Convenient handlers, but the heaviest tree and least active. |
| `github.com/sourcegraph/jsonrpc2` + own types | `gorilla/websocket` | Only framing and dispatch; we would still write all protocol types. |
| gopls's `protocol` package | Not importable (`internal`) | Would need copying generated code with its license; large surface to own. |
| **Hand-rolled (chosen)** | Standard library only (`encoding/json`, `bufio`, `io`, `os/exec`, `context`) | Framing (`Content-Length` headers) and a JSON-RPC 2.0 dispatcher are a few hundred lines; the protocol structs cover only the subset above. |

**Why:** AGENTS.md asks for a dependency-light verifier and `go.mod` has none. `terminal-report` made the same choice for terminal detection (no `golang.org/x/term`). The subset is small and stable (LSP 3.17 has been current since 2022), and the exact 100% coverage gate is easier to meet for code we own and test through `io.Pipe` than to argue around for third-party glue. JSON is encoded with explicit struct fields in a fixed order, which also supports determinism.

**Revisit trigger:** if the server later needs large parts of the protocol (semantic tokens, completion, many code actions), generate the protocol types from the official LSP meta-model into an internal package instead of hand-writing them, still without runtime dependencies.

**Position encoding:** the server offers `utf-16` (the LSP default) and `utf-8` when the client lists it in `general.positionEncodings`, choosing `utf-8` when offered. Line and column conversion lives in one tested helper.

### 3. Structure inside `internal/stele`

AGENTS.md keeps the implementation in one private package. A language server is a boundary that could justify a split, but splitting would expose the shared model. Everything stays in `internal/stele` with an `lsp_` file prefix, each file with its co-located test:

- `lsp_rpc.go`: framing and dispatch;
- `lsp_protocol.go`: protocol types;
- `lsp_server.go`: lifecycle, state, and determinism;
- `lsp_hover.go`, `lsp_definition.go` (definition and references), `lsp_codelens.go`, `lsp_codeaction.go`;
- `lsp_run.go`: run commands and progress;
- `lsp_diagnostics.go`;
- `lsp_watch.go`.

`cli.go` gains the `lsp` subcommand. If the package becomes unwieldy, extracting `internal/stele/lsp` is a follow-up refactor with no behavior change.

### 4. Workspace model and incremental rebuild

- **Roots:** each workspace folder from `initialize` (or `rootUri` when no folders are sent) that contains `openspec/` or `stele.config.json` is a project. Folders without either are ignored with a log line on standard error. Nested projects are not supported in v1.
- **Scope:** every project is served like `--all`: current specifications plus every active change, each item labelled with its scope. The server reuses `everyScope`, so it sees exactly the scopes `stele index --all` sees.
- **State:** the server keeps, per project, the parsed result of every input file keyed by path: spec files (with their annotation classification), source and test files (anchors), linkage plans, `stele.config.json`, and the evidence file. Open documents override disk content with their in-memory text (full-text sync, `TextDocumentSyncKind.Full`, to keep the code small; incremental sync can come later).
- **Rebuild:** a change re-parses only the affected file, then re-runs `BuildIndex` and the verification checks (Decision 5) over the cached parse results. Rebuilds are debounced (100 ms after the last change) and serialized; requests are answered from the latest completed index. Because both are pure over the cached inputs, an incremental update equals a full rebuild of the same state, which a property-style unit test checks by replaying seeded edit sequences.
- **Staleness from saved files:** `IndexInput.InputDigest` is computed with `ComputeInputDigest` over the files on disk, not over open buffers. Tests run against saved files, so an unsaved edit does not mark results stale; saving does. The digest is recomputed on save and on watched-file events, not on every keystroke.
- **Watching:** when the client declares `workspace.didChangeWatchedFiles.dynamicRegistration`, the server registers globs for `openspec/**/*.md`, `openspec/**/linkage-plan.json`, the configured source and test directories, `stele.config.json`, and the evidence file. Otherwise it runs a standard-library poller that compares size and modification time of the same file set every 2 seconds, with an injected clock. No `fsnotify` dependency.
- **Evidence written elsewhere:** a terminal `stele test` rewrites the evidence file; the watcher sees it and the server reloads outcomes, then sends `workspace/codeLens/refresh` (when supported) and republishes diagnostics.

### 5. Diagnostics: the CLI's findings, the catalogue's words

**Source of findings.** A refactor without behavior change splits `verifyScope` into loading (`loadVerifyInput`, which reads the disk as today) and a pure `verifyLoaded(verifyInput) Report`, the same split `loadIndex` and `BuildIndex` already have. The CLI calls both; the server calls only `verifyLoaded` with its cached, overlaid inputs. The existing verify tests guard the refactor, and there is no server-only rule.

**Stage per scope.** The editor cannot know whether a change is being planned or implemented, so the server decides it from the plan, following the Stele workflow (approval before implementation):

- the current specifications: `implementation`;
- an active change: `implementation` once its `linkage-plan.json` has at least one approved entry, `proposal` before that.

This keeps `LINK_CODE_MISSING` and similar findings off changes that are still being planned, where they would be expected and noisy.

**What is published.** Every finding of every served scope, with its code and severity unchanged, except `PLAN_UNAPPROVED`, which is expected on every entry during planning and is shown as "(unapproved)" in CodeLens and hover instead. That includes the `ID_*`, `SCENARIO_MISSING`, `SPEC_ANNOTATION_*` (with the project's `unannotatedSpecs` policy applied), `SPEC_REMOVED_UNMATCHED`, `PLAN_*`, `LINK_*` (including `LINK_REMOVED_BEHAVIOR_ANCHORED`), and `ANCHOR_*` codes.

- **Location:** the finding's `Source` (path and line) becomes a range covering that line's text. A finding without `Source` but with `IdentityID` goes on the identity's heading in that scope. A finding with neither is not published per file; it still shows in `stele check`.
- **Once per place:** with `--all`-style scopes, the same anchor can be reported by several scopes. Diagnostics are deduplicated by path, range, code, and message, keeping the first scope in scope order (current specifications, then changes in directory order).
- **Message:** `<finding message>` + newline + `<catalogue meaning>` + newline + `Fix: <catalogue fix>`, with `{scope}` replaced by `--specs` or `--change <id>` for the finding's scope. This is the same text the terminal report prints under the group, so a person sees the same guidance in the editor, in `stele check`, and in CI. `Diagnostic.code` is the code, and `source` is `stele`. `codeDescription` is left out in v1 (no per-code documentation URL yet).
- **Execution findings.** Two codes come from the index's execution state rather than from verification, because `stele verify` reports execution in `verdicts` and the report's test list, not as diagnostics: `EXECUTION_FAILED` (warning) for a last outcome `failed`, and `EXECUTION_STALE` (information) for state `stale`, on the scenario heading and on each test anchor of the evidence. Both get catalogue entries with a new stage constant `Test execution`, meaning, and fix (`run stele test <evidence-id>`, or the CodeLens run button), so the catalogue test keeps passing and the diagnostics reference documents them. The CLI never emits them, so the terminal report is unchanged.
- **Clearing:** diagnostics are published per file, and a file whose problems are gone gets an empty list.

### 6. Hover cards, references, and CodeLens

- **Hover** looks up the ID under the cursor (anchor comments in sources; headings and `Verification-ID` lines in `spec.md`) in the index and renders a card, a pure function from index entries to Markdown or plain text, chosen by `hover.contentFormat`:

  ```markdown
  **Scenario:** Add non-empty todo · `todo-basics`
  **WHEN** the user adds "milk"
  **THEN** the list shows "milk"
  **AND** the input is cleared
  ---
  unit · approved · passed (stale)  — tests/todo.test.mts:5
  ```

  Steps come from the index's `steps` (keyword and joined text), in spec order; a step without a keyword prints its text alone, and a scenario without steps prints its raw `text`. A requirement card shows its `text` and its scenario titles. The plain-text form uses the same lines without emphasis. When the ID exists in several scopes, one card per scope, labelled, in scope order.
- **Definition** returns `Location[]`, ordered by path and line: anchor → declaring headings (one per scope); requirement → `@implements` anchors; scenario → `@verifies` anchors of its evidence.
- **References** (`textDocument/references`) answers "where is this used?": a requirement gives its `@implements` anchors and the `@verifies` anchors of its scenarios' evidence; a scenario or an evidence ID gives the `@verifies` anchors of that scenario's evidence; with `context.includeDeclaration`, the declaring headings in every scope are added. Results are unique and ordered by path and line. Definition stays narrow (the direct counterpart), references stay wide; some editors present one better than the other, so both exist.
- **CodeLens on headings** with planned evidence or anchors: `▶ Run all` (target: the requirement or scenario ID), `▶ unit`, `▶ integration`, `▶ e2e` for the levels present (target: the evidence IDs of that level), and a status lens such as `unit ✓ · e2e ✗ · stale`, `unit – not run`, or `e2e (unapproved)`. A requirement's combined mark uses `aggregateExecution`, the rule behind `verdicts.execution`, so the editor never shows a pass that `stele verify` would call `not-run` or `stale`. The status lens carries `stele.showStatus`, which the server answers with `window/showMessage` listing each evidence entry.
- **CodeLens on anchors:** each `@implements` and `@verifies` anchor gets a summary lens: `Scenario: <title> — WHEN <text> · THEN <text>` (or `Requirement: <title> — <first line of text>`), cut at 120 characters on a rune boundary with `…`, so the lens is identical everywhere. Its command `stele.showSpecification` sends `window/showDocument` with the heading's range when the client declares `window.showDocument.support`, and otherwise `window/showMessage` with the plain-text card. Test anchors also get `▶ Run` for their own evidence ID.
- **Commands** (advertised in `executeCommandProvider.commands`):

  | Command | Arguments |
  |---|---|
  | `stele.runTests` | `{ "root": "<project root URI>", "targets": ["scn.…", …] }` |
  | `stele.showStatus` | `{ "root", "id" }` |
  | `stele.showSpecification` | `{ "root", "id" }` |
  | `stele.addAnnotation` | `{ "uri" }` (fallback for clients without code action literals, Decision 7) |

  All logic runs on the server; a client only forwards the click.

### 7. The "Add Stele annotation" quick fix

- **When:** `textDocument/codeAction` for a range that touches a `SPEC_ANNOTATION_MISSING` diagnostic of an open specification file, with `only` empty or containing `quickfix`.
- **What:** one `CodeAction` of kind `quickfix`, title "Add Stele annotation", `isPreferred: true`, `diagnostics: [that diagnostic]`, and an edit computed from the document text the client sent: `insertAnnotation(text)` from `annotation.go` gives the new text, and the edit is the inserted prefix at line 0, character 0, or after the byte order mark when the text starts with one. The line ending is the text's first line ending, or `\n` when it has none, exactly as `stele annotate` writes it. A unit test applies the edit to the text and compares it with `insertAnnotation`'s output byte for byte.
- **Edit form:** `documentChanges` with the open document's version when the client declares `workspace.workspaceEdit.documentChanges`, so a stale buffer rejects the edit; otherwise `changes`.
- **Not offered** for `SPEC_ANNOTATION_MALFORMED`, `_UNSUPPORTED`, `_MISPLACED`, or `_FIELD_IGNORED`: `stele annotate` leaves those for a person, and the server does the same. The diagnostic's fix text already says what to do.
- **Without code action literals** (`textDocument.codeAction.codeActionLiteralSupport` missing), the server returns a `Command` `stele.addAnnotation` with `{ "uri" }`; executing it sends `workspace/applyEdit` with the same edit.
- Annotating a whole scope stays `stele annotate`; a `source` action for it is a follow-up.

### 8. Running tests from the server

- **One runner.** `stele.runTests` validates the targets against the index (an unknown target gives JSON-RPC error `-32602` naming it, and nothing runs), then calls the same entry point as `stele test <targets...>` (`runScopeTests` with `targets` and `merge`) in a goroutine, one call per affected scope. With `fast-runs` that entry point goes through the scheduler, so the server gets batching, the job count (from `execution.jobs`, else `auto`; there is no `--jobs` in the editor), `execution.groups`, group setup and teardown, `STELE_WORKER_ID`, `STELE_JOBS`, `STELE_GROUP`, and per-group timeouts without code of its own.
- **Cancellation.** `fast-runs` gives the scheduler a run context that the CLI cancels on `SIGINT` or `SIGTERM`. The server passes its own context instead and cancels it on `$/cancelRequest` for the command or `window/workDoneProgress/cancel` for its token. The scheduler then does what it does when `stele test` is interrupted: it kills the running process groups, runs the teardown of every started group, and writes no evidence, so the stored file stays as it was. The command ends with the LSP `RequestCancelled` error (`-32800`), the editor equivalent of exit `130`. A teardown failure (exit `2` in the CLI) is reported in the result's `problems` and as a `window/showMessage` warning, after evidence is written.
- **One run per project.** A second `stele.runTests` for a project with a run in progress gets `RequestFailed` (`-32803`) naming the running targets. Concurrent runs from a terminal are not blocked; the atomic evidence writer and merge by evidence ID keep the file intact, and the last writer wins per entry.
- **Progress.** When the request carries a `workDoneToken`, the server uses it; otherwise, when the client declares `window.workDoneProgress`, it creates one with `window/workDoneProgress/create`. A progress adapter implements `testObserver` (plus `fast-runs`' `batchEnded`, `groupSetup`, and `groupTeardown`):
  - `planned(total, …)` → `begin` with title `Stele: running <total> tests`, `cancellable: true`, `percentage: 0`;
  - `finished(event)` → `report` with `<run>/<total> · <passed> passed · <failed> failed`, the percentage, and `failed: <title> (<level>)` when that test failed; with several tests in flight it adds `· <n> running`, as the terminal line does;
  - `groupSetup` and `groupTeardown` → `report` naming the group;
  - the end of the run → `end` with the summary line.

  Progress messages are the only output that may follow completion order; everything else is built from the sorted result.
- **Result.** `{ "executions": [{ "evidenceId", "path", "line", "selector", "outcome", "reason" }], "scopes": [{ "scope", "verdicts": { "linkage", "execution", "overall" } }], "problems": [] }`, sorted by path and selector, and by scope order. Verdicts come from `verifyLoaded` and `reportVerdicts` over the evidence after the run at the scope's stage (Decision 5), so a green run in a scope with a linkage error reads `overall: fail`, as in `stele verify`.
- **After a run** the evidence file is written by the runner's writer, which the watcher observes like any other change; the server also refreshes directly so the result and the lenses agree without waiting for the watcher.
- **Atomic evidence writes.** Today `writeJSON` (used for the evidence file) calls `os.WriteFile`, so a reader can see a partly written file. With a server that watches and reloads the file while a terminal run writes it, that becomes visible. This change makes the evidence write atomic for the CLI and the server alike: a temporary file in the same directory, then `os.Rename`. The bytes are unchanged, so no output changes; a unit test checks that an interrupted write leaves the previous file intact.

### 9. Staying in sync with the CLI

- One implementation: the server links the same parser, scanners, `BuildIndex`, verification checks, catalogue, annotation functions, runner, and evidence writer as the CLI.
- A custom request `stele/index` with `{ "root": "<URI>" }` returns the document `stele index --json --all` prints for the saved state, now including `specFiles` and `specVersion`. An e2e test runs both through the shipped binary and compares bytes, so any divergence fails CI.
- Diagnostic codes, severities, meanings, and fixes are the CLI's; the editor guide says that a diagnostic in the editor is the same finding `stele check` reports.
- `serverInfo` is `{ "name": "stele", "version": Version }`, the value `stele --version` prints.
- Evidence written by either side is the same format and is merged, not overwritten.

### 10. Determinism and editor neutrality

Responses and diagnostics contain no timestamps, maps are emitted through sorted slices, locations are ordered by path then line, CodeLens items by line then a fixed order (summary, run all, `unit`, `integration`, `e2e`, status), and code actions by title. The server never branches on `clientInfo`; only declared capabilities change behavior, each with a documented fallback:

| Capability | With it | Without it |
|---|---|---|
| `hover.contentFormat` includes `markdown` | Markdown card | Plain-text card |
| `didChangeWatchedFiles.dynamicRegistration` | Client watches | Stat poller |
| `window.workDoneProgress` or a request `workDoneToken` | `$/progress` | Run without progress |
| `workspace.codeLens.refreshSupport` | `workspace/codeLens/refresh` | Lenses refresh on the next request |
| `window.showDocument.support` | Show the heading | Show the card as a message |
| `codeAction.codeActionLiteralSupport` | Code action with edit | `stele.addAnnotation` command |
| `workspace.workspaceEdit.documentChanges` | Versioned edit | `changes` |
| `general.positionEncodings` includes `utf-8` | UTF-8 columns | UTF-16 columns |

### 11. Verification strategy

**Status: approved by jensvansteen on 2026-09-18, before implementation started (via: cli).** All 47 entries are approved in `linkage-plan.json`. This revision keeps the 31 entries of the first draft (five with reworded scenarios, marked "reworded") and adds 16 entries for the new scenarios (marked "new").

Placement follows this repository's AGENTS.md: co-located Go tests in `internal/stele` (the `lsp_*_test.go` files beside each `lsp_*.go` file), and executable contract tests of the shipped binary in `tests/cli.test.mts`. Placement is advisory. Unit tests drive the server in process through `io.Pipe` with fixture repositories in temporary directories, a recording stub for the runner's batch and group-command seams (from `fast-runs`), and an injected clock. e2e is used only where the shipped binary adds a distinct risk: the real stdio framing, process lifecycle, and build-time version, and byte equality with the CLI. No integration level is planned: real test processes are already covered by the runner's own evidence in `execution`, `go-support`, and `fast-runs`.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|
| `scn.languageserver.3a0c2fb46bad` Complete a session through the installed binary (reworded: references, code actions, version) | e2e | `….3a0c2fb46bad.e2e` | `tests/cli.test.mts` (executable contract of the shipped binary) | Risk: stray bytes on stdout (logging, progress, prints), a wrong exit code, or a version not wired in by the build break every editor at once. Only a real process with real pipes built by the release script exposes buffering, framing, exit behavior, and the injected version. |
| `scn.languageserver.16fcd1dc9b41` Survive malformed input | unit | `….16fcd1dc9b41.unit` | `internal/stele/lsp_rpc_test.go`, beside framing | Risk: one bad message kills the session. Framing and dispatch are pure over an `io.Pipe`. |
| `scn.languageserver.be03d984234b` Refuse requests before initialization | unit | `….be03d984234b.unit` | `internal/stele/lsp_server_test.go`, beside lifecycle | Risk: half-initialized state serves wrong roots. Lifecycle state machine in process. |
| `scn.languageserver.e311dc0148f1` Exit without shutdown | unit | `….e311dc0148f1.unit` | `lsp_server_test.go` | Risk: wrong exit code confuses clients' restart logic. The run function returns the code in process; the e2e session covers the real exit path for `0`. |
| `scn.languageserver.59068d53a952` Hover an implementation anchor | unit | `….59068d53a952.unit` | `internal/stele/lsp_hover_test.go` | Risk: the wrong text or scope is shown. Lookup and rendering are pure over a fixture index. |
| `scn.languageserver.015bb7af18f8` Hover an evidence anchor | unit | `….015bb7af18f8.unit` | `lsp_hover_test.go` | Risk: a stale pass reads as green. Pure over a fixture index with a changed digest. |
| `scn.languageserver.5c3579497b2b` Render scenario steps as a card (new) | unit | `….5c3579497b2b.unit` | `lsp_hover_test.go` | Risk: the card drops a continued line, reorders steps, or loses a keyword-less step, so the doc-comment view misstates the behavior. Rendering is pure over index steps. |
| `scn.languageserver.aa36d54d66ec` Hover a scenario heading in a specification | unit | `….aa36d54d66ec.unit` | `lsp_hover_test.go` | Risk: missing tests are hidden on the spec side. Pure over a fixture index. |
| `scn.languageserver.cd81e42944ee` Show both scopes of a modified requirement | unit | `….cd81e42944ee.unit` | `lsp_hover_test.go` | Risk: reviewers see only one version of a changed requirement. Fixture with an active change and current specs. |
| `scn.languageserver.62861d0e5875` Fall back to plain text | unit | `….62861d0e5875.unit` | `lsp_hover_test.go` | Risk: raw Markdown in plain-text clients. Capability-driven rendering is pure. |
| `scn.languageserver.08b3c3878af0` Jump from an anchor to its specification | unit | `….08b3c3878af0.unit` | `internal/stele/lsp_definition_test.go` | Risk: wrong line or column (UTF-16 vs UTF-8). Pure position mapping over fixtures with non-ASCII text. |
| `scn.languageserver.3f9633e42fd4` Jump from a requirement to its implementations | unit | `….3f9633e42fd4.unit` | `lsp_definition_test.go` | Risk: missing or unordered locations. Pure. |
| `scn.languageserver.28e4bd385efa` Jump from a scenario to its tests | unit | `….28e4bd385efa.unit` | `lsp_definition_test.go` | Risk: one level's tests are missed. Pure. |
| `scn.languageserver.100060104a1b` Find the uses of a requirement (new) | unit | `….100060104a1b.unit` | `lsp_definition_test.go`, beside definition (same lookup) | Risk: a scenario's tests or a scope's heading are missing, or a location repeats. Pure over a two-scope fixture index. |
| `scn.languageserver.e42ad028df53` Find the tests of a scenario from one of its tests (new) | unit | `….e42ad028df53.unit` | `lsp_definition_test.go` | Risk: an evidence ID is not reduced to its scenario, so sibling levels are missed, or the declaration leaks in. Pure. |
| `scn.languageserver.f584d3ec6c44` Show run actions and status on a scenario | unit | `….f584d3ec6c44.unit` | `internal/stele/lsp_codelens_test.go` | Risk: wrong targets or a misleading badge. Lens building is pure over a fixture index. |
| `scn.languageserver.f1cf94d0c1d6` Summarize a requirement (reworded: combined like the execution verdict) | unit | `….f1cf94d0c1d6.unit` | `lsp_codelens_test.go` | Risk: "not run" is hidden by a pass, or the editor combines differently from `verdicts.execution`. Pure aggregation through the shared rule. |
| `scn.languageserver.cdca0ea9f34c` Run one evidence entry from a test | unit | `….cdca0ea9f34c.unit` | `lsp_codelens_test.go` | Risk: the test lens runs other levels. Pure. |
| `scn.languageserver.28e44f61a51f` Summarize the linked scenario above a test (new) | unit | `….28e44f61a51f.unit` | `lsp_codelens_test.go` | Risk: the lens cuts a multi-byte character, varies in length, or shows the wrong steps. Pure over index steps with non-ASCII text. |
| `scn.languageserver.53ed0e506ce6` Show the specification from a summary lens (new) | unit | `….53ed0e506ce6.unit` | `lsp_codelens_test.go`, with an in-process session | Risk: clients without `window/showDocument` get nothing. Two in-process sessions with different capabilities record the server-to-client request. |
| `scn.languageserver.074136f1f47f` Run a scenario from its heading | unit | `….074136f1f47f.unit` | `internal/stele/lsp_run_test.go`, with the runner's recording stub | Risk: extra tests run or other outcomes are lost. The recording stub plus a real temporary evidence file show both; real test execution is already covered by `link-index`'s runner evidence. |
| `scn.languageserver.a12e36dfdfb8` Reject an unknown target | unit | `….a12e36dfdfb8.unit` | `lsp_run_test.go` | Risk: a typo runs everything. The stub proves nothing ran. |
| `scn.languageserver.3c56123372c1` Cancel a running command (reworded: teardown, no evidence written) | unit | `….3c56123372c1.unit` | `lsp_run_test.go`, with a helper process that blocks until killed and a recording teardown | Risk: orphaned test processes, a leaked group environment, or a half-written evidence file. A real child process started through the scheduler's process-group code, in process, with the evidence file's bytes compared before and after; no editor is involved. |
| `scn.languageserver.ddba3d8c9626` Refuse a second concurrent run | unit | `….ddba3d8c9626.unit` | `lsp_run_test.go` | Risk: two runs race on the evidence file. A blocking stub makes the overlap deterministic. |
| `scn.languageserver.159800f00b8d` Report progress while tests run (new) | unit | `….159800f00b8d.unit` | `lsp_run_test.go`, with an in-process session | Risk: progress never ends, counts go backwards, or a failure is not named, so the editor looks hung. The recording stub emits observer events; the session records `$/progress`. |
| `scn.languageserver.dfffd1e421b4` Report the verdicts of the scope after a run (new) | unit | `….dfffd1e421b4.unit` | `lsp_run_test.go` | Risk: a green run hides a linkage failure, the old `verdict` confusion in the editor. Pure over stubbed outcomes and a fixture scope. |
| `scn.languageserver.cbace7f5db1a` Use the project's execution settings (new) | unit | `….cbace7f5db1a.unit` | `lsp_run_test.go`, with `fast-runs`' batch and group-command seams | Risk: the server builds its own run path and skips groups, setup, or worker IDs, so container tests collide. The seams record the job count, setup, teardown, and environment; real processes are covered by `fast-runs`. |
| `scn.languageserver.e6a664740343` Flag an unknown ID in an anchor | unit | `….e6a664740343.unit` | `internal/stele/lsp_diagnostics_test.go` | Risk: typos stay invisible, or the code differs from the CLI's. Pure over fixtures, asserting the shared code. |
| `scn.languageserver.e33a998bdf85` Flag approved evidence without a test (reworded: the CLI's severity) | unit | `….e33a998bdf85.unit` | `lsp_diagnostics_test.go` | Risk: approved but unimplemented evidence is silent in the editor, or its severity differs from `stele verify`. Pure. |
| `scn.languageserver.72a3e3ef9ecb` Flag stale results | unit | `….72a3e3ef9ecb.unit` | `lsp_diagnostics_test.go` | Risk: old greens look current. Pure with a changed digest. |
| `scn.languageserver.106cfbc87c5b` Clear a fixed problem | unit | `….106cfbc87c5b.unit` | `lsp_diagnostics_test.go` | Risk: diagnostics linger after a fix. In-process session over `io.Pipe` checks the empty republish. |
| `scn.languageserver.3efdcf7c5cf2` Explain a finding with the diagnostic catalogue (new) | unit | `….3efdcf7c5cf2.unit` | `lsp_diagnostics_test.go` | Risk: editor wording drifts from the terminal report, or the fix names the wrong scope. Pure over the catalogue. |
| `scn.languageserver.2d35637c302f` Show annotation problems in a specification (new) | unit | `….2d35637c302f.unit` | `lsp_diagnostics_test.go` | Risk: unannotated or unsupported specs are silent, or the policy severity is ignored. Pure over fixture files. |
| `scn.languageserver.8431050348d1` Flag a test anchored to removed behavior (new) | unit | `….8431050348d1.unit` | `lsp_diagnostics_test.go` | Risk: tests for removed behavior stay green in the editor. Pure over a fixture change with a REMOVED requirement. |
| `scn.languageserver.5df656e62280` Check a change in planning at the proposal stage (new) | unit | `….5df656e62280.unit` | `lsp_diagnostics_test.go` | Risk: a change being planned is flooded with expected findings, or never switches to implementation checks. Fixture plan before and after an approval. |
| `scn.languageserver.4edc5f1a06b4` Offer the annotation quick fix (new) | unit | `….4edc5f1a06b4.unit` | `internal/stele/lsp_codeaction_test.go` | Risk: the fix is missing, not preferred, or not linked to its diagnostic. Pure over a fixture document. |
| `scn.languageserver.fe68c635aa85` Write the same bytes as stele annotate (new) | unit | `….fe68c635aa85.unit` | `lsp_codeaction_test.go` | Risk: the edit loses a CRLF, lands before a byte order mark, or adds a stray newline, so `stele annotate --check` disagrees. The edit is applied in process and compared with `insertAnnotation`. |
| `scn.languageserver.4b2f05995999` Leave broken annotations to a person (new) | unit | `….4b2f05995999.unit` | `lsp_codeaction_test.go` | Risk: a second annotation is added above a broken one. Pure. |
| `scn.languageserver.53c81eb42bf4` Apply the fix through a command (new) | unit | `….53c81eb42bf4.unit` | `lsp_codeaction_test.go`, with an in-process session | Risk: clients without code action literals cannot use the fix. The session records `workspace/applyEdit`. |
| `scn.languageserver.ba15e715dc8b` Reflect an unsaved edit | unit | `….ba15e715dc8b.unit` | `internal/stele/lsp_watch_test.go` | Risk: the server reads disk instead of the buffer. In-process session with `didChange` only. |
| `scn.languageserver.4aa8ef7297c3` Pick up results from a terminal run | unit | `….4aa8ef7297c3.unit` | `lsp_watch_test.go` | Risk: badges stay old after a CLI run. In process: rewrite the evidence file and send `didChangeWatchedFiles`, assert refresh and republished diagnostics. |
| `scn.languageserver.39afeb90a2e9` Pick up a new specification file | unit | `….39afeb90a2e9.unit` | `lsp_watch_test.go` | Risk: new specs are invisible until restart. In process with a created file event. |
| `scn.languageserver.a8fc0b7b6625` Watch files without client support | unit | `….a8fc0b7b6625.unit` | `lsp_watch_test.go`, with an injected clock for the poller | Risk: editors without dynamic registration (several built-in clients) never refresh. The poller is exercised in process with a fake clock, deterministically. |
| `scn.languageserver.82c1741b92aa` Match a full rebuild after incremental updates | unit | `….82c1741b92aa.unit` | `lsp_watch_test.go` | Risk: cache bugs make the editor disagree with the CLI. Seeded pseudo-random edit sequences compared with a from-scratch build, pure. |
| `scn.languageserver.8d66c5680555` Answer identically for identical state | unit | `….8d66c5680555.unit` | `lsp_server_test.go` | Risk: map order leaks into responses. Two in-process sessions compared byte for byte; the CLI's own determinism e2e already covers cross-process environment effects. |
| `scn.languageserver.bc6723089290` Match the command-line index (reworded: an unannotated spec in the fixture) | e2e | `….bc6723089290.e2e` | `tests/cli.test.mts`, next to `link-index`'s index tests | Distinct risk: the shipped binary's two entry points (`stele lsp` and `stele index`) drift in flags, scope, `specFiles`, or encoding. Only running both through the installed executable shows the editor and CI see the same thing. |
| `scn.languageserver.300e9d84bbbd` Ignore the client identity | unit | `….300e9d84bbbd.unit` | `lsp_server_test.go` | Risk: editor-specific branches creep in. Two in-process sessions with different `clientInfo`. |

Requirement implementation anchors are checked during implementation, not planned. Suggested homes: `lsp_rpc.go` and `lsp_server.go` (transport), `lsp_hover.go`, `lsp_definition.go` (navigation and references), `lsp_codelens.go`, `lsp_run.go`, `lsp_diagnostics.go`, `lsp_codeaction.go`, `lsp_watch.go`, and `lsp_server.go` (determinism).

## Risks / Trade-offs

- [`fast-runs` is planned, not built, and may still change] → Only the run feature depends on it (task 0.3), and the dependency table names exactly what is consumed. Hover, navigation, CodeLens, diagnostics, and the quick fix can ship without it.
- [Hand-rolled protocol code can have subtle bugs] → The subset is small, fully unit-tested at 100% coverage, and the e2e session exercises real framing. The revisit trigger names the path to generated types.
- [The `verifyScope` split and the atomic evidence write touch the core verification path] → Both are refactors with no change in output, guarded by the existing verify, validate, test, and self-verification tests before any server code uses them (tasks 2.1 and 2.2).
- [The stage rule (approved entry → implementation) can be wrong for a change] → It matches the Stele workflow, where approval comes before implementation. A change that is approved but not started shows its missing anchors, which is true. Open question 1 asks whether to make it configurable.
- [Whole-repository staleness makes every status stale after any save] → Stale is informational and shown softly (`stale` suffix, information severity). Finer digests are a follow-up owned by the index, not the server.
- [More diagnostics than the first draft] → The editor now shows everything `stele check` would fail on (except `PLAN_UNAPPROVED`). That is the point: no surprise in CI. Severity comes from the CLI, and the annotation policy still governs `SPEC_ANNOTATION_MISSING`.
- [Large repositories make rebuilds slow] → Only changed files are re-parsed; index building and checks are linear in links. Debounce keeps typing responsive. If needed later, cache anchor scans per file digest.
- [Polling fallback costs CPU] → Only used when the client cannot watch; stat-only, every 2 seconds, over the configured directories.
- [Two runs from CLI and editor at once] → The server serializes its own runs; the atomic evidence writer plus merge by evidence ID keeps concurrent CLI runs from corrupting the file. The last writer wins per evidence entry, which is documented. Two runs with group setups may contend on the same containers; `fast-runs`' worker isolation is per run, so the editor guide recommends not running container groups from both at once.
- [Clients differ in CodeLens support] → CodeLens is optional in LSP; Zed has it behind a setting and JetBrains only from 2026.1 (see the `stele-editors` plans). Run actions stay reachable through `workspace/executeCommand`, and hover shows the status too.
- [Clients differ in how they present a byte order mark] → The quick fix computes the edit from the text the client sent, so it matches the client's own view of the document; the byte comparison test covers both forms.

## Migration Plan

Additive: a new subcommand and two catalogue entries that only the server emits. No existing output changes. Rollback is removing the subcommand; editor clients check `stele lsp --help` exit status or the `initialize` result before enabling features. The run feature ships in the first release that contains both this change and `fast-runs`.

## Scope decision (2026-09-18)

The run feature assumed a full parallel `fast-runs` with `execution.jobs` and `execution.groups`. The reviewer (jensvansteen, in chat on 2026-09-18) has since trimmed `fast-runs` to batching only: sequential batches without configuration, with parallelism and groups deferred. Therefore:

- The test-running part is **held back**: tasks section 6, Decision 8, the run lenses (`▶ Run all`, `▶ <level>`, and `▶ Run` on test anchors), the `stele.runTests` command, `$/progress`, and cancellation. It will be re-planned and re-approved after `fast-runs` merges. Its scenarios and approved entries stay as they are until then.
- Everything else is implemented now: the stdio JSON-RPC transport and lifecycle, the workspace state and incremental rebuild (file watching with the polling fallback), the core refactor (`verifyScope` split into loading and a pure `verifyLoaded`, atomic evidence writes), hover (including the step cards), definition in both directions, references, the CodeLens status and summary lenses, diagnostics from every verify finding with the catalogue's meaning and fix, the "Add Stele annotation" quick fix and its command fallback, determinism, and the `stele/index` request.
- Consequence: until section 6 is done, the approved evidence of the held-back scenarios has no tests, so `stele check --change editor-language-server` reports them as missing, and the change does not merge before section 6 is complete. Besides the seven scenarios of "Run selected tests and merge their results" (and `LINK_CODE_MISSING` for that requirement), that includes the three CodeLens scenarios whose outcome is a run action: `scn.languageserver.f584d3ec6c44` (Show run actions and status on a scenario), `scn.languageserver.f1cf94d0c1d6` (Summarize a requirement, whose lens carries "Run all"), and `scn.languageserver.cdca0ea9f34c` (Run one evidence entry from a test). The status lenses those scenarios also describe are implemented and tested; their evidence is anchored once the run lenses exist.

## Open Questions

All decided by the reviewer on 2026-09-18:

1. **Stage per change.** Decided: the rule as proposed (an active change is checked at the implementation stage once its plan has an approved entry, at the proposal stage before that). `stele check --stage proposal` is a separate follow-up.
2. **References in v1.** Decided: yes, references ship in v1.
3. **More quick fixes.** Decided: follow-ups later; v1 has only "Add Stele annotation".
4. **Summary lens length.** Decided: 120 characters is fine.
5. **Ship before `fast-runs`?** Decided: ship without the run lenses first (see the scope decision above).
6. Debounce and polling intervals (100 ms, 2 s): decided to tune later with real repositories.
