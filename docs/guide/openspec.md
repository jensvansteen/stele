# Use Stele with OpenSpec

Stele currently builds on [OpenSpec](https://github.com/Fission-AI/OpenSpec) (MIT, Fission AI). OpenSpec owns the proposal, specifications, design, tasks, and the change lifecycle; Stele adds identities, the verification plan, anchors, and deterministic evidence.

The `stele-propose`, `stele-apply`, and `stele-archive` skills are the default entry points. They are short lists of steps that call OpenSpec's own skills at the right moment. This page is for teams that prefer to keep using OpenSpec's skills and commands directly.

## What `stele init` adds to OpenSpec

`stele init` uses the OpenSpec CLI bundled with `stele-spec`, so the OpenSpec version always matches the one Stele was tested with.

| Addition | Where | Effect |
|---|---|---|
| OpenSpec setup | `openspec/`, `.agents/skills/`, `.claude/skills` | Only when the project has no `openspec/` directory. `--tools` selects OpenSpec's tools. |
| `stele` workflow schema | `openspec/schemas/stele/` | A fork of `spec-driven` with a `verification` artifact between `design` and `tasks`. It generates `linkage-plan.json`, and `tasks` and apply require it. |
| Default schema | `schema:` in `openspec/config.yaml` | Switched from `spec-driven` to `stele`. Any other default stays selected, and init prints how to switch. |
| Apply and archive guidance | `operations` in `openspec/config.yaml` | Reminders to confirm the verification levels, add anchors, run the Stele gates, and restore the annotation after archiving. An archive entry written by an earlier Stele is replaced, not duplicated. |
| Annotated specification template | `openspec/schemas/stele/templates/spec.md` | Starts new delta specs with the [`<!-- stele: spec v1 -->` annotation](/concepts/spec-format). |
| Planning rule | `rules.verification` in `openspec/config.yaml` | Points the `verification` artifact to the `stele-plan` skill. |

Your own configuration and comments are kept, and init only adds entries that are missing.

With the schema in place, OpenSpec's `openspec-propose` skill builds the `verification` artifact like any other: its instructions tell the agent to run `stele ids`, propose a level per scenario, write the plan, and run the proposal gate. `openspec instructions apply` reports a change as blocked until the plan exists.

## Run the Stele gates around OpenSpec's skills

OpenSpec shows the apply and archive guidance to the agent, but cannot enforce it. Run the Stele commands at these points, or let CI run them:

| OpenSpec step | Stele step |
|---|---|
| After writing specifications (`openspec-propose`, `openspec-update-change`) | `stele ids --change <change>`, then `stele verify --stage proposal --change <change>` |
| Before `openspec-apply-change` starts | Confirm the verification levels in the conversation, then `stele approve --change <change> --confirmed-in-chat`, as the `stele-plan` skill describes |
| After `openspec-apply-change` | `stele validate --change <change>` |
| OpenSpec's `openspec-verify-change` | Run it after `stele validate` passes; it reviews the implementation, Stele proves the links and tests |
| Before `openspec-archive-change` | `stele validate --change <change>` must pass |
| After archiving | `stele annotate --specs --targets-from openspec/changes/archive/<date>-<change>`, then `stele validate --specs` |
| `openspec-explore`, `openspec-sync-specs` | No Stele step is needed |

`openspec-verify-change` is not in OpenSpec's core profile. Enable it with `npx openspec config profile`.

When archiving a change creates a new current specification, OpenSpec writes it without the Stele annotation. `stele annotate --specs` restores it and changes nothing else, which is why it runs before `stele validate --specs`. `--targets-from` also copies each capability's [targets](/guide/targets) from the archived delta spec.

## Move an existing change to the `stele` schema

`stele init` never switches changes that are already in progress. To move one:

1. Change `schema: spec-driven` to `schema: stele` in `openspec/changes/<change>/.openspec.yaml`.
2. Run `stele ids --change <change>`.
3. Follow the `stele-plan` skill to add the verification table to `design.md` and write a version 2 `linkage-plan.json`. If the change already has a version 1 plan, run `stele plan migrate --change <change>` instead and review the migrated levels.
4. Run `stele verify --stage proposal --change <change>`.

Changes that stay on `spec-driven` still verify with Stele; they just do not get the planning step from OpenSpec.

## After upgrading OpenSpec

The `stele` schema is a copy of `spec-driven` from the OpenSpec version that created it, and `openspec update` leaves it alone. After upgrading `stele-spec`, refresh it:

```bash
npx stele init --refresh-schema
```

This forks and patches the schema again. Any edits you made to `openspec/schemas/stele/` are replaced, so reapply them afterwards.
