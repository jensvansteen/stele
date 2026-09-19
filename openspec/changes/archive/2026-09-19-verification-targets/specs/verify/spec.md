<!-- stele: spec v1 -->
## MODIFIED Requirements

### Requirement: Verify proposal and implementation stages
Verification-ID: req.verify.1d6031f2d3dd

The `stele verify` command SHALL apply the rules of the plan's schema version. For a v1 plan, the proposal stage requires a planned target for every identity, and the implementation stage requires every identity to resolve to an anchor that matches its planned target. For a v2 plan, the proposal stage requires complete, valid, and approved evidence entries, and the implementation stage requires anchors for every approved evidence entry and every requirement, as defined by the `verification-strategy` capability. For targeted specifications, both stages also apply the per-target rules of the `verification-targets` capability.

#### Scenario: Accept planned targets in the proposal stage
Verification-ID: scn.verify.14c6b4fe39bc

- **WHEN** a v1 plan gives every requirement and scenario a planned target that does not exist yet
- **THEN** proposal verification passes and marks the linkage as planned

#### Scenario: Reject a mismatched implementation target
Verification-ID: scn.verify.c4db6a432869

- **WHEN** a v1 plan applies and an anchor resolves to a declaration other than the planned target
- **THEN** implementation verification fails with `LINK_TARGET_MISMATCH`

#### Scenario: Apply v2 rules without targets
Verification-ID: scn.verify.1d0f8685d8c6

- **WHEN** a v2 plan applies and an anchored test lives in a different file than the design suggested
- **THEN** verification passes and reports no diagnostic about the location

#### Scenario: Apply the per-target rules to targeted specifications
Verification-ID: scn.verify.1bab272a56e4
- **WHEN** a v2 plan applies to a change with one targeted and one untargeted specification, the targeted scenario has approved `ios` and `android` entries, and only the `ios` entry has an anchor
- **THEN** implementation verification reports `LINK_EVIDENCE_MISSING` for the `android` evidence ID only and verifies the untargeted specification as before

### Requirement: Report linkage and execution verdicts separately
Verification-ID: req.verify.27a52b8cfbd6

Verification reports SHALL contain a `verdicts` object with a `linkage` verdict from the diagnostics, an `execution` verdict from the current evidence, and an `overall` verdict that passes only when both pass in the implementation stage. The top-level `verdict` field SHALL equal the overall verdict. When the scope has targeted specifications, the report SHALL also contain a `targets` object with, for each covered target ordered by name, its `linkage`, `execution`, and `overall` verdicts computed from that target's evidence and diagnostics, and a scope without targeted specifications SHALL have no `targets` field. Human output SHALL name both results.

#### Scenario: Separate a failed execution from passing linkage
Verification-ID: scn.verify.140b21cbc3f0

- **WHEN** an implementation report has no linkage errors and its current evidence records a failed scenario
- **THEN** the report has linkage `pass`, execution `failed`, and overall and `verdict` `fail`, and the human summary names the execution failure

#### Scenario: Treat missing evidence as an incomplete overall verdict
Verification-ID: scn.verify.a7ae8afade0a

- **WHEN** an implementation report has no linkage errors and no current evidence
- **THEN** the report has linkage `pass`, execution `not-run`, and overall and `verdict` `incomplete`

#### Scenario: Report verdicts per target
Verification-ID: scn.verify.05d59c6b97af
- **WHEN** an implementation report covers targets `api` and `web`, every `api` test passes, and one `web` test fails
- **THEN** the `targets` object has overall `pass` for `api` and execution `failed` and overall `fail` for `web`, and the top-level `verdict` is `fail`
