## MODIFIED Requirements

### Requirement: Scope anchors to declared identities
Verification-ID: req.verificationscope.9c81618619df

An anchor SHALL be reported as `ANCHOR_DANGLING` only when no specification under `openspec/`, whether an active change, an archived change, or a current specification, declares its identity. Anchors for identities declared outside the selected scope SHALL NOT affect its verdict, with one exception: when the current specifications are verified, an anchor whose identity, or the scenario of whose evidence ID, only archived changes declare names removed behavior and SHALL be reported as a `LINK_REMOVED_BEHAVIOR_ANCHORED` error with its path and line.

#### Scenario: Ignore anchors of another active change
Verification-ID: scn.verificationscope.ee5d7b261d62

- **WHEN** change `a` is verified and an anchor names an identity declared only by change `b`
- **THEN** the report for `a` has no diagnostic for that anchor

#### Scenario: Report an anchor that no specification declares
Verification-ID: scn.verificationscope.101e06b07f39

- **WHEN** an anchor names an identity that appears in no specification under `openspec/`
- **THEN** verification fails with `ANCHOR_DANGLING`

#### Scenario: Report anchors to behavior removed by an archived change
Verification-ID: scn.verificationscope.9c3ef2025b56

- **WHEN** `stele verify --specs` runs after a change that removed a requirement was archived, and a test still has `@verifies` for an evidence ID of one of its scenarios, which only archived changes declare
- **THEN** verification reports a `LINK_REMOVED_BEHAVIOR_ANCHORED` error with the test's path and line, and fails

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
