<!-- stele: spec v1 -->
# spec-annotation Specification

## Purpose
Let Stele and editors recognize Stele specification files deterministically from a versioned annotation on their first line, keep that annotation in place through authoring and archiving, and expose it to tools.

## Requirements

### Requirement: Declare a Stele specification on the first line
Verification-ID: req.specannotation.331a671f3614

A specification file SHALL be a Stele specification of format version 1 when its first line, after an optional UTF-8 byte order mark, is an HTML comment of the form `<!-- stele: spec v1 -->`. Spaces and tabs SHALL be allowed before and after the comment and around every token, and at least one space or tab SHALL separate `spec` from the version. The words `stele` and `spec` are lowercase. After the version, the comment MAY contain fields, each written as `;` followed by `key: value`, where a key is a lowercase letter followed by lowercase letters, digits, or hyphens, and a value is any text without `;` or `--`, with surrounding whitespace trimmed. Version 1 defines no fields, so every field SHALL be ignored with a `SPEC_ANNOTATION_FIELD_IGNORED` warning, including a field that does not follow this form. Stele SHALL report a first-line comment that starts with `stele:` but does not match the form as `SPEC_ANNOTATION_MALFORMED`, a version other than `v1` as `SPEC_ANNOTATION_UNSUPPORTED`, and a line other than the first that consists only of a Stele annotation as `SPEC_ANNOTATION_MISPLACED`. Malformed and unsupported annotations SHALL be errors.

#### Scenario: Recognize the canonical annotation
Verification-ID: scn.specannotation.0de8bd2cbe54

- **WHEN** a specification's first line is `<!-- stele: spec v1 -->`
- **THEN** Stele treats it as a Stele specification of version `v1` and reports no annotation diagnostic

#### Scenario: Tolerate whitespace, line endings, and a byte order mark
Verification-ID: scn.specannotation.0d7c07caa518

- **WHEN** a specification starts with a byte order mark and its first line is `  <!--stele:   spec	v1-->` followed by a CRLF line ending
- **THEN** Stele recognizes version `v1` exactly as for the canonical form

#### Scenario: Ignore fields with a warning
Verification-ID: scn.specannotation.6434ff2d493c

- **WHEN** the first line is `<!-- stele: spec v1; targets: vscode, zed; owner -->`
- **THEN** Stele recognizes version `v1`, reports one `SPEC_ANNOTATION_FIELD_IGNORED` warning for each of `targets` and `owner`, and the file's verification is otherwise unchanged

#### Scenario: Reject an unsupported version
Verification-ID: scn.specannotation.c1a89d2bc235

- **WHEN** the first line is `<!-- stele: spec v2 -->`
- **THEN** verification reports a `SPEC_ANNOTATION_UNSUPPORTED` error naming the file and the version, and fails

#### Scenario: Reject a malformed annotation
Verification-ID: scn.specannotation.10178af5c550

- **WHEN** the first line is `<!-- stele: spec -->` or `<!-- stele: specification v1 -->`
- **THEN** verification reports a `SPEC_ANNOTATION_MALFORMED` error naming the file, and fails

#### Scenario: Report an annotation below the first line
Verification-ID: scn.specannotation.2681b7fc2880

- **WHEN** a specification's first line is a heading and its third line is `<!-- stele: spec v1 -->`
- **THEN** Stele does not treat the file as annotated and reports `SPEC_ANNOTATION_MISPLACED` for line 3 instead of `SPEC_ANNOTATION_MISSING`, with the severity that the project's policy gives a missing annotation

### Requirement: Report unannotated specifications according to the project policy
Verification-ID: req.specannotation.ec94d82dea47

Stele SHALL keep reading and verifying every specification file of the selected scope, annotated or not, and SHALL never skip a file because it lacks an annotation. For each specification without a valid first-line annotation, verification SHALL report `SPEC_ANNOTATION_MISSING` with the file path and the command that adds it. The `unannotatedSpecs` field of `stele.config.json` SHALL set the severity of `SPEC_ANNOTATION_MISSING` and `SPEC_ANNOTATION_MISPLACED`: `warn` reports a warning and `error` reports an error. Until Stele 0.2.0 the default SHALL be `warn`. Any other value SHALL stop the command with exit code `2`. Archived changes SHALL NOT be checked.

#### Scenario: Warn about an unannotated specification by default
Verification-ID: scn.specannotation.0f1fab2a02b8

- **WHEN** `stele validate --specs` runs in a project without `unannotatedSpecs` whose current specification has no annotation and whose checks otherwise pass
- **THEN** the report contains a `SPEC_ANNOTATION_MISSING` warning that names the file and `stele annotate --specs`, the file's requirements are still verified, and the command exits with `0`

#### Scenario: Fail on an unannotated specification when the policy requires it
Verification-ID: scn.specannotation.3c8b5da99a7e

- **WHEN** `stele verify --change <id>` runs with `unannotatedSpecs` set to `error` and one delta spec of the change has no annotation
- **THEN** it reports a `SPEC_ANNOTATION_MISSING` error for that file and exits with `1`

#### Scenario: Reject an unknown policy value
Verification-ID: scn.specannotation.ee994e1a42b9

- **WHEN** `unannotatedSpecs` is set to `ignore`
- **THEN** `stele verify` exits with `2` before verifying and names the accepted values `warn` and `error`

### Requirement: Annotate specification files
Verification-ID: req.specannotation.c4d7843868f5

The `stele annotate [--change <id> | --specs | --all]` command SHALL insert `<!-- stele: spec v1 -->` as the first line of every specification file in the scope that has no annotation, after a byte order mark if there is one, ending it with the file's first line ending, or with a line feed when the file has none. It SHALL preserve every other byte and change nothing in a file that already has a valid annotation, so a second run changes nothing. It SHALL leave a file with a malformed, unsupported, or misplaced annotation unchanged, report it, and exit with code `1`. With `--check` it SHALL write nothing and exit with code `1` while any file lacks a valid annotation. With `--json` it SHALL print deterministic JSON listing each file of the scope with its state. `--all` covers the current specifications and the delta specs of every active change, never archived changes.

#### Scenario: Annotate the files of a scope and preserve every other byte
Verification-ID: scn.specannotation.c1e6a83408c6

- **WHEN** `stele annotate --change <id>` runs for a change with one delta spec using CRLF line endings without a final newline and one delta spec that is already annotated
- **THEN** the first file gains `<!-- stele: spec v1 -->` followed by CRLF as its first line with every other byte unchanged, the second file is unchanged, and the command reports the annotated file

#### Scenario: Change nothing on a second run
Verification-ID: scn.specannotation.6bd9c9356646

- **WHEN** `stele annotate --all` runs twice on the same project
- **THEN** the second run writes no file, reports every file as already annotated, and exits with `0`

#### Scenario: Check annotations without writing
Verification-ID: scn.specannotation.a518efc1ddd8

- **WHEN** `stele annotate --specs --check --json` runs while one current specification lacks an annotation
- **THEN** no file changes, the JSON output lists that file as missing and the others as annotated, and the command exits with `1`, or with `0` once every file is annotated

#### Scenario: Leave broken annotations for a person to fix
Verification-ID: scn.specannotation.db2f08beff7a

- **WHEN** `stele annotate --all` finds one file with a malformed first-line annotation, one with an unsupported version, and one with an annotation on a later line
- **THEN** it leaves all three files unchanged, names each with its problem, still annotates the other files that lack an annotation, and exits with `1`

### Requirement: Annotate delta specs while assigning identities
Verification-ID: req.specannotation.809d0513cbaa

`stele ids` SHALL add the annotation to every delta spec of the change that lacks one, in the same write as the missing Verification-IDs, following the insertion rules of `stele annotate`, and SHALL report the line numbers of inserted IDs as they are in the written file. It SHALL NOT modify a delta spec whose annotation is malformed or unsupported, and SHALL report it and exit with code `1`. With `--check` it SHALL report missing annotations next to missing IDs, and a missing annotation SHALL fail the check only when `unannotatedSpecs` is `error`. With `--json` it SHALL list the annotated or unannotated files in an `annotations` array.

#### Scenario: Add the annotation together with missing IDs
Verification-ID: scn.specannotation.7a12f1bf3cf0

- **WHEN** `stele ids --change <id>` runs for a delta spec without an annotation and without IDs
- **THEN** the file starts with `<!-- stele: spec v1 -->`, every heading has an ID, the reported ID line numbers match the written file, and a second run changes nothing

#### Scenario: Check a missing annotation according to the policy
Verification-ID: scn.specannotation.f0d8e6fbaa2d

- **WHEN** `stele ids --check` runs for a change whose delta specs have every ID but one lacks an annotation
- **THEN** it lists the unannotated file and exits with `0` under the default `warn` policy, and with `1` when `unannotatedSpecs` is `error`

### Requirement: Annotate specifications during initialization
Verification-ID: req.specannotation.0784be141698

`stele init` SHALL annotate the current specifications and the delta specs of every active change that lack an annotation, following the insertion rules of `stele annotate`, and list each annotated file in its output. It SHALL NOT modify archived changes, and a repeated run SHALL change no specification. When `stele init` installs or refreshes the `stele` workflow schema, the schema's specification template SHALL start with the annotation.

#### Scenario: Annotate existing specifications once
Verification-ID: scn.specannotation.dfe775967c44

- **WHEN** `stele init` runs in an OpenSpec project with an unannotated current specification, an unannotated active change, and an archived change, and then runs again
- **THEN** the first run annotates the current specification and the active change's delta specs and lists them, the archived change is unchanged, and the second run changes no specification

#### Scenario: Start new delta specs from an annotated template
Verification-ID: scn.specannotation.83d1cca317e3

- **WHEN** `stele init` installs the `stele` schema and OpenSpec prints the instructions for a change's `specs` artifact
- **THEN** the specification template in those instructions starts with `<!-- stele: spec v1 -->`

### Requirement: Restore the annotation after archiving
Verification-ID: req.specannotation.1707277552af

The `stele-archive` skill SHALL run `stele annotate --specs` after the `openspec-archive-change` step and before `stele validate --specs`. The archive guidance that `stele init` merges into `openspec/config.yaml` SHALL name the same step. After a change that creates a new capability is archived, `stele annotate --specs` SHALL restore the annotation on the current specification that the archive created, and SHALL keep a merged current specification's existing annotation as the only one.

#### Scenario: Archive through Stele restores the annotation
Verification-ID: scn.specannotation.3baa32066f7a

- **WHEN** an agent follows `stele-archive`
- **THEN** it runs `stele validate --change`, stops if it fails, uses `openspec-archive-change`, runs `stele annotate --specs`, and finishes with `stele validate --specs`

#### Scenario: Direct OpenSpec archiving names the repair step
Verification-ID: scn.specannotation.23d533778e10

- **WHEN** `stele init` merges its guidance into `openspec/config.yaml`
- **THEN** the archive guidance tells the agent to run `stele annotate --specs` before `stele validate --specs`

#### Scenario: Restore the annotation on a new current specification
Verification-ID: scn.specannotation.b7e05fe76741

- **WHEN** OpenSpec archives an annotated change that creates one new capability and modifies another whose current specification is annotated, and `stele annotate --specs` runs afterwards
- **THEN** both current specifications start with exactly one `<!-- stele: spec v1 -->` line, and `stele validate --specs` reports no annotation diagnostic

### Requirement: Expose annotation state in the link index
Verification-ID: req.specannotation.a6d30cbac541

`stele index` SHALL list, for every scope, each specification file with its path, its annotation state (`annotated`, `missing`, `misplaced`, `malformed`, or `unsupported`), and its declared version, or `null` when there is none. Each requirement and scenario SHALL carry the version of its file as `specVersion`, or `null` unless the file is annotated with a supported version. The output SHALL stay deterministic.

#### Scenario: Index records the annotation of each file and item
Verification-ID: scn.specannotation.4432e2c478c1

- **WHEN** `stele index --change <id>` runs for a change with one annotated and one unannotated delta spec, next to annotated current specifications
- **THEN** the index lists each file once per scope with state `annotated` and version `v1`, or `missing` and `null`, and each requirement and scenario carries the `specVersion` of its file
