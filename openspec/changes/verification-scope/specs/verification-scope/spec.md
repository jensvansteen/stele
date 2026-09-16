## Purpose

Define what a Stele run verifies, which linkage plan applies to it, and which anchors belong to it, so that several changes and the current specifications can be verified in one repository.

## ADDED Requirements

### Requirement: Use the selected change's own linkage plan
Verification-ID: req.verificationscope.713ceb31377c

Verification of a change SHALL read `openspec/changes/<change>/linkage-plan.json`, SHALL fall back to `artifacts/linkage-plan.json` only when the change has no plan of its own, and SHALL reject a plan whose `changeId` names a different change.

#### Scenario: Read the plan stored with the change
Verification-ID: scn.verificationscope.20fde975089b

- **WHEN** both `openspec/changes/example/linkage-plan.json` and `artifacts/linkage-plan.json` exist
- **THEN** proposal verification of `example` uses the plan stored with the change

#### Scenario: Fall back to the shared plan
Verification-ID: scn.verificationscope.d477fd8e9980

- **WHEN** the change has no plan of its own and `artifacts/linkage-plan.json` names that change
- **THEN** proposal verification uses the shared plan

#### Scenario: Reject a plan for another change
Verification-ID: scn.verificationscope.784db68ce025

- **WHEN** the plan that applies to change `example` has `changeId` `other`
- **THEN** verification fails with `PLAN_CHANGE_MISMATCH`

### Requirement: Scope anchors to declared identities
Verification-ID: req.verificationscope.9c81618619df

An anchor SHALL be reported as `ANCHOR_DANGLING` only when no specification under `openspec/`, whether an active change, an archived change, or a current specification, declares its identity. Anchors for identities declared outside the selected scope SHALL NOT affect its verdict.

#### Scenario: Ignore anchors of another active change
Verification-ID: scn.verificationscope.ee5d7b261d62

- **WHEN** change `a` is verified and an anchor names an identity declared only by change `b`
- **THEN** the report for `a` has no diagnostic for that anchor

#### Scenario: Report an anchor that no specification declares
Verification-ID: scn.verificationscope.101e06b07f39

- **WHEN** an anchor names an identity that appears in no specification under `openspec/`
- **THEN** verification fails with `ANCHOR_DANGLING`

### Requirement: Verify the current specifications
Verification-ID: req.verificationscope.bee89d6750ed

The `stele verify`, `stele test`, and `stele validate` commands SHALL accept `--specs` to verify every requirement and scenario in `openspec/specs/`, using a plan that combines the linkage plans of archived changes, with the most recently archived plan winning for an identity.

#### Scenario: Verify archived behavior
Verification-ID: scn.verificationscope.8b577e9712e7

- **WHEN** a change with a linkage plan has been archived and its anchors resolve to the planned targets
- **THEN** `stele verify --specs` passes for the requirements now in `openspec/specs/`

#### Scenario: Prefer the most recent archived plan
Verification-ID: scn.verificationscope.70c3ee060846

- **WHEN** two archived changes plan different targets for the same identity
- **THEN** `--specs` verification uses the target from the change archived last

#### Scenario: Run OpenSpec validation for the current specifications
Verification-ID: scn.verificationscope.f8ab6e2e1812

- **WHEN** `stele validate --specs` runs
- **THEN** it runs OpenSpec strict validation of all specifications and fails when that validation fails

#### Scenario: Reject a conflicting selection
Verification-ID: scn.verificationscope.79a83cf82aba

- **WHEN** `--specs` and `--change` are passed together
- **THEN** the command exits with `2` and explains that they cannot be combined
