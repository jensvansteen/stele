<!-- stele: spec v1 -->
## REMOVED Requirements

- `### Requirement: Initialize a consumer project`

**Reason**: Replaced by "Initialize a project" with the same behavior and Verification-IDs, so that scenario `scn.init.754fd262e114` can be renamed. OpenSpec rejects a MODIFIED requirement that omits a current scenario name, so a scenario can be renamed only by replacing its requirement.

**Migration**: None. The behavior, the requirement ID `req.init.eed35c447821`, and every scenario ID are unchanged; only the requirement name and the title of `scn.init.754fd262e114` change.

## ADDED Requirements

### Requirement: Initialize a project
Verification-ID: req.init.eed35c447821

The `stele init` command SHALL do the following, without overwriting files that already exist:

- create `stele.config.json`, with the selected change as the default when `--change` is given;
- install the `stele-propose`, `stele-apply`, and `stele-archive` lifecycle skills and the `stele-plan` and `stele-verify` reference skills under `.agents/skills/`;
- install and select the Stele workflow schema, and merge Stele guidance into the OpenSpec configuration, as defined by the `workflow-schema` capability;
- ensure an `artifacts/` directory exists;
- print the default workflow and the commands to run when a project uses OpenSpec skills directly.

#### Scenario: Create configuration, skills, and artifacts directory
Verification-ID: scn.init.cbf5781012fa

- **WHEN** `stele init --change example` runs in a project without Stele files
- **THEN** it creates `stele.config.json` naming `example` as the change, all five Stele skill files, and the `artifacts/` directory

#### Scenario: Preserve existing files on a repeated run
Verification-ID: scn.init.e841b29256e0

- **WHEN** `stele init` runs in a project that already contains the configuration and skills
- **THEN** it leaves those files unchanged and reports that Stele is already initialized

#### Scenario: Initialize without a default change
Verification-ID: scn.init.754fd262e114

- **WHEN** `stele init` runs without `--change`
- **THEN** it succeeds without a default change, because the change is required only later: a `stele verify` without `--change` or `--specs` exits with code `2` and reports that no OpenSpec change is selected

#### Scenario: Print the default workflow
Verification-ID: scn.init.c8813c799047

- **WHEN** `stele init` completes
- **THEN** its output names `stele-propose`, `stele-apply`, and `stele-archive` as the default workflow, and, for projects that use OpenSpec skills directly, says that the Stele schema adds the planning step and lists `stele ids`, `stele verify --stage proposal`, `stele validate --change`, and `stele validate --specs` as the commands to run
