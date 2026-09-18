<!-- stele: spec v1 -->
# validate Specification

## Purpose
Provide one deterministic gate that combines OpenSpec strict validation, exact scenario execution, and implementation verification for CI and review.

## Requirements

### Requirement: Run the complete deterministic gate
Verification-ID: req.validate.56cc774dc871

The `stele validate` command SHALL run scenario tests, OpenSpec strict validation, and implementation verification, write evidence and report files to their default paths, and pass only when all three pass.

#### Scenario: Pass only when every check passes
Verification-ID: scn.validate.d9553f1a4c1c

- **WHEN** any of scenario execution, OpenSpec validation, or implementation verification fails
- **THEN** `stele validate` exits with `1` and names the failing check

#### Scenario: Validate a separately installed package
Verification-ID: scn.validate.10388560c2dc

- **WHEN** the packed npm package is installed into a separate consumer project with a linked OpenSpec change
- **THEN** `stele init` and `stele validate` succeed in that consumer project

### Requirement: Check a project with one command
Verification-ID: req.validate.70d1435b3ff3

The `stele check [--change <id> | --specs | --all]` command SHALL, for the selected scope, check that every requirement and scenario has a Verification-ID as `stele ids --check` does, check specification annotations as `stele annotate --check` does, and run `stele validate`. `--all` SHALL cover the current specifications and every active change in each step, and the output file, strict version, detail, quiet, and color options SHALL pass through to validation. It SHALL run every step even when an earlier step fails, print one summary line per step followed by the validation report, and exit with the worst exit code of its steps: `2` over `1` over `0`. With `--json` it SHALL print one deterministic document listing each step with its scope, exit code, and machine-readable result.

#### Scenario: Run every step and report each one
Verification-ID: scn.validate.ed99aac59b49

- **WHEN** `stele check --change example` runs, a scenario of `example` lacks a Verification-ID, and annotation and validation pass
- **THEN** all three steps run, the summary marks the ID step failed and names the heading, the other steps are marked passed, and the command exits with code `1`

#### Scenario: Exit with the worst code
Verification-ID: scn.validate.d768594936b1

- **WHEN** the ID check fails with code `1` and validation cannot run OpenSpec and ends with code `2`
- **THEN** `stele check` still runs the annotation check and exits with code `2`

#### Scenario: Check every scope
Verification-ID: scn.validate.704b1662c309

- **WHEN** `stele check --all` runs in a project with current specifications and two active changes
- **THEN** the ID and annotation checks cover all three scopes, validation runs as `stele validate --all`, and the summary lists each step's result

#### Scenario: Gate CI with the installed command
Verification-ID: scn.validate.ddff25a243df

- **WHEN** CI runs `stele check --all` from the installed package, without a terminal, in a project where one change has an unapproved plan
- **THEN** standard output ends with the combined summary and the verdict line, standard error has only plain progress lines, and the exit code is `1`
