## Why

Two gaps surfaced while Stele verified itself.

- `stele verify --change <name>` passes with "0 requirements, 0 scenarios" when the change does not exist or has no delta specs. After an archive, or after a typo, a verification gate can go green without checking anything.
- When `stele test` runs inside a `node --test` process, Node sets `NODE_TEST_CONTEXT` for child processes. The nested Node test then reports to the outer runner instead of printing TAP, so Stele records every Node scenario as not executed. Stele's own end-to-end tests work around this today.

## What Changes

- `stele verify`, `stele test`, and `stele validate` stop with exit code `2` when the selected change has no delta specs, or when `--specs` finds no current specifications.
- The Node test runner removes `NODE_TEST_CONTEXT` from the environment of the exact test it starts. Linked Node tests therefore execute and report normally, even when Stele itself runs inside `node --test`.
- The end-to-end tests drop their environment workaround.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `verification-scope`: adds a requirement that the selected scope contains specifications.
- `execution`: adds a requirement that Node scenario tests are isolated from an enclosing Node test runner.

## Impact

- `internal/stele/scope.go`, `internal/stele/runner.go`, and their tests.
- `tests/cli.test.mts` loses its `NODE_TEST_CONTEXT` workaround.
- The CLI reference and changelog.
