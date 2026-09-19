<!-- stele: spec v1 -->
# verification-strategy Specification

## Purpose
Record, per scenario, the approved kinds of evidence that prove it, and verify that anchored tests supply that evidence, without prescribing where code or tests live.

## Requirements

### Requirement: Plan evidence per scenario
Verification-ID: req.verificationstrategy.04519b98b8bb

A `linkage-plan.json` with `schemaVersion: 2` SHALL list one or more evidence entries for every scenario of its scope, and, for a scenario of a targeted specification, one or more entries for every target the scenario applies to. Requirements are not listed; their implementation is checked through anchors. Each evidence entry has:

- an evidence ID: `<scenario>.<level>[.<n>]` for a scenario of an untargeted specification, and `<scenario>.<target>.<level>[.<n>]` for a scenario of a targeted specification, where `<n>` counts entries of the same scenario, target, and level from `2`;
- a `target`, present exactly when the scenario's specification declares targets, equal to the target in the evidence ID;
- a level of `unit`, `integration`, or `e2e`;
- a rationale;
- an optional advisory placement;
- an optional approval record.

A v2 plan SHALL NOT carry code or test locations that verification enforces. The approval digest of an entry SHALL include its target. An untargeted plan keeps its format, IDs, and approvals unchanged. Verification SHALL report `PLAN_TARGET_NOT_APPLICABLE` for an entry whose target the scenario does not apply to.

#### Scenario: Accept a complete evidence plan
Verification-ID: scn.verificationstrategy.2779f7f381d3

- **WHEN** a v2 plan lists an approved evidence entry for every scenario
- **THEN** proposal verification passes and reports each scenario's planned evidence IDs and levels

#### Scenario: Reject malformed evidence entries
Verification-ID: scn.verificationstrategy.45a816cfbf7f

- **WHEN** an evidence entry has an unknown level, an evidence ID that does not match its scenario and level, a missing rationale, or the same evidence ID as another entry
- **THEN** proposal verification fails with `PLAN_EVIDENCE_INVALID` for that entry

#### Scenario: Plan evidence per target
Verification-ID: scn.verificationstrategy.159fc8c6d372
- **WHEN** a scenario applies to `ios` and `android`, and the plan lists `<scenario>.ios.unit` with target `ios`, `<scenario>.android.unit` with target `android`, and `<scenario>.android.e2e` with target `android`, all approved
- **THEN** proposal verification passes and reports the entries of each target

#### Scenario: Reject a mismatched or missing target
Verification-ID: scn.verificationstrategy.ef89652e0e06
- **WHEN** a scenario of a targeted specification has an entry without a `target`, an entry whose `target` differs from the target in its evidence ID, and a scenario of an untargeted specification has an entry with a `target`
- **THEN** proposal verification fails with `PLAN_EVIDENCE_INVALID` for each of the three entries

#### Scenario: Require evidence for every scenario
Verification-ID: scn.verificationstrategy.aef528d23484

- **WHEN** a v2 plan omits a scenario or lists a scenario with no evidence entries
- **THEN** proposal verification fails with `PLAN_EVIDENCE_MISSING` for that scenario

#### Scenario: Require evidence for every target of a scenario
Verification-ID: scn.verificationstrategy.8d540b6ad130
- **WHEN** a scenario applies to `vscode`, `zed`, and `jetbrains`, and the plan lists entries only for `vscode` and `zed`
- **THEN** proposal verification fails with one `PLAN_EVIDENCE_MISSING` for that scenario naming `jetbrains`

#### Scenario: Reject evidence for a target the scenario does not apply to
Verification-ID: scn.verificationstrategy.53e78ff15ea6
- **WHEN** a scenario has `Targets: web` and the plan lists an entry with target `api` for it
- **THEN** proposal verification fails with `PLAN_TARGET_NOT_APPLICABLE` for that entry

#### Scenario: Reject plan entries for unknown identities
Verification-ID: scn.verificationstrategy.509b8e9fab98

- **WHEN** a v2 plan lists a scenario that the scope does not declare, or an evidence ID whose scenario differs from the entry it is listed under
- **THEN** proposal verification fails with `PLAN_UNKNOWN_ID`

### Requirement: Record and enforce approvals
Verification-ID: req.verificationstrategy.c09342159cf0

An approval record SHALL hold:

- the approver, by default `git config user.name`;
- the approval date;
- a digest of the approved entry together with its whitespace-normalized scenario text;
- `via`, with the value `cli` or `agent-confirmed`;
- optionally the revision the entry was approved at.

The `stele approve` command SHALL write these records:

- In an interactive terminal, it reviews each selected entry and asks approve, reject, or skip. For each entry it shows the scenario text, level, rationale, and advisory placement.
- `--all --yes` approves every pending entry without prompts.
- `--confirmed-in-chat` records a confirmation that a human gave in an agent conversation, with `via: agent-confirmed`.

Without a terminal, `--confirmed-in-chat`, or `--yes`, it SHALL approve nothing. Proposal and implementation verification SHALL fail for entries that are unapproved or whose digest no longer matches.

#### Scenario: Report unapproved entries
Verification-ID: scn.verificationstrategy.7efc234ae03e

- **WHEN** a v2 plan contains an evidence entry without an approval record
- **THEN** proposal verification fails with `PLAN_UNAPPROVED` naming that entry

#### Scenario: Report entries changed since approval
Verification-ID: scn.verificationstrategy.9f94fdeabd16

- **WHEN** an approved entry's level or rationale, or the wording of its scenario, changes after approval, while another scenario only changes whitespace
- **THEN** proposal verification fails with `PLAN_APPROVAL_STALE` for the first entry, and the whitespace-only change keeps its approval

#### Scenario: Approve entries from the command line
Verification-ID: scn.verificationstrategy.122427afc2f9

- **WHEN** `stele approve --change <change>` runs in an interactive terminal and the reviewer approves one entry, rejects one, and skips one
- **THEN** only the approved entry gets a record with the approver, date, digest, and `via: cli`, the other entries stay unapproved, and running the command again asks only about entries that are still pending or stale

#### Scenario: Record a confirmation given in the conversation
Verification-ID: scn.verificationstrategy.1eaf664142c0

- **WHEN** an agent runs `stele approve --change <change> --confirmed-in-chat` after the human explicitly confirmed the levels it showed
- **THEN** every pending or stale entry of the change is approved with `via: agent-confirmed`, and the output names each approved entry

#### Scenario: Refuse silent approval
Verification-ID: scn.verificationstrategy.4e5b236cc0c8

- **WHEN** `stele approve` runs without an interactive terminal and without `--confirmed-in-chat` or `--all --yes`
- **THEN** it approves nothing, exits with code `2`, and explains how to confirm approvals

### Requirement: Verify planned evidence through anchors
Verification-ID: req.verificationstrategy.c852cba36427

In the implementation stage, every approved evidence entry SHALL be matched by a `@verifies <evidence-id>` anchor on a resolvable test, and every requirement SHALL be matched by at least one `@implements` anchor. Scenario execution SHALL run every such test, and a scenario passes only when all of its evidence tests pass. The location of code and tests SHALL NOT affect the result.

#### Scenario: Accept anchored evidence wherever it lives
Verification-ID: scn.verificationstrategy.1b1070d81978

- **WHEN** every approved evidence entry has a `@verifies` anchor on a named test and every requirement has an `@implements` anchor, in any directory the scanner reads and regardless of the advisory placement
- **THEN** implementation verification passes and reports each evidence ID with its test location

#### Scenario: Report approved evidence without an anchor
Verification-ID: scn.verificationstrategy.2bcfb4ad1dbd

- **WHEN** an approved evidence entry has no matching `@verifies` anchor
- **THEN** implementation verification fails with `LINK_EVIDENCE_MISSING` for that evidence ID

#### Scenario: Reject anchors for unplanned evidence
Verification-ID: scn.verificationstrategy.6d8dbaecdc2f

- **WHEN** a `@verifies` anchor names a level-qualified ID that the plan does not list, or names a bare scenario ID while a v2 plan applies
- **THEN** implementation verification fails with `ANCHOR_EVIDENCE_UNPLANNED`

#### Scenario: Fail a scenario when one of its evidence tests fails
Verification-ID: scn.verificationstrategy.2f332f9ca4e2

- **WHEN** a scenario's unit evidence test passes and its e2e evidence test fails
- **THEN** the scenario is recorded as failed and the failing evidence ID is named

### Requirement: Accept and migrate v1 plans
Verification-ID: req.verificationstrategy.ef0c2440e601

Verification SHALL keep accepting v1 path-based plans until the 0.2.0 release, with a `PLAN_V1_DEPRECATED` warning that names that release and does not change the verdict. The `stele plan migrate --change <change>` command SHALL convert a v1 plan to v2 and leave every converted entry unapproved.

#### Scenario: Verify a v1 plan with a deprecation warning
Verification-ID: scn.verificationstrategy.6eea05b9b024

- **WHEN** a change still uses a v1 plan with `path#selector` targets
- **THEN** verification behaves as before and adds a `PLAN_V1_DEPRECATED` warning

#### Scenario: Migrate a v1 plan
Verification-ID: scn.verificationstrategy.5cb8e9ef14e7

- **WHEN** `stele plan migrate` converts a v1 plan whose test anchors carry level-qualified IDs
- **THEN** the v2 plan lists one unapproved entry per anchored level, drops every target, and gives a scenario without a level-qualified anchor an unapproved `unit` entry flagged for review

### Requirement: Guide planning and approval with the stele-plan skill
Verification-ID: req.verificationstrategy.ace39f5009c6

The `stele-plan` reference skill SHALL:

- define the three levels by what a test reaches;
- have the agent propose one or more levels per scenario, with a rationale that names the risk and why that level is the lowest convincing one;
- have the agent suggest advisory placement from the project's `AGENTS.md`, `CLAUDE.md`, skills, and existing layout;
- describe the v2 plan format, including targets and targeted evidence IDs;
- explain targets: that behavior differences belong in the specification, as separate scenarios narrowed with `Targets:` under one shared requirement, and testing differences belong in the plan, as evidence levels per target;
- give a short worked example of each of the three target patterns: replicas (the same behavior built for several targets, each needing its own evidence), split (requirements divided by responsibility with `Targets:`, and a contract requirement that lists both sides, each proving its own side), and journey (an end-to-end flow narrowed to a `system` target).

It SHALL also describe the apply-time approval step:

1. Show every pending or stale entry (scenario, level, why) in the conversation.
2. Ask "Approve these levels and start implementing?".
3. Only after an explicit yes, run `stele approve --confirmed-in-chat`.
4. Otherwise, revise the plan with the update-change workflow and ask again.
5. Never approve silently.

#### Scenario: Install planning guidance
Verification-ID: scn.verificationstrategy.f60e81cbfa96

- **WHEN** `stele init` installs the `stele-plan` skill
- **THEN** the skill contains the level definitions, the per-scenario rationale rule, the instruction to read project conventions for advisory placement, and the v2 plan format

#### Scenario: Install guidance for targets
Verification-ID: scn.verificationstrategy.1ca6af4620e9
- **WHEN** `stele init` installs the `stele-plan` skill
- **THEN** the skill explains where behavior and testing differences between targets belong, the targeted evidence ID form, and a worked example of the replica, split, and journey patterns

#### Scenario: Describe the conversational approval step
Verification-ID: scn.verificationstrategy.17786fd23375

- **WHEN** an agent reads the `stele-plan` skill at the start of applying a change
- **THEN** the skill tells it to show the pending levels, ask the approval question, run `stele approve --confirmed-in-chat` only after an explicit yes, revise through update-change otherwise, and never approve silently

#### Scenario: Keep project placement skills
Verification-ID: scn.verificationstrategy.9dad8263a6c0

- **WHEN** a project has its own placement skill beside `stele-plan` and `stele init` runs again
- **THEN** the project skill and the existing Stele skills are left unchanged
