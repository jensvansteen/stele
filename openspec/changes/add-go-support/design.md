## Context

See proposal.md for motivation. Today `internal/stele/anchors.go` scans only `.ts`, `.tsx`, and `.mts` files with line-based regular expressions, and matches annotation text anywhere on a line. `internal/stele/runner.go` dispatches test execution by file extension and supports only Node. The evidence digest already includes `.go`, `go.mod`, and `go.sum`. The linkage plan, report, and evidence schemas need no change: Go targets use the same `path#selector` form.

Constraints: Stele stays dependency-light and deterministic, and core coverage stays at exactly 100%.

## Goals / Non-Goals

**Goals:**

- Resolve Go code and test anchors precisely enough to verify this repository.
- Run exactly one Go test per linked scenario, so that caching, name collisions, skips, and build tags cannot produce false passes.

**Non-Goals:**

- Subtests (`TestName/case`), examples, benchmarks, and fuzz targets as scenario targets.
- Batching several Go tests into one process for speed.
- Changing TypeScript scanning, including its string-literal matching; that needs its own change.
- Multi-module workspaces beyond using the nearest `go.mod`.

## Decisions

### Parse Go with the standard library

Go files are parsed with `go/parser` and comments are read from `go/ast`. Only real comments are searched for annotations, which rules out string literals, and declarations come from the syntax tree rather than from regular expressions. Alternative considered: extending the line-based regular expressions. It cannot distinguish comments from literals, and it misreads receivers and generic types.

### Adjacency matches the TypeScript rule

An annotation binds to the first declaration that starts within the same six-line window used for TypeScript, with only comments or blank lines in between. Otherwise the anchor stays unresolved, and implementation verification reports `ANCHOR_TARGET_MISSING`. Keeping one rule keeps both languages predictable.

### Level-qualified anchors

Go comments use the same identity pattern as TypeScript. An evidence ID such as `// @verifies scn.todo.591a3b429cf0.unit` therefore resolves to its scenario until `verification-strategy` starts checking the level.

### Selector grammar

| Declaration | Selector |
|---|---|
| `func Name(...)` | `Name` |
| `func (r *Receiver[T]) Method(...)` | `Receiver.Method`, without pointer or type parameters |
| `type Name ...` (single spec) | `Name` |
| `func TestName(t *testing.T)` in `_test.go` | `TestName` |

In a `_test.go` file, a test anchor binds only to a top-level function whose name starts with `Test` and that takes a single `*testing.T` parameter. `_test.go` files are test files; other `.go` files are code files. Go files are scanned under the existing source roots (`bin`, `cmd`, `internal`, `src`, `public`, `tools`, `scripts`, `tests`) plus `pkg` and the repository root directory itself, without recursing from the root.

### Invalid Go source is a tool failure

A Go file that fails to parse stops verification with an error naming the file, and the command exits with `2`. Alternative considered: a diagnostic that skips the file, which could silently drop anchors.

### Execute one test with `go test -json`

For each group, the runner calls `go test -json -count=1 -run '^TestName$' [-tags tags] ./<package-dir>` from the directory of the nearest `go.mod` above the test file.

- `-count=1` prevents cached results from standing in for execution.
- The anchored pattern prevents `TestName` from also selecting `TestNameExtra`.
- JSON events are read for the top-level test only. Action `pass` means passed, `skip` means reason `test-skipped`, and `fail` or a failing process means `test-process-failed`. No event for the test means `test-not-executed`.
- `STELE_CHILD_TEST=1` is set, as for Node.

Alternative considered: parsing `-v` text output, which is fragile across Go versions.

### Build tags come from the file's constraint

The runner reads the test file's `//go:build` line with `go/build/constraint`, collects the tag names it mentions other than the current `GOOS` and `GOARCH`, and passes them with `-tags`. If the constraint still evaluates to false for the current platform, the test is recorded as `test-not-executed` without starting a process. Alternative considered: a `stele.config.json` tag list. It adds configuration that the source already expresses.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-16, before implementation started.

Each row is one piece of evidence. A scenario would get a second row only for a risk the first cannot cover; none is proposed here. The ★ row is the one the v1 linkage plan records. "Checkable now" says whether the published 0.1.0-rc.1 verifier can resolve and execute the target.

Levels, defined by what the test reaches: **unit** calls code directly, in process, possibly with temporary files or stubbed dependencies. **integration** exercises our code together with one real outside tool, such as Node, OpenSpec, or the Go toolchain. **e2e** uses the real product through its user-facing entry point: the UI for applications, and the installed executable or packed package for command-line tools such as Stele.

Evidence IDs extend the scenario's single behavior ID with the level, for example `scn.todo.591a3b429cf0.e2e`. A second entry at the same level adds a number, as in `.e2e.2`. The spec keeps only the behavior ID. Anchors name the evidence ID, as in `// @verifies scn.todo.591a3b429cf0.e2e`. The published verifier reads such an anchor as the plain scenario ID, and the v1 linkage plan stays keyed by that ID until `verification-strategy` adds per-entry plans.

| Scenario | Level | Evidence ID | Boundary | Target | Risk and why this level is the lowest convincing one | Checkable now |
|---|---|---|---|---|---|---|
| `scn.gosupport.7798d2c7e562` Bind a code anchor to a Go function, method, or type | unit ★ | `scn.gosupport.7798d2c7e562.unit` | `ScanAnchors` on Go fixtures | `internal/stele/goanchors_test.go#TestScanAnchorsResolvesGoDeclarations` | Risk: wrong selector for pointer or generic receivers. Syntax-tree mapping is pure and needs no toolchain. | No, Go |
| `scn.gosupport.9bd4f6844031` Bind a test anchor to a Go test function | unit ★ | `scn.gosupport.9bd4f6844031.unit` | `ScanAnchors` on Go fixtures | `internal/stele/goanchors_test.go#TestScanAnchorsResolvesGoTestFunctions` | Risk: helpers or non-test functions in `_test.go` become targets. Classification is pure. | No, Go |
| `scn.gosupport.351e919a25ad` Ignore annotations outside comments | unit ★ | `scn.gosupport.351e919a25ad.unit` | `ScanAnchors` on Go fixtures | `internal/stele/goanchors_test.go#TestScanAnchorsIgnoresGoStringLiterals` | Risk: fixture strings create dangling anchors, as Stele's own tests do today. Comment extraction is pure. | No, Go |
| `scn.gosupport.29a77eab096c` Leave an unattached Go anchor unresolved | unit ★ | `scn.gosupport.29a77eab096c.unit` | `ScanAnchors` and `RunVerification` | `internal/stele/goanchors_test.go#TestScanAnchorsLeavesUnattachedGoAnchorsUnresolved` | Risk: an anchor binds to a distant or wrong declaration. Adjacency and the diagnostic are deterministic. | No, Go |
| `scn.gosupport.8954a51cba0f` Reject a Go file that cannot be parsed | unit ★ | `scn.gosupport.8954a51cba0f.unit` | CLI `Run` with a broken fixture | `internal/stele/goanchors_test.go#TestVerifyRejectsInvalidGoSource` | Risk: a broken file silently drops anchors. `Run` returns the exit code in process, so no binary is needed. | No, Go |
| `scn.gosupport.d195095fc292` Pass a scenario whose Go test passes | integration ★ | `scn.gosupport.d195095fc292.integration` | real `go test` on a fixture module | `internal/stele/gorunner_test.go#TestExecuteExactGoTestPasses` | Risk: the pattern selects extra tests or a cached result is reused. Only the real toolchain proves `-run`, `-count`, and JSON behavior. | No, Go |
| `scn.gosupport.208b6a95ea3f` Fail a scenario whose Go test fails | integration ★ | `scn.gosupport.208b6a95ea3f.integration` | real `go test` on a fixture module | `internal/stele/gorunner_test.go#TestExecuteExactGoTestFails` | Risk: a failing test is read as passed. Needs real failure events. | No, Go |
| `scn.gosupport.75d202e0ed36` Fail a scenario whose Go test is skipped | integration ★ | `scn.gosupport.75d202e0ed36.integration` | real `go test` on a fixture module | `internal/stele/gorunner_test.go#TestExecuteExactGoTestReportsSkip` | Risk: `go test` exits with `0` for skips, so a skip looks like a pass. Needs the real skip event. | No, Go |
| `scn.gosupport.203331e4d990` Fail a scenario whose Go test did not run | integration ★ | `scn.gosupport.203331e4d990.integration` | real `go test` on a fixture module | `internal/stele/gorunner_test.go#TestExecuteExactGoTestDetectsNoMatchingExecution` | Risk: "no tests to run" exits with `0`. Only the real toolchain shows that output. | No, Go |
| `scn.gosupport.4a0637d44b7a` Honor build constraints of the test file | integration ★ | `scn.gosupport.4a0637d44b7a.integration` | real `go test` on a tagged fixture | `internal/stele/gorunner_test.go#TestExecuteExactGoTestAppliesBuildConstraints` | Risk: a missing tag excludes the file and reads as not executed; this repository's package test uses such a tag. Needs the real toolchain. | No, Go |
| `scn.gosupport.2c88ed381429` Mix TypeScript and Go scenario tests | e2e ★ | `scn.gosupport.2c88ed381429.e2e` | built `dist/stele` on a mixed fixture with Node and Go | `tests/cli.test.mts#runs TypeScript and Go scenario tests together` | Risk: the shipped binary dispatches only one runner, or evidence merging drops one side. Only the executable with both toolchains proves it. | Resolves after the `void test` fix |

Requirement implementation targets:

| Requirement | Target |
|---|---|
| `req.gosupport.4c353f2b171a` Resolve Go anchors to declarations | `internal/stele/goanchors.go#scanGoAnchorFile` |
| `req.gosupport.d8c058038daa` Execute exact Go scenario tests | `internal/stele/gorunner.go#executeGoTest` |

## Risks / Trade-offs

- [Tests that start `go test` are slower and need the Go toolchain] → Fixture modules stay minimal and have no dependencies. CI already installs Go.
- [One process per scenario is slow for large suites] → Accepted for exactness. Batching is a later, separate change.
- [Stele cannot verify this change with itself until a release contains it] → Proposal verification uses the published package. Implementation evidence is collected after the next release, or with the local build as an explicitly unofficial check.
- [The published verifier checks one change but scans anchors across the repository, so the baseline's Node anchors count as `ANCHOR_DANGLING` when this change is selected] → Proposal verification for this change runs on an isolated copy that contains only this change and the linkage plan, until Stele can verify several active changes together.
- [Changing the default for `.go` files alters `scn.verify.0d43596abe4a`'s fixture] → The unsupported-language test drops its `.go` case in the same change.

## Open Questions

- Should subtests become selectable targets later (`TestName/case`)? This does not change this change's scope.
