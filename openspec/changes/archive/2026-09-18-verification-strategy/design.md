## Context

See proposal.md for the product direction. This change builds on `authoring-workflow`, which installs the Stele OpenSpec workflow schema. That schema's `verification` artifact writes the change's plan following `stele-plan`, and its apply instruction starts with the approval step described in `stele-plan`. This change fills in both: the plan format and the approval flow. It changes only `stele-plan`, `stele-verify`, and the CLI, not the schema.

The current state of the code:
- **Plan:** `linkage-plan.json` v1 maps each ID to one `path#selector`.
- **Anchors:** the scanners keep only the base ID of an anchor.
- **Execution:** scenario execution already fails a scenario when any of its tests fails.
- **Approval:** approvals are prose in design.md.

## Goals / Non-Goals

**Goals:**

- The plan records human decisions: levels, rationales, advisory placement, and approvals.
- Approval happens naturally in the conversation at apply time, and is never silent.
- Anchors decide where code lives, and verification never judges placement.
- v1 users keep working and have a mechanical path to v2.

**Non-Goals:**

- Authenticating approvers. Stele records approvals, and teams can require PR review of plan files on top.
- The editor "Approve" button (Step 3). The `via` field already reserves `editor`.
- Evidence kinds other than tests.

## Decisions

### Plan schema v2

```json
{
  "schemaVersion": 2,
  "changeId": "todo-basics",
  "scenarios": {
    "scn.todo.0a1b2c3d4e5f": {
      "evidence": [
        {
          "id": "scn.todo.0a1b2c3d4e5f.unit",
          "level": "unit",
          "rationale": "Normalization is pure logic; no I/O is needed.",
          "placement": "tests/todo.test.mts, next to the existing Todo unit tests (AGENTS.md: tests beside modules)",
          "approval": {
            "approver": "Jens",
            "date": "2026-09-17",
            "digest": "sha256:…",
            "via": "agent-confirmed",
            "revision": "16f57cf"
          }
        }
      ]
    }
  }
}
```

- **Evidence IDs.** The first entry of a level is `<scenario>.<level>`, and later entries of the same level are `.<level>.<n>` with `n` starting at 2.
- **Placement is advisory.** It is shown during approval and never verified. It lives in the plan, so the `verification` artifact, `stele approve`, and later `stele index` all read one source.
- **No requirement entries.** Requirements need at least one `@implements` anchor, which is an implementation rule, not an approval.
- **Unapproved entries** are entries without an `approval`.

### Digest and staleness

- **Digest input.** The digest is SHA-256 over canonical JSON of `{scenarioId, id, level, rationale, scenarioText}`.
- **Scenario text.** `scenarioText` is the scenario block with the `Verification-ID` line removed and every run of whitespace collapsed to one space.
- **Stale entries.** Rewording a scenario, or changing the level or rationale, makes an approval stale.
- **What does not make an entry stale** (confirmed in review): whitespace-only edits and placement changes. Placement is advisory.
- **Informational fields.** `date` and `revision` are informational and never compared. The plan is an input file, so dates do not affect report determinism.

### Approval flow

**Default: conversation at apply time.** The approval step in `stele-plan`, referenced by the schema's apply instruction:

1. Run `stele verify --stage proposal --change <change>`, and list every entry reported as unapproved or stale.
2. Show them in the conversation as a table: scenario, level, why, and suggested placement.
3. Ask: "Approve these levels and start implementing?"
4. Only after an explicit yes, run `stele approve --change <change> --confirmed-in-chat`.
5. On anything else, revise the plan through the update-change workflow, run the proposal gate again, and ask again.
6. Never approve silently, and never treat a request to "apply" or "implement" as approval.

Changed entries become stale, so they come back in step 1 at the next apply.

**`stele approve [--change <id> | --specs]`:**

- **In a terminal:** reviews pending and stale entries one at a time. It shows the scenario text, level, rationale, and placement, and asks `approve / reject / skip`. Approved entries get `via: cli`. Rejected entries stay unapproved and are listed at the end, so they can be revised.
- **`--evidence <id>...` or `--scenario <id>...`:** limits the selection.
- **`--all --yes`:** approves all pending and stale entries without prompts (`via: cli`), for a human's bulk use.
- **`--confirmed-in-chat`:** the agent path. It approves the pending and stale entries of the selection with `via: agent-confirmed`, and prints each approved entry. It is documented as a human-confirmed action that an agent may only run after an explicit yes in the conversation.
- **Otherwise:** without a terminal, it approves nothing and exits `2` with an explanation.
- **`--by <name>`:** overrides the approver. The default is `git config user.name`; if that is empty, the command exits `2` and asks for `--by`.

**Rejected alternatives:**
- Approval only in a terminal. Agents cannot drive it, and the default path must work inside the conversation.
- Approval inferred from an "apply" request. That would be silent approval.

### Evidence verification

The anchor scanners keep the full annotated ID and split it into the base identity and the `.<level>[.<n>]` suffix. Anchors gain `evidenceId` and `level`.

When a v2 plan applies:

1. **Missing evidence.** Every approved entry needs a `@verifies <evidence-id>` anchor with a resolved selector; otherwise `LINK_EVIDENCE_MISSING`.
2. **Unplanned evidence.** In-scope `@verifies` anchors must name a planned evidence ID; otherwise `ANCHOR_EVIDENCE_UNPLANNED`.
3. **Implementations.** Every requirement needs at least one resolved `@implements` anchor; otherwise `LINK_CODE_MISSING`.
4. **Approval state.** Unapproved or stale entries fail both stages: `PLAN_UNAPPROVED` and `PLAN_APPROVAL_STALE`. This makes `stele validate --change` the backstop when an agent skipped the approval step.
5. **Execution.** It runs every anchored evidence test, and a failure wins. Evidence records name their evidence ID.
6. **Placement.** No diagnostic compares locations.

### v1 compatibility and migration

- **v1 plans.** A `schemaVersion` of `1`, or none, keeps today's behavior and adds a `PLAN_V1_DEPRECATED` warning. Warnings never change the verdict.
- **Removal** (confirmed in review): v1 support is removed in 0.2.0, and the warning names that release.
- **`stele plan migrate`** creates unapproved entries:
  - one per distinct level-qualified test anchor, with the rationale "Migrated from v1; review level and rationale";
  - otherwise one `unit` entry, with the rationale "Migrated from v1 without a level; choose the level".

  It drops targets, writes the file in place, and for `--specs` migrates each archived plan.

### Reference skills

- **`stele-plan`** is rewritten. It covers:
  - level definitions, by what a test reaches (see below);
  - reading `AGENTS.md`, `CLAUDE.md`, installed skills, and the existing test layout before suggesting placement;
  - one entry per piece of evidence, with a risk rationale saying why this level is the lowest convincing one, and why any second level covers a distinct risk;
  - the v2 format;
  - the apply-time approval step above.
- **`stele-verify`** learns the evidence rules.
- **`stele init`** only creates missing Stele skill files, so project placement skills in `.agents/skills/` stay untouched.

**Level definitions:**
- **unit:** code called directly, in process, possibly with temporary files or stubbed dependencies.
- **integration:** our code working with one real outside tool or service.
- **e2e:** the real product through its user-facing entry point. That is the UI for applications, and the installed executable or package for command-line tools.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-17, before implementation started (via: agent-confirmed, chat review). The review confirmed: v1 plans are accepted until 0.2.0; rewording a scenario invalidates its approvals and whitespace-only edits do not; `stele approve` offers `--confirmed-in-chat` and `--all --yes`.

Placement follows this repository's AGENTS.md: co-located `_test.go` files in `internal/stele`, and executable tests in `tests/cli.test.mts`. Placement is advisory. This change is verified with the published v1 verifier, so its linkage plan is v1 and records the ★ row.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Target | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|---|
| `scn.verificationstrategy.2779f7f381d3` Accept a complete evidence plan | unit ★ | `….2779f7f381d3.unit` | `internal/stele/plan_test.go`, beside `plan.go` | `internal/stele/plan_test.go#TestProposalAcceptsApprovedEvidencePlan` | Risk: valid plans are rejected. Pure parsing over fixtures. |
| `scn.verificationstrategy.45a816cfbf7f` Reject malformed evidence entries | unit ★ | `….45a816cfbf7f.unit` | `plan_test.go` | `internal/stele/plan_test.go#TestProposalRejectsMalformedEvidence` | Risk: the ID contract breaks. Table-driven checks. |
| `scn.verificationstrategy.aef528d23484` Require evidence for every scenario | unit ★ | `….aef528d23484.unit` | `plan_test.go` | `internal/stele/plan_test.go#TestProposalRequiresEvidencePerScenario` | Risk: a scenario has no approved proof. |
| `scn.verificationstrategy.509b8e9fab98` Reject plan entries for unknown identities | unit ★ | `….509b8e9fab98.unit` | `plan_test.go` | `internal/stele/plan_test.go#TestProposalRejectsUnknownPlanIDs` | Risk: stale entries survive renames. |
| `scn.verificationstrategy.7efc234ae03e` Report unapproved entries | unit ★ | `….7efc234ae03e.unit` | `internal/stele/approve_test.go`, beside `approve.go` | `internal/stele/approve_test.go#TestProposalReportsUnapprovedEntries` | Risk: agent-written plans pass without review. |
| `scn.verificationstrategy.9f94fdeabd16` Report entries changed since approval | unit ★ | `….9f94fdeabd16.unit` | `approve_test.go` | `internal/stele/approve_test.go#TestProposalReportsStaleApprovals` | Risk: approvals survive changed meaning, or go stale on whitespace. Digest normalization is pure. |
| `scn.verificationstrategy.122427afc2f9` Approve entries from the command line | unit ★ | `….122427afc2f9.unit` | `approve_test.go` | `internal/stele/approve_test.go#TestApproveReviewsEntriesInteractively` | Risk: wrong entries are approved, or rejects are recorded as approvals. The prompt reads from an injected terminal reader and writer. |
| | e2e | `….122427afc2f9.e2e` | `tests/cli.test.mts` (executable tests) | `tests/cli.test.mts#runs the approved evidence workflow end to end` | Distinct risk: `approve`, `verify`, and `test` do not compose in the shipped binary on a real project. |
| `scn.verificationstrategy.1eaf664142c0` Record a confirmation given in the conversation | unit ★ | `….1eaf664142c0.unit` | `approve_test.go` | `internal/stele/approve_test.go#TestApproveRecordsConversationConfirmation` | Risk: the agent path records the wrong `via` or approver, or misses stale entries. |
| `scn.verificationstrategy.4e5b236cc0c8` Refuse silent approval | unit ★ | `….4e5b236cc0c8.unit` | `approve_test.go` | `internal/stele/approve_test.go#TestApproveRefusesWithoutConfirmation` | Risk: an agent approves by just calling the command. The terminal check is injected, so the test proves the refusal. |
| `scn.verificationstrategy.1b1070d81978` Accept anchored evidence wherever it lives | unit ★ | `….1b1070d81978.unit` | `internal/stele/evidence_test.go`, beside `evidence.go` | `internal/stele/evidence_test.go#TestImplementationAcceptsEvidenceAnywhere` | Risk: the verifier judges placement after all. |
| | e2e | `….1b1070d81978.e2e` | `tests/cli.test.mts` | `tests/cli.test.mts#runs the approved evidence workflow end to end` | Distinct risk: the shipped scanner roots miss real layouts. |
| `scn.verificationstrategy.2bcfb4ad1dbd` Report approved evidence without an anchor | unit ★ | `….2bcfb4ad1dbd.unit` | `evidence_test.go` | `internal/stele/evidence_test.go#TestImplementationReportsMissingEvidence` | Risk: an approved level is never proven. |
| `scn.verificationstrategy.6d8dbaecdc2f` Reject anchors for unplanned evidence | unit ★ | `….6d8dbaecdc2f.unit` | `evidence_test.go` | `internal/stele/evidence_test.go#TestImplementationRejectsUnplannedEvidence` | Risk: unreviewed tests count as evidence. |
| `scn.verificationstrategy.2f332f9ca4e2` Fail a scenario when one of its evidence tests fails | unit ★ | `….2f332f9ca4e2.unit` | `internal/stele/runner_test.go`, beside outcome merging | `internal/stele/runner_test.go#TestScenarioFailsWhenOneEvidenceFails` | Risk: a passing unit test masks a failing e2e test. Aggregation logic with a stubbed runner. |
| `scn.verificationstrategy.6eea05b9b024` Verify a v1 plan with a deprecation warning | unit ★ | `….6eea05b9b024.unit` | `plan_test.go` | `internal/stele/plan_test.go#TestV1PlanStillVerifiesWithWarning` | Risk: existing users break. |
| `scn.verificationstrategy.5cb8e9ef14e7` Migrate a v1 plan | unit ★ | `….5cb8e9ef14e7.unit` | `internal/stele/migrate_test.go`, beside `migrate.go` | `internal/stele/migrate_test.go#TestPlanMigrateConvertsV1Plans` | Risk: migration invents approvals or loses levels. |
| `scn.verificationstrategy.f60e81cbfa96` Install planning guidance | unit ★ | `….f60e81cbfa96.unit` | `internal/stele/init_test.go`, beside the skill installer | `internal/stele/init_test.go#TestPlanningSkillContainsStrategyGuidance` | Risk: agents prescribe placement or skip rationales. Template content check. |
| `scn.verificationstrategy.17786fd23375` Describe the conversational approval step | unit ★ | `….17786fd23375.unit` | `init_test.go` | `internal/stele/init_test.go#TestPlanningSkillDescribesConversationalApproval` | Risk: agents approve silently, or treat "apply" as consent. Template content check; the CLI refusal is the enforced part. |
| `scn.verificationstrategy.9dad8263a6c0` Keep project placement skills | unit ★ | `….9dad8263a6c0.unit` | `init_test.go` | `internal/stele/init_test.go#TestInitializeKeepsProjectSkills` | Risk: re-running init overwrites project guidance. |
| `scn.verify.14c6b4fe39bc` Accept planned targets in the proposal stage (modified) | unit ★ | `….14c6b4fe39bc.unit` (existing) | `internal/stele/verify_test.go` (existing) | `internal/stele/verify_test.go#TestRunVerificationProposalPlans` | Risk: v1 regression. |
| `scn.verify.c4db6a432869` Reject a mismatched implementation target (modified) | unit ★ | `….c4db6a432869.unit` (existing) | `verify_test.go` (existing) | `internal/stele/verify_test.go#TestRunVerificationRejectsMismatchedTarget` | Risk: v1 regression. |
| `scn.verify.1d0f8685d8c6` Apply v2 rules without targets | unit ★ | `….1d0f8685d8c6.unit` | `evidence_test.go` | `internal/stele/evidence_test.go#TestV2VerificationIgnoresLocation` | Risk: v1 target logic leaks into v2. |

Requirement implementation targets (v1 plan today):

| Requirement | Target |
|---|---|
| `req.verificationstrategy.04519b98b8bb` Plan evidence per scenario | `internal/stele/plan.go#parseLinkagePlan` |
| `req.verificationstrategy.c09342159cf0` Record and enforce approvals | `internal/stele/approve.go#approveEvidence` |
| `req.verificationstrategy.c852cba36427` Verify planned evidence through anchors | `internal/stele/evidence.go#evidenceDiagnostics` |
| `req.verificationstrategy.ef0c2440e601` Accept and migrate v1 plans | `internal/stele/migrate.go#migrateV1Plan` |
| `req.verificationstrategy.ace39f5009c6` Guide planning and approval with the stele-plan skill | `internal/stele/init.go#Initialize` |
| `req.verify.1d6031f2d3dd` Verify proposal and implementation stages (modified) | `internal/stele/verify.go#RunVerification` |

## Risks / Trade-offs

- [An agent can run `--confirmed-in-chat` without a real yes] → The flag name and docs make the claim explicit. The output names each approval for the human to see. `via: agent-confirmed` shows in review diffs, and teams can require PR review of plan files. Stricter review is available with `stele approve` in a terminal.
- [The approver default depends on local git config] → An empty name exits `2`, and `--by` overrides it.
- [Rewording a scenario invalidates its approvals] → This is intended. Re-approval is one question at the next apply.
- [Two plan schemas until 0.2.0] → Both are covered by the 100% coverage gate, and migration shortens the overlap.
- [This repository's plans are v1] → After the release, migrate them, review them against the design tables, and approve them through the conversational step.

## Migration Plan

1. Release v2 support with v1 accepted and warned.
2. In this repository, run `stele plan migrate` for archived and active plans, then have the maintainer approve them in the conversation or with `stele approve`.
3. Remove v1 in 0.2.0.

## Implementation notes

Accepted by jensvansteen on 2026-09-17: the schema template, the warning handling with per-scenario mixed archives, and the package files below.

- **Deviation from "not the schema":** the `stele` schema installed by `authoring-workflow` ships a `linkage-plan.json` template. It now contains the v2 skeleton (`schemaVersion: 2`, `changeId`, `scenarios`), so agents that follow the schema write the format `stele-plan` describes. The schema's artifacts and instructions are unchanged. Projects refresh it with `stele init --refresh-schema`.
- **Mixed archives:** with `--specs`, a scenario follows the v2 rules when the latest archived plan that lists it is a v2 plan. A requirement follows them when no v1 plan targets it and one of its scenarios follows them. `PLAN_UNKNOWN_ID` for unlisted scenarios applies only to a change plan, because archived plans may name scenarios that later changes removed.
- **Reports:** v2 scenarios report their planned evidence (ID, level, approval state, placement), and links carry `evidenceId` and `level`. Test executions list `evidenceIds`, and a failed scenario names its `failedEvidence`. All new fields are omitted when empty, so v1 reports are unchanged.
- **Warnings:** `PLAN_V1_DEPRECATED` is a warning and no longer marks the proposal stage as failed; only errors do.
- **Package files:** `docs/intent/` is added to the npm package's `files`, next to the other documentation directories.

## Open Questions

- Should `stele approve` also be able to list pending entries without prompting (`--list`), so agents and editors can show them without parsing JSON? Deferred: `stele index` from `link-index` covers this.
