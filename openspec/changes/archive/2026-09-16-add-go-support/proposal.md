## Why

Stele verifies only TypeScript consumers, yet Stele itself is written in Go. Its requirements cannot resolve to the Go declarations that implement them, and its Go tests cannot count as scenario evidence. Go support is the first step to verifying Stele with Stele, and it opens Stele to Go projects in general.

## What Changes

- Resolve `@implements` and `@verifies` annotations written in Go comments to the adjacent Go function, method, type, or top-level test function.
- Ignore annotation text outside Go comments, such as anchors spelled inside string literals in test fixtures.
- Execute each Go scenario test on its own through the Go toolchain, without cached results. Count it as passed only when that exact test ran and passed.
- Report skipped Go tests as not passed, and honor the build constraints of the test file.
- Let one project mix TypeScript and Go scenario tests in a single `stele test` or `stele validate` run.
- The TypeScript scanner, TypeScript runner, report schema, evidence schema, and linkage plan schema stay unchanged.

## Capabilities

### New Capabilities

- `go-support`: resolving Go anchors to declarations and executing exact Go scenario tests.

### Modified Capabilities

None. The `verify` and `execution` capabilities from `baseline-verification-core` are not yet archived into `openspec/specs/`, so this change adds its behavior as a separate capability rather than a delta against them.

## Impact

- Go source scanning and Go test execution are added in `internal/stele`, using only the Go standard library.
- The Go toolchain becomes a runtime requirement for consumers that link Go scenario tests. TypeScript-only consumers are unaffected.
- The documentation that currently limits v0.1 consumer support to TypeScript changes, including the CLI reference, architecture, and IDs and anchors pages.
- This repository can verify its Go requirements and tests once a published release contains this change.
