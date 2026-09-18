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

Propose one or more evidence entries per scenario. Give each a rationale that names the risk it covers and why its level is the lowest convincing one. Add a second level only for a distinct risk the first cannot cover, and say which.

## Placement

Read the project's `AGENTS.md`, `CLAUDE.md`, installed skills, and existing code and test layout, and suggest where each implementation and test should live, with a reason tied to those conventions. Placement is advisory: Stele never checks it. Project skills beside this one may add placement rules.

## The plan

Record the verification table in the change's `design.md`: one row per evidence entry with the scenario, level, evidence ID, advisory placement, and rationale. Mark it "proposed, awaiting approval".

Write `openspec/changes/<change>/linkage-plan.json` in schema version 2:

```json
{
  "schemaVersion": 2,
  "changeId": "<change>",
  "scenarios": {
    "scn.todo.0a1b2c3d4e5f": {
      "evidence": [
        {
          "id": "scn.todo.0a1b2c3d4e5f.unit",
          "level": "unit",
          "rationale": "Normalization is pure logic; no I/O is needed.",
          "placement": "tests/todo.test.mts, beside the existing Todo tests"
        }
      ]
    }
  }
}
```

- List every scenario of the change. Requirements are not listed; their `@implements` anchors are checked during implementation.
- The evidence ID is the scenario ID plus the level, such as `.unit`. A second entry of the same level uses `.unit.2`, then `.unit.3`.
- Never write code or test locations other than the advisory `placement`, and never write an `approval`: only `stele approve` records approvals.

Run `stele verify --stage proposal --change <change>` and fix what it reports. Until the human approves, it reports `PLAN_UNAPPROVED` for every new entry; that is expected and is resolved only by the approval step.

## Approval

At the start of applying a change, before any implementation:

1. Run `stele verify --stage proposal --change <change>` and collect the entries reported as `PLAN_UNAPPROVED` or `PLAN_APPROVAL_STALE`.
2. Show them in the conversation as a table: scenario, level, why, and suggested placement.
3. Ask: "Approve these levels and start implementing?"
4. Only after an explicit yes, run `stele approve --change <change> --confirmed-in-chat`, and mark the design table approved.
5. Otherwise, revise the plan with the `openspec-update-change` workflow, run the proposal check again, and ask again.
6. Never approve silently, and never treat a request to apply or implement as approval.

Changing a scenario's wording, or an entry's level or rationale, makes its approval stale, so it comes back in step 1.

## Version 1 plans

Plans with `schemaVersion` 1 map each ID to a `path#selector` target. They are accepted with a `PLAN_V1_DEPRECATED` warning until Stele 0.2.0. Convert one with `stele plan migrate --change <change>`, review the migrated levels and rationales, and approve them.
