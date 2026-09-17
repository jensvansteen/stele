## Why

The published verifier cannot select a test declared as `void test(...)`, the form that typed ESLint rules require for floating promises. The Build a Todo guide teaches that same form, so users who follow it hit `ANCHOR_TARGET_MISSING`. The TypeScript scanner also matches annotation text anywhere on a line, so test fixtures that write anchors into strings create `ANCHOR_DANGLING` errors. Both problems block this repository's own Node end-to-end tests from counting as evidence.

## What Changes

- Resolve a test anchor to a named test called as an expression statement: `test(...)`, `it(...)`, `void test(...)`, `void it(...)`, `await test(...)`, and `await it(...)`.
- Read TypeScript annotations only from line comments, block comments, and JSDoc comments, never from string or template literals.
- Keep the Build a Todo guide's `void test(...)` example and state that it is selectable.

## Capabilities

### New Capabilities

- `ts-anchors`: how the TypeScript scanner finds annotations and binds test anchors.

### Modified Capabilities

None. The TypeScript anchor behavior in `baseline-verification-core` is not archived yet, so this change adds a separate capability instead of a delta.

## Impact

- `internal/stele/anchors.go` and its tests.
- The four `scn.verify.*` Node scenarios of `baseline-verification-core` become executable once a published release contains this change.
- Documentation: the IDs and anchors page, the Build a Todo guide, and the changelog.
