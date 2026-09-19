## Why

Many products build one behavior in several places: an iOS and an Android app, a VS Code, Zed, and JetBrains extension, a web front end and an API, and an end-to-end suite that proves the whole flow. OpenSpec can hold that behavior in one specification, but Stele then treats every scenario as proven once. A scenario whose iOS test passes is green even when Android has no implementation and no test at all, and nothing shows which platform is behind.

The spec-annotation change reserved `; key: value` fields on the first line of a specification for exactly this purpose (its scenario already uses `targets: vscode, zed` as the example of an ignored field).

The product direction is **one spec, several targets**. A *target* is a place that produces evidence: a codebase, an app, or a repository, such as `ios`, `android`, `web`, `api`, `vscode`, `zed`, `jetbrains`, or `system` for full end-to-end journeys. Behavior differences between targets belong in the specification. Testing differences belong in the plan.

This change plans phase 1 in detail: targets within one repository. Phases 2 to 4 (an OpenSpec store as the pinned spec source, evidence and progress reported back to the store, and agent orchestration across targets) are outlined in design.md only, without requirements.

## What Changes

**Targets are optional.** Without them, a specification describes the project itself and Stele works exactly as today. A specification without `targets` keeps that meaning in a project that does use targets: it applies to the whole project, whichever target is selected.

**Phase 1, targets within one repository:**

- **Project targets.** `stele.config.json` gets a `targets` object that maps each name to `paths` globs and an optional `evidenceOnly` flag (for a `system` end-to-end suite). The configuration is the only registry of target names, so a typo such as `andriod` is an error.
- **Specification targets.** The first-line annotation reads `<!-- stele: spec v1; targets: ios, android -->`. `targets` becomes the first field that version 1 defines, and other fields keep their `SPEC_ANNOTATION_FIELD_IGNORED` warning. By default every requirement and scenario applies to every declared target.
- **Narrowing.** A requirement or scenario may narrow with a `Targets: a, b` line in its metadata block, next to `Verification-ID:`. A scenario may only narrow its requirement's targets, never widen them. The line is metadata, so it stays out of the approval digest and the index text.
- **Per-target evidence.** Plan v2 entries of targeted scenarios carry a `target` and a targeted evidence ID, `<scenario>.<target>.<level>[.<n>]`. Every applicable target needs its own entry, and approval stays per entry. Untargeted specifications keep `<scenario>.<level>[.<n>]`, their plans, and their approvals unchanged.
- **Per-target verification.**
  - `@verifies` anchors take their target from the evidence ID and must live inside that target's paths.
  - `@implements` anchors are attributed to targets by path. Every requirement needs an implementation in each target it applies to, except `evidenceOnly` targets.
  - Reports get per-target verdicts.
- **The matrix.** A scenario × target matrix, with `n/a`, `missing`, `unapproved`, `not-run`, `stale`, `failed`, and `passed` cells, appears in `stele index` and in the human report.
- **Target selection.** `stele test`, `verify`, `validate`, `check`, and `index` accept `--target <name>`, for example one CI job per target. The positional arguments of `stele test` are renamed "selections" in the specification and documentation, to avoid a clash with the new word.
- **Keeping targets through the workflow.**
  - `stele ids` and `stele annotate --change` copy the capability's targets into new delta spec annotations.
  - `stele annotate --specs --targets-from <archive-dir>` restores them after an OpenSpec archive.
  - A change that alters a capability's targets must list every requirement it affects.
- **New diagnostics,** each with a stage, meaning, and fix in the catalogue:
  - `SPEC_TARGETS_MALFORMED`, `SPEC_TARGET_UNKNOWN`, `SPEC_TARGETS_WIDENED`, `SPEC_TARGETS_MISPLACED`, `SPEC_TARGETS_UNDECLARED`, `SPEC_TARGETS_MISMATCH`, `SPEC_TARGETS_CHANGE_UNCOVERED`;
  - `PLAN_TARGET_NOT_APPLICABLE`;
  - `ANCHOR_TARGET_OUTSIDE_PATHS`;
  - `LINK_TARGET_IMPLEMENTATION_MISSING`.

  `PLAN_EVIDENCE_MISSING` and `PLAN_EVIDENCE_INVALID` extend to targets.
- **Guidance and documentation.**
  - The `stele-plan` skill explains where differences live and gives a worked example of the three patterns: replicas, split, and journey.
  - A new guide has worked examples: the `stele-editors` replicas (vscode, zed, jetbrains), OpenSpec's checkout example split over `api` and `web` with a contract, iOS and Android replicas with platform-specific scenarios, and a `system` journey.
- **Compatibility.** A project without targets produces byte-identical output. An integration test compares the new binary with the previous release.

**Follow-up, not in this change:** per-target code lenses and diagnostics in the language server (the `-lsp` work), which reads the same in-process matrix.

**Phases 2 to 4, design outline only (design.md):**

- the store as a pinned spec source, with `stele store sync`;
- evidence and progress back to the store, with the archive rule;
- `stele-apply --target X [--reference Y]` and a rollout skill.

## Capabilities

### New Capabilities

- `verification-targets`: project and specification targets, narrowing, anchor attribution, the scenario × target matrix, `--target` selection, and compatibility for untargeted projects.

### Modified Capabilities

- `spec-annotation`: version 1 defines the `targets` field. `annotate` and `ids` copy a capability's targets into delta specs. `annotate --specs --targets-from` restores targets after archiving. The index lists each file's targets.
- `verification-strategy`: plan v2 entries carry targets and targeted evidence IDs, with evidence required per applicable target. The `stele-plan` skill covers the three patterns.
- `link-index`: the index gains targets, per-item targets, and the matrix. `stele test` accepts targeted evidence IDs, and its positional arguments are called selections. `--all` combines with `--target`.
- `terminal-report`: the human report shows the target matrix.
- `verify`: the stages apply the per-target rules, and reports gain per-target verdicts.

`validate` (`stele check`) gets no delta. `--target` is defined once in `verification-targets` for every command, which avoids a conflict with the unarchived `ci-gates` delta of the same requirement.

## Impact

- `internal/stele`:
  - configuration loading (a `targets` registry and a glob matcher, without dependencies);
  - the annotation field parser, spec parsing (`Targets:` metadata), plan validation (targeted IDs and completeness), the anchor scanners (the targeted ID grammar in TypeScript and Go), and linkage (path attribution);
  - the new matrix component used by `index.go` and `report.go`;
  - the diagnostics catalogue, CLI flags, and the embedded `stele-plan`, `stele-archive`, and `openspec-extend` templates.
- **Output schemas.** Only additive fields, present only when targets are in use: in the index (`targets`, `declaredTargets`, `applicableTargets`, `matrix`), in reports (`targets` verdicts, and an `evidenceTarget` on links, because `target` already names v1 path targets), and in plan entries (`target`). The evidence file schema is unchanged; targets derive from evidence IDs.
- **Older Stele versions** read a targeted project incorrectly: rc.4 warns `SPEC_ANNOTATION_FIELD_IGNORED` for `targets` and reads `@verifies <scn>.ios.unit` as a bare scenario anchor. Targets therefore need the release that ships this change, which the changelog states.
- **Documentation:**
  - a new "Targets" guide with the four worked examples;
  - the spec format and link-index references, the CLI reference, the diagnostics list, and the changelog.
- **Order.** `ci-gates` and `fast-runs` are implemented but not archived. They should be archived before this change is implemented, so that its MODIFIED blocks can be rechecked against the current specifications.
