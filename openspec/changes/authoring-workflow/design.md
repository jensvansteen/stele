## Context

See proposal.md. The current state:

- **IDs.** A prototype of `stele ids` exists on the unmerged `feat/stele-ids` branch. It is inspiration only: it predates per-change plans, archived specs in `openspec/specs`, Go anchors, and evidence-ID suffixes.
- **Skills.** `stele init` installs `stele-plan` and `stele-verify`. OpenSpec installs and updates its own skills through `openspec init` and `openspec update`. Those skills record the OpenSpec version that generated them (`metadata.generatedBy`).
- **Backend.** Stele currently builds on OpenSpec (MIT, Fission AI), pinned to 1.13.0. The adapter seam and version drift checks are planned in `specification-adapter`.

### Experiment findings (OpenSpec 1.13.0, scratch copy of `stele-examples/apps/todo`)

1. **Forking works.** `openspec schema fork spec-driven stele` copies the schema to `openspec/schemas/stele/` (`schema.yaml` and `templates/`). The `schema` commands are marked experimental. `openspec schema validate stele` accepts the modified schema.
2. **Schema fields.**
   - An artifact supports `id`, `generates`, `description`, `template`, `instruction`, and `requires`.
   - The schema-level `apply` supports `requires`, `tracks`, and `instruction`.
   - There is no archive section.
   - A JSON template and `generates: linkage-plan.json` work.
3. **Default schema.** With `schema: stele` in `openspec/config.yaml`, new changes record `schema: stele` in `.openspec.yaml`, and `openspec status` lists `proposal`, `specs`, `design`, `verification`, `tasks`. Existing changes keep the schema recorded in their `.openspec.yaml`.
4. **Verification instructions.** `openspec instructions verification` returns the artifact's instruction, the config `rules.verification` entries, the template, and the output path.
5. **Gap: apply was not gated.** The forked schema's `apply.requires` lists only `tasks`, so apply reported `ready` while `linkage-plan.json` was missing. With `apply.requires: [tasks, verification]`, apply reports `blocked` until the plan exists, and then lists the plan in `contextFiles`.
6. **Apply guidance channels.** The schema's `apply.instruction` is returned as the apply instruction, which the apply skill shows and follows as the "dynamic instruction". `operations.apply.guidance` is returned as `operationGuidance`, which the apply skill treats as "optional additive advice", "prompt-level, not enforceable".
7. **Gap: archive cannot block.** `operations.archive.guidance` is returned by `openspec instructions archive --json`. The archive skill treats it as advisory and states the lookup "must never block archiving". No schema hook exists for archive.
8. **Propose follows the schema.** The propose skill walks the schema's artifacts in dependency order, follows each artifact's `instruction` as authoritative (including delegating to a named skill), and builds everything that apply transitively requires. It will build `verification`.
9. **`openspec-verify-change`** is not in OpenSpec's default core skill profile. It is enabled with `openspec config profile`.
10. **OpenSpec tooling leaves the fork alone.**
    - `openspec update --force` leaves `openspec/config.yaml` and the forked schema unchanged.
    - `openspec validate --strict` and `openspec archive` work for a `stele`-schema change.
    - The archive keeps `linkage-plan.json` with the change.

## Goals / Non-Goals

**Goals:**

- Users start every step from Stele's own lifecycle skills, which stay thin enough that the backend can change behind them.
- Projects that use OpenSpec skills directly still get the Stele steps through the schema and configuration.
- IDs are generated, stable, and checked.
- Installation is idempotent and never overwrites user choices.

**Non-Goals:**

- Logic inside the lifecycle skills. Ordering only; logic lives in the CLI and in the reference skills.
- Changing, forking, or copying OpenSpec skills.
- The plan format and the `stele approve` command, which `verification-strategy` adds inside `stele-plan` and the CLI.
- The adapter seam and version drift checks, which `specification-adapter` covers.

## Decisions

### `stele ids`

The command is rebuilt in `internal/stele/ids.go` from the prototype's ideas.

- **Derivation:** the token is `sha256("stele-ids/v1", change, capability path, kind, requirement title, scenario title, occurrence, attempt)[:12]`. On a collision, `attempt` is incremented.
- **Collision set:** all tokens in Markdown under `openspec/`, plus the base IDs of every scanned code and test anchor. The anchor pattern reduces `scn.x.y.e2e` to its base.
- **Base-ID reuse:** `MODIFIED` headings reuse the ID from `openspec/specs/<capability>/spec.md`, following `RENAMED` pairs, unless the change already uses it. `REMOVED` and `RENAMED` sections are skipped.
- **Output:** byte-preserving insertion; `--check` exits `1` while IDs are missing; `--json` uses the canonical marshaller. Archived specs are never modified.
- **`--check` stays separate** (decided). `stele verify --stage proposal` already fails on missing IDs, so `stele ids --check` remains a separate, cheap CI or pre-commit step instead of being folded into the gate.

### Lifecycle skills (default entry points)

The three templates live in `internal/stele/templates/`. Each is a short, ordered list.

**`stele-propose`**
1. Use the `openspec-propose` skill for the change the user describes.
2. If the change is on the Stele schema, its `verification` artifact has already run the planning step.
3. Otherwise, run `stele ids --change <change>`, follow `stele-plan`, and run `stele verify --stage proposal --change <change>`.
4. Stop with: "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."

**`stele-apply`**
1. Run `stele verify --stage proposal --change <change>`.
2. Follow the approval step in `stele-plan`: show the pending levels, ask for confirmation, and record it only after an explicit yes. After `verification-strategy`, recording means `stele approve --confirmed-in-chat`; before that, the step records the confirmation in design.md.
3. Use the `openspec-apply-change` skill.
4. Add `@implements` and `@verifies` anchors as `stele-verify` describes.
5. Run `stele validate --change <change>` and fix failures before reporting done.

**`stele-archive`**
1. Run `stele validate --change <change>`, and stop if it fails.
2. Use the `openspec-archive-change` skill.
3. Run `stele validate --specs`.

**Design choices:**
- **Approval semantics.** `stele-apply` and the schema path share the same approval step, because both point to `stele-plan`. When `verification-strategy` changes that step, the lifecycle skills stay unchanged.
- **Backend names in one place.** The skills name OpenSpec skills only in their delegation lines. If the backend changes, those lines change and the workflow users know stays the same.
- **Why lifecycle skills are the default.** Habits attach to `stele-*`, which keeps the backend replaceable. They also close the archive gap that the schema and configuration cannot close (see enforcement below).

**Rejected alternative:** wrappers that copy OpenSpec's instructions. They would drift from `openspec update` and double the maintenance.

### Workflow schema installation (support for direct OpenSpec use)

`stele init` does the following, in order:

1. **Initializes OpenSpec when it is missing.**
   - If the project has no `openspec/` directory, `init` runs the bundled, pinned `openspec init --tools <tools>`. `--tools` defaults to `agents`, the tool-neutral `.agents/skills` target, and is passed through unchanged.
   - For the default target, `init` links `.claude/skills` to `../.agents/skills`. If `.claude/skills` already exists, it warns and leaves it alone.
   - An existing `openspec/` directory is never re-initialized.
2. **Forks the schema.**
   - If `openspec/schemas/stele/` does not exist, `init` runs the bundled `openspec schema fork spec-driven stele`.
   - An existing `stele` schema is kept and reported, unless `--refresh-schema` is given. That flag re-forks with `--force` and re-patches, and is the documented step after an OpenSpec upgrade (decided).
3. **Patches the fork** with the embedded Node script `internal/stele/templates/openspec-extend.mjs`. The script uses the `yaml` package resolved from the bundled OpenSpec, so edits preserve comments and order. The patch:
   - sets the description;
   - inserts the `verification` artifact before `tasks`: `generates: linkage-plan.json`, a JSON template, `requires: [specs, design]`, and the five-step instruction below;
   - adds `verification` to `tasks.requires`;
   - sets `apply.requires` to `[tasks, verification]` (this closes experiment gap 5);
   - appends the Stele apply steps to `apply.instruction`.

   A header comment records the Stele and OpenSpec versions that produced the fork.
4. **Selects the schema.** It sets `schema: stele` only when the config currently says `spec-driven`. Any other value is kept, with a note. Active changes keep the schema recorded in their `.openspec.yaml`. `init` never switches them (decided); the OpenSpec guide explains how to move one by hand.
5. **Merges guidance.** It merges `operations.apply.guidance`, `operations.archive.guidance`, and `rules.verification` into the config. Entries are identified by exact text, only missing ones are appended, and the file is written only when something changed.

A Node, OpenSpec, or parse failure makes `init` exit with code `2`, and files are only written after all edits have succeeded.

**`verification` instruction** (it points to `stele-plan` for every format detail):
1. Run `stele ids --change <change>`.
2. Propose evidence for every scenario (level, rationale, advisory placement from the project's AGENTS.md, CLAUDE.md, skills, and layout), following the `stele-plan` skill.
3. Write the change's `linkage-plan.json` as `stele-plan` describes, with every entry unapproved.
4. Run `stele verify --stage proposal --change <change>`, and fix what it reports.
5. Finish with: "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."

**Apply steps** (appended to `apply.instruction`, and repeated as `operations.apply.guidance` for changes that are not on the Stele schema):
1. Before the first task, confirm the verification levels with the user, as the approval step in `stele-plan` describes.
2. Add `@implements` and `@verifies` anchors while writing code and tests.
3. Run `stele validate --change <change>` and fix failures before reporting the change done.

**Archive guidance:** `stele validate --change <change>` must pass before archiving, and `stele validate --specs` runs afterwards.

**Rejected alternatives:**
- Shipping a hand-written schema instead of a fork. It would drift from the installed OpenSpec's `spec-driven` instructions.
- A Go YAML dependency. The Go core stays dependency-light, and Node is already required.

### Enforcement: how much Stele relies on the schema and configuration

The experiment decides how much the direct OpenSpec path is trusted:

- **Propose: reliable.** OpenSpec's propose skill builds every artifact apply depends on and follows each artifact's instruction, so the `verification` artifact runs. `stele-propose` relies on this for changes on the Stele schema.
- **Apply: reliable enough.** Apply is blocked until the plan exists (gap 5 is closed by `apply.requires`), and the Stele steps are in the apply instruction. Operation guidance is advisory only and is not the primary channel. The CLI is the hard backstop: `stele validate --change` fails on missing anchors and, after `verification-strategy`, on unapproved or stale evidence.
- **Archive: not enforceable through OpenSpec.** The archive skill treats guidance as advisory and never blocks. Direct users of `openspec-archive-change` can therefore archive without `stele validate --change`.

**Conclusion.**
- The lifecycle skills are the reliable default path. `stele-archive` closes the archive gap by running the gate first.
- For direct OpenSpec use, the schema and configuration are good support for propose and apply. For archive, the documentation recommends a CI gate that runs `stele validate --specs` on every pull request, as this repository does, which catches archived behavior that no longer verifies.

### `init` output and documentation

- **`init` output** says:
  - to use `stele-propose`, `stele-apply`, and `stele-archive` (the default);
  - that direct OpenSpec users get the planning step from the Stele schema;
  - how to enable `openspec-verify-change`;
  - which `stele` commands to run when a project uses OpenSpec without the Stele schema.
- **The new "Using Stele with OpenSpec" guide:**
  - credits OpenSpec (Fission AI) and explains that Stele extends it through workflow schemas and project configuration;
  - walks through explore, propose, apply, verify-change, and archive;
  - positions `openspec-verify-change` as the AI review of intent, design, and patterns after `stele validate`;
  - states that explore and sync-specs need no Stele steps;
  - explains how to move an existing change to the Stele schema;
  - gives the fallback table:

    | After this OpenSpec step | Run |
    |---|---|
    | Specs written | `stele ids`, the `stele-plan` step, `stele verify --stage proposal` |
    | Apply | `stele validate --change` |
    | Archive | `stele validate --specs` |

- Getting started, Build a Todo, and the README use the lifecycle skills. The OpenSpec guide covers direct use, and its credit line reads "Stele currently builds on OpenSpec (MIT, Fission AI)".

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-17, before implementation started (via: agent-confirmed, chat review). The review added: `stele init` initializes OpenSpec when it is missing, `--change` is optional, and `--tools` passes through.

Levels follow the definitions in `verification-strategy`: unit calls code directly, integration reaches one real outside tool, and e2e uses the installed executable. Placement follows this repository's AGENTS.md and is advisory. This change's linkage plan is v1 for the published verifier and records the ★ row.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Target | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|---|
| `scn.ids.83b2e0efd623` Insert IDs below headings that lack them | unit ★ | `….83b2e0efd623.unit` | `internal/stele/ids_test.go`, beside `ids.go` | `internal/stele/ids_test.go#TestIdsInsertsMissingIdentities` | Risk: IDs are misplaced or not accepted by verification. In-process run plus `ParseSpecs`. |
| | e2e | `….83b2e0efd623.e2e` | `tests/cli.test.mts` (executable tests) | `tests/cli.test.mts#inserts and checks verification IDs` | Distinct risk: `ids --check`, then `ids`, then `verify --stage proposal` do not compose in the shipped binary, which is how CI and pre-commit use them. |
| `scn.ids.1f44342aea82` Preserve existing IDs and every other byte | unit ★ | `….1f44342aea82.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsPreservesBytesAndExistingIdentities` | Risk: formatting damage to user specs. Byte comparison. |
| `scn.ids.9211a14a8ec9` Check for missing IDs without writing | unit ★ | `….9211a14a8ec9.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsCheckWritesNothing` | Risk: the check writes files or returns the wrong exit code. `Run` in process. |
| `scn.ids.0db6a666b9d4` Report IDs as JSON | unit ★ | `….0db6a666b9d4.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsReportsJSON` | Risk: agents and CI cannot parse the result. |
| `scn.ids.ed0d3d9619a5` Derive the same IDs for the same draft | unit ★ | `….ed0d3d9619a5.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsAreReproducible` | Risk: nondeterministic IDs. |
| `scn.ids.d83366faeaf7` Avoid IDs that already exist | unit ★ | `….d83366faeaf7.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsAvoidDeclaredAndAnchoredIdentities` | Risk: a new ID silently links to an old spec or a leftover anchor. |
| `scn.ids.11a49d3afd32` Reuse the current ID of a modified requirement | unit ★ | `….11a49d3afd32.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsReuseLivingSpecIdentities` | Risk: modified behavior loses its identity. |
| `scn.ids.03c9b0ba7536` Leave removed and renamed sections alone | unit ★ | `….03c9b0ba7536.unit` | `ids_test.go` | `internal/stele/ids_test.go#TestIdsSkipRemovedAndRenamedSections` | Risk: retired behavior gets new IDs. |
| `scn.lifecycle.5a216e97cca6` Propose through Stele | unit ★ | `….5a216e97cca6.unit` | `internal/stele/init_test.go`, beside the skill installer | `internal/stele/init_test.go#TestProposeSkillOrdersPlanningSteps` | Risk: the default entry point skips planning or does not stop for the user. The ordered steps in the embedded template are checked; the planning itself is enforced by the proposal gate. |
| `scn.lifecycle.b91a6d6dd399` Apply through Stele | unit ★ | `….b91a6d6dd399.unit` | `init_test.go` | `internal/stele/init_test.go#TestApplySkillConfirmsBeforeDelegating` | Risk: implementation starts before confirmation, or validation is missing. Template check; unapproved entries are also caught by `stele validate` after `verification-strategy`. |
| `scn.lifecycle.08814fe1e646` Archive through Stele | unit ★ | `….08814fe1e646.unit` | `init_test.go` | `internal/stele/init_test.go#TestArchiveSkillGatesOnValidation` | Risk: archiving without a passing validation, which is the gap OpenSpec cannot close. Template check. |
| `scn.lifecycle.5df1b597ff80` Keep lifecycle skills thin | unit ★ | `….5df1b597ff80.unit` | `init_test.go` | `internal/stele/init_test.go#TestLifecycleSkillsStayThin` | Risk: the wrappers grow logic or copy OpenSpec text and drift. The test compares against the installed OpenSpec skill text and forbids plan-format terms. |
| `scn.workflowschema.b4a054dbe512` Create new changes with the Stele schema | integration ★ | `….b4a054dbe512.integration` | `internal/stele/workflowschema_test.go`, beside the installer | `internal/stele/workflowschema_test.go#TestInitInstallsStelePipelineSchema` | Risk: the fork, patch, or default selection does not reach OpenSpec. Needs the real bundled OpenSpec CLI (`schema fork`, `new change`, `status`). |
| `scn.workflowschema.270587f7a4c3` Plan verification as a workflow artifact | integration ★ | `….270587f7a4c3.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestVerificationInstructionsReachOpenSpec` | Risk: agents never see the Stele planning steps. Only `openspec instructions` shows what the propose skill will read. |
| `scn.workflowschema.a444787573ca` Block apply until the verification plan exists | integration ★ | `….a444787573ca.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestApplyWaitsForVerificationPlan` | Risk: apply starts without a plan (experiment gap 5). Needs real apply instructions. |
| `scn.workflowschema.3aef78a653b4` Keep existing changes and customized schemas | integration ★ | `….3aef78a653b4.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestInitKeepsExistingSchemas` | Risk: init silently changes a user's workflow. Needs the real fork step to prove it is skipped. |
| `scn.workflowschema.779f1d7cebcb` Initialize OpenSpec when missing | integration ★ | `….779f1d7cebcb.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestInitInitializesOpenSpecWhenMissing` | Risk: a fresh project ends up without OpenSpec, with the wrong tool target, or with a clobbered `.claude/skills`. Needs the real bundled `openspec init`. |
| `scn.workflowschema.6ffeeaa11cce` Merge guidance into existing configuration | integration ★ | `….6ffeeaa11cce.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestMergeKeepsUserConfiguration` | Risk: user config or comments are lost. Needs Node and the bundled `yaml` package. |
| `scn.workflowschema.61671da05c73` Merge guidance only once | integration ★ | `….61671da05c73.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestMergeIsIdempotent` | Risk: repeated init duplicates entries or reformats files. Real `yaml` round trip. |
| `scn.workflowschema.b184c4d2986a` Deliver guidance through OpenSpec itself | integration ★ | `….b184c4d2986a.integration` | `workflowschema_test.go` | `internal/stele/workflowschema_test.go#TestOpenSpecInstructionsShowGuidance` | Risk: OpenSpec ignores or renames the fields. Only the pinned OpenSpec CLI shows it. |
| `scn.init.cbf5781012fa` Create configuration, skills, and artifacts directory (modified) | unit ★ | `….cbf5781012fa.unit` (existing) | `internal/stele/cli_test.go` (existing) | `internal/stele/cli_test.go#TestInitializeWritesConfigAndSkills` | Risk: a lifecycle or reference skill is missing. The existing test is extended to five skills. |
| | e2e | `….cbf5781012fa.e2e` (existing) | `tests/package_install_test.go` (existing) | `tests/package_install_test.go#TestPackedPackageInitializesAndValidatesSeparateConsumer` | Distinct risk: the packed package lacks the skill templates or schema script, or cannot run the bundled OpenSpec and `yaml` in a real consumer install. The test runs `stele init` in an empty consumer without a prior `openspec init`. |
| `scn.init.e841b29256e0` Preserve existing files on a repeated run (modified) | unit ★ | `….e841b29256e0.unit` (existing) | `internal/stele/init_test.go` (existing) | `internal/stele/init_test.go#TestInitializeIsIdempotent` | Risk: re-running init overwrites user edits. |
| `scn.init.754fd262e114` Require a change for initialization (modified: `--change` is optional; a change is required at verification) | unit ★ | `….754fd262e114.unit` (existing) | `cli_test.go` (existing) | `internal/stele/cli_test.go#TestRunHandlesHelpVersionAndInitialization` | Risk: init still demands a change, or a later bare verify silently passes. `Run` in process with the OpenSpec setup stubbed. |
| `scn.init.c8813c799047` Print the default workflow | unit ★ | `….c8813c799047.unit` | `cli_test.go`, beside init output | `internal/stele/cli_test.go#TestInitPrintsDefaultWorkflow` | Risk: users are not pointed to the lifecycle skills, or not shown the fallback. `Run` output in process. |

Requirement implementation targets (v1 plan today):

| Requirement | Target |
|---|---|
| `req.ids.5cd09e44308f` Insert missing verification IDs | `internal/stele/ids.go#AssignIdentities` |
| `req.ids.320cb32c3b8a` Derive stable and unique IDs | `internal/stele/ids.go#deriveIdentity` |
| `req.ids.877eabdee0b4` Keep the identity of modified behavior | `internal/stele/ids.go#loadBaseIdentities` |
| `req.lifecycle.4c9232a6cf44` Install thin lifecycle skills | `internal/stele/init.go#installSkills` |
| `req.workflowschema.97dec0adc04d` Install the Stele workflow schema | `internal/stele/workflowschema.go#installWorkflowSchema` |
| `req.workflowschema.db5e762d55ea` Guide apply and archive through OpenSpec configuration | `internal/stele/workflowschema.go#mergeOpenSpecGuidance` |
| `req.init.eed35c447821` Initialize a consumer project (modified) | `internal/stele/init.go#Initialize` |

## Risks / Trade-offs

- [`openspec schema` commands are experimental] → OpenSpec is pinned. The integration tests run the pinned CLI and fail on drift. The fork records the versions that produced it.
- [A forked schema freezes OpenSpec's `spec-driven` instructions at install time] → `openspec update` leaves the fork alone. Refreshing the fork after an OpenSpec upgrade is an open question.
- [Apply and archive guidance are prompt-level] → The apply steps live in the schema instruction, the CLI gates are the backstop, and archive relies on CI (see the wrapper follow-up).
- [Existing changes stay on `spec-driven`] → The guide explains how to switch an active change by editing its `.openspec.yaml` and adding a plan.
- [Node and the bundled `yaml` are needed for edits] → Node is already required. Failures exit with code `2` without partial writes.

## Decided questions

- **Schema refresh after an OpenSpec upgrade:** `stele init --refresh-schema`.
- **Active `spec-driven` changes:** `init` does not switch them; the guide documents the manual move.
- **`stele ids --check`:** stays a separate step.

## Accepted deviations

Accepted by jensvansteen on 2026-09-17:

- **Init scenario title:** OpenSpec's strict validation does not allow renaming a modified scenario, so `scn.init.754fd262e114` keeps the title "Require a change for initialization". Its text now says that `init` succeeds without a change and that a bare `stele verify` exits with code `2`.
- **Proposal gate wording:** `stele-propose` and the schema's `verification` instruction say to run `stele verify --stage proposal` and fix what it reports, instead of requiring it to pass, because an unapproved plan is reported until the approval step.
