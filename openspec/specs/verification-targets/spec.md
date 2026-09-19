<!-- stele: spec v1 -->
# verification-targets Specification

## Purpose
Let one specification describe behavior that several places build and prove, such as an iOS and an Android app, an API and a web front end, or an end-to-end suite, so every target proves each scenario with its own evidence and the gaps between targets stay visible. Targets are optional: without them a specification describes the project itself.

## Requirements

### Requirement: Declare the targets of a project
Verification-ID: req.verificationtargets.fd65304d2bf7
A target is a place in the repository that produces evidence, such as an app, a codebase, or an end-to-end suite. `stele.config.json` MAY contain a `targets` object that maps each target name to a definition with:

- `paths`: a non-empty array of glob patterns, relative to the project root, in which `*` matches within one path segment and `**` matches any number of segments;
- `evidenceOnly`: an optional boolean, `false` by default, for a target that proves behavior without implementing it, such as an end-to-end suite.

A target name SHALL be a lowercase letter followed by lowercase letters, digits, or hyphens, and SHALL NOT be `unit`, `integration`, or `e2e`. Targets are optional: without them, Stele verifies the project itself as one whole. The configured targets SHALL be the only targets a specification may name, and Stele SHALL order them by name wherever it lists them. The paths of different targets MAY overlap. Every command that reads the configuration SHALL stop with exit code `2`, naming the target and the problem, when a name is invalid or reserved, when `paths` is missing, empty, or holds an invalid pattern, or when a definition has an unknown key. A project whose configuration has no `targets` SHALL be verified exactly as before.

#### Scenario: Accept a target configuration
Verification-ID: scn.verificationtargets.eb1c6810d349
- **WHEN** `stele.config.json` declares `vscode`, `zed`, and `jetbrains` with one path pattern each and `system` with `paths: ["e2e/**"]` and `evidenceOnly: true`
- **THEN** `stele index --specs` lists the four targets ordered by name with their paths and `evidenceOnly` values, and reports no configuration problem

#### Scenario: Reject an invalid target definition
Verification-ID: scn.verificationtargets.36d207a647d0
- **WHEN** `stele.config.json` declares a target named `e2e`, a target named `iOS`, or a target whose `paths` is empty
- **THEN** `stele verify` exits with code `2` before verifying and names the target and its problem

### Requirement: Declare the targets of a specification
Verification-ID: req.verificationtargets.8d4a7d3bc335
A specification SHALL declare its targets with the `targets` field of its first-line annotation, such as `<!-- stele: spec v1; targets: ios, android -->`: a comma-separated list of target names with surrounding whitespace trimmed. Every requirement and scenario of the file SHALL apply to all declared targets unless it narrows them. Verification SHALL report:

- `SPEC_TARGETS_MALFORMED` when the list is empty, repeats a name, or holds a name that is not a valid target name;
- `SPEC_TARGET_UNKNOWN` when a name is not a configured target, naming the file, the target, and the configured targets.

A specification without the field is untargeted: it describes the project as a whole rather than a named target, and its plan entries and anchors follow the rules without targets. A delta spec of a capability that already has a current specification SHALL declare targets exactly when that current specification does, or else verification SHALL report `SPEC_TARGETS_MISMATCH`. A delta spec that declares a different list changes the targets of its capability, and it SHALL list, as modified, renamed, or removed requirements, every current requirement of the capability whose applicable targets would change; otherwise verification SHALL report `SPEC_TARGETS_CHANGE_UNCOVERED` naming each requirement it misses. These diagnostics SHALL be errors.

#### Scenario: Apply a specification's targets to every scenario
Verification-ID: scn.verificationtargets.e8cd45d85205
- **WHEN** a specification annotated with `targets: ios, android` has a requirement with two scenarios and no `Targets:` lines, and both targets are configured
- **THEN** both scenarios apply to `android` and `ios`, and verification reports no target diagnostic

#### Scenario: Reject an unknown or malformed target list
Verification-ID: scn.verificationtargets.975c05dc71d9
- **WHEN** one specification is annotated with `targets: ios, andriod` in a project that configures `ios` and `android`, and another with `targets: ios, ios`
- **THEN** verification reports `SPEC_TARGET_UNKNOWN` for `andriod` naming the configured targets, and `SPEC_TARGETS_MALFORMED` for the repeated name, and fails

#### Scenario: Cover every requirement when a change alters targets
Verification-ID: scn.verificationtargets.ca80f845a9cc
- **WHEN** a change's delta spec of capability `share` declares `targets: ios` while the current specification of `share` declares `targets: ios, android`, and the delta modifies only one of its three requirements
- **THEN** verification of the change reports `SPEC_TARGETS_CHANGE_UNCOVERED` naming the two current requirements that the delta does not list, and fails

### Requirement: Narrow the targets of a requirement or scenario
Verification-ID: req.verificationtargets.ab2f8aad3e78
A requirement or scenario MAY narrow its targets with one `Targets: a, b` line in the metadata block directly below its heading, which holds its `Verification-ID` line and at most one `Targets:` line, in either order. A requirement's `Targets:` line SHALL list a subset of its specification's targets, and a scenario's SHALL list a subset of its requirement's applicable targets: a scenario may only narrow, never widen. An item without a `Targets:` line SHALL apply to the targets of its parent. The `Targets:` line SHALL NOT be part of the item's text, so it never enters an approval digest, the index `text` of an item, or the scenario steps. Verification SHALL report, as errors:

- `SPEC_TARGETS_WIDENED` for a name the parent does not apply to;
- `SPEC_TARGET_UNKNOWN` for a name that is not a configured target;
- `SPEC_TARGETS_MALFORMED` for an empty list, a repeated name, or a second `Targets:` line;
- `SPEC_TARGETS_MISPLACED` for a `Targets:` line outside a heading's metadata block;
- `SPEC_TARGETS_UNDECLARED` for a `Targets:` line in a specification without a `targets` field.

#### Scenario: Narrow a scenario to one platform
Verification-ID: scn.verificationtargets.577d723898cd
- **WHEN** a specification with `targets: ios, android` has a requirement without a `Targets:` line whose scenarios are one without a `Targets:` line, one with `Targets: ios`, and one with `Targets: android`
- **THEN** the first scenario applies to `android` and `ios`, the second only to `ios`, the third only to `android`, and verification reports no target diagnostic

#### Scenario: Split requirements by responsibility
Verification-ID: scn.verificationtargets.6616ae0098fb
- **WHEN** a specification with `targets: api, web` has one requirement with `Targets: api`, one with `Targets: web`, and one without a `Targets:` line
- **THEN** the first requirement and its scenarios apply only to `api`, the second only to `web`, and the third, a contract, to both

#### Scenario: Reject a scenario that widens its requirement
Verification-ID: scn.verificationtargets.7e8b768b3da0
- **WHEN** a requirement has `Targets: api` and one of its scenarios has `Targets: api, web`
- **THEN** verification reports `SPEC_TARGETS_WIDENED` for `web` on that scenario's line, and fails

#### Scenario: Report misplaced and undeclared target lines
Verification-ID: scn.verificationtargets.e324fcddf99b
- **WHEN** one scenario has a `Targets:` line after its first step bullet, and a specification without a `targets` field has a requirement with a `Targets:` line
- **THEN** verification reports `SPEC_TARGETS_MISPLACED` for the first line and `SPEC_TARGETS_UNDECLARED` for the second, each with its file and line

#### Scenario: Keep approvals when only the targets of a scenario change
Verification-ID: scn.verificationtargets.bb95c914ccac
- **WHEN** an approved scenario gains a `Targets: ios` line and its wording is unchanged
- **THEN** its remaining `ios` evidence entries keep their approval, and its `android` entries are reported as `PLAN_TARGET_NOT_APPLICABLE`

### Requirement: Attribute anchors to targets
Verification-ID: req.verificationtargets.d1ac12f01077
The anchor scanners of every supported language SHALL read targeted evidence IDs (`<scenario>.<target>.<level>[.<n>]`) in `@verifies` anchors. Stele SHALL take the target of a `@verifies` anchor from its evidence ID, and SHALL report `ANCHOR_TARGET_OUTSIDE_PATHS` as an error when the anchor's file matches none of that target's `paths`. A file that matches the paths of several targets MAY carry evidence for each of them. Stele SHALL attribute an `@implements` anchor to every target whose `paths` match its file. In the implementation stage, for every requirement and every target it applies to that is not `evidenceOnly`, verification SHALL report `LINK_TARGET_IMPLEMENTATION_MISSING` as an error when no `@implements` anchor for the requirement is attributed to that target. The rule that every requirement has at least one `@implements` anchor SHALL still apply, so a requirement that applies only to `evidenceOnly` targets needs an anchor anywhere in the project.

#### Scenario: Read targeted evidence IDs in TypeScript and Go
Verification-ID: scn.verificationtargets.1a22d82ecf41
- **WHEN** a TypeScript test under `web/` carries `@verifies <scenario>.web.unit` and a Go test under `api/` carries `@verifies <scenario>.api.integration.2`, with matching target paths and plan entries
- **THEN** both anchors resolve to their tests with their full evidence IDs, targets `web` and `api`, and levels `unit` and `integration`, and no anchor is reported as a bare scenario anchor

#### Scenario: Report evidence anchored in the wrong target
Verification-ID: scn.verificationtargets.880647fbb533
- **WHEN** a test under `android/` carries `@verifies <scenario>.ios.unit` and the `ios` target's paths are `ios/**`
- **THEN** implementation verification reports `ANCHOR_TARGET_OUTSIDE_PATHS` with the test's path and line, naming `ios` and its paths, and fails

#### Scenario: Require an implementation in every replica
Verification-ID: scn.verificationtargets.c3f36c37e37a
- **WHEN** a requirement applies to `vscode`, `zed`, and `jetbrains`, and `@implements` anchors for it exist only in files under `vscode/` and `zed/`
- **THEN** implementation verification reports one `LINK_TARGET_IMPLEMENTATION_MISSING` error naming the requirement and `jetbrains`

#### Scenario: Accept shared code and journeys
Verification-ID: scn.verificationtargets.3574271b7f35
- **WHEN** one test in `shared/`, which both the `ios` and `android` paths match, carries `@verifies <scenario>.ios.unit` and `@verifies <scenario>.android.unit`, and a requirement with `Targets: system` has an `@implements` anchor under `api/` while `system` is `evidenceOnly`
- **THEN** implementation verification reports no target diagnostic for either

### Requirement: Compute a scenario by target matrix
Verification-ID: req.verificationtargets.4ed90b4188d7
For a scope with at least one targeted specification, Stele SHALL compute one row per targeted scenario, in specification order, and one column per configured target that a specification of the scope declares, ordered by name. Each cell SHALL list the evidence IDs of that scenario and target and SHALL have exactly one state:

- `n/a`: the scenario does not apply to the target;
- `missing`: the scenario applies, and the target has no plan entry, or has an approved entry without a resolvable `@verifies` anchor;
- `unapproved`: at least one entry is unapproved or its approval is stale, whether or not it has an anchor;
- `not-run`, `stale`, `failed`, or `passed`: every entry is approved and anchored, and the cell takes the worst execution state of its entries in the order `failed`, `stale`, `not-run`, `passed`.

When a cell qualifies for several states, the order `failed`, `missing`, `unapproved`, `stale`, `not-run`, `passed` SHALL decide. The matrix SHALL be computed by one in-process component used by the index, the report, and later the language server, and SHALL be deterministic.

#### Scenario: Show gaps between replicas
Verification-ID: scn.verificationtargets.e03ff09a1cd4
- **WHEN** a scenario applies to `ios` and `android`, its `ios` evidence is approved, anchored, and passed, and it has no `android` entry in the plan
- **THEN** its row has `passed` for `ios` with the `ios` evidence ID and `missing` for `android` with no evidence IDs

#### Scenario: Mark scenarios that do not apply
Verification-ID: scn.verificationtargets.250dbc097c43
- **WHEN** a specification with `targets: api, web` has a scenario with `Targets: web`
- **THEN** its row has `n/a` for `api`

#### Scenario: Prefer a failure over other states
Verification-ID: scn.verificationtargets.35a9719d9446
- **WHEN** a cell has one entry whose test failed and another approved entry without an anchor
- **THEN** the cell state is `failed` and lists both evidence IDs

### Requirement: Select targets in commands
Verification-ID: req.verificationtargets.366797f24e77
The `stele test`, `stele verify`, `stele validate`, `stele check`, and `stele index` commands SHALL accept `--target <name>`, repeatable, and `--target` SHALL combine with `--change`, `--specs`, `--all`, and test selections. With `--target`, a command SHALL select only the scenarios that apply to a selected target, the requirements that apply to one, and only the evidence entries and anchors of the selected targets:

- `stele test` SHALL run only the tests of the selected entries, merging their outcomes into the stored evidence as a selection does;
- verification SHALL report plan, linkage, and execution findings only for the selected entries and targets, and SHALL still report every specification, identity, and annotation finding of the scope;
- untargeted specifications describe the project as a whole, so they SHALL stay in scope with all their findings, entries, and tests;
- the report and the JSON document SHALL name the selected targets.

A `--target` that names no configured target, or any `--target` in a project without targets, SHALL exit with code `2` before anything runs. Without `--target`, commands SHALL cover every target.

#### Scenario: Run the tests of one target
Verification-ID: scn.verificationtargets.258ffea7adf7
- **WHEN** `stele test --specs --target android` runs in a project whose scenarios have passing `ios` and `android` evidence tests
- **THEN** only the tests of `android` evidence entries run, the stored `ios` outcomes are unchanged, and the report names the target `android`

#### Scenario: Verify one target without the others' gaps
Verification-ID: scn.verificationtargets.fc749b62bad7
- **WHEN** `stele verify --change <id> --target ios` runs for a change whose `android` entries have no anchors, while every `ios` entry is anchored and one scenario lacks a Verification-ID
- **THEN** the report has no `LINK_EVIDENCE_MISSING` for `android` entries, reports the missing Verification-ID, and names `ios` as the selected target

#### Scenario: Check one target in CI
Verification-ID: scn.verificationtargets.db222c63967d
- **WHEN** the installed `stele check --all --target web` runs in a project whose `api` entries are unapproved and whose `web` entries are approved, anchored, and passing
- **THEN** the ID and annotation steps cover every scope, validation covers only `web` evidence, no `PLAN_UNAPPROVED` is reported for `api` entries, and the command exits with `0`

#### Scenario: Reject an unknown target selection
Verification-ID: scn.verificationtargets.857e3746c99d
- **WHEN** `stele test --target androd` runs in a project that configures `ios` and `android`, or `stele verify --target ios` runs in a project without targets
- **THEN** the command exits with code `2`, names the accepted targets or says that the project configures none, and runs nothing

### Requirement: Keep untargeted projects unchanged
Verification-ID: req.verificationtargets.d74fcbc51f5c
In a project whose configuration has no `targets` and whose specifications declare none, every command SHALL produce the same standard output, JSON documents, report files, evidence files, and index as before this change, byte for byte, and every existing v2 plan and approval SHALL stay valid.

#### Scenario: Produce identical output without targets
Verification-ID: scn.verificationtargets.add73aad6d42
- **WHEN** `stele validate --specs --json`, `stele index --specs`, and the human report run on an untargeted fixture project whose outputs were recorded from the release before this change
- **THEN** the outputs match the recorded ones byte for byte, and no approval becomes stale
