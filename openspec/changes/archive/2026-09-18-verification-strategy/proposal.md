## Why

Stele aims to make a codebase readable as plain English. Every behavior in the specification links to the code that implements it and to the tests that prove it. Those links serve two equal purposes: verifying the implementation, and letting people review and navigate between specification, code, and tests.

Today's v1 linkage plan works against that. It holds one `path#selector` target per identity, so it records where code must live, allows only one test per scenario, and has no record of who approved the chosen test levels. The `@implements` and `@verifies` comments in the code already say where things live. The plan should hold only the decisions a person has to approve: which kinds of evidence prove each scenario, and why.

## What Changes

- **Plan schema v2.** A `linkage-plan.json` with `schemaVersion: 2` holds no code or test locations.
  - Per scenario, it lists one or more evidence entries. Each entry has an evidence ID `<scenario>.<level>[.<n>]`, a level (`unit`, `integration`, or `e2e`), a rationale, and an approval record.
- **Anchors are the source of truth for links.** For every approved evidence entry, verification requires a `@verifies <evidence-id>` anchor on a real test, and that test must pass. Every requirement needs at least one `@implements` anchor. Unknown IDs, and anchors for evidence the plan does not list, are errors.
- **Approvals in the conversation.** When applying a change, the agent shows the evidence plan and asks "Approve these levels and start implementing?". Only after an explicit yes does it record the approval with `stele approve --confirmed-in-chat`. A human can also review entries one by one with `stele approve` in a terminal. Each approval records the approver, date, digest, and whether it came from the command line or an agent conversation. Verification fails for entries that are unapproved or have changed since approval.
- **Placement is advisory.** Stele never checks where code or tests live. The `stele-plan` skill has the agent read the project's `AGENTS.md`, `CLAUDE.md`, skills, and existing layout, and suggest a location with a reason. The suggestion is stored in the plan as advisory information. Projects can add their own placement skills next to `stele-plan`.
- **Level definitions.** Unit, integration, and e2e are defined by what the test reaches, in the skill and in the documentation.
- **v1 plans during a deprecation period.** Path-based v1 plans stay accepted, with a deprecation warning, until 0.2.0. A new `stele plan migrate` command converts them to v2 with unapproved entries.
- **Recorded vision.** The product vision is written down in the repository documentation.

## Capabilities

### New Capabilities

- `verification-strategy`: evidence plans, approvals, evidence verification through anchors, v1 compatibility and migration, and planning guidance.

### Modified Capabilities

- `verify`: the proposal and implementation stage rules now depend on the plan schema version.

## Impact

- `internal/stele`: plan parsing (v1 and v2), approval digests, evidence validation, new `approve` and `plan migrate` commands, and the `stele-plan` and `stele-verify` skill templates.
- The report and evidence schemas gain evidence IDs and levels. The JSON output stays deterministic.
- The documentation covers the CLI reference, IDs and anchors, test levels, both guides, `docs/intent/`, and the changelog.
- This repository migrates its own archived and active v1 plans once a release contains the change.
- It is implemented after `authoring-workflow`. Of the lifecycle skills, only `stele-plan` and `stele-verify` change.
