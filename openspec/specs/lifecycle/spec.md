# lifecycle Specification

## Purpose
Give users Stele's own entry points for proposing, applying, and archiving a change, as thin skills that delegate to the current specification backend.

## Requirements

### Requirement: Install thin lifecycle skills
Verification-ID: req.lifecycle.4c9232a6cf44

`stele init` SHALL install `stele-propose`, `stele-apply`, and `stele-archive` skills as the default entry points. Each skill SHALL contain only the ordering of steps: it delegates backend work to the OpenSpec skill it names (`openspec-propose`, `openspec-apply-change`, `openspec-archive-change`), and all Stele logic lives in the CLI and the `stele-plan` and `stele-verify` reference skills. A skill SHALL NOT copy OpenSpec skill text or describe the plan format.

#### Scenario: Propose through Stele
Verification-ID: scn.lifecycle.5a216e97cca6

- **WHEN** an agent follows `stele-propose`
- **THEN** it uses `openspec-propose` to create the change, makes sure the Stele planning step ran (the Stele schema's `verification` artifact, or otherwise `stele ids`, the `stele-plan` step, and `stele verify --stage proposal`), and stops with "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."

#### Scenario: Apply through Stele
Verification-ID: scn.lifecycle.b91a6d6dd399

- **WHEN** an agent follows `stele-apply`
- **THEN** it first shows the pending verification levels and asks the human to confirm them, records the confirmation only after an explicit yes as the approval step in `stele-plan` describes, then uses `openspec-apply-change`, and finishes with `stele validate --change`

#### Scenario: Archive through Stele
Verification-ID: scn.lifecycle.08814fe1e646

- **WHEN** an agent follows `stele-archive`
- **THEN** it runs `stele validate --change` and stops if it fails, then uses `openspec-archive-change`, and finishes with `stele validate --specs`

#### Scenario: Keep lifecycle skills thin
Verification-ID: scn.lifecycle.5df1b597ff80

- **WHEN** the lifecycle skills are installed
- **THEN** each is only an ordered list of steps that names the CLI commands, reference skills, and OpenSpec skills it uses, without plan-format details or copied OpenSpec text
