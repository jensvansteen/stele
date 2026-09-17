## Why

Stele is now published, but its own behavior has no OpenSpec specification, no Verification-IDs, and no linkage plan. The Node CLI tests already name `scn.verify.*` identities that no specification in this repository declares. Recording the existing v0.1 behavior as a baseline lets every later change, starting with Go support, be planned and verified the Stele way.

## What Changes

- Record the existing `init`, `verify`, `test`, and `validate` behavior as OpenSpec requirements with stable Verification-IDs. No product behavior changes.
- Reuse the four `scn.verify.*` identities already anchored in `tests/cli.test.mts`.
- Add a verification strategy that maps every requirement to the Go declaration that implements it and every scenario to the test that proves it.
- Verify this repository with the published `stele-spec` package, pinned as a development dependency, instead of the checkout under development.

## Capabilities

### New Capabilities

- `init`: bootstrapping a consumer project with configuration, skills, and an artifacts directory.
- `verify`: parsing OpenSpec identities, resolving anchors, and producing deterministic proposal and implementation reports.
- `execution`: running each linked scenario test exactly and binding the evidence to the verified inputs.
- `validate`: the combined OpenSpec, execution, and verification gate.

### Modified Capabilities

None.

## Impact

- New files under `openspec/`, `.agents/skills/`, `.claude/skills` (a link to `.agents/skills`), `stele.config.json`, and `artifacts/linkage-plan.json`.
- `package.json` gains the pinned `stele-published` alias of `stele-spec` and a `stele:published` script. `npx stele` inside this repository resolves to the checkout's own `dist/stele`, so self-verification runs through the script.
- `.gitignore` keeps generated artifacts ignored but tracks `artifacts/linkage-plan.json`.
- A test fixture in `tests/cli.test.mts` stops spelling anchors literally, because the published scanner also matches anchors inside string literals.
- The published verifier checks only TypeScript anchors and Node tests. Go declarations and Go tests in this plan resolve only after `add-go-support` ships in a published release.
