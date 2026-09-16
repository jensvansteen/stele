## Why

Stele verifies one change at a time but assumes it is the only behavior in the repository. Every change shares one `artifacts/linkage-plan.json`, whose `changeId` is never checked. Anchors that name identities from other active or archived changes count as `ANCHOR_DANGLING`. Once a change is archived, its requirements move to `openspec/specs/`, which Stele cannot verify at all. A repository with more than one change, including this one, cannot keep all of its behavior verified.

## What Changes

- Read the linkage plan from the selected change's directory, `openspec/changes/<change>/linkage-plan.json`, falling back to `artifacts/linkage-plan.json` when the change has none. Reject a plan whose `changeId` names another change. The plan moves with the change when it is archived.
- Treat an anchor as dangling only when no specification under `openspec/` declares its identity. Anchors for identities of other changes or of the current specifications are ignored for the selected change.
- Add `--specs` to `stele verify`, `stele test`, and `stele validate` to verify the current specifications in `openspec/specs/`. Their plan combines the plans of archived changes, and the most recent archive wins for an identity.
- **BREAKING** for plans that list several changes: a plan in `artifacts/` that names another change is rejected. Moving each change's entries into its own directory fixes this.

## Capabilities

### New Capabilities

- `verification-scope`: selecting what to verify (one change or the current specifications), where its plan comes from, and which anchors belong to it.

### Modified Capabilities

None. The related `verify` behavior in `baseline-verification-core` is not archived yet.

## Impact

- `internal/stele/cli.go`, `verify.go`, `specs.go`, `runner.go`, and `openspec.go`.
- `validate --specs` runs `openspec validate --specs --strict`.
- This repository moves each change's plan into its change directory once a release contains this change, and can then archive `baseline-verification-core` and verify it with `--specs`.
- Documentation: the CLI reference, getting started, the Build a Todo guide, and the changelog.
