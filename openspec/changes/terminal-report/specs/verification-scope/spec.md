## MODIFIED Requirements

### Requirement: Verify the current specifications
Verification-ID: req.verificationscope.bee89d6750ed

The `stele verify`, `stele test`, and `stele validate` commands SHALL accept `--specs` to verify every requirement and scenario in `openspec/specs/`, using a plan that combines the linkage plans of archived changes. For each identity, the combined plan SHALL use the entries of the archived change whose archived delta spec declares that identity with the same text as the current specification. When several archived changes qualify, or none does, it SHALL use the one archived last, ordered by the archive date prefix and then by change name. When changes archived on the same date both qualify, or none qualifies, and their plans give the identity different entries, verification SHALL report a `PLAN_ARCHIVE_ORDER_AMBIGUOUS` warning that names both plans.

#### Scenario: Verify archived behavior
Verification-ID: scn.verificationscope.8b577e9712e7

- **WHEN** a change with a linkage plan has been archived and its anchors resolve to the planned targets
- **THEN** `stele verify --specs` passes for the requirements now in `openspec/specs/`

#### Scenario: Prefer the most recent archived plan
Verification-ID: scn.verificationscope.70c3ee060846

- **WHEN** two archived changes plan different targets for the same identity
- **THEN** `--specs` verification uses the target from the change archived last

#### Scenario: Prefer the plan whose specification matches on the same day
Verification-ID: scn.verificationscope.4ad1b478a310

- **WHEN** changes `alpha` and `beta` were archived on the same date, both plan the same scenario, and only the archived delta spec of `alpha` has the scenario's current text
- **THEN** `--specs` verification uses the entries of `alpha`, although `beta` sorts after it

#### Scenario: Warn when the archive order cannot be decided
Verification-ID: scn.verificationscope.1c6a44335f91

- **WHEN** two changes archived on the same date both declare a scenario with its current text and plan different entries for it
- **THEN** `--specs` verification uses the entries of the change whose name sorts last and reports a `PLAN_ARCHIVE_ORDER_AMBIGUOUS` warning naming both plans

#### Scenario: Run OpenSpec validation for the current specifications
Verification-ID: scn.verificationscope.f8ab6e2e1812

- **WHEN** `stele validate --specs` runs
- **THEN** it runs OpenSpec strict validation of all specifications and fails when that validation fails

#### Scenario: Reject a conflicting selection
Verification-ID: scn.verificationscope.79a83cf82aba

- **WHEN** `--specs` and `--change` are passed together
- **THEN** the command exits with `2` and explains that they cannot be combined
