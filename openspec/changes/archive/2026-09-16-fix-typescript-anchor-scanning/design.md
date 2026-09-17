## Context

See proposal.md. `internal/stele/anchors.go` applies the annotation pattern to every line of a TypeScript file, and resolves test anchors with `^(?:test|it)\(`, which rejects `void test(` and `await test(`. The runner already selects Node tests by title, so the declaration form does not affect execution.

## Goals / Non-Goals

**Goals:**

- Make the `void test(...)` form taught in the guides, and used in this repository, selectable.
- Stop fixture strings from producing anchors.

**Non-Goals:**

- A full TypeScript parser. Regular-expression literals are not recognized, so a `//` or quote inside a regular expression can still confuse the lexer.
- `describe` blocks, `test.each`, and `test.only` as targets.

## Decisions

### A small comment-aware lexer

Before matching annotations, a lexer walks each file once and keeps only comment text. It tracks line comments, block comments, and single-quoted, double-quoted, and template literals, including `${...}` nesting in templates, across lines. Annotation matching then runs only on comment text, so reported line numbers stay those of the original file. Alternative considered: bundling a TypeScript parser. It would add a large dependency to a dependency-light Go binary.

### Accept `void` and `await` prefixes

The test declaration pattern becomes `^(?:(?:void|await)\s+)?(?:test|it)\(\s*["'\x60]([^"'\x60]+)["'\x60]`. The adjacency window and the comment-skipping rule stay unchanged.

### Keep reading level-qualified anchors

The comment-only matching keeps today's identity pattern, so an evidence ID such as `@verifies scn.todo.591a3b429cf0.e2e` still resolves to its scenario. A test fixture asserts this, so the lexer cannot break it before `verification-strategy` starts checking the level.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-16, before implementation started. No scenario needs a second level. The four baseline e2e scenarios becoming executable with a release that contains this change is the acceptance check for the whole change, not per-scenario evidence.

Levels, defined by what the test reaches: **unit** calls code directly, in process, possibly with temporary files or stubbed dependencies. **integration** exercises our code together with one real outside tool, such as Node, OpenSpec, or the Go toolchain. **e2e** uses the real product through its user-facing entry point: the UI for applications, and the installed executable or packed package for command-line tools such as Stele.

Evidence IDs extend the scenario's single behavior ID with the level, for example `scn.todo.591a3b429cf0.e2e`. A second entry at the same level adds a number, as in `.e2e.2`. The spec keeps only the behavior ID. Anchors name the evidence ID, as in `// @verifies scn.todo.591a3b429cf0.e2e`. The published verifier reads such an anchor as the plain scenario ID, and the v1 linkage plan stays keyed by that ID until `verification-strategy` adds per-entry plans.

| Scenario | Level | Evidence ID | Boundary | Target | Risk and why this level is the lowest convincing one | Checkable now |
|---|---|---|---|---|---|---|
| `scn.tsanchors.f4c1eb23c2ab` Bind a test anchor to a void or awaited test call | unit ★ | `scn.tsanchors.f4c1eb23c2ab.unit` | `ScanAnchors` on fixtures | `internal/stele/anchors_test.go#TestScanAnchorsResolvesExpressionTestCalls` | Risk: the documented form stays unselectable. Selector extraction is pure text analysis. | No, Go |
| `scn.tsanchors.f5c807c92cad` Leave other expressions unresolved | unit ★ | `scn.tsanchors.f5c807c92cad.unit` | `ScanAnchors` on fixtures | `internal/stele/anchors_test.go#TestScanAnchorsIgnoresOtherExpressionCalls` | Risk: widening the pattern binds anchors to non-test calls. Same pure analysis. | No, Go |
| `scn.tsanchors.e3be49f14532` Ignore annotations inside string and template literals | unit ★ | `scn.tsanchors.e3be49f14532.unit` | lexer and `ScanAnchors` | `internal/stele/anchors_test.go#TestScanAnchorsIgnoresTypeScriptStringLiterals` | Risk: fixtures create dangling anchors, and multi-line templates hide lexer state bugs. The lexer is pure. | No, Go |
| `scn.tsanchors.e6114f49ffdc` Keep annotations in every comment form | unit ★ | `scn.tsanchors.e6114f49ffdc.unit` | lexer and `ScanAnchors` | `internal/stele/anchors_test.go#TestScanAnchorsReadsEveryCommentForm` | Risk: the lexer drops real anchors, which would break existing consumers. Pure. | No, Go |

Requirement implementation targets:

| Requirement | Target |
|---|---|
| `req.tsanchors.3529ec7b6931` Resolve TypeScript tests called as expressions | `internal/stele/anchors.go#testSelector` |
| `req.tsanchors.c02ad141ee6a` Read TypeScript annotations only from comments | `internal/stele/anchors.go#typeScriptCommentText` |

## Risks / Trade-offs

- [A regular-expression literal containing a quote can desynchronize the lexer] → State the limitation in the IDs and anchors page. Anchors in comments after such a literal on the same line may be missed, not invented.
- [Consumers who relied on anchors inside strings lose them] → That use is unintended. The changelog states the behavior change.
