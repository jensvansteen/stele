# init Specification

## Purpose
Bootstrap a consumer project for Stele by selecting its default OpenSpec change and installing the repository-local planning and verification skills.

## Requirements

### Requirement: Initialize a consumer project
Verification-ID: req.init.eed35c447821

The `stele init` command SHALL create `stele.config.json` with the selected change, install the `stele-plan` and `stele-verify` skills under `.agents/skills/`, and ensure an `artifacts/` directory exists, without overwriting files that already exist.

#### Scenario: Create configuration, skills, and artifacts directory
Verification-ID: scn.init.cbf5781012fa

- **WHEN** `stele init --change example` runs in a project without Stele files
- **THEN** it creates `stele.config.json` naming `example` as the change, both skill files, and the `artifacts/` directory

#### Scenario: Preserve existing files on a repeated run
Verification-ID: scn.init.e841b29256e0

- **WHEN** `stele init` runs in a project that already contains the configuration and skills
- **THEN** it leaves those files unchanged and reports that Stele is already initialized

#### Scenario: Require a change for initialization
Verification-ID: scn.init.754fd262e114

- **WHEN** `stele init` runs without `--change`
- **THEN** it exits with code `2` and explains that `--change` is required
