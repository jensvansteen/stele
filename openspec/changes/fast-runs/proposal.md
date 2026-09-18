## Why

`stele validate --specs` on this repository takes about 95 seconds warm and 111 seconds cold, and 99% of that is test execution (see the baseline in design.md). The runner starts one process per linked test: 157 `go test` processes and 15 `node --test` processes for 173 tests, and each process builds, links, and starts its test binary again. Batched by Go package and by Node file, the same tests take about 30 seconds warm and 42 seconds cold, still one process at a time.

## What Changes

- **Batching.** One test process per Go package and build tag set, and one per Node test file, holding all selected exact test names:
  - Go: `go test -json -count=1 -run '^(A|B|C)$' [-tags …] ./pkg`
  - Node: `node --test --test-reporter=tap --test-name-pattern='^(a|b|c)$' file`

  Batches still run one at a time.
- **Per-test outcomes.** Each test's outcome comes from its own Go `-json` events or Node TAP line. Skipped and not-executed tests are still detected per test.
- **Crashed batches.** A batch that does not reach its end-of-run summary has crashed. Its tests without a result fail with `test-process-failed`. The batch and its exit status appear in progress and in the human report, never in evidence.
- **Node skipped tests** fail with `test-skipped` instead of `test-not-executed`, as Go skipped tests already do.
- **Progress.** Batching uses the existing `terminal-report` observer: a test's result is reported as soon as the batch output shows it, failures appear immediately, and a file's line appears when its last test has a result.
- **Determinism.** Evidence, report files, `--json`, and the final report are identical to those of one process per test, for tests that do not crash. An integration test compares the two.

There are no new flags or configuration. Parallel execution, execution groups with setup and teardown, worker identity, overlapping stages, timeouts, and interruption handling are deferred to a future change (design.md, "Deferred (future change)").

Expected result on this repository: a warm `validate --specs` drops from about 95 seconds to about 30 seconds, and a cold one from about 111 seconds to about 42 seconds.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `execution`: a linked test still runs exactly, but it may share its process with other selected tests of the same Go package or Node file. A new requirement defines batches, per-test outcomes, crashed batches, and per-test progress.
- `go-support`: a Go test shares its `go test` process only with selected tests of the same package and build tags.

## Impact

- `internal/stele`:
  - a new `batch.go` with batch planning and streaming result readers;
  - `runner.go` and `gorunner.go` run batches, and `executeExactTest` stays as the per-test test oracle;
  - the observer gets a batch-end event for crash lines.
- Evidence schema version 3 is unchanged. Node skips change reason from `test-not-executed` to `test-skipped`.
- Test semantics change for tests that depend on running alone in their process, such as Go package-level state or Node module state. Batches match how `go test ./pkg` and `node --test file` already run them.
- Documentation: the CLI reference (batching, the Node skip reason, crashed batches) and the changelog.
