## MODIFIED Requirements

### Requirement: Parse OpenSpec identities
Verification-ID: req.verify.9dbf2146c01f

The verifier SHALL read every Markdown delta spec of the selected change, associate each `Verification-ID` with the requirement or scenario heading above it, and report missing, malformed, repeated, and duplicate identities as errors. Requirements listed under `## REMOVED Requirements`, whether written as `### Requirement:` headers or as bullets naming such a header, SHALL NOT be parsed as requirements of the change, so they need no Verification-ID and no scenarios.

#### Scenario: Preserve requirement and scenario relationships
Verification-ID: scn.verify.6c22483ab3c3

- **WHEN** a delta spec declares a requirement ID followed by a scenario ID
- **THEN** the parsed requirement contains that scenario with both identities and no diagnostics

#### Scenario: Report identity shape errors
Verification-ID: scn.verify.c5fd3656da59

- **WHEN** a delta spec has a requirement without an ID, a requirement without scenarios, an ID of the wrong kind, two IDs for one heading, a scenario without an ID, or the same ID twice
- **THEN** the verifier reports `ID_REQUIREMENT_MISSING`, `SCENARIO_MISSING`, `ID_FORMAT`, `ID_MULTIPLE`, `ID_SCENARIO_MISSING`, and `ID_DUPLICATE`

#### Scenario: Ignore removed requirements in both forms
Verification-ID: scn.verify.9fb98ac252a0

- **WHEN** a delta spec lists one requirement under `## REMOVED Requirements` as a `### Requirement:` header with **Reason** and **Migration** and another as a bullet naming such a header, and declares one requirement with its scenarios under `## ADDED Requirements`
- **THEN** the parsed change contains only the added requirement, and verification reports no `ID_REQUIREMENT_MISSING` or `SCENARIO_MISSING` for the removed ones

## ADDED Requirements

### Requirement: Verify that removed behavior is gone
Verification-ID: req.verify.511302d1be0b

When verifying a change, Stele SHALL resolve each requirement name under `## REMOVED Requirements` against the current specification of the same capability, matching the name exactly after trimming surrounding whitespace, as OpenSpec does. The removed identities SHALL be the matched requirement's Verification-ID and the Verification-IDs of all its scenarios, except identities that the change declares again in an added, modified, or renamed requirement, because that behavior moves instead of disappearing. Verification SHALL report:

- in the implementation stage, a `LINK_REMOVED_BEHAVIOR_ANCHORED` error for every `@implements` or `@verifies` anchor that names a removed identity, including evidence IDs derived from a removed scenario such as `<scenario>.unit.2`, with the anchor's path and line;
- in both stages, a `PLAN_REMOVED_BEHAVIOR_PLANNED` error for every entry of the change's linkage plan that names a removed identity, instead of `PLAN_UNKNOWN_ID`;
- in both stages, a `SPEC_REMOVED_UNMATCHED` error for a removed name that matches no requirement in the current specification of that capability, naming the delta spec and the name.

After the change is archived, `--specs` verification continues the check for anchors, as the `verification-scope` capability defines. Requirements under `## RENAMED Requirements` keep their Verification-IDs and are not checked as removed.

#### Scenario: Report code and tests still anchored to removed behavior
Verification-ID: scn.verify.5172c64aec19

- **WHEN** a change removes a requirement whose current specification declares one requirement ID and two scenario IDs, and the code still has `@implements` for the requirement and a test has `@verifies <scenario>.unit`
- **THEN** implementation verification reports a `LINK_REMOVED_BEHAVIOR_ANCHORED` error for each of the two anchors with its path and line, and fails

#### Scenario: Pass when removed behavior has no anchors left
Verification-ID: scn.verify.c03b05c1c11e

- **WHEN** a change removes a requirement and no anchor names its requirement ID, its scenario IDs, or evidence IDs derived from them
- **THEN** implementation verification reports no removed-behavior diagnostic for it

#### Scenario: Report plan entries for removed behavior
Verification-ID: scn.verify.42bc49e1f29f

- **WHEN** the change's linkage plan lists evidence for a scenario of a removed requirement
- **THEN** proposal verification reports `PLAN_REMOVED_BEHAVIOR_PLANNED` for that entry and no `PLAN_UNKNOWN_ID` for it

#### Scenario: Reject a removed name that matches nothing
Verification-ID: scn.verify.70eaaa57724a

- **WHEN** a change lists `### Requirement: Export todos` under `## REMOVED Requirements` and the current specification of that capability has no requirement with that name
- **THEN** verification reports a `SPEC_REMOVED_UNMATCHED` error naming the delta spec and the name, and fails

#### Scenario: Accept behavior that moves to a new requirement
Verification-ID: scn.verify.ec83e3f24b0a

- **WHEN** a change removes a requirement and adds a requirement under a new name that declares the same requirement and scenario IDs, and the existing anchors name those IDs
- **THEN** verification reports no removed-behavior diagnostic for those identities
