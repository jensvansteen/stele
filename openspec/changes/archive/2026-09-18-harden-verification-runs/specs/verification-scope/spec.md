## ADDED Requirements

### Requirement: Reject an empty verification scope
Verification-ID: req.verificationscope.270b822fff6b

The `stele verify`, `stele test`, and `stele validate` commands SHALL exit with code `2` and name the scope when the selected change has no delta specs, or when `--specs` finds no current specifications, instead of reporting a result for zero requirements.

#### Scenario: Reject a change that does not exist
Verification-ID: scn.verificationscope.0246717fcb77

- **WHEN** `stele verify --change missing` runs and `openspec/changes/missing/` does not exist
- **THEN** the command exits with code `2` and reports that change `missing` has no delta specs

#### Scenario: Reject a change without delta specs
Verification-ID: scn.verificationscope.2af83d65e804

- **WHEN** the selected change directory exists but contains no Markdown files under `specs/`
- **THEN** `stele verify`, `stele test`, and `stele validate` each exit with code `2`

#### Scenario: Reject empty current specifications
Verification-ID: scn.verificationscope.6f3a1ce24f05

- **WHEN** `stele verify --specs` runs and `openspec/specs/` contains no specifications
- **THEN** the command exits with code `2` and reports that there are no current specifications
