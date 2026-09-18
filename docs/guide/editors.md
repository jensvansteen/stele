# Editor integration

`stele lsp` puts Stele's links in front of people where they read and change code. It is a [Language Server Protocol](https://microsoft.github.io/language-server-protocol/) server in the same binary and npm package as the CLI, so VS Code, Zed, JetBrains IDEs, Neovim, Helix, and any other LSP client can use it, and an editor plugin only has to start it.

In the editor you can:

- hover an `@implements` or `@verifies` anchor and read the requirement or the scenario's steps, the way a doc comment shows up on hover;
- jump from an anchor to its specification, and from a requirement or scenario heading to its implementations and tests;
- find every implementation and test of a behavior;
- see each heading's test status, and a summary of the linked behavior above every anchor;
- see the findings of `stele check` while you type, with the same meaning and fix;
- add the Stele annotation to a specification with a quick fix.

This page describes the protocol surface for people who write or configure a client. A diagnostic in the editor is the same finding `stele check` reports; the editor never has rules of its own.

## Start the server

```bash
npx stele lsp
```

The client starts the command and talks to it over standard input and output with `Content-Length` framing. Standard output carries only protocol messages; logs go to standard error. The server serves each workspace folder from `initialize` (or `rootUri` without folders) that contains `openspec/` or `stele.config.json`, with every scope, like `--all`: the current specifications and every active change. Other folders are ignored with a line on standard error.

The `initialize` result names the server `stele` with the version that `stele --version` prints. A request before `initialize` gets the "server not initialized" error, and a malformed message gets a JSON-RPC parse error without ending the session. The process exits with code `0` on `exit` after `shutdown`, and with code `1` on `exit` or the end of input without `shutdown`.

A minimal client configuration, for Neovim:

```lua
vim.lsp.start({ name = "stele", cmd = { "npx", "stele", "lsp" }, root_dir = vim.fs.root(0, { "openspec", "stele.config.json" }) })
```

## Features

| Feature | Method | Where |
|---|---|---|
| Hover card | `textDocument/hover` | The ID of an anchor; a requirement or scenario heading or its `Verification-ID` line |
| Definition | `textDocument/definition` | From an anchor to the heading that declares its ID in every scope; from a requirement to its `@implements` anchors; from a scenario to the `@verifies` anchors of its evidence |
| References | `textDocument/references` | A requirement's `@implements` anchors and its scenarios' `@verifies` anchors; for a scenario or an evidence ID, the scenario's `@verifies` anchors; with `includeDeclaration`, the headings in every scope |
| CodeLens | `textDocument/codeLens` | A status lens on every heading with planned evidence or tests, and a summary lens on every anchor |
| Quick fix | `textDocument/codeAction` | "Add Stele annotation" on a `SPEC_ANNOTATION_MISSING` diagnostic |
| Diagnostics | `textDocument/publishDiagnostics` | Every file with a finding |
| Index | `stele/index` | The whole link index |

Locations are ordered by path and line, each once. An ID that no specification declares has no definition; its references are the anchors that name it.

### Hover cards

On an anchor, the card shows the requirement's title, scope, and text, or the scenario's title, scope, and steps, one line per step in specification order with the keyword emphasized:

```markdown
**Scenario:** Save entered text · `todo-basics`

- **WHEN** a user enters "milk"
- **THEN** the list shows "milk"
- **AND** the input is cleared

---

`unit` · approved · passed (stale)
```

An evidence anchor such as `@verifies scn.todo.591a3b429cf0.unit` adds its evidence entry's level, approval, and last outcome, marked `(stale)` when the verified inputs changed since the run. When a requirement or scenario exists in several scopes, for example in the current specifications and in a change that modifies it, the hover shows one card per scope, labelled.

On a heading, the card lists where the behavior is implemented and tested: for a scenario each evidence entry with its level, approval, outcome, and test locations, or `planned, no test`; for a requirement its implementations and every scenario's evidence.

### CodeLens

A scenario heading shows each level's combined outcome, unapproved levels, and a stale suffix, such as `unit ✓ · e2e ✗ · stale` or `unit – not run (unapproved)`. A requirement heading combines its scenarios with the rule of the execution verdict, so the editor never shows a pass that `stele verify` would call not run or stale: `not run · 1 passed · 1 not run`. The status lens runs `stele.showStatus`.

Above each anchor, a summary lens shows the linked behavior, cut to 120 characters: `Scenario: Save entered text — WHEN a user enters "milk" · THEN the list shows "milk"`, or `Requirement: Add a todo — <first line of its text>`. It runs `stele.showSpecification`, which opens the heading.

Run actions (`▶ Run all`, one per level, and `▶ Run` on test anchors), progress, and cancellation are not part of this release; they follow after the `fast-runs` change. Run tests with `stele test <id>` in a terminal meanwhile: the server picks up the new outcomes.

### Diagnostics

The server publishes, per file, the findings that `stele verify` reports for every served scope, with the same code and severity. A finding without a location goes on the heading of the requirement or scenario it names. Each message contains the finding's own message, then the meaning and the fix from the diagnostic catalogue that the terminal report prints, with the fix naming the scope (`--specs` or `--change <id>`):

```text
No test anchor resolves for evidence scn.todo.591a3b429cf0.e2e (e2e).
No test has `@verifies` for an approved evidence entry.
Fix: add `@verifies <evidence-id>` above the test the plan's placement names
```

`code` is the Stele code and `source` is `stele`. The stage of each scope follows the Stele workflow:

| Scope | Checked at |
|---|---|
| Current specifications | `implementation` |
| Active change whose `linkage-plan.json` has an approved entry | `implementation` |
| Active change without an approved entry | `proposal` |

`PLAN_UNAPPROVED` is never published: it is expected on every entry while a change is planned, and the status lens marks unapproved entries instead. Two codes come from the stored test outcomes, on the scenario heading and on each test anchor of the entry: `EXECUTION_FAILED` (warning) when the last run failed, and `EXECUTION_STALE` (information) when it ran before the verified inputs changed. A finding that several scopes report at the same place is published once. When a problem is fixed, the file gets an empty list.

### The annotation quick fix

For a `SPEC_ANNOTATION_MISSING` diagnostic, `textDocument/codeAction` offers one preferred `quickfix`, "Add Stele annotation", linked to the diagnostic. Its edit inserts `<!-- stele: spec v1 -->` as the first line, after a byte order mark if there is one, ended with the file's first line ending, or a line feed without one: exactly the bytes `stele annotate` writes. The edit uses `documentChanges` with the open document's version when the client supports it, and `changes` otherwise. A file with a malformed, unsupported, or misplaced annotation gets no fix; `stele annotate` leaves those to a person too.

## Commands

Every command runs on the server; a client only forwards the click. `workspace/executeCommand` takes one argument object:

| Command | Argument | Effect |
|---|---|---|
| `stele.showStatus` | `{ "root": "<project root URI>", "id": "req.… or scn.…" }` | `window/showMessage` listing each evidence entry |
| `stele.showSpecification` | `{ "root", "id" }` | `window/showDocument` for the heading, or `window/showMessage` with the plain-text card |
| `stele.addAnnotation` | `{ "uri": "<spec file URI>" }` | `workspace/applyEdit` with the annotation edit |

## Capabilities and fallbacks

The server adapts only to the capabilities the client declares, never to the client's name or version:

| Client capability | With it | Without it |
|---|---|---|
| `textDocument.hover.contentFormat` includes `markdown` | Markdown card | Plain-text card |
| `workspace.didChangeWatchedFiles.dynamicRegistration` | The client watches the files | The server compares file sizes and modification times every 2 seconds |
| `workspace.codeLens.refreshSupport` | `workspace/codeLens/refresh` after files change on disk | Lenses refresh on the next request |
| `window.showDocument.support` | Show the heading | Show the card as a message |
| `textDocument.codeAction.codeActionLiteralSupport` | Code action with its edit | The `stele.addAnnotation` command |
| `workspace.workspaceEdit.documentChanges` | Versioned edit | `changes` |
| `general.positionEncodings` includes `utf-8` | UTF-8 columns | UTF-16 columns |

The server advertises `experimental.stele` with `{"diagnostics": "publish", "requests": ["stele/index"]}`.

## Keeping current

The server reflects open documents, including unsaved edits, and the files on disk: specifications, sources, tests, linkage plans, `stele.config.json`, and the evidence file that `stele test` writes in a terminal. It uses full document sync. A change re-reads only the changed file, rebuilds 100 ms after the last change or before the next request, and the result equals a full rebuild of the same state. Whether an outcome is stale is decided from the saved files, because tests run against them: typing does not make results stale, saving does. When a file cannot be read or parsed, for example a Go file in the middle of an edit, the server keeps its previous state and logs the reason on standard error.

Identical workspace content and identical requests give byte-identical responses and diagnostics, without timestamps.

## The `stele/index` request

```json
{ "jsonrpc": "2.0", "id": 7, "method": "stele/index", "params": { "root": "file:///path/to/project" } }
```

The result is the document that `stele index --json --all` prints for the saved files, byte for byte, in the [link index](/reference/link-index) format. Clients can build their own views, such as a test explorer, from it without Stele logic.
