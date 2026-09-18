# verify Specification

## Purpose
Deterministically join OpenSpec requirements and scenarios to their planned and actual implementation and test anchors, and report the result for proposal and implementation review.

## Requirements

### Requirement: Parse OpenSpec identities
Verification-ID: req.verify.9dbf2146c01f

The verifier SHALL read every Markdown delta spec of the selected change, associate each `Verification-ID` with the requirement or scenario heading above it, and report missing, malformed, repeated, and duplicate identities as errors.

#### Scenario: Preserve requirement and scenario relationships
Verification-ID: scn.verify.6c22483ab3c3

- **WHEN** a delta spec declares a requirement ID followed by a scenario ID
- **THEN** the parsed requirement contains that scenario with both identities and no diagnostics

#### Scenario: Report identity shape errors
Verification-ID: scn.verify.c5fd3656da59

- **WHEN** a delta spec has a requirement without an ID, a requirement without scenarios, an ID of the wrong kind, two IDs for one heading, a scenario without an ID, or the same ID twice
- **THEN** the verifier reports `ID_REQUIREMENT_MISSING`, `SCENARIO_MISSING`, `ID_FORMAT`, `ID_MULTIPLE`, `ID_SCENARIO_MISSING`, and `ID_DUPLICATE`

### Requirement: Resolve anchors to declarations
Verification-ID: req.verify.43d1d0d9f883

The verifier SHALL find `@implements` and `@verifies` annotations in supported consumer sources and bind each one to the adjacent declaration or named test that it annotates.

#### Scenario: Bind anchors to adjacent TypeScript declarations
Verification-ID: scn.verify.36f0ef1f337f

- **WHEN** an `@implements` comment precedes a TypeScript function and an `@verifies` comment precedes a named test
- **THEN** the anchors resolve to that function name and that test title

#### Scenario: Ignore unsupported consumer languages
Verification-ID: scn.verify.0d43596abe4a

- **WHEN** an annotation appears in a JavaScript or other unsupported source file
- **THEN** the verifier records no anchor for it

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

### Requirement: Expose verification through the command line
Verification-ID: req.verify.999a5d082295

The `stele` executable SHALL run verification from the command line, print canonical JSON with `--json`, and exit with `0` when the selected checks pass, `1` when a deterministic check fails, and `2` when the invocation is invalid.

#### Scenario: Verify a passing change from the command line
Verification-ID: scn.verify.5e9a130cd7b4

- **WHEN** `stele verify --json` runs for a change whose anchors resolve to their planned targets
- **THEN** it exits with `0` and prints a schema `2.0` report with verdict `pass`

#### Scenario: Return stable failure exit codes
Verification-ID: scn.verify.a2c7e48b610f

- **WHEN** verification finds a policy failure, or the command is unknown
- **THEN** the executable exits with `1` for the policy failure and `2` for the unknown command

### Requirement: Produce deterministic reports
Verification-ID: req.verify.aa9f017c4cb4

Verification reports SHALL be byte-for-byte identical for identical inputs and SHALL exclude volatile metadata such as generation or recording timestamps.

#### Scenario: Emit identical JSON for identical inputs
Verification-ID: scn.verify.d6f8012b3ea5

- **WHEN** `stele verify --json` runs twice on unchanged inputs
- **THEN** both runs print identical output

#### Scenario: Omit volatile metadata
Verification-ID: scn.verify.3a70c9d1ef24

- **WHEN** a verification report is produced
- **THEN** it contains no `generatedAt` field and its execution stage contains no `recordedAt` field

### Requirement: Report linkage and execution verdicts separately
Verification-ID: req.verify.27a52b8cfbd6

Verification reports SHALL contain a `verdicts` object with a `linkage` verdict from the diagnostics, an `execution` verdict from the current evidence, and an `overall` verdict that passes only when both pass in the implementation stage. The top-level `verdict` field SHALL equal the overall verdict. Human output SHALL name both results.

#### Scenario: Separate a failed execution from passing linkage
Verification-ID: scn.verify.140b21cbc3f0

- **WHEN** an implementation report has no linkage errors and its current evidence records a failed scenario
- **THEN** the report has linkage `pass`, execution `failed`, and overall and `verdict` `fail`, and the human summary names the execution failure

#### Scenario: Treat missing evidence as an incomplete overall verdict
Verification-ID: scn.verify.a7ae8afade0a

- **WHEN** an implementation report has no linkage errors and no current evidence
- **THEN** the report has linkage `pass`, execution `not-run`, and overall and `verdict` `incomplete`

### Requirement: Name output files explicitly
Verification-ID: req.verify.3624e3449f6a

`stele test` and `stele validate` SHALL accept `--evidence-file PATH` for the evidence file, and `stele verify` and `stele validate` SHALL accept `--report-file PATH` for the report file. Until the 0.2.0 release, `--evidence PATH` and `--report PATH` SHALL keep working as aliases and print a deprecation warning that names the new flag, without changing the exit code.

#### Scenario: Write output files with the new flags
Verification-ID: scn.verify.06f2be2af1e7

- **WHEN** `stele validate --evidence-file out/evidence.json --report-file out/report.json` runs
- **THEN** the evidence and the report are written to those paths

#### Scenario: Accept the deprecated file flags with a warning
Verification-ID: scn.verify.70e950bf161c

- **WHEN** `stele verify --report out/report.json` runs
- **THEN** the report is written to that path, standard error warns that `--report` is deprecated in favor of `--report-file` until 0.2.0, and the exit code is unchanged
