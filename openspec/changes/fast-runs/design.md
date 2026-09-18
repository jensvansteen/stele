## Context

See proposal.md for the problem. The current state that shapes the approach:

- **One process per test.** `groupScenarioTests` groups anchors by (path, selector), so scenarios sharing a test run it once. `executeTestGroups` then runs the groups one at a time, calling `executeTestGroup` and `runExactTest` (`executeExactTest`), which starts:
  - Go: `go test -json -count=1 -run '^Name$' [-tags …] ./pkg`. `goTestAction` reads the last `pass`, `fail`, or `skip` event of the exact top-level test.
  - Node: `node --test --test-reporter=tap --test-name-pattern='^name$' file`. The runner looks for `ok N - name` and ignores `# SKIP`, so a skipped Node test fails with `test-not-executed`.
- **Evidence is already order-independent.** Groups are sorted before they run, `assembleEvidence` sorts scenarios, and evidence has no timing fields.
- **Progress** comes from `terminal-report`, now on main (`progress.go`). `executeTestGroups` calls `observer.planned`, then `started` and `finished` per test, with events keyed by test identity. The plain reporter writes a line per failed test, and one per test file when its last test finishes. The observer is safe for concurrent use, a property this change does not need.
- **The seam style.** Collaborators are package-level function variables (`runExactTest`, `computeScenarioDigest`, `runProjectScenarios`). New seams follow it.

This plan was first drafted before `terminal-report` merged, with parallelism and execution groups. It was rebased onto `6bbb599` (rc.4, with `terminal-report` and `spec-annotation` archived) and trimmed to batching by the user's decision. The rest is recorded under "Deferred (future change)".

## Baseline measurement

**Machine:** Apple Silicon Mac, 10 logical CPUs (6 performance, 4 efficiency cores), macOS (Darwin 25.3), Go 1.27.1, Node 24.16.0. "Warm" means the default Go build cache after earlier runs; "cold" means an empty `GOCACHE`.

### Current main (`6bbb599`, rc.4), measured 2026-09-18

`./dist/stele`, built with `npm ci && npm run build`. The run selects 173 tests (172 processes today: 157 Go, 15 Node).

| Stage | Command timed on its own | Warm | After (task 5.1) |
|---|---|---|---|
| Full `validate --specs` | `./dist/stele validate --specs --json` | 94.9 s | 30.9 to 32.3 s (cold: 46.5 s) |
| OpenSpec strict validation | `node node_modules/@fission-ai/openspec/bin/openspec.js validate --specs --strict --no-interactive` | 0.49 s | 0.48 s |
| Static verification | `./dist/stele verify --specs --json` | 0.28 s | 0.06 s |
| Test execution (one process per test before, batches after) | `./dist/stele test --specs --json` | 97.8 s (cold: 110.6 s) | 31.4 s |

Batches of the same 173 selected tests, one at a time (the plan), prototyped with throwaway shell scripts:

| Batch | Tests | Warm, 3 runs |
|---|---|---|
| `internal/stele` (one package) | 157 | 16.7 to 17.4 s |
| `tests` (`-tags=integration`) | 1 | 8.9 to 10.8 s |
| `tests/cli.test.mts` | 15 | 2.9 s |
| **All three, one after another** | 173 | **28.5 to 31.1 s** (cold: 42.2 s) |

The batches reported the same outcome for every test: 157 of 157 top-level Go passes in the `-json` events, and 15 of 15 Node passes in TAP.

### Earlier measurement (`754f62c`, rc.3, 119 selected tests), measured 2026-09-18

This run broke the time down per process and prototyped parallel strategies, which are now deferred:

- Go processes averaged 0.62 s and Node processes 0.31 s.
- The 107 `internal/stele` tests reported 13.4 s of test time in their own `Elapsed` fields but took 58.3 s as 107 processes, so about 45 s was process start-up.

| Strategy | Warm | Cold |
|---|---|---|
| One process per test, one after another (as today) | 68.1 to 70.6 s | 80.8 s |
| One process per test, 10 workers | 30.3 to 31.9 s | — |
| **Batches, one after another (this plan)** | **24.3 to 26.0 s** | **37.7 s** |
| Batches, 3 workers | 14.3 to 14.5 s | 24.9 s |
| Batches, `internal/stele` split into 4 shards, 10 workers | 10.0 to 10.3 s | 31.1 s |
| Prebuilt `go test -c` binary run as 4 shards, 10 workers | 10.0 s | 24.5 s |

### After implementation (task 5.1), measured 2026-09-18

Same machine, same 173 selected tests, `./dist/stele` built from this change with `npm run build`, timed with `/usr/bin/time -p`. The run now starts three test processes: `internal/stele` (157 tests), `tests` with `-tags=integration` (1), and `tests/cli.test.mts` (15). All 173 tests passed, as before.

| Run | Before | After | Target |
|---|---|---|---|
| `validate --specs`, warm (3 runs) | 94.9 s | 30.9, 31.4, 32.3 s | at most 40 s |
| `validate --specs`, cold (empty `GOCACHE`) | 110.6 s | 46.5 s | at most 55 s |

Warm is 3.0× and cold 2.4× faster, within both targets. Cold is 4 s slower than the prototype's 42.2 s; most of the cold time is compiling `internal/stele` and the `tests` package once.

### Expected result

A warm `validate --specs` should drop from about 95 s to about 30 s (3.2×), and a cold one from about 111 s to about 42 s (2.6×). The 0.8 s of static stages stays in sequence. On rc.3 the same comparison was about 70 s to 25 s warm and 81 s to 38 s cold. Task 5.1 re-measures after implementation.

### How to reproduce

Scratch files go in the ignored `artifacts/` directory.

1. `npm ci && npm run build`, then time `./dist/stele validate --specs --json --evidence-file artifacts/m/evidence.json`, and the other commands in the stage table.
2. List the tests: `node -e 'for (const x of require("./artifacts/m/evidence.json").executions) console.log(x.path + "\t" + x.selector)' > artifacts/m/tests.tsv`.
3. For each Go package and tag set, and each Node file, join the names with `regexp.QuoteMeta` escaping into `-run '^(A|B|…)$'` (adding `-tags=integration` for `tests/`) or `--test-name-pattern='^(a|b|…)$'`. Run the commands one after another with `STELE_CHILD_TEST=1`.
4. Count the top-level `pass` events and the `ok` lines to compare outcomes.
5. Cold: repeat steps 1 and 3 with `GOCACHE=$PWD/artifacts/gocache-cold` after deleting that directory.

## Goals / Non-Goals

**Goals:**

- Remove per-test process start-up: a full run of this repository in about 30 s instead of about 95 s.
- Every test still gets its own outcome and reason. Evidence, reports, and JSON are unchanged for tests that do not crash.
- Progress still reports each test and each failure as it happens.

**Non-Goals:**

- Anything in "Deferred (future change)" below: parallel execution, configuration, worker identity, overlapping stages, timeouts, interruption handling, and splitting batches.
- Running tests inside one process in parallel (`t.Parallel`, Node `concurrency`). Stele passes no flags that change this.
- Caching test results across runs. `-count=1` stays.

## Decisions

### 1. Batches: package and tag set for Go, file for Node

A new `batch.go` turns the selected test groups (`testGroup`, unchanged) into batches keyed by:

- Go: module root, package directory, and the sorted custom build tags from `goBuildTags`. Files whose constraint does not hold on this platform keep today's outcome, `test-not-executed`, without starting a process.
- Node: the file path.

Names are sorted, escaped with `regexp.QuoteMeta`, and joined into `^(A|B|…)$`. Batches run one at a time, sorted by key (path, then tags), so the order is fixed. `testGroup` still carries scenario and evidence IDs, and the batch maps each exact name back to its groups. A test shared by several scenarios still runs once.

The argument length is not a practical limit: this repository's largest batch is 157 names in under 6 KB, far below `ARG_MAX`.

*Alternative:* one `go test ./pkg1 ./pkg2 …` for the whole module. It was rejected: Go would then run packages in parallel (`-p`), which is the deferred feature, arriving without its controls.

### 2. Per-test results and batch completion

The Go reader extends `goTestAction` to every selected name, in one streaming pass over the `-json` output. It keeps the last `pass`, `fail`, or `skip` of each exact top-level test. It also records whether the test binary printed its final summary line: an `output` event without `Test` whose text is exactly `PASS` or `FAIL`.

The Node reader matches, at any indentation:

- `ok N - name`;
- `not ok N - name`;
- `ok N - name # SKIP …`.

It also records whether the TAP ended with the `# tests` summary, and without a failed entry named after the test file itself.

Probing crashes while planning showed why completion must be explicit:

- A Go test that panics gets a `fail` event, but later tests of the batch never start and get no event. The binary prints no final `PASS` or `FAIL` line.
- A Go test that calls `os.Exit` gets a `run` event and no final event, and the final line is missing too.
- A Node test that calls `process.exit` loses the results of the whole file: TAP shows only `not ok 1 - <file>`.

So:

| Batch | Test has a final result | Test has none |
|---|---|---|
| Completed | its result: passed, `test-process-failed`, or `test-skipped` | `test-not-executed` |
| Did not complete (crash, early exit, build failure, killed) | its result | `test-process-failed` |

A passed test stays passed even when the process exits non-zero because another test failed. Today a pass also needs a zero exit status. That is the only way to keep per-test outcomes in a shared process, and the failing sibling is reported on its own.

Crash details (the exit status, and the last 20 lines of output) go to the observer and the human report, never into evidence, because panics print addresses and timings.

### 3. Node skipped tests fail with `test-skipped`

Batching needs the new reader anyway, and it maps `# SKIP` to `test-skipped`, as Go does. The outcome is still failed; only the reason becomes accurate. `# TODO` lines keep the result they report.

### 4. Progress through the existing observer

The runner keeps the observer contract:

- `planned` once, with the per-file counts;
- `started` for every test of a batch when its process starts (group `default`, slot `0`);
- `finished` per test as soon as the streaming reader sees its final result, so a failure is printed before the batch ends;
- when the process ends, `finished` for every test still without a result, with its completion outcome.

The plain reporter's per-file lines therefore still appear when a file's last test finishes. A Go package batch produces one line per file, in the order the package's tests complete.

One small addition: an optional `batchEnded(path, tags, exitStatus, completed)` event. It prints one line, and adds a report entry, for a batch that did not complete. The terminal line shows the running file instead of a single test while a batch runs. It is rendered from counters, as before.

### 5. Determinism, and the per-test oracle

- Executions are keyed by `testGroupKey` and assembled in `groups` order. Batch order never reaches evidence. `Runner` stays `stele-go/exact-scenario`.
- `executeExactTest` stays in the code as the per-test oracle. It uses the same readers, so the Node skip mapping matches in both paths.
- The integration test (`e49b224babb8`) runs the same Go and Node fixtures (pass, fail, skip, missing name, build tag) both ways. It compares the per-test outcomes and reasons, and the bytes of the evidence file and the `--json` output.
- Crashing tests are excluded from the byte comparison by design (Decision 2): alone, a test after a crash would pass.

### 6. The packaging test and `dist/`

`tests/package_install_test.go` runs `npm pack`, whose `prepack` runs `npm run build`. That script deletes and recreates `dist/` (`rmSync(dist)`, then `go build -o dist/stele-<platform>` and a copy to `dist/stele`). `tests/cli.test.mts` executes `dist/stele`.

With batches still running one at a time, the two never overlap, exactly as with one process per test today. The Node file batch finishes before the Go `tests` batch starts, or the other way round, and each rebuild produces the same binary from the same sources. No configuration is needed.

This becomes a real conflict only with parallelism, which is why the deferred design keeps a `jobs: 1` group pattern for it.

### 7. Code layout

- `batch.go`: keys, planning, command arguments, streaming readers, and completion.
- `gorunner.go` and `runner.go`: the Go and Node batch commands. `executeTestGroups` iterates batches, and `executeExactTest` stays as the oracle.
- `progress.go`: the `batchEnded` event.

## Verification strategy

**Status: approved** by jensvansteen on 2026-09-18, before implementation started (via: cli). All 15 entries are approved in `linkage-plan.json`.

Placement follows AGENTS.md: co-located Go tests in `internal/stele`, and CLI tests in `tests/cli.test.mts`. Placement is advisory. Levels:

- **integration:** every risk that sits in real `go test -json` or `node --test` output.
- **unit:** pure batch planning, and progress over a fake output stream.
- **e2e:** none new. Batching adds no shipped-binary risk that the existing e2e test `2c88ed381429` (Go and Node through the built CLI) does not already cover.

The existing scenarios of the two MODIFIED requirements keep their evidence IDs. Their tests are re-pointed from `executeGoTest` and `executeNodeTest` to the batch runner (a batch of one), so the evidence proves the shipped path.

| Scenario | Level | Evidence ID | Advisory placement | Risk rationale (why this level is the lowest convincing one) |
|---|---|---|---|---|
| `scn.execution.bce9246e1444` Select one named TypeScript test (existing) | integration | `….bce9246e1444.integration` | `internal/stele/runner_test.go` (existing, re-pointed) | A single selected Node test could run other tests of its file through the batch path. Needs real `node --test`. |
| `scn.execution.8371d74b5134` Fail a scenario whose test did not run (existing) | integration | `….8371d74b5134.integration` | `runner_test.go` (existing, re-pointed) | A missing name in a completed Node batch could be reported as passed or crashed. Needs real TAP. |
| `scn.execution.a9dde959cf57` Reject unsupported test files (existing) | unit | `….a9dde959cf57.unit` | `runner_test.go` (existing) | Batch planning could drop or misroute a file without a runner. Pure planning. |
| `scn.execution.500afe3889f6` Batch the tests of one Go package | integration | `….500afe3889f6.integration` | `internal/stele/gorunner_test.go` | Per-test outcomes could be lost or misattributed in one `go test -json` stream. Needs real Go events. |
| `scn.execution.8653331d296a` Batch the tests of one Node test file | integration | `….8653331d296a.integration` | `runner_test.go` | Per-test outcomes, including the new `# SKIP` mapping, could be misread from one TAP stream. Needs real Node. |
| `scn.execution.0aa78cf92f34` Keep different build tag sets apart | unit | `….0aa78cf92f34.unit` | `internal/stele/batch_test.go` (new) | Tests needing different tags could share one command. The key and arguments are pure planning; real tag handling is covered by `4a0637d44b7a`. |
| `scn.execution.aba48bf93edd` Mark the unreported tests of a crashed batch | integration | `….aba48bf93edd.integration` | `batch_test.go`, fixtures in a temporary directory | The completion markers exist only in real output after `os.Exit` and `process.exit` (probed while planning). |
| `scn.execution.e49b224babb8` Match the outcomes of one process per test | integration | `….e49b224babb8.integration` | `batch_test.go` | Batching could change an outcome, a reason, the evidence, or the JSON. Only real runs show the equivalence. |
| `scn.execution.93ab1bbe751c` Show failures before the batch ends | unit | `….93ab1bbe751c.unit` | `internal/stele/progress_test.go` or `batch_test.go` | Batching could hold every result until the process ends. A fake output stream and the plain reporter in process. |
| `scn.gosupport.d195095fc292` Pass a scenario whose Go test passes (existing) | integration | `….d195095fc292.integration` | `gorunner_test.go` (existing, re-pointed) | A one-test Go batch could run other tests. Needs real `go test`. |
| `scn.gosupport.208b6a95ea3f` Fail a scenario whose Go test fails (existing) | integration | `….208b6a95ea3f.integration` | same | A failing test in a batch could be reported as crashed or passed. Real events. |
| `scn.gosupport.75d202e0ed36` Fail a scenario whose Go test is skipped (existing) | integration | `….75d202e0ed36.integration` | same | A skip could read as passed or not executed. Real events. |
| `scn.gosupport.203331e4d990` Fail a scenario whose Go test did not run (existing) | integration | `….203331e4d990.integration` | same | A missing name in a completed batch could be reported as crashed. Needs the real final line. |
| `scn.gosupport.4a0637d44b7a` Honor build constraints of the test file (existing) | integration | `….4a0637d44b7a.integration` | same | A tagged test could compile without its tag in a batch. Real constraints. |
| `scn.gosupport.2c88ed381429` Mix TypeScript and Go scenario tests (existing) | e2e | `….2c88ed381429.e2e` | `tests/cli.test.mts` (existing) | The shipped binary could fail to run Go and Node batches together. |

Requirement anchors (`@implements`) expected during implementation:

- `req.execution.f9056cdc6fe6`: `runScopeTests` (existing anchor on `RunScenarioTests`).
- `req.execution.5aa38806b077`: `batch.go` (planner and readers).
- `req.gosupport.d8c058038daa`: the Go batch command in `gorunner.go` (existing anchor on `executeGoTest` moves).

## Risks / Trade-offs

- **[Tests that relied on running alone may change outcome in a batch.]** Package-level state, `TestMain`, `init`, and Node module state are shared within a batch. Batches match `go test ./pkg` and `node --test file`, which projects already run, so such tests are rare and already fragile there. The changelog names the change.
- **[A crash now fails tests that would pass on their own.]** This is the agreed behavior. The report names the batch and its exit status, and the crash is a real defect.
- **[A pass no longer needs a zero exit status.]** Decision 2 explains why; the failing sibling is reported on its own.
- **[One large package dominates.]** `internal/stele` takes about 17 s of the 30 s. Only the deferred splitting or parallelism would reduce it.
- **[Merge conflicts]** with work that touches `runner.go` or `progress.go` after this plan. The plan is based on `6bbb599`.

## Migration Plan

- Ship in the next release candidate, with a changelog entry for batching and its semantics, crashed batches, and the Node skip reason.
- Existing evidence stays valid: the schema is unchanged, and outcomes for passing suites are identical.
- Rollback: reverting the release restores one process per test without any data migration. No switch is added.

## Deferred (future change)

The first draft of this plan included the items below. The user trimmed the change to batching so that the largest, lowest-risk gain ships first. The motivating use case for bringing them back is the user's other project: its end-to-end tests start containers and could run in parallel if each worker had its own. The rc.3 prototypes above give their expected effect: batches with 3 workers took 14.3 s against 24.3 s sequentially.

- **`--jobs N|auto`.** The value comes from the flag, then `execution.jobs`, then `auto` (`runtime.NumCPU()`). `--jobs 1` overrides every configured count, and invalid values exit `2`. *Why deferred:* concurrency changes the execution semantics for every project; batching alone gets two-thirds of the gain here.
- **`execution.groups[]` in `stele.config.json`:** `name`, `match` globs (first match wins, the rest in `default`), `jobs` (a cap within the global budget; `jobs: 1` acts as a mutex), `timeout`, and `setup` and `teardown` run with `sh -c`.
  - Groups run concurrently, sharing the budget. A free worker takes the next batch from declared groups first, then `default`, largest batch first.
  - Setups run one at a time, in configuration order, while other tests run.
  - A failed setup gives `not-run` with `group-setup-failed`. Teardown always runs, once per command even under `--all`, and a failed teardown exits `2` after the files are written.
  - *Why deferred:* it only matters with parallelism, and its semantics (precedence, lifecycle across scopes) deserve their own review.
- **`STELE_WORKER_ID`, `STELE_JOBS`, and `STELE_GROUP`** in each test process. The worker ID is a slot within the group, from 0 to jobs−1, so a group's setup can create exactly `STELE_JOBS` isolated stacks, with ports, databases, and compose project names derived from the ID. *Why deferred:* meaningless without parallel workers.
- **Overlapping OpenSpec validation and static checks with test execution,** with an observer gate so that the fast stages are still reported first. *Why deferred:* the stages take 0.8 s, which is small next to 30 s.
- **Process groups, per-group timeouts, and Ctrl-C handling:**
  - every child starts with `Setpgid`, and killing targets `-pgid`;
  - a timed-out batch gives `test-timeout`;
  - on `SIGINT` or `SIGTERM`, Stele kills the running processes, runs the teardowns, writes no files, and exits `130`.

  *Why deferred:* most valuable once setups start containers. Sequential batches behave like today's per-test processes on Ctrl-C.
- **Splitting large batches across workers.** Four shards of `internal/stele` took 10.0 s warm on rc.3. Cold, concurrent shards compile the same package several times (31.1 s) unless the binary is built once with `go test -c` and run with `-test.v=test2json | go tool test2json` (24.5 s). *Why deferred:* it needs parallelism.
- **This repository's `packaged-binary` group** (`tests/**`, `jobs: 1`). It keeps `npm pack`, which rebuilds `dist/`, apart from the Node CLI tests that execute `dist/stele`. Not needed while batches run one at a time (Decision 6).
- **A per-package or per-file opt-out of batching** that runs a suite with one process per test, through the per-test oracle. *Why deferred:* it needs configuration (Open Question 1, decided 2026-09-18).

## Open Questions

Both were answered in chat on 2026-09-18, before implementation started.

1. **A per-package or per-file opt-out of batching** for suites that need one process per test. **Decision: deferred** to the future change with configuration ("Deferred (future change)"), because it needs configuration and this change adds none. The per-test path stays in the code as the oracle, so adding the opt-out later costs little.
2. **Re-run the unreported tests of a crashed batch alone?** **Decision: no.** Tests left without a result after a crash are not re-run; they are recorded as failed with `test-process-failed`, as Decision 2 describes, and the report names the batch and its exit status.
