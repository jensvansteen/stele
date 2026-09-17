## Why

Working the Stele way currently takes manual steps around each OpenSpec step:

- after writing the specs: add Verification-IDs and plan the evidence;
- while implementing: add anchors;
- before and after archiving: validate.

Agents that use OpenSpec's skills skip these steps, and IDs are still written by hand or with a prototype.

Users should build their habits around Stele's own entry points. Stele currently builds on OpenSpec (MIT, Fission AI). Thin Stele lifecycle skills that delegate to OpenSpec let Stele change or replace that backend later without changing how people work. Projects that use OpenSpec's skills directly should still get the Stele steps.

## What Changes

- **`stele ids`.** A new command inserts deterministic Verification-IDs below requirement and scenario headings that have none.
  - It preserves every other byte.
  - It supports `--check` and `--json`.
  - It keeps the IDs of modified and renamed behavior.
  - It avoids collisions with every ID in `openspec/` and in code anchors, including evidence-ID suffixes.
  - It is built fresh; the earlier prototype only serves as inspiration.
- **Lifecycle skills (the default).** `stele init` installs three thin skills, listed first in `init` output and in the documentation. They only order steps and delegate backend work to OpenSpec skills. All logic stays in the CLI and in the `stele-plan` and `stele-verify` reference skills.
  - `stele-propose`: uses `openspec-propose`, makes sure the planning step ran, and hands back to the user.
  - `stele-apply`: shows the levels, records the human's explicit confirmation, uses `openspec-apply-change`, then runs `stele validate --change`.
  - `stele-archive`: runs `stele validate --change` as a gate, uses `openspec-archive-change`, then runs `stele validate --specs`.
- **Stele workflow schema (support for direct OpenSpec use).** `stele init` forks OpenSpec's `spec-driven` schema into a project schema named `stele`, with a `verification` artifact between `design` and `tasks` and an apply phase that requires it. It selects that schema as the project default. `stele-propose` relies on this artifact too.
- **OpenSpec configuration guidance (support).** `stele init` merges apply and archive guidance and `verification` rules into `openspec/config.yaml`. The merge keeps user configuration and comments, and is idempotent.
- **Documentation.**
  - The lifecycle skills are the default path.
  - A "Using Stele with OpenSpec" guide credits OpenSpec, explains the schema and configuration support, positions `openspec-verify-change` after `stele validate`, notes that explore and sync-specs need no Stele steps, and documents the fallback commands.
- **Backend independence** is planned separately in `specification-adapter`. That covers the adapter seam, Stele's own concepts in the documentation, and OpenSpec version pinning and drift warnings.

## Capabilities

### New Capabilities

- `ids`: inserting and checking Verification-IDs.
- `lifecycle`: the thin `stele-propose`, `stele-apply`, and `stele-archive` skills.
- `workflow-schema`: the Stele OpenSpec workflow schema and configuration guidance, for direct OpenSpec use.

### Modified Capabilities

- `init`: installs the lifecycle and reference skills and the workflow schema, merges the configuration guidance, and prints the default workflow.

## Impact

- `internal/stele`: a new `ids` command, three skill templates, schema installation through the bundled OpenSpec CLI, and an embedded Node script that edits `openspec/config.yaml` and the forked schema with the `yaml` package bundled with OpenSpec. `init` output also changes.
- It relies on `openspec schema fork`, which is experimental in OpenSpec 1.13. The pinned OpenSpec version and integration tests guard that dependency.
- Documentation covers the README, getting started, Build a Todo, the CLI reference, the new OpenSpec guide, and the changelog.
- Order: this change, then `verification-strategy`, then `link-index`, then the VS Code extension. `specification-adapter` can land independently, before `link-index`.
