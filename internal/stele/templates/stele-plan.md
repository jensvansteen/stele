---
name: stele-plan
description: Reference for planning Stele verification of OpenSpec scenarios and for confirming the planned levels. Used by stele-propose, stele-apply, and the stele schema's verification step.
---

# Plan Stele verification

Read the change's proposal, delta specs, and design before planning.

## Identities

Run `stele ids --change <change>`. Never write or edit a `Verification-ID` by hand: an ID names the behavior, not its wording.

## Levels

Choose, per scenario, the lowest level that convincingly proves it. Levels are defined by what the test reaches:

- **unit**: calls code directly, in process, possibly with temporary files or stubbed dependencies.
- **integration**: exercises the code together with one real outside tool or service.
- **e2e**: uses the real product through its user-facing entry point: the UI for applications, the installed executable or package for command-line tools.

Add a second level only for a distinct risk the first cannot cover, and say which.

## Placement

Read the project's `AGENTS.md`, `CLAUDE.md`, installed skills, and existing code and test layout, and suggest where each implementation and test should live, with a reason tied to those conventions. Placement is advisory: Stele never checks it.

## The plan

Record the verification table in the change's `design.md`: one row per piece of evidence with the scenario, level, evidence ID (`<scenario>.<level>`, `.<level>.2` for a second test of the same level), advisory placement, and the risk rationale. Mark it "proposed, awaiting approval".

Write `openspec/changes/<change>/linkage-plan.json` with `schemaVersion` 1, `changeId` set to the change, and the exact `path#selector` target of every requirement and of each scenario's first evidence entry. For Go, the selector is a function, `Type.Method`, or type name; for TypeScript tests, the test title.

Run `stele verify --stage proposal --change <change>` and fix what it reports.

## Approval

Before implementation, show the planned levels in the conversation (scenario, level, why) and ask: "Approve these levels and start implementing?"

- Only after an explicit yes, record the approval in `design.md`: "Status: approved by <name> on <date>, before implementation started (via: agent-confirmed)".
- Otherwise revise the plan with the user and ask again.
- Never approve silently, and never treat a request to apply or implement as approval.
