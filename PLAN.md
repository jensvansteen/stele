# Stele Spec — framework plan

Status: local showcase implemented, 10 September 2026. The Todo product, artifact dashboard, repository CLI, hook, and local evidence workflow described by the example change are implemented. Cloud runners and external approval integrations remain roadmap work. Here “ID verification” means behavioral requirement identity and linkage, not verification of a person's identity.

## Recommendation and purpose

Start a small separate project that extends OpenSpec with stable requirement IDs, resolvable code/test links, and a deterministic JSON report. Preserve Stele as a design reference. Prove compatibility and usefulness on one local Coloc delivery before building adapters or automation around it.

The useful reviewer question is: “For this behavior at this revision, where is the implementation, what tests cover it, and what evidence was actually produced?” An ID joins those records; it does not prove that the implementation is correct.

## Ownership

| Owner | Responsibility |
|---|---|
| OpenSpec | One authoritative capability-spec tree, proposed deltas, proposal/design/tasks, synchronization and archive lifecycle |
| This project | Behavioral identity, explicit anchors, deterministic linkage checks, lifecycle consistency and a versioned machine-readable report |
| Coloc | Ticket intake, draft PR and approval labels, conversations with running agents, runner scheduling, Daytona integration, team dashboard and publication of review evidence |
| Tests and reviewers | Executed behavior and semantic assessment; linked source alone cannot substitute for either |

Keep specs, plans, configuration and task checkpoints in Git so a replacement runner can resume. Large recordings can remain outside Git with revision-bound manifests and URLs. The verifier must work locally without Coloc, Daytona, network access, or a dashboard.

## What survives from Stele

Retain stable behavioral identity, meaningful anchors near enforcement/assertions, readable requirements, gradual brownfield adoption and explicit exceptions. Adapt “fulfillment” into separate linkage, execution and review results; adapt lifecycle to OpenSpec changes; use one root OpenSpec workspace for the Coloc monorepo and scoped source paths.

Drop the second `docs/spec` canon, claim-table grammar, custom spec sync engine, compulsory draft/active promotion, universal code-comment quotas, broad audit engine, and inherited Go scaffolding as a default. Do not copy Stele's per-package canon architecture or cross-repository federation ambitions into the MVP. Team workflow remains outside this library.

Laptop findings supplied with the task identify a newer hierarchical ID policy but older regex/template defaults, conflicting layout/lifecycle conventions, and no Go implementation files. These laptop findings are coordinator-reported, not independently verified in this task. The prepared source archive reportedly has HEAD `c192258d765f78bf8ecddc1b6fc67f60cc0cdd8c`; transfer is pending authentication. Verify these findings against the preserved transfer before implementation; see [transfer status](TRANSFER.md). The existing Mac mini Stele checkout is older and must not be mistaken for that source. Methodology promises are not executable capabilities.

## Supported OpenSpec integration and limits

Official upstream `main` was inspected on 10 September 2026; these are observations, not a pinned release compatibility guarantee. Before implementation, select and record an exact released version plus source revision and validate the fixtures against it.

OpenSpec supports project context/rules in `openspec/config.yaml` and version-controlled custom schemas/templates in `openspec/schemas`. A schema can add artifacts and dependencies. Rules guide generation, and artifact presence does not establish human approval. Start with the standard schema plus rules; introduce a small schema variant only if the local proof needs a durable linkage-review artifact. This is schema customization, not an OpenSpec fork. [Customization](https://github.com/Fission-AI/OpenSpec/blob/main/docs/customization.md)

Keep the standard requirement and scenario headings and ADDED/MODIFIED/REMOVED/RENAMED delta operations. Requirements remain normative behavior with scenarios; IDs are supplemental metadata, not replacement grammar. The default schema also distinguishes changes with no spec-level behavior change using `skip_specs`; do not invent behavior to satisfy an ID quota. [Default schema](https://github.com/Fission-AI/OpenSpec/blob/main/schemas/spec-driven/schema.yaml)

Use supported CLI JSON status/instruction surfaces for change discovery and resolved artifact paths where the selected version provides them. Validate JSON shape and version; unsupported versions or failed lookups are errors, not empty workspaces. MVP supports a repository-local planning root; detect standalone stores and report unsupported rather than accidentally reading the wrong root. [CLI reference](https://github.com/Fission-AI/OpenSpec/blob/main/docs/cli.md)

Upstream synchronization resolves requirements by names and applies rename/remove/modify/add operations; a rename rewrites a block's heading while a modification replaces its contents. Therefore IDs in block bodies are a promising convention, not a supported native identity mechanism. The compatibility proof must cover both CLI synchronization/archive and agent-driven sync output. Do not import private parser functions as a stable API or silently reproduce the entire merge engine. [Spec application source](https://github.com/Fission-AI/OpenSpec/blob/main/src/core/specs-apply.ts)

Resolve delta files from the selected schema’s concrete artifact paths; do not assume every Markdown artifact is a spec. Read current specs and the explicitly selected change separately. Build an in-memory effective view for checks using a narrowly versioned adapter and compare it with OpenSpec's output in fixtures. The verifier never writes or archives specs. If parity cannot be demonstrated, stop at a compatibility limitation before implementation expands.

## Stable identity policy

Recommended MVP: every adopted requirement and every scenario receives a stable ID. Requirement IDs allow product-level grouping; scenario IDs distinguish success, rejection and recovery evidence. This small extra annotation avoids reporting an entire requirement as tested after only its happy path ran. IDs are not task numbers, file paths, headings or test framework names.

Proposed format: `req.<namespace>.<token>` and `scn.<namespace>.<token>`, lowercase, where token is a generated 12-character lowercase hexadecimal value. Example: `req.auth.a13f79b20c84`. Namespace is an allocation aid, immutable after assignment even if code moves; no central service is required. Collision checks remain mandatory. The allocation helper is later implementation work. This deliberately replaces the inconsistent Stele numbering schemes.

Proposed storage: a dedicated plain Markdown line `Verification-ID: req.auth.a13f79b20c84` immediately inside its requirement block; analogous scenario line inside the scenario block. Do not put IDs in heading text, whose names OpenSpec uses for delta matching. This exact metadata placement is a milestone-1 hypothesis: ship it only after validation and round-trip preservation pass.

| Event | Identity rule |
|---|---|
| Proposal adds behavior | Allocate new IDs; identity is proposed, with no required implementation yet |
| Modify same obligation | Keep ID; changed content invalidates old evidence and requires review |
| Rename heading / move source | Keep ID; record and validate location changes; capability moves require an explicit supported migration, not guessed OpenSpec rename behavior |
| Split or merge distinct obligations | Allocate new IDs, record predecessor/successor relations and retire originals |
| Remove behavior | OpenSpec removal governs specs; preserve an ID tombstone in a Git-tracked identity ledger; never reuse IDs |
| Sync/archive | Keep identity and ancestry; archived change text is historical occurrence, not another live definition |
| Abandon published proposal | Record IDs as abandoned/reserved; do not later reuse them |
| Concurrent proposals | Check each against the same declared baseline and detect conflicting allocation or incompatible transitions before merge |

The proposed ledger stores only retired/reserved IDs and identity transitions, never a second copy of requirement prose. Removing a requirement retires its scenarios too; removing a scenario within a MODIFIED block retires that scenario alone. Resolve heading-only removals/renames through the declared baseline; do not require incompatible metadata in OpenSpec’s FROM/TO grammar. Detect unexplained ID replacement as a transition error. A scenario move keeps its ID only when its obligation is unchanged and its parent transition is explicit. Require it only when retirement/abandonment happens. Live identity is derived from OpenSpec. Existing and proposed occurrences of the same ID are valid only when they form one explicit modification/rename transition, or a verified already-synced occurrence of the selected change. Early sync must be reconciled against the declared original baseline and current specs; an equal ID alone does not establish provenance. Two live requirements sharing an ID, two unrelated ADDED blocks, or a requirement/scenario kind mismatch are errors. Exclude archived changes from the live index, but consult their history/ledger to prevent reuse. Re-run against the latest integration baseline before merge to catch IDs allocated on other branches.

## Two verification stages

| Check | Proposal review, including approved but unimplemented | Implementation review |
|---|---|---|
| OpenSpec shape and selected-change resolution | Required | Required |
| IDs, uniqueness, parent relation, transition validity | Required for adopted scope | Required |
| Verification strategy for each changed scenario | Planned test/manual method required; target file may not exist | Actual resolvable links required or explicit reviewed exception |
| Code links | Optional planned locations; never called resolved | Meaningful enforcement/configuration links where applicable |
| Tests and recordings | Not run / not expected yet is explicit | Actual execution remains separate from linkage; required missing evidence blocks Coloc's delivery gate |
| Human approval | External approval metadata, never inferred from an artifact | Separate implementation review at the reviewed revision |

Proposal checks can pass with zero implementation files. Approval permits implementation on the same branch; it does not promote proposed behavior to delivered truth. Draft/active markers must not create Stele's circular requirement to implement before approval. Baseline specs describe current contracts; the effective proposed view describes the target. Existing baseline linkage diagnostics remain visible without making new planned links into failures.

Coloc owns a `plan-review` draft PR label and the approved label transition, recorded against the exact plan revision. Editing the approved plan/spec scope requires renewed approval. A distinct `needs-input` state routes a teammate to the existing agent. None of these labels, external events or enforcement mechanisms are built here in the MVP.

## Mechanical scope and honest limits

The MVP checks declared IDs and transitions, scenario-to-requirement ownership, syntactically valid anchors, dangling references, conflicting declarations, scoped link requirements, stable JSON output and clear failure codes. Scan explicitly configured source/test roots; exclude generated/vendor/build files. Treat Markdown fences as examples, not live definitions. Paths must stay within allowed roots after symlink resolution; unreadable files, unsupported schemas and ambiguous mappings cannot silently pass.

Prefer explicit test metadata or an annotation attached to a real named test; code links identify an enforcement symbol or a marker adjacent to relevant logic. A small versioned sidecar mapping is acceptable for configuration, declarative behavior or languages lacking annotation support. It contains ID-to-location data, not specifications. Resolve file plus unique marker/symbol/test selector; line numbers are derived presentation data at a revision, not stable identities. Regex matches alone cannot certify semantic relevance. MVP may use an exact unique marker resolver; AST adapters can follow.

One requirement can have multiple code links and multiple scenario tests, and one test can cover several scenarios. Require at least one test link or an explicit alternative validation record per adopted scenario. Where implementation is a configuration change or external contract, accept a meaningful configuration/contract link or a scoped exception rather than decorative code comments. An exception records reason, owner/reviewer, scope and expiry; report `excepted`, never `passed`. Cosmetic documentation/refactors with no behavior change can declare no new requirement and use existing validation.

“Linked” means a reference resolves. “Executed” means a run record exists. “Passed” is a runner assertion outcome. “Reviewed” means an attributed review covers this revision and scope. The report never emits a single unqualified “fulfilled” or correctness score. Semantic adequacy, mutation testing, code-first drift detection and comprehensive language analysis are separate later work.

## Proposed report contract

Version 1 JSON should be sufficient for a future consumer without embedding dashboard policy. All fields below are proposed, not existing OpenSpec output.

| Area | Required data |
|---|---|
| Envelope | `schemaVersion`, verifier version, OpenSpec version, mode, timestamp, repository identity, baseline SHA, reviewed head SHA, change ID, dirty flag/input digest, config digest |
| Scope | Selected roots/capabilities, adopted/baseline scope, exclusions and reasons, unsupported inputs |
| Requirement | ID, title, current/proposed/retired status, source path and revision/line, delta operation, predecessor/successor IDs, scenario IDs |
| Links | Requirement/scenario ID, kind (code/test/config/manual), repository-relative path, stable selector, derived line, resolution state, planned vs actual |
| Execution | Run ID, runner/adapter version, tested SHA and input digest, environment, timestamps, test ID/selector, attempt number, outcome (passed/failed/skipped/not-run/unknown), flaky flag |
| Artifacts | Artifact ID, related test/run/scenario IDs, kind (recording/trace/log), URI, checksum when known, revision, creation/expiry, availability |
| Review | Scope, reviewer identity, reviewed revision, decision and timestamp, or explicit not-reviewed/unknown; supplied externally |
| Diagnostics | Stable rule code, severity, requirement/scenario ID, source location, actionable message; deterministic ordering |
| Summary | Link counts, exceptions, unresolved IDs, evidence availability and stage verdict; no conversion from missing data to success |

Status dimensions are independent: execution can pass while its recording is missing or expired; an old passing run is stale for a new revision. Missing, unavailable, unexecuted, flaky, skipped, expired and stale evidence stay visible. Local dirty runs bind to a content digest and cannot claim a clean commit result. MVP requires the identity/linkage envelope and explicit unknown/not-run/not-reviewed states; execution, artifact and review records may be empty. It validates the envelope and can ingest a tiny neutral fixture; it does not implement every test runner adapter or fetch remote evidence. No network availability check is implied by a URI being present.

Proposed CLI outcome: 0 means this mode's mechanical checks passed, 1 means findings violate the selected policy, 2 means invalid input/unsupported version/tool failure. Include `complete: false` on partial/error reports. Execution/review thresholds belong to an explicit consumer policy, not a hidden interpretation of exit 0.

## Small worked example

Illustration only; these are future fixture paths, not files created in Coloc. Proposed delta at `openspec/changes/reject-expired-session/specs/auth/spec.md`:

```markdown
## MODIFIED Requirements

### Requirement: Session validation
Verification-ID: req.auth.a13f79b20c84
The system SHALL reject expired sessions before returning protected data.

#### Scenario: Expired session
Verification-ID: scn.auth.c29e10a7b635
- **WHEN** a protected request contains an expired session
- **THEN** access is denied without returning protected data
```

Assume baseline has the same requirement/scenario IDs; the delta includes the full requirement and all existing scenarios in a real fixture. Proposed test target is `tests/auth/session.test.ts` / `rejects expired session`; proposed code target is `src/auth/session.ts` / `validateSession`. An implementation might attach an explicit `@verifies scn.auth.c29e10a7b635` test annotation and `@implements req.auth.a13f79b20c84` enforcement marker. Their exact supported syntax is to be decided by the compatibility/anchor fixture work.

| At revision | Expected report |
|---|---|
| P: plan drafted | One effective requirement, not two; proposed link targets; mechanical proposal pass; execution not-run; review not-reviewed |
| P: plan approved externally | Same mechanical result; plan review approved at P; implementation still pending; no fabricated code/test evidence |
| I: implementation added | Actual code/test selectors resolve; linkage pass; until a run is supplied execution remains not-run |
| I: runner passes test | Execution passed at I, not flaky; recording references run/scenario at I if produced; semantic review still not-reviewed |
| J: later edit | Earlier run remains historical at I, stale for J; rerun required by Coloc delivery policy |

Missing enforcement link at I produces `LINK_CODE_MISSING` unless an explicit applicable exception exists. A failed test does not erase a valid link. A pure heading rename retains IDs, and successful sync/archive leaves one live definition plus historical occurrences.

## Minimal Coloc adoption

Use one monorepo OpenSpec root for TypeScript backend, React and Community/React Native. Begin with one small delivery (Chat was suggested, not selected or authorized for implementation here). Establish the spec workflow first and complete that delivery locally using the project's existing validation even if this verifier is not yet ready. The reusable verifier's MVP can proceed as a separate bounded project and then attach to that workflow.

The task's supplied Coloc reference points are `e2e/tooling/evidence-manifest.ts`, `docs/testing/e2e-testing.md`, and the shared-session authentication journey under `e2e/shared/features/authentication/journey-groups/cross-app-authentication/journeys/shared-session.spec.ts`. Treat the manifest as the future adapter boundary, not a schema to overwrite. The prior planning task reported that its mini checkout lacked the named manifest; its latest contract has not been independently verified in this task. Confirm it in the authoritative checkout before adapter work; supplied context says automatic spec-to-test anchor verification is not wired.

Adopt only the pilot capability and touched paths; show legacy coverage as out of scope or baseline debt. CI can initially publish the JSON report beside existing validation results. Review should lead with specs, plan and checklist, followed by requirement-to-test links and recordings. A teammate can approve a concrete plan; implementation remains on the same branch. Later pilot Daytona, then dashboard/automation, then expansion. Live preview and terminal/desktop intervention follow useful review evidence. No provider API belongs in the verifier core.

## Milestones and acceptance

1. **Compatibility and identity proof.** Select exact OpenSpec version; approve metadata syntax and report schema using throwaway fixtures. Accept only if native validation, add/modify/rename/remove, sync twice, archive and re-read preserve intended IDs; same-ID current/delta pairs are accepted, duplicate live IDs/reuse rejected; scenario changes and concurrent collisions are tested. Check agent-driven sync output too; guidance alone cannot prove it will preserve metadata. Produce a compatibility matrix and documented unsupported cases. A failed round-trip blocks MVP release, not an invitation to fork upstream immediately.
2. **Minimal standalone checker.** Implement read-only proposal/implementation modes, scoped IDs, one explicit code/test marker resolver, transition ledger checks and JSON diagnostics. Accept a proposal with nonexistent planned files; reject dangling IDs, missing required actual links, ambiguous selectors and path escapes; accept genuine non-code validation only as explicit alternatives/exceptions. Repeat identical inputs to obtain stable semantic output (timestamps excluded). No runner, hooks or cloud service required.
3. **One local adoption.** Fit a small approved Coloc delivery to OpenSpec, run existing product validation, and use the checker when available. Accept when a reviewer can trace one requirement through multiple scenario outcomes and a relevant recording where the delivery produces one, with missing/stale evidence obvious. A neutral recorded-run fixture is enough to validate the contract before a richer adapter. Demonstrate runner replacement can resume from Git artifacts without relying on chat memory.
4. **Evidence adapters and review.** After the local proof, map the actual Coloc manifest into the neutral contract, preserving attempts/flakiness/revision/expiry. Accept passed/failed/skipped/not-run/stale/expired fixtures without misreporting success. Add semantic-review workflow separately, with attributed scope and revision; it must not alter deterministic linkage facts.
5. **Operational adoption outside core.** Coloc pilots Daytona, then adds approval-triggered execution and the team dashboard, then expands scope. Accept that changing runner provider leaves verifier output unchanged and that changed plans require fresh approval. These are roadmap dependencies, not implementation tasks authorized by this document.

## Runtime and decisions for approval

Recommend TypeScript on Node for the initial CLI/core: it fits Coloc and upstream OpenSpec's ecosystem and lowers adapter friction. Keep parsing/report logic independent of test runners. Do not inherit Go simply because a `go.mod` exists, and do not select packaging or publish a name before the compatibility proof. A standalone binary can be reconsidered if distribution becomes the dominant constraint.

Open questions and recommended defaults for approval: separate provisional project, requirement plus scenario IDs, body metadata subject to round-trip proof, explicit stage modes, repository-local OpenSpec root, TypeScript/Node, marker-based MVP, and no mandatory custom schema unless needed. Open material choices for the next approval are exact OpenSpec release/compatibility range, final public identity/package name, the first Coloc pilot and its evidence policy, and reviewers/expiry for exceptions. These recommendations permit review of this plan; they do not authorize coding. Resolve choices that affect milestone scope before starting the corresponding implementation.
