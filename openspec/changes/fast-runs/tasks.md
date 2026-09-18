## 0. Approval

- [x] 0.1 Get the reviewer's explicit approval of the verification levels in design.md (proposed, awaiting approval), the decisions, and the open questions. Only then run `stele approve --change fast-runs --confirmed-in-chat`, mark the design table approved, and verify `stele check --change fast-runs` reports no `PLAN_UNAPPROVED`. Start no task in sections 1 to 5 before this.
- [x] 0.2 Record the answers to the open questions in design.md. If an opt-out or crash re-run is accepted, update the specs and plan with `openspec-update-change` first, and re-run `stele check --change fast-runs`.
- [x] 0.3 If main has moved, rebase onto it, re-run `stele ids --change fast-runs` and `stele annotate --change fast-runs --check`, and check that the MODIFIED blocks still copy the current `execution` and `go-support` requirements exactly.

## 1. Batch planning

- [x] 1.1 Add `batch.go` with batch keys (module, package, tag set for Go; file for Node), sorted and escaped names joined into `^(…)$`, and a fixed batch order; verify the unit tests for `0aa78cf92f34` and the existing `a9dde959cf57` pass.

## 2. Results and completion

- [x] 2.1 Add streaming readers: for `go test -json`, the last final action per exact top-level name and the binary's final `PASS` or `FAIL` line; for Node TAP, `ok`, `not ok`, `# SKIP`, the `# tests` summary, and a failed file entry. Apply the completion table from design Decision 2; verify the integration tests for `500afe3889f6`, `8653331d296a`, and `aba48bf93edd` pass.
- [x] 2.2 Run batches from `executeTestGroups`, keep `executeExactTest` (on the shared readers) as the per-test oracle, and re-point the existing Go and Node execution tests to the batch runner; verify the existing tests for `bce9246e1444`, `8371d74b5134`, `d195095fc292`, `208b6a95ea3f`, `75d202e0ed36`, `203331e4d990`, and `4a0637d44b7a`, and the e2e test for `2c88ed381429`, pass.
- [x] 2.3 Add the integration test that runs the same Go and Node fixtures batched and one per test, and compares outcomes, reasons, the evidence file, and `--json` byte for byte; verify the test for `e49b224babb8` passes five times in a row.

## 3. Progress

- [x] 3.1 Emit `started` for a batch's tests, `finished` as each result streams in, and the new `batchEnded` event for a batch that did not complete (exit status, last output lines in the human report only); verify the unit test for `93ab1bbe751c` and the existing progress and report tests pass.

## 4. Documentation

- [x] 4.1 Document batching and its process semantics, crashed batches, and the Node `test-skipped` reason in the CLI reference, and add the changelog entry; verify `npm run docs:build` and `npm run verify` pass.

## 5. Re-measure and close

- [x] 5.1 Repeat the baseline measurement from design.md on the same machine with the implemented binary: `validate --specs` warm (three runs) and cold, plus the stage breakdown. Add an "After" column, and verify the warm run is at most 40 s (expected about 30 s against about 95 s) and the cold run at most 55 s (expected about 42 s against about 111 s).
- [x] 5.2 Add `@implements` anchors for the requirements listed in design.md, run `stele check --change fast-runs`, and fix every failure before reporting the change done.
