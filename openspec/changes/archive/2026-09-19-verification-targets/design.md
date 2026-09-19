## Context

See proposal.md for the problem. The current state that shapes the approach:

- **The annotation reserves fields.** `<!-- stele: spec v1; key: value -->` already parses `; key: value` fields, and warns `SPEC_ANNOTATION_FIELD_IGNORED` for each one (`annotation.go`, `annotationFieldPattern`). Its living scenario uses `targets: vscode, zed` as the example.
- **Metadata lines.** The spec parser recognizes `Verification-ID:` lines and keeps them out of requirement and scenario text (`specs.go`, `parseLine`). The approval digest hashes the entry and the whitespace-normalized scenario text (`plan.go`, `normalizedScenarioText`). The user checked that OpenSpec 1.13.0 strict validation accepts an extra `Targets:` line under a heading.
- **Evidence IDs.** Evidence IDs are `<scenario>.<level>[.<n>]`. Both `evidenceSuffixPattern` in `plan.go` and the anchor regex in `anchors.go` accept only a level after the scenario ID, so today `@verifies scn.x.ios.unit` would be read as the bare scenario `scn.x` followed by text.
- **"Target" already means two other things.**
  - In v1 plans, a *planned target* is a `path#selector`: the `target` field of report links, and `LINK_TARGET_MISMATCH`. v1 plans are deprecated until 0.2.0.
  - The positional arguments of `stele test` are called "targets" in the `link-index` specification.

  This change renames the second to *selections* in requirement text and documentation. OpenSpec keys MODIFIED scenarios by title, so the scenario titles "Combine several targets" and "Reject an unknown target" must stay. It also keeps the JSON name `target` for v1 links until 0.2.0 (Decision 7).
- **Combined plans.** For `--specs`, the plan entries of each identity come from the archived change whose delta spec declares it with the same text, or else from the change archived last (`scope.go`, `loadArchivedPlans`). So a change that re-plans a scenario through a MODIFIED requirement replaces its archived entries. Decision 5 depends on this.
- **Unarchived work.** `ci-gates` and `fast-runs` are merged but not archived. `ci-gates` MODIFIES `validate` ("Check a project with one command") and ADDS a requirement to `terminal-report`. This change stays out of `validate`, and modifies a different `terminal-report` requirement.
- **CI checks active changes at the implementation stage** (`ci-gates` Decision 7), so a plan-only change can never land on `main`. This change lives on the branch `chore/plan-verification-targets` until it is implemented.
- **OpenSpec stores** (1.13.0, beta), which phases 2 to 4 build on:
  - `openspec/config.yaml` may name a `store: <id>`, the default home of specs and changes, and `references: [<id> | {id, remote}]`, stores read one level deep, from disk.
  - A store is a standalone OpenSpec repository registered on the machine (`openspec store setup|register|list|doctor`). Its committed `.openspec-store/store.yaml` holds its id and an optional `remote` clone URL.
  - Worksets are personal compositions of several roots.
  - OpenSpec never clones, syncs, or pushes. It reads references live, with no version pinning, no per-repository progress, and no notion of platforms or targets.

## Goals / Non-Goals

**Goals:**

- One specification describes one behavior for several targets, and each target proves it with its own evidence.
- Gaps are visible: a scenario × target matrix in the index and the report, with `n/a` and `missing`.
- The three patterns (replicas, split, and journey) are expressible, documented, and guided by the `stele-plan` skill.
- Single-target projects are byte-for-byte unchanged, and existing plans and approvals stay valid.
- Phase 1 stays within one repository. Phases 2 to 4 get a design outline that later changes can plan from.

**Non-Goals:**

- Stores, sync, progress, and orchestration (phases 2 to 4, outlined below, with no requirements here).
- Language server lenses and diagnostics per target (a follow-up after the `-lsp` change, reading the same matrix component).
- Prescribing code structure. Stele never decides where a target's code lives. The user declares it with `paths`, and placement stays advisory.
- Target-specific test commands or runners. Execution still infers the runner from the test file.
- Per-target approval rules. Approval stays per entry.

## The three patterns

Where the difference lives:

- **Behavior differences go in the specification,** as separate scenarios narrowed with `Targets:` under one shared requirement.
- **Testing differences go in the plan,** as evidence levels per target: an iOS scenario may be proven by a unit test while Android uses an instrumented integration test.

### 1. Replicas: the same behavior built several times

Every listed target needs its own evidence, and the matrix shows the gaps. This is the `stele-editors` case:

```markdown
<!-- stele: spec v1; targets: vscode, zed, jetbrains -->
## ADDED Requirements

### Requirement: Show a code lens above every anchored declaration
Verification-ID: req.lens.1a2b3c4d5e6f

The editor SHALL show a code lens naming the requirement above each `@implements` anchor.

#### Scenario: Lens on an implemented function
Verification-ID: scn.lens.0a1b2c3d4e5f

- **WHEN** a TypeScript function has `@implements req.todo.…`
- **THEN** a lens above it names the requirement and opens its specification
```

```json
"targets": {
  "vscode":    { "paths": ["vscode/**"] },
  "zed":       { "paths": ["zed/**"] },
  "jetbrains": { "paths": ["jetbrains/**"] }
}
```

The plan has one entry per target, for example `scn.lens.0a1b2c3d4e5f.vscode.e2e`, `….zed.integration`, and `….jetbrains.integration`. Each replica's tests carry their own `@verifies`, and each replica's code carries its own `@implements` (otherwise `LINK_TARGET_IMPLEMENTATION_MISSING`).

For iOS and Android, the behavior difference lives in the specification: one shared requirement, one shared scenario, and one platform-specific scenario per target.

```markdown
<!-- stele: spec v1; targets: ios, android -->
### Requirement: Share a list
Verification-ID: req.share.2b3c4d5e6f7a

The app SHALL let the user share a list as plain text.

#### Scenario: Share text contains every item
Verification-ID: scn.share.3c4d5e6f7a8b

- **WHEN** the user shares a list with three items
- **THEN** the shared text lists the three items in order

#### Scenario: Share through the iOS share sheet
Verification-ID: scn.share.4d5e6f7a8b9c
Targets: ios

- **WHEN** the user taps Share
- **THEN** the system share sheet opens with the list text

#### Scenario: Share through an Android share intent
Verification-ID: scn.share.5e6f7a8b9c0d
Targets: android

- **WHEN** the user taps Share
- **THEN** an `ACTION_SEND` chooser opens with the list text as `EXTRA_TEXT`
```

The matrix for this requirement:

| Scenario | android | ios |
|---|---|---|
| Share text contains every item | ✗ missing | ✓ passed |
| Share through the iOS share sheet | n/a | ✓ passed |
| Share through an Android share intent | ✗ missing | n/a |

### 2. Split: one behavior divided over parts

Requirements are split by responsibility. A contract requirement lists both sides: the back end proves that it serves the contract, and the front end proves that it uses it correctly. This is OpenSpec's `add-checkout-promo` example in one repository:

```markdown
<!-- stele: spec v1; targets: api, web -->
### Requirement: Validate promo codes
Verification-ID: req.checkout.6f7a8b9c0d1e
Targets: api

The checkout API SHALL reject expired promo codes with `410` and the code `PROMO_EXPIRED`.

#### Scenario: Reject an expired code
Verification-ID: scn.checkout.7a8b9c0d1e2f
…

### Requirement: Show the promo discount
Verification-ID: req.checkout.8b9c0d1e2f3a
Targets: web

The checkout page SHALL show the discount line once a code is accepted.
…

### Requirement: Promo contract
Verification-ID: req.checkout.9c0d1e2f3a4b

`POST /checkout/promo` SHALL accept `{ "code": string }` and return `{ "discount": { "amount": integer, "currency": string } }`.

#### Scenario: Apply a valid code
Verification-ID: scn.checkout.0d1e2f3a4b5c

- **WHEN** a valid code is submitted
- **THEN** the response has the discount amount and currency, and the page shows it
```

For the contract scenario, the plan holds `….api.integration` (the handler serves the documented shape) and `….web.unit` (the client parses and renders the shape, against a stub built from the same contract).

### 3. Journey: an end-to-end flow across everything

The flow is proven once, by a `system` target: an end-to-end suite folder, marked `evidenceOnly` because it implements nothing.

```markdown
<!-- stele: spec v1; targets: api, web, system -->
### Requirement: Complete a purchase with a promo code
Verification-ID: req.checkout.1e2f3a4b5c6d
Targets: system

A shopper SHALL be able to apply a promo code and pay the discounted total.

#### Scenario: Pay the discounted total
Verification-ID: scn.checkout.2f3a4b5c6d7e

- **WHEN** a shopper adds an item, applies a valid code, and pays
- **THEN** the order total is the discounted amount
```

```json
"system": { "paths": ["e2e/**"], "evidenceOnly": true }
```

The plan has one entry, `….system.e2e`, anchored in `e2e/`. The requirement still needs one `@implements` anywhere in the project, as every requirement does, but no per-target implementation.

## Decisions

### 1. The evidence ID carries the target: `<scenario>.<target>.<level>[.<n>]`

**Chosen:** a targeted scenario's evidence ID puts the target between the scenario and the level, and the entry also has a `target` field that must equal it, just as `level` must equal the level in the ID.

- **Anchors describe themselves.** `@verifies scn.share.3c4d5e6f7a8b.android.integration` says which replica it proves. `ANCHOR_TARGET_OUTSIDE_PATHS` can catch a test copied between replicas with its ID unchanged, which is the main mistake in the reference-target workflow of phase 4.
- **The grammar stays unambiguous.** Scenario IDs end in 12 hex digits, levels are a closed set, and target names may not be level names (they are reserved in the configuration). So `.<level>` and `.<target>.<level>` can never be confused, and `.<n>` is always numeric.
- **Ordinals count within a scenario, target, and level,** so `….ios.unit.2` does not depend on Android's entries.
- **Compatibility.** Untargeted specifications keep `<scenario>.<level>[.<n>]` exactly, with no `target` field. Their digests, and therefore their approvals, are unchanged. Adding `targets` to an existing specification is an explicit migration (see Decision 5 and Open Question 2).

*Alternative: a `target` field only, with an unchanged ID.* Rejected. Two targets' unit entries would become `.unit` and `.unit.2`, so the anchor would not say which replica it proves, the ordinal would depend on the other targets, and `stele test <evidence-id>` could not select by target.

*Alternative: a target prefix, `<target>:<scenario>.<level>`.* Rejected. It breaks the rule that an evidence ID starts with its scenario ID, which removed-behavior checks, selection, and the index all rely on.

### 2. Targets map to folders through configuration, not inference

**Chosen:** `stele.config.json` `targets: { <name>: { paths: [globs], evidenceOnly?: bool } }`. The configuration is the registry of names, and `paths` is required.

- **A registry catches typos.** Without it, `andriod` in one specification silently creates a fourth target with no evidence.
- **Paths are needed to attribute `@implements`.** Requirement IDs have no target, so only the file's location can say which replica an implementation belongs to. That is what makes `LINK_TARGET_IMPLEMENTATION_MISSING` possible.
- **Paths catch misplaced evidence** (`ANCHOR_TARGET_OUTSIDE_PATHS`).
- **Selection and the matrix work before anything is anchored.**
- **Overlapping paths are allowed,** for shared code such as a Kotlin Multiplatform module that both `ios` and `android` match. One test may then carry evidence for both.
- **This is not prescribing structure.** The user declares where each target lives, and Stele only checks consistency with that declaration. Placement inside a target stays advisory.

*Alternative: infer targets from anchor paths.* Rejected. It cannot distinguish a misplaced anchor from a new target, and it cannot attribute an `@implements` that has not been written yet.

*Alternative: `paths` optional, with the target taken from the ID only.* Rejected for phase 1, because it gives up both checks above. Phase 2 gives a single-target code repository a default of `paths: ["**"]` (see below).

Globs: `*` matches within a segment and `**` across segments, matched against slash-separated paths relative to the root. A small dependency-free matcher is needed, because `path.Match` has no `**`.

### 3. `Targets:` is metadata beside `Verification-ID:`

- **Placement.** The line lives in the metadata block directly under a heading, in either order with `Verification-ID:`. `stele ids` inserts the ID directly below the heading, so a hand-written `Targets:` line ends up second.
- **Text and digest.** It is excluded from requirement and scenario text, steps, and the approval digest. Narrowing a scenario therefore does not stale its remaining approvals, and the entries of targets that no longer apply are caught by `PLAN_TARGET_NOT_APPLICABLE`. Widening is caught by `PLAN_EVIDENCE_MISSING`. The plan corrects itself in both directions without re-approving unchanged entries.
- **Narrowing only.** A scenario narrows its requirement, and a requirement narrows its specification. Narrowing only keeps one reading of "applies": the chain of `Targets:` lines from the file down.

### 4. Matrix states and precedence

`n/a` means the scenario does not apply, and `missing` means it applies without planned-and-anchored evidence. `unapproved`, `not-run`, `stale`, `failed`, and `passed` follow the existing evidence states. When several entries of one cell differ, the worst state wins: `failed`, then `missing`, then `unapproved`, then `stale`, then `not-run`, then `passed`. A failure is the most urgent thing a person must see, and a gap comes next.

- **One in-process component** (`matrix.go`) is used by `index.go`, `report.go`, and later the language server.
- **Columns** are the configured targets that at least one specification in the scope declares, ordered by name.
- **Rows** are the targeted scenarios, in specification order.
- **Untargeted scenarios have no row,** and a scope without targeted specifications has no matrix at all.

In the report, a summary row per capability gives, per target, the number of passing applicable scenarios. At most ten scenario rows whose cells are not all `passed` or `n/a` follow, then `… N more (--details)`, as for diagnostic groups.

### 5. Keeping targets through the OpenSpec workflow

OpenSpec copies requirement blocks, not first lines. Three rules keep targets intact:

1. **New delta specs inherit.** `stele ids` and `stele annotate --change` write the current specification's `targets` into a new delta spec annotation of an existing capability. `SPEC_TARGETS_MISMATCH` catches a delta that has lost them or gained them.
2. **Changing a capability's targets is explicit.** A delta spec with a different list must list every current requirement whose applicable targets change (`SPEC_TARGETS_CHANGE_UNCOVERED`). The change's plan then re-plans those scenarios, and, through the combined-plan rule, its entries replace the archived ones after archiving. Adding `web` to an `ios, android` capability therefore means modifying its requirements (narrowing those that stay mobile-only) and planning `web` evidence, in one reviewable change.
3. **Archive restores.** `stele annotate --specs --targets-from <archive-dir>` copies each delta spec's `targets` onto its current specification. The `stele-archive` skill and the `openspec/config.yaml` archive guidance run it right after `openspec archive`.

*Alternative: let `stele annotate --specs` guess from the latest archive.* Rejected. It is implicit, and wrong when a person edits a current specification by hand.

### 6. `--target` on every command, defined once

`--target <name>` is repeatable, and combines with scopes, `--all`, and `stele test` selections (an intersection).

- **Filtering.** It filters plan, linkage, and execution findings, the matrix columns, and the tests that run.
- **What stays whole.** Specification, identity, and annotation findings stay whole, because a broken specification is everyone's problem.
- **Untargeted specifications stay in scope.** They describe the project as a whole, so every per-target job also checks them and nothing falls through the cracks (Open Question 3, answered).
- **The main use** is one CI job per target: `stele check --all --target android` in the Android pipeline.
- **One place.** The option is defined in `verification-targets` rather than in `validate`, whose "Check a project with one command" requirement is still an unarchived `ci-gates` delta.

### 7. Output schemas: additive, and only when targets are used

- **Index:**
  - top-level `targets` (name, paths, `evidenceOnly`);
  - `targets` per file entry;
  - per item, `declaredTargets` (the `Targets:` line, or `null`) and `applicableTargets`;
  - `target` on evidence entries and test anchors;
  - `matrix` per scope.
- **Verification report:** a `targets` verdict object, and `evidenceTarget` on links, because `target` on links is v1's `path#selector` until 0.2.0 (Open Question 4).
- **Plan:** `target` on entries.
- **Evidence file:** unchanged (schema 3). A result's target derives from its evidence ID.

Every field is omitted when no targeted specification is in scope, which gives the byte-identical guarantee. It is proven against the previous release by `add73aad6d42`.

### 8. Diagnostics catalogue additions

Every new code gets a stage, a meaning, and a fix step, enforced by the existing `78fda44eee93` test:

| Code | Stage | Meaning | Fix |
|---|---|---|---|
| `SPEC_TARGETS_MALFORMED` | specifications | A target list is empty, repeats a name, holds an invalid name, or a heading has two `Targets:` lines. | Write one comma-separated list of distinct configured names. |
| `SPEC_TARGET_UNKNOWN` | specifications | A specification or `Targets:` line names a target that `stele.config.json` does not configure. | Fix the name, or add the target with its `paths` to `stele.config.json`. |
| `SPEC_TARGETS_WIDENED` | specifications | A `Targets:` line names a target its parent does not apply to. | Narrow only: remove the name, or add it to the requirement or the annotation. |
| `SPEC_TARGETS_MISPLACED` | specifications | A `Targets:` line is not in a heading's metadata block. | Move it directly below the heading, next to `Verification-ID:`. |
| `SPEC_TARGETS_UNDECLARED` | specifications | A `Targets:` line appears in a specification without a `targets` field. | Add `; targets: …` to the first line, or remove the line. |
| `SPEC_TARGETS_MISMATCH` | specifications | A delta spec declares targets differently from its current specification, with or without the field. | Copy the current `targets` field, or change targets deliberately and list the affected requirements. |
| `SPEC_TARGETS_CHANGE_UNCOVERED` | specifications | A delta spec changes its capability's targets without listing a current requirement whose targets change. | Add that requirement as MODIFIED (narrow it if it should keep its targets), and plan its evidence. |
| `PLAN_TARGET_NOT_APPLICABLE` | plan approval | A plan entry names a target its scenario does not apply to. | Remove the entry, or widen the scenario deliberately. |
| `ANCHOR_TARGET_OUTSIDE_PATHS` | linkage | A `@verifies` anchor for target X lives outside X's `paths`. | Fix the evidence ID to the target the test belongs to, or move the test. |
| `LINK_TARGET_IMPLEMENTATION_MISSING` | linkage | A requirement has no `@implements` anchor within the paths of a non-`evidenceOnly` target it applies to. | Add `@implements <req>` to that target's implementation, or narrow the requirement. |

`PLAN_EVIDENCE_MISSING` names the target when a scenario lacks one target's evidence. `PLAN_EVIDENCE_INVALID` covers a missing or mismatched `target`. Their fix steps gain the targeted ID form.

### 9. Code layout (advisory)

- `targets.go`: the configuration, the glob matcher, applicable-target resolution, and anchor attribution.
- `matrix.go`: the matrix.
- Existing files gain small extensions:
  - `annotation.go`: the `targets` field and `--targets-from`;
  - `specs.go`: the `Targets:` line;
  - `plan.go`: the targeted ID grammar and completeness;
  - `anchors.go` and `goanchors.go`: the anchor grammar;
  - `verify.go`, `index.go`, `report.go`, `cli.go`, and `check.go`;
  - the diagnostics catalogue.

Placement is advisory, per AGENTS.md.

## Phases 2 to 4: design outline (no requirements in this change)

The model follows OpenSpec's two layers. A **store** holds the shared product contract: specs, and store-level changes such as `add-checkout-promo`. Each **code repository** has its own local change (`implement-checkout-promo-api`, `implement-checkout-promo-ui`), with its own tasks, branch, review, plan, and evidence, and it references the store. Scenario IDs link the layers: the store's scenario `scn.checkout.0d1e2f3a4b5c` is the one that `api` and `web` plan evidence for, as `….api.integration` and `….web.unit`.

### Phase 2: the store as the pinned spec source (store → code)

- **Declaration.** The code repository declares its target, and pins the spec source, in `stele.config.json`:
  ```json
  "target": "web",
  "spec": { "store": "checkout", "commit": "<40-hex sha>", "paths": ["openspec/specs/checkout/**"] }
  ```
  `store` is an OpenSpec store id, the same one that `openspec/config.yaml` `references:` lists. Stele resolves it to a local path through OpenSpec's registry (`openspec store list --json`), and reads the specs at the pinned commit with `git show <commit>:<path>`, never from the working tree. Like OpenSpec, Stele never clones, fetches, or pushes. When the commit is missing locally, it reports the store's `remote` from `.openspec-store/store.yaml` and the command to fetch.
- **The target in a code repository.** In a single-target repository, `target: "web"` makes every store scenario that applies to `web` in scope, with `paths` defaulting to `["**"]`. A multi-target repository (phase 1 layout) may still reference a store.
- **`stele store sync [--to <commit>]`.** It moves the pin, and prints for this target:
  - the scenarios added, changed (with a text diff), removed, or newly applicable or no longer applicable, by scenario ID;
  - the plan entries that the move makes stale, missing, or unplanned.

  It writes only `stele.config.json` (and a `store-sync.md` summary if asked), so the move is reviewed as an ordinary commit or pull request. It never edits specs, plans, or code.
- **Determinism.** Every command's result is a function of the repository commit plus the pinned store commit. A floating reference (OpenSpec's live read) is available only as an explicit `--spec-source live` for exploration, and marks every result as "unpinned".
- **Builds on OpenSpec:** `references:` for discovery, the store registry for resolution, and `store.yaml` `remote` for the fetch hint. The additions are the pin, the per-target filter, and the diff.

### Phase 3: evidence back to the store, progress, and the archive rule (code → store)

- **Evidence summary.** A code repository writes an evidence summary per store change and target: `stele store report --change add-checkout-promo` writes `openspec/changes/add-checkout-promo/progress/web.json` for the store, delivered as a store pull request by the team's own tooling, since Stele never pushes. The summary holds per scenario the evidence IDs, levels, approval state, and outcome, with no durations. It records `codeRepo`, `codeCommit`, `specCommit` (the pin), and the Stele version.
- **Progress per target per change:** `planned` → `in progress` → `implemented` → `verified`.
  - *planned*: the store change lists the target.
  - *in progress*: a local change references it.
  - *implemented*: linkage passes.
  - *verified*: linkage and execution pass at `codeCommit`.

  "Verified against an older spec commit" is its own state: `specCommit` is older than the store change's head, and the scenarios that changed since then are listed.
- **The archive rule.** `stele verify --store-change <id>` in the store passes only when every target the change applies to is `verified` against the change's current spec commit, or is `deferred` with a recorded decision (who, when, why, and until when) in `progress/<target>.json`. The `stele-archive` skill in a store checks it before `openspec archive`.
- **Determinism.** Summaries are checked-in files, tied to two commits. The store never runs code tests, and its verdict is a pure function of the committed summaries plus its own specs.

### Phase 4: agent orchestration

- **`stele-apply --target X`** implements one target's part of a change: only the scenarios that apply to X, and only X's plan entries and paths.
- **`--reference Y`** gives the agent, per scenario, Y's implementation and tests, found through the shared scenario IDs: every `@verifies <scn>.Y.*` test and every `@implements <req>` in Y's paths, with their source, from the link index. For example, the Android agent sees the exact iOS code and tests for "Share text contains every item", and writes Kotlin with `@verifies <scn>.android.<level>`. `ANCHOR_TARGET_OUTSIDE_PATHS` catches the copy-paste mistake of keeping `.ios.`.
- **A `stele-rollout` skill** plans the order:
  - parallel: one agent per target, in worktrees;
  - stepped: a reference target first, then the others with `--reference`.

  It tracks the matrix until every cell is `passed` or deferred.
- **Across repositories** (after phases 2 and 3), the reference comes from the other repository's evidence summary plus a local checkout that the workset points to. OpenSpec worksets are the natural way to open the store and all code repositories together.

### Open questions for phases 2 to 4

- Should the pin live in `stele.config.json` or in `openspec/config.yaml` next to `references:` (OpenSpec may reject unknown keys, or add pinning itself)?
- Are summaries per store change or per store commit? What happens to them when the store change is archived?
- How is a deferral approved: by the store owner, and through `stele approve`?
- Can a code repository be several targets at once, for example `web` and a `system` e2e suite?
- Should Stele verify a store at all, or only produce summaries that OpenSpec tooling consumes?

## Verification strategy

**Status: approved in chat on 2026-09-19.** The reviewer approved the levels, Decisions 1 to 9, and the proposed answers to the open questions, with question 3 changed; `stele approve --change verification-targets` records the approvals.

Placement follows AGENTS.md: co-located Go tests in `internal/stele`, and CLI tests in `tests/cli.test.mts`. Placement is advisory. Stele itself is untargeted, so all entries use the untargeted ID form. Levels:

- **unit** (78 entries): nearly all risks are in pure parsing, resolution, plan validation, matrix computation, and rendering, over temporary files in process.
- **integration** (4 entries): the risks that sit in real outside tools. An `openspec archive` rewriting current specifications (`0020f5fc5ce4`, and the existing `b7e05fe76741` and `23d533778e10`), and target filtering reaching real `go test` and `node --test` batches (`258ffea7adf7`).
- **e2e** (6 entries): the shipped CLI. The existing e2e entries (`30ab15bc8293`, `c0e515c9575a`, `7c1d78728036`, and `a518efc1ddd8`) are kept, plus two new ones:
  - `db222c63967d`: `check --target` passes through in the installed package;
  - `add73aad6d42`: byte-identical untargeted output against the `stele-published` previous release, with only the version normalized.

The 50 entries marked "(existing)" re-plan scenarios that the MODIFIED requirements carry over. They keep their evidence IDs and tests, and their rationales name the regression risk this change adds. They need fresh approval because this change's plan replaces their archived entries (see Context, "Combined plans").

| Scenario | Level | Evidence ID | Advisory placement | Risk rationale (why this level is the lowest convincing one) |
|---|---|---|---|---|
| `scn.linkindex.30ab15bc8293` Index a change (existing) | unit | `….30ab15bc8293.unit` | internal/stele/index_test.go (existing) | Adding target fields could reorder or drop the existing requirement, anchor, and approval data of an untargeted change. In-process index over a temporary project. |
| `scn.linkindex.30ab15bc8293` Index a change (existing) | e2e | `….30ab15bc8293.e2e` | tests/cli.test.mts (existing) | The shipped `stele index` could fail to emit the unchanged document for an untargeted change. Only the built CLI proves the command end to end. |
| `scn.linkindex.2bec33059b75` Keep requirement text and scenario steps (existing) | unit | `….2bec33059b75.unit` | internal/stele/specs_test.go (existing, extended) | A `Targets:` line could leak into requirement text or become a scenario step. Pure parsing of fixture Markdown. |
| `scn.linkindex.48b3c24773d9` Include the current specifications (existing) | unit | `….48b3c24773d9.unit` | internal/stele/index_test.go (existing) | Target resolution per scope could mislabel current specifications. In-process index over temporary files. |
| `scn.linkindex.c0e515c9575a` Emit identical bytes for identical inputs (existing) | e2e | `….c0e515c9575a.e2e` | tests/cli.test.mts (existing) | Matrix maps or target lists could introduce map-order nondeterminism into the shipped output. Needs the built CLI run twice. |
| `scn.linkindex.331076ad0227` Mark stale execution outcomes (existing) | unit | `….331076ad0227.unit` | internal/stele/index_test.go (existing) | Per-target outcomes could bypass the stale check. In-process with a stored evidence fixture. |
| `scn.linkindex.560aeb5e3970` Flag anchors that no specification declares (existing) | unit | `….560aeb5e3970.unit` | internal/stele/index_test.go (existing) | The new targeted ID grammar could turn a targeted anchor with an unknown scenario into a declared one. In-process scan of temporary sources. |
| `scn.linkindex.34d29862cf10` Index targets and the matrix | unit | `….34d29862cf10.unit` | internal/stele/index_test.go, beside the change index tests | Targets, declared and applicable lists, evidence targets, or matrix cells could be missing or wrong in the index. The index is an in-process component over temporary files. |
| `scn.linkindex.352d7120ec6c` Run the tests of one scenario (existing) | unit | `….352d7120ec6c.unit` | internal/stele/runner_test.go (existing, stubbed runner) | Selection could change when evidence gains targets. Selection is pure planning with a stubbed runner. |
| `scn.linkindex.1f063f3c5e49` Run the tests of one evidence entry (existing) | unit | `….1f063f3c5e49.unit` | internal/stele/runner_test.go (existing, stubbed runner) | An untargeted evidence ID could be misparsed by the new grammar and select nothing. Pure selection logic. |
| `scn.linkindex.49a524bcf0ac` Run the tests of one requirement (existing) | unit | `….49a524bcf0ac.unit` | internal/stele/runner_test.go (existing, stubbed runner) | A requirement selection could miss targeted scenarios. Pure selection logic. |
| `scn.linkindex.e0b21aa95623` Run the tests of one specification file (existing) | unit | `….e0b21aa95623.unit` | internal/stele/runner_test.go (existing, stubbed runner) | A file selection could pick up tests of another file through shared targets. Pure selection logic. |
| `scn.linkindex.e5c1d7ab73d5` Run the tests of one targeted evidence entry | unit | `….e5c1d7ab73d5.unit` | internal/stele/runner_test.go, beside the evidence-entry selection test | A targeted evidence ID could be parsed as scenario plus unknown suffix and run every target's tests. Pure selection with a stubbed runner. |
| `scn.linkindex.6265c70bed70` Combine several targets (existing) | unit | `….6265c70bed70.unit` | internal/stele/runner_test.go (existing, stubbed runner) | The renamed selections could lose union and de-duplication. Pure selection logic. |
| `scn.linkindex.5c91d0c168f0` Reject an unknown target (existing) | unit | `….5c91d0c168f0.unit` | internal/stele/cli_test.go (existing) | The rename could change the exit code or message for an unknown selection. In-process CLI call. |
| `scn.linkindex.57dcee8c30a8` Leave parameterized test names unresolved (existing) | unit | `….57dcee8c30a8.unit` | internal/stele/anchors_test.go (existing) | The widened anchor grammar could give parameterized tests a partial selector. Pure scanner. |
| `scn.linkindex.7c1d78728036` Verify every scope separately (existing) | unit | `….7c1d78728036.unit` | internal/stele/cli_test.go (existing) | Per-scope target handling could merge scope results. In-process over temporary scopes. |
| `scn.linkindex.7c1d78728036` Verify every scope separately (existing) | e2e | `….7c1d78728036.e2e` | tests/cli.test.mts (existing) | The shipped `verify --all` could change its combined exit code. Built CLI. |
| `scn.linkindex.bda4420352ef` Index every scope (existing) | unit | `….bda4420352ef.unit` | internal/stele/index_test.go (existing) | Per-scope matrices could collide in `--all`. In-process index. |
| `scn.linkindex.4d6ba51a47bc` Reject conflicting scope options (existing) | unit | `….4d6ba51a47bc.unit` | internal/stele/cli_test.go (existing, extended) | Allowing `--target` with `--all` could also let selections through. Argument parsing, in process. |
| `scn.linkindex.094a437d7c9c` Combine every scope with a target | unit | `….094a437d7c9c.unit` | internal/stele/scope_test.go or cli_test.go | `--all --target` could filter only one scope or be rejected as a conflict. In-process over temporary scopes. |
| `scn.specannotation.0de8bd2cbe54` Recognize the canonical annotation (existing) | unit | `….0de8bd2cbe54.unit` | internal/stele/annotation_test.go (existing) | Field parsing could break the canonical form. Pure parser. |
| `scn.specannotation.0d7c07caa518` Tolerate whitespace, line endings, and a byte order mark (existing) | unit | `….0d7c07caa518.unit` | internal/stele/annotation_test.go (existing) | Field parsing could break BOM, CRLF, or whitespace tolerance. Pure parser. |
| `scn.specannotation.6434ff2d493c` Ignore fields with a warning (existing) | unit | `….6434ff2d493c.unit` | internal/stele/annotation_test.go (existing, rewritten) | The `targets` field could still warn, or other fields could stop warning. Pure parser plus diagnostics over a fixture file. |
| `scn.specannotation.c1a89d2bc235` Reject an unsupported version (existing) | unit | `….c1a89d2bc235.unit` | internal/stele/annotation_test.go (existing) | Unchanged behavior next to a new field; regression only. Pure parser. |
| `scn.specannotation.10178af5c550` Reject a malformed annotation (existing) | unit | `….10178af5c550.unit` | internal/stele/annotation_test.go (existing) | A field-aware grammar could accept malformed first lines. Pure parser. |
| `scn.specannotation.2681b7fc2880` Report an annotation below the first line (existing) | unit | `….2681b7fc2880.unit` | internal/stele/annotation_test.go (existing) | A targeted annotation below the first line could escape `SPEC_ANNOTATION_MISPLACED`. Pure classifier. |
| `scn.specannotation.c1e6a83408c6` Annotate the files of a scope and preserve every other byte (existing) | unit | `….c1e6a83408c6.unit` | internal/stele/annotation_test.go (existing) | Target copying could alter bytes of other files. Temporary files in process. |
| `scn.specannotation.e419861e4e50` Annotate a delta spec with its capability's targets | unit | `….e419861e4e50.unit` | internal/stele/annotation_test.go, beside the annotate scope tests | A delta spec of a targeted capability could be annotated without its targets and then fail with `SPEC_TARGETS_MISMATCH`. Temporary change and current spec in process. |
| `scn.specannotation.6bd9c9356646` Change nothing on a second run (existing) | unit | `….6bd9c9356646.unit` | internal/stele/annotation_test.go (existing) | Copying targets could make a second run rewrite files. Temporary files in process. |
| `scn.specannotation.a518efc1ddd8` Check annotations without writing (existing) | unit | `….a518efc1ddd8.unit` | internal/stele/cli_test.go (existing) | `--check --json` could change shape. In process. |
| `scn.specannotation.a518efc1ddd8` Check annotations without writing (existing) | e2e | `….a518efc1ddd8.e2e` | tests/cli.test.mts (existing) | The shipped check could change its exit code. Built CLI. |
| `scn.specannotation.db2f08beff7a` Leave broken annotations for a person to fix (existing) | unit | `….db2f08beff7a.unit` | internal/stele/annotation_test.go (existing) | A malformed targets field could be repaired silently. Temporary files in process. |
| `scn.specannotation.7a12f1bf3cf0` Add the annotation together with missing IDs (existing) | unit | `….7a12f1bf3cf0.unit` | internal/stele/ids_test.go (existing) | Target copying could shift the reported ID line numbers. Temporary files in process. |
| `scn.specannotation.1aaf0a9528b5` Carry the capability's targets into a new delta spec annotation | unit | `….1aaf0a9528b5.unit` | internal/stele/ids_test.go, beside the annotation-with-IDs test | `stele ids` could insert a bare annotation for a targeted capability, or rewrite it on a second run. Temporary files in process. |
| `scn.specannotation.f0d8e6fbaa2d` Check a missing annotation according to the policy (existing) | unit | `….f0d8e6fbaa2d.unit` | internal/stele/ids_test.go (existing) | Policy handling is unchanged; regression only. In process. |
| `scn.specannotation.3baa32066f7a` Archive through Stele restores the annotation (existing) | unit | `….3baa32066f7a.unit` | internal/stele/init_test.go, beside TestArchiveSkillGatesOnValidation (existing, updated) | The archive skill could omit `--targets-from` or run it after validation. The skill template is embedded text, checked in process. |
| `scn.specannotation.23d533778e10` Direct OpenSpec archiving names the repair step (existing) | integration | `….23d533778e10.integration` | internal/stele/workflowschema_test.go, beside TestMergeKeepsUserConfiguration (existing, updated) | The merged archive guidance could lack `--targets-from`, or the merge could stop being idempotent. Runs openspec-extend.mjs with OpenSpec's yaml package. |
| `scn.specannotation.b7e05fe76741` Restore the annotation on a new current specification (existing) | integration | `….b7e05fe76741.integration` | internal/stele/annotation_test.go (existing) | Untargeted archive restoration could regress. Needs a real OpenSpec archive. |
| `scn.specannotation.0020f5fc5ce4` Restore the targets of archived delta specs | integration | `….0020f5fc5ce4.integration` | internal/stele/annotation_test.go, beside the existing archive restoration test | OpenSpec's archive rewrites current specifications, so targets could be lost, duplicated, or mangled. Only a real `openspec archive` shows the files that `--targets-from` must repair. |
| `scn.specannotation.4432e2c478c1` Index records the annotation of each file and item (existing) | unit | `….4432e2c478c1.unit` | internal/stele/index_test.go (existing) | The files list could change shape for untargeted files. In-process index. |
| `scn.specannotation.f69cc0129044` Index the declared targets of a file | unit | `….f69cc0129044.unit` | internal/stele/index_test.go, beside the annotation index test | A file's targets could be missing, reordered, or emitted for untargeted files. In-process index. |
| `scn.terminalreport.00fe379ba4b4` Name the stage that failed (existing) | unit | `….00fe379ba4b4.unit` | internal/stele/report_test.go (existing) | New target diagnostics could be put in the wrong stage line. Rendering in process. |
| `scn.terminalreport.99cec4c2f435` Overview per capability (existing) | unit | `….99cec4c2f435.unit` | internal/stele/report_test.go (existing) | The matrix section could displace or change the capability overview. Rendering in process. |
| `scn.terminalreport.5b46740a15da` Count tests by level and time the run (existing) | unit | `….5b46740a15da.unit` | internal/stele/report_test.go (existing, fake clock) | Per-target counting could change the execution line. Rendering with a fake clock. |
| `scn.terminalreport.29b8400fc1b1` Show the target matrix | unit | `….29b8400fc1b1.unit` | internal/stele/report_test.go, beside the overview tests | Matrix counts, marks, row selection, or truncation could be wrong or nondeterministic. The renderer is pure over a verification result. |
| `scn.terminalreport.33eebddcfa33` Leave the matrix out without targets | unit | `….33eebddcfa33.unit` | internal/stele/report_test.go | An empty matrix section could appear in untargeted reports. Rendering in process; the byte comparison is covered by scn.verificationtargets.add73aad6d42. |
| `scn.verificationstrategy.2779f7f381d3` Accept a complete evidence plan (existing) | unit | `….2779f7f381d3.unit` | internal/stele/plan_test.go (existing) | The targeted ID grammar could reject complete untargeted plans. Pure plan validation. |
| `scn.verificationstrategy.45a816cfbf7f` Reject malformed evidence entries (existing) | unit | `….45a816cfbf7f.unit` | internal/stele/plan_test.go (existing) | The widened ID grammar could accept a malformed untargeted ID. Pure plan validation. |
| `scn.verificationstrategy.159fc8c6d372` Plan evidence per target | unit | `….159fc8c6d372.unit` | internal/stele/plan_test.go | Per-target entries, ordinals, and the target field could be validated or reported wrongly. Pure plan validation over fixtures. |
| `scn.verificationstrategy.ef89652e0e06` Reject a mismatched or missing target | unit | `….ef89652e0e06.unit` | internal/stele/plan_test.go | A target field that disagrees with its ID, or a target in an untargeted plan, could pass. Pure plan validation. |
| `scn.verificationstrategy.aef528d23484` Require evidence for every scenario (existing) | unit | `….aef528d23484.unit` | internal/stele/plan_test.go (existing) | Untargeted completeness could regress. Pure plan validation. |
| `scn.verificationstrategy.8d540b6ad130` Require evidence for every target of a scenario | unit | `….8d540b6ad130.unit` | internal/stele/plan_test.go | A scenario with evidence for some targets could count as complete. Pure plan validation over a fixture with three targets. |
| `scn.verificationstrategy.53e78ff15ea6` Reject evidence for a target the scenario does not apply to | unit | `….53e78ff15ea6.unit` | internal/stele/plan_test.go | Evidence for a target narrowed away could be accepted or reported as `PLAN_UNKNOWN_ID`. Pure plan validation. |
| `scn.verificationstrategy.509b8e9fab98` Reject plan entries for unknown identities (existing) | unit | `….509b8e9fab98.unit` | internal/stele/plan_test.go (existing) | A targeted evidence ID under the wrong scenario could escape `PLAN_UNKNOWN_ID`. Pure plan validation. |
| `scn.verificationstrategy.f60e81cbfa96` Install planning guidance (existing) | unit | `….f60e81cbfa96.unit` | internal/stele/init_test.go (existing) | Adding target guidance could drop the existing sections. Embedded template text, in process. |
| `scn.verificationstrategy.1ca6af4620e9` Install guidance for targets | unit | `….1ca6af4620e9.unit` | internal/stele/init_test.go, beside the stele-plan guidance test | The skill could omit where differences live, the targeted ID form, or one of the three patterns. Embedded template text, in process. |
| `scn.verificationstrategy.17786fd23375` Describe the conversational approval step (existing) | unit | `….17786fd23375.unit` | internal/stele/init_test.go (existing) | The approval step must stay intact. Embedded template text. |
| `scn.verificationstrategy.9dad8263a6c0` Keep project placement skills (existing) | unit | `….9dad8263a6c0.unit` | internal/stele/init_test.go (existing) | Refreshing the skill could overwrite project skills. Temporary project in process. |
| `scn.verificationtargets.eb1c6810d349` Accept a target configuration | unit | `….eb1c6810d349.unit` | internal/stele/targets_test.go (new) | The configuration could be misread: order, paths, or `evidenceOnly`. Pure configuration loading over a temporary file. |
| `scn.verificationtargets.36d207a647d0` Reject an invalid target definition | unit | `….36d207a647d0.unit` | internal/stele/targets_test.go (new) and cli_test.go | An invalid or reserved name, such as `e2e` colliding with a level, could be accepted and make evidence IDs ambiguous. Configuration validation and the exit code, in process. |
| `scn.verificationtargets.e8cd45d85205` Apply a specification's targets to every scenario | unit | `….e8cd45d85205.unit` | internal/stele/targets_test.go (new) | The default inheritance of a specification's targets could be wrong. Pure resolution over parsed specifications. |
| `scn.verificationtargets.975c05dc71d9` Reject an unknown or malformed target list | unit | `….975c05dc71d9.unit` | internal/stele/targets_test.go (new) | A typo in a target list could create a silent extra target. Pure parsing and validation. |
| `scn.verificationtargets.ca80f845a9cc` Cover every requirement when a change alters targets | unit | `….ca80f845a9cc.unit` | internal/stele/targets_test.go (new), with a temporary change and current specification | A change could alter a capability's targets and leave current requirements unplanned, which would only surface after archiving. Needs change and current specs in process. |
| `scn.verificationtargets.577d723898cd` Narrow a scenario to one platform | unit | `….577d723898cd.unit` | internal/stele/targets_test.go (new) | Narrowing could fail to apply, or apply to sibling scenarios. Pure resolution. |
| `scn.verificationtargets.6616ae0098fb` Split requirements by responsibility | unit | `….6616ae0098fb.unit` | internal/stele/targets_test.go (new) | Requirement-level narrowing could fail to propagate to its scenarios. Pure resolution. |
| `scn.verificationtargets.7e8b768b3da0` Reject a scenario that widens its requirement | unit | `….7e8b768b3da0.unit` | internal/stele/targets_test.go (new) | A scenario could widen its requirement unnoticed. Pure validation. |
| `scn.verificationtargets.e324fcddf99b` Report misplaced and undeclared target lines | unit | `….e324fcddf99b.unit` | internal/stele/targets_test.go (new) and specs_test.go | A `Targets:` line in prose or in an untargeted spec could be silently ignored or applied. Pure parsing. |
| `scn.verificationtargets.bb95c914ccac` Keep approvals when only the targets of a scenario change | unit | `….bb95c914ccac.unit` | internal/stele/targets_test.go (new) and plan_test.go | Narrowing could invalidate approvals if the line entered the digest, or stale entries could pass. Pure digest and plan validation. |
| `scn.verificationtargets.1a22d82ecf41` Read targeted evidence IDs in TypeScript and Go | unit | `….1a22d82ecf41.unit` | internal/stele/anchors_test.go and goanchors_test.go | The anchor regexes stop at the level, so a targeted ID would be read as a bare scenario anchor. Pure scanners over fixture sources in both languages. |
| `scn.verificationtargets.880647fbb533` Report evidence anchored in the wrong target | unit | `….880647fbb533.unit` | internal/stele/targets_test.go (new), with temporary sources | An agent copying tests between replicas could leave evidence under the wrong target. Path matching and linkage, in process. |
| `scn.verificationtargets.c3f36c37e37a` Require an implementation in every replica | unit | `….c3f36c37e37a.unit` | internal/stele/targets_test.go (new), with temporary sources | A replica without an implementation could pass because the global requirement rule is satisfied elsewhere. Linkage in process. |
| `scn.verificationtargets.3574271b7f35` Accept shared code and journeys | unit | `….3574271b7f35.unit` | internal/stele/targets_test.go (new), with temporary sources | Overlapping paths or journeys could produce false errors. Linkage in process. |
| `scn.verificationtargets.e03ff09a1cd4` Show gaps between replicas | unit | `….e03ff09a1cd4.unit` | internal/stele/matrix_test.go (new) | The matrix could show a passing row while a replica has no evidence. Pure matrix computation. |
| `scn.verificationtargets.250dbc097c43` Mark scenarios that do not apply | unit | `….250dbc097c43.unit` | internal/stele/matrix_test.go (new) | A narrowed scenario could show `missing` instead of `n/a`. Pure matrix computation. |
| `scn.verificationtargets.35a9719d9446` Prefer a failure over other states | unit | `….35a9719d9446.unit` | internal/stele/matrix_test.go (new) | The precedence could hide a failure behind another state. Pure matrix computation. |
| `scn.verificationtargets.258ffea7adf7` Run the tests of one target | integration | `….258ffea7adf7.integration` | internal/stele/runner_test.go or batch_test.go, with Go and Node fixtures under ios/ and android/ | The target filter must reach batch planning: a batch of another target could still run, or its stored outcomes could be discarded. Only real `go test` and `node --test` batches show which tests ran. |
| `scn.verificationtargets.fc749b62bad7` Verify one target without the others' gaps | unit | `….fc749b62bad7.unit` | internal/stele/targets_test.go (new) and verify_test.go | `--target` could hide specification findings, or leak other targets' linkage findings. Verification in process. |
| `scn.verificationtargets.db222c63967d` Check one target in CI | e2e | `….db222c63967d.e2e` | tests/cli.test.mts, beside the CI gate test | The installed `stele check` could fail to pass `--target` through to validation, so a per-target CI job would fail on another target's gaps. Only the built CLI proves the pass-through end to end. |
| `scn.verificationtargets.857e3746c99d` Reject an unknown target selection | unit | `….857e3746c99d.unit` | internal/stele/cli_test.go | An unknown `--target` could silently select nothing and pass. Argument handling and exit codes, in process. |
| `scn.verificationtargets.add73aad6d42` Produce identical output without targets | e2e | `….add73aad6d42.e2e` | tests/cli.test.mts, running the built binary and the `stele-published` binary on the same fixture | Any shared code path (annotation, plans, anchors, index, report) could change untargeted output. Comparing the built CLI with the previous release, with only the version normalized, is the only convincing proof. |
| `scn.verify.14c6b4fe39bc` Accept planned targets in the proposal stage (existing) | unit | `….14c6b4fe39bc.unit` | internal/stele/verify_test.go (existing) | v1 plans must keep working; regression only. In process. |
| `scn.verify.c4db6a432869` Reject a mismatched implementation target (existing) | unit | `….c4db6a432869.unit` | internal/stele/verify_test.go (existing) | v1 plans must keep working; regression only. In process. |
| `scn.verify.1d0f8685d8c6` Apply v2 rules without targets (existing) | unit | `….1d0f8685d8c6.unit` | internal/stele/evidence_test.go (existing) | Adding target paths could turn location into a diagnostic for untargeted specs. In process. |
| `scn.verify.1bab272a56e4` Apply the per-target rules to targeted specifications | unit | `….1bab272a56e4.unit` | internal/stele/verify_test.go | Per-target rules could leak into untargeted specs, or miss the gap of one target. Verification in process over a mixed change. |
| `scn.verify.140b21cbc3f0` Separate a failed execution from passing linkage (existing) | unit | `….140b21cbc3f0.unit` | internal/stele/verify_test.go (existing) | Per-target verdicts could change the top-level verdicts. In process. |
| `scn.verify.a7ae8afade0a` Treat missing evidence as an incomplete overall verdict (existing) | unit | `….a7ae8afade0a.unit` | internal/stele/verify_test.go (existing) | Same risk for the incomplete verdict. In process. |
| `scn.verify.05d59c6b97af` Report verdicts per target | unit | `….05d59c6b97af.unit` | internal/stele/verify_test.go | Per-target verdicts could be computed from the wrong target's evidence or be missing from the report. In process. |

Requirement anchors (`@implements`) expected during implementation:

- `req.verificationtargets.fd65304d2bf7`: configuration loading in `targets.go`.
- `req.verificationtargets.8d4a7d3bc335`: target list validation, and the delta and current comparison.
- `req.verificationtargets.ab2f8aad3e78`: the `Targets:` parsing in `specs.go`, and resolution in `targets.go`.
- `req.verificationtargets.d1ac12f01077`: anchor attribution and the per-target linkage checks.
- `req.verificationtargets.4ed90b4188d7`: `matrix.go`.
- `req.verificationtargets.366797f24e77`: `--target` handling in `cli.go`, `check.go`, and the selection code.
- `req.verificationtargets.d74fcbc51f5c`: the omit-when-untargeted serialization.
- The MODIFIED requirements keep their existing anchors (`annotation.go`, `plan.go`, `index.go`, `report.go`, `verify.go`, and `runner.go`).

## Implementation notes

- **Catalogue fix texts.** `PLAN_EVIDENCE_MISSING` and `PLAN_EVIDENCE_INVALID` name the target in their messages, but their catalogue meaning and fix stay unchanged, because the report prints them and "Keep untargeted projects unchanged" requires byte-identical untargeted output.
- **Scan roots.** Anchors are only scanned below conventional folders (`src`, `internal`, `tests`, …). The literal prefix of every target path, such as `ios` for `ios/**`, is scanned too, and fingerprinted as an input; a target at the repository root (`**`) adds no input folder, so generated artifacts stay out of the digest.
- **Index naming.** A targeted evidence entry's target uses the existing `target` field of index evidence, which a version 1 entry keeps using for its planned path until 0.2.0 (`approval` tells them apart). Anchors gain `target`; report links use `evidenceTarget`.
- **This repository's own workflow** keeps `stele annotate --specs` in `openspec/config.yaml` and the installed `stele-archive` skill until the release that ships `--targets-from`, because it archives with the published Stele. Re-run `stele init` after that release.
- **Language server follow-up (task 8.3):** per-target code lenses, hover lines, and diagnostics, read from `buildTargetMatrix`, and watching the folders of target paths outside the conventional roots. Plan it as its own change after this one is released.

## Risks / Trade-offs

- **[Older Stele misreads targeted projects.]** rc.4 warns about the `targets` field and reads `@verifies scn.x.ios.unit` as a bare anchor. Mitigation: the changelog and the guide state the minimum version, and `stele init` can pin it.
- **[Configuration is required.]** Targets cannot be used without `paths`. That is a small cost for typo detection and implementation attribution (Decision 2).
- **[Mandatory per-target `@implements` may be noisy for thin targets,]** such as a web front end that only renders what the API computes. Mitigation: narrow the requirement with `Targets:`, or ask for `evidenceOnly`. Open Question 5 asks whether a softer mode is needed.
- **[The matrix can be large]** (for example 300 scenarios × 7 targets). The report shows only rows with gaps, capped at ten; the index has everything.
- **[Retargeting is verbose.]** Adding a target to a mature capability means listing its affected requirements as MODIFIED. That is intended, because it makes the planning cost visible, but it is heavy.
- **[Merge conflicts]** with `-lsp` (index consumers) and with the unarchived `ci-gates` and `fast-runs`. Mitigation: archive those two first, and rebase and re-run `stele ids` and `annotate --check` (task 0.3).
- **[The terminology collision with v1 "planned targets"]** lasts until 0.2.0 removes v1 plans.

## Migration Plan

- Ship in the next release candidate, with a changelog entry and the new "Targets" guide.
- Untargeted projects need no action. Output, plans, and approvals are unchanged.
- To adopt targets in an existing project:
  1. Add `targets` to `stele.config.json`.
  2. Add `; targets: …` to the specifications, through a change, so the scenarios are re-planned per target.
  3. Plan per-target evidence, approve it, and rename anchors to the targeted IDs.

  Open Question 2 asks whether a `stele plan migrate --targets` helper should do the renaming.
- Rollback: remove the `targets` fields and the configuration, and restore the untargeted plans from version control. Nothing else is stored.

## Open Questions (answered)

The reviewer approved the plan with the proposed answers, except for question 3.

1. **The `system` target name.** Conventional, not reserved. `evidenceOnly` is explicit, and some teams name the suite `e2e-suite` or `journeys`.
2. **A migration helper.** `stele plan migrate --targets <name>` is worth building, in a follow-up. Phase 1 documents the manual steps.
3. **Untargeted specifications under `--target`.** Included. Targets are optional, and a specification without them describes the project itself, so it applies whichever target is selected. Leaving it out would mean nobody checks it when every CI job uses `--target`.
4. **JSON naming.** `evidenceTarget` on links now, renamed to `target` at 0.2.0 when v1's `target` disappears. This avoids a schema bump.
5. **A softer implementation rule.** No. Narrowing with `Targets:` already expresses "this target does not implement this".
6. **Column order.** Name order: deterministic and independent of the specification.
