## ADDED Requirements

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
