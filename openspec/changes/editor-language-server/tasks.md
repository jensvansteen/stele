## 0. Approval and prerequisites

- [x] 0.1 Get the reviewer's explicit approval of the verification levels in design.md (Verification strategy, proposed, awaiting approval), the decisions, and the open questions. Only after an explicit yes run `stele approve --change editor-language-server --confirmed-in-chat` and mark the design table approved; verify `stele verify --stage proposal --change editor-language-server` reports no `PLAN_UNAPPROVED`. Never approve silently, and never treat a request to implement as approval. Start no task in sections 1 to 7 before this.
- [x] 0.2 Rebase on the current `main`, re-run `stele ids --change editor-language-server`, `stele annotate --change editor-language-server --check`, and `npx openspec validate --all --strict`, and re-check the merged rows of the dependency table in design.md against the code (`BuildIndex`, `insertAnnotation`, `diagnosticGuides`, `reportVerdicts`, `aggregateExecution`, `testObserver`, `Version`); re-approve any entry that `stele verify --stage proposal` reports as `PLAN_APPROVAL_STALE`
- [x] 0.3 Before section 6: confirm `fast-runs` is merged on `main` (`git log origin/main`), re-check the `fast-runs` row of the dependency table against its final design (run context, scheduler seams, observer events, interruption behavior and exit code), update design Decision 8 if it changed, and re-approve stale entries
- [x] 0.4 Record the answers to the open questions in design.md. If references are dropped from v1 (open question 2), remove their requirement and scenarios with `openspec-update-change` and re-run the proposal check

## 1. Transport and lifecycle

- [x] 1.1 Add `Content-Length` framing and a JSON-RPC 2.0 dispatcher (requests, notifications, responses to server-initiated requests, `$/cancelRequest`) using only the standard library; verify `lsp_rpc_test.go` covers parse errors and interleaved messages, and `go.mod` is unchanged
- [x] 1.2 Add the protocol types for the subset listed in design.md Decision 2 and a UTF-16/UTF-8 position helper; verify position tests with non-ASCII lines pass
- [x] 1.3 Add the lifecycle (`initialize` with capability-driven advertising and `serverInfo` from `Version`, `initialized`, `shutdown`, `exit`), workspace-folder project detection, and the `stele lsp` subcommand in `cli.go`; verify the lifecycle unit tests for the not-initialized error and exit codes pass

## 2. Core seams (no change in CLI output)

- [x] 2.1 Split `verifyScope` into `loadVerifyInput` (disk) and a pure `verifyLoaded`, and route the CLI through both; verify `npm run test:go`, the verify and validate tests, and `npm run verify:self` pass unchanged
- [x] 2.2 Write the evidence file atomically (temporary file in the same directory, then rename) for every writer; verify the bytes are unchanged and a unit test shows an interrupted write leaves the previous file intact
- [x] 2.3 Add `EXECUTION_FAILED` and `EXECUTION_STALE` to `diagnosticGuides` with the stage `Test execution`, a meaning, and a fix; verify the catalogue test (`scn.terminalreport.78fda44eee93`) still passes and the terminal report output is unchanged

## 3. Workspace state

- [x] 3.1 Keep per-file parse results for specs (with annotation classification), sources, tests, plans, configuration, and evidence, with open-document overlays, the input digest from saved files, and debounced, serialized rebuilds through `BuildIndex` and `verifyLoaded`; verify the unsaved-edit and full-rebuild-equivalence tests pass
- [x] 3.2 Register client file watchers when supported, and add the stat-based poller with an injected clock otherwise; verify the terminal-run, new-spec, and no-client-watch tests pass

## 4. Reading and navigation

- [x] 4.1 Hover cards for anchors, headings, and `Verification-ID` lines, rendering requirement text and scenario steps in Markdown or plain text, per scope; verify the hover tests, including the step-card test, pass
- [x] 4.2 Definition in both directions and `textDocument/references` with `includeDeclaration`, unique locations ordered by path and line; verify the definition and references tests pass
- [ ] 4.3 (status and summary lenses and `stele.showStatus`/`stele.showSpecification` done; the run actions are held back with section 6) CodeLens on headings (run actions and status combined with `aggregateExecution`) and on anchors (summary lens cut at 120 characters on a rune boundary, run action on test anchors), with the `stele.showStatus` and `stele.showSpecification` commands and their fallbacks; verify the CodeLens tests pass
- [x] 4.4 The `stele/index` request over the saved state; verify the determinism and client-identity unit tests pass

## 5. Diagnostics and the annotation quick fix

- [x] 5.1 Diagnostics from `verifyLoaded` per served scope at the stage chosen from the plan, without `PLAN_UNAPPROVED`, with the catalogue meaning and scope-specific fix in the message, deduplicated across scopes, plus `EXECUTION_FAILED` and `EXECUTION_STALE`, cleared when fixed; verify the diagnostics tests pass
- [x] 5.2 The "Add Stele annotation" quick fix from `insertAnnotation`, with `documentChanges` or `changes`, not offered for broken annotations, and the `stele.addAnnotation` command with `workspace/applyEdit` for clients without code action literals; verify the code action tests, including the byte comparison with `insertAnnotation`, pass

## 6. Running tests (re-planned 2026-09-18 against the merged batching runner; awaiting approval)

- [ ] 6.0 Get the reviewer's explicit approval of the re-planned entries (`….3c56123372c1.integration`, `….40c7e3ae9243.unit`) and of Decision 8; only after an explicit yes run `stele approve --change editor-language-server --confirmed-in-chat` for them, and verify `stele verify --stage proposal --change editor-language-server` reports no `PLAN_UNAPPROVED` or `PLAN_APPROVAL_STALE`
- [ ] 6.1 `stele.runTests` through `runScopeTests` with targets and merge, one call per affected scope, target validation (`-32602` naming the target, nothing runs), and a one-run-per-project guard (`-32803` naming the running targets); verify the run, unknown-target, concurrent-run, and verdict tests pass
- [ ] 6.2 A progress adapter implementing `testObserver` and `batchObserver`, with a client or server-created work-done token; verify the progress test and the incomplete-batch test pass
- [ ] 6.3 An opt-in `cancel` context on `testRequest`: batch commands in their own process group only when it is set, the group killed on cancellation, no further batch, `errRunCancelled` before evidence is assembled or written, and `RequestCancelled` for `$/cancelRequest` and `window/workDoneProgress/cancel`; verify the integration cancel test passes under `-race` and the CLI runner tests pass unchanged
- [ ] 6.4 The command result with sorted executions, `incomplete` batches, and per-scope `verdicts` from `verifyLoaded` and `reportVerdicts`, followed by a rebuild, CodeLens refresh, and republished diagnostics; verify the verdict test passes
- [ ] 6.5 The run lenses (`▶ Run all`, one per level, `▶ Run` on test anchors) with `stele test` targets; verify the three run-lens CodeLens tests pass, and update the editor guide and changelog

## 7. Contract tests, documentation, and gate

- [x] 7.1 Add `tests/cli.test.mts` tests for a full stdio session (including `serverInfo.version` equal to `stele --version`) and for byte equality between `stele/index` and `stele index --json --all` on a fixture with an unannotated spec; verify both pass in `npm run test:node`
- [x] 7.2 Document `stele lsp` in the CLI reference, add the two `EXECUTION_*` codes to the diagnostics reference, add an "Editor integration" guide (capabilities and fallbacks, commands and their arguments, the run result, `stele/index`, diagnostics and stages, the quick fix) for client authors, and update the changelog; verify `npm run docs:build` passes
- [ ] 7.3 (done except the anchors of section 6 and the run lenses) Add `@implements` and `@verifies` anchors for every requirement and approved evidence entry, then run `npm run verify`; verify it passes with exactly 100% core coverage and `./dist/stele check --change editor-language-server` passes
- [ ] 7.4 After a release that contains this change, archive it with `stele-archive` and verify `npm run verify:self` passes
