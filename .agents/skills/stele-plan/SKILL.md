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

## Targets

Skip this section unless the change's specifications declare targets on their first line, such as `<!-- stele: spec v1; targets: ios, android -->`. Targets are optional: without them a specification describes the project itself. A target is a place that produces evidence, configured in `stele.config.json` with its `paths`.

Where differences between targets belong:

- **Behavior differences go in the specification**, as separate scenarios narrowed with a `Targets:` line under one shared requirement. Put `Targets:` directly below the heading, next to `Verification-ID:`. It may only narrow its parent, never widen it.
- **Testing differences go in the plan**, as evidence levels per target: iOS may prove a scenario with a unit test while Android uses an integration test.

A scenario of a targeted specification needs one or more entries for every target it applies to. Each entry has a `target`, and its evidence ID puts the target between the scenario and the level: `scn.share.3c4d5e6f7a8b.ios.unit`, then `….ios.unit.2`. Ordinals count per scenario, target, and level. Do not plan a target a scenario does not apply to (`PLAN_TARGET_NOT_APPLICABLE`).

```json
{
  "id": "scn.share.3c4d5e6f7a8b.android.integration",
  "target": "android",
  "level": "integration",
  "rationale": "The share intent needs the real Android framework.",
  "placement": "android/app/src/androidTest, beside the list tests"
}
```

Three patterns:

- **Replicas**: the same behavior built for several targets, such as `vscode`, `zed`, and `jetbrains`. Every target needs its own evidence entry, test, and `@implements` anchor. Example: `scn.lens.0a1b2c3d4e5f.vscode.e2e`, `….zed.integration`, and `….jetbrains.integration`, plus one platform-only scenario per replica narrowed with `Targets: vscode`.
- **Split**: one behavior divided over parts, such as `api` and `web`. Narrow each requirement to its side with `Targets: api` or `Targets: web`. A contract requirement lists both sides, and each proves its own side: `….api.integration` shows the handler serves the documented shape, and `….web.unit` shows the client parses it against a stub built from the same contract.
- **Journey**: an end-to-end flow across everything, narrowed to a `system` target that is `evidenceOnly` in the configuration. It has one entry, such as `….system.e2e`, anchored in the end-to-end suite. The requirement still needs one `@implements` anchor anywhere in the project.

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
