## MODIFIED Requirements

### Requirement: Verify proposal and implementation stages
Verification-ID: req.verify.1d6031f2d3dd

The `stele verify` command SHALL apply the rules of the plan's schema version. For a v1 plan, the proposal stage requires a planned target for every identity, and the implementation stage requires every identity to resolve to an anchor that matches its planned target. For a v2 plan, the proposal stage requires complete, valid, and approved evidence entries, and the implementation stage requires anchors for every approved evidence entry and every requirement, as defined by the `verification-strategy` capability.

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
