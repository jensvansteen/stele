<!-- stele: spec v1 -->
# workflow-schema Specification

## Purpose
Extend OpenSpec's own workflow with Stele's verification planning, apply, and archive steps through a project workflow schema and project configuration, so users keep working with OpenSpec's skills.

## Requirements

### Requirement: Install the Stele workflow schema
Verification-ID: req.workflowschema.97dec0adc04d

`stele init` SHALL fork OpenSpec's `spec-driven` schema into a project schema named `stele`, using the bundled OpenSpec CLI. The fork SHALL add a `verification` artifact between `design` and `tasks` that generates the change's `linkage-plan.json`, SHALL make `tasks` and the apply phase require it, SHALL extend the apply instruction with the Stele apply steps, and SHALL select `stele` as the project's default schema when the project still uses `spec-driven`. When the project has no OpenSpec setup, `stele init` SHALL first run the bundled OpenSpec initialization, with the tool-neutral `.agents/skills` target unless `--tools` names other tools, and SHALL link `.claude/skills` to `.agents/skills` for the default target unless `.claude/skills` already exists. An existing OpenSpec setup, existing changes, and an existing `stele` schema SHALL be left unchanged, except that `--refresh-schema` re-forks and re-patches the `stele` schema.

#### Scenario: Create new changes with the Stele schema
Verification-ID: scn.workflowschema.b4a054dbe512

- **WHEN** `stele init` runs in a project whose `openspec/config.yaml` selects `spec-driven`, and then `openspec new change demo` runs
- **THEN** the change uses the `stele` schema, and `openspec status` lists `verification` after `design` and before `tasks`

#### Scenario: Plan verification as a workflow artifact
Verification-ID: scn.workflowschema.270587f7a4c3

- **WHEN** an agent asks OpenSpec for the instructions of the `verification` artifact
- **THEN** the instructions tell it to run `stele ids`, propose evidence per scenario with level, rationale, and advisory placement as described in the `stele-plan` skill, write the plan unapproved, run `stele verify --stage proposal`, and finish with "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."

#### Scenario: Block apply until the verification plan exists
Verification-ID: scn.workflowschema.a444787573ca

- **WHEN** a change on the `stele` schema has `tasks.md` but no `linkage-plan.json`
- **THEN** OpenSpec's apply instructions report the change as blocked by the missing `verification` artifact. Once the plan exists, they list it among the files to read, and their instruction tells the agent to confirm the verification levels as the `stele-plan` skill describes, add anchors while coding, and run `stele validate --change` before reporting done

#### Scenario: Keep existing changes and customized schemas
Verification-ID: scn.workflowschema.3aef78a653b4

- **WHEN** `stele init` runs in a project with an active change on `spec-driven`, an already customized `stele` schema, or a default schema other than `spec-driven`
- **THEN** the change keeps its schema, the existing `stele` schema files are unchanged unless `--refresh-schema` is given, and a non-`spec-driven` default stays selected with a note explaining how to switch

#### Scenario: Initialize OpenSpec when missing
Verification-ID: scn.workflowschema.779f1d7cebcb

- **WHEN** `stele init` runs in a project without an `openspec/` directory
- **THEN** it runs the bundled OpenSpec initialization for `.agents/skills`, or for the tools passed with `--tools`, links `.claude/skills` to `.agents/skills` for the default target or warns and leaves an existing `.claude/skills` alone, and then installs the Stele schema, guidance, and skills

### Requirement: Guide apply and archive through OpenSpec configuration
Verification-ID: req.workflowschema.db5e762d55ea

`stele init` SHALL merge Stele guidance into `openspec/config.yaml`:

- `operations.apply.guidance`: confirm the verification levels first, add `@implements` and `@verifies` anchors while coding, and run `stele validate --change` before reporting the change done;
- `operations.archive.guidance`: `stele validate --change` must pass before archiving, and `stele validate --specs` runs afterwards;
- rules for the `verification` artifact.

It SHALL keep every existing value and comment, add only missing entries, and leave the file byte-for-byte unchanged when nothing is missing.

#### Scenario: Merge guidance into existing configuration
Verification-ID: scn.workflowschema.6ffeeaa11cce

- **WHEN** `openspec/config.yaml` already has a context, comments, and user rules
- **THEN** after `stele init` the file keeps all of them and also contains the Stele guidance

#### Scenario: Merge guidance only once
Verification-ID: scn.workflowschema.61671da05c73

- **WHEN** `stele init` runs again on a configuration that already contains the guidance
- **THEN** the file is byte-for-byte unchanged

#### Scenario: Deliver guidance through OpenSpec itself
Verification-ID: scn.workflowschema.b184c4d2986a

- **WHEN** an agent asks OpenSpec for the apply or archive instructions of a change after `stele init`
- **THEN** the JSON output's `operationGuidance` contains the Stele guidance for that operation
