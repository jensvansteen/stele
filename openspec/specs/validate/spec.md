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
