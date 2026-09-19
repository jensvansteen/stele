## 0. Approval gate

- [x] 0.1 Get the reviewer's explicit approval of the verification levels in design.md ("proposed, awaiting approval"), Decisions 1 to 9, and Open Questions 1 to 6. Only then run `stele approve --change verification-targets --confirmed-in-chat`, mark the design table approved, and check that `stele check --change verification-targets` reports no `PLAN_UNAPPROVED`. Start no task in sections 1 to 8 before this.
- [x] 0.2 Record the answers to the open questions in design.md. If an answer changes behavior (for example reserving `system`, including untargeted specifications under `--target`, or a warning mode), update the specs and plan with `openspec-update-change` first, and re-run `stele verify --stage proposal --change verification-targets`.
- [x] 0.3 Make sure `ci-gates` and `fast-runs` are archived, and rebase onto `main`. Then re-run `stele ids --change verification-targets` and `stele annotate --change verification-targets --check`, and check that every MODIFIED block still copies the current `spec-annotation`, `verification-strategy`, `link-index`, `terminal-report`, and `verify` requirements exactly, apart from this change's edits.
- [x] 0.4 Before changing any code, record the untargeted baseline outputs for the compatibility test (`add73aad6d42`). The test compares with the `stele-published` binary, so check that the fixture produces identical output with it and with the current build.

## 1. Configuration and specification targets

- [x] 1.1 Add `targets.go`: load and validate `stele.config.json` `targets` (names, reserved level names, `paths`, `evidenceOnly`, unknown keys, exit `2`), and add a dependency-free `**` glob matcher. Verify the unit tests for `eb1c6810d349` and `36d207a647d0` pass.
- [x] 1.2 Recognize the `targets` annotation field in `annotation.go`, keep `SPEC_ANNOTATION_FIELD_IGNORED` for other fields, and validate the list (`SPEC_TARGETS_MALFORMED`, `SPEC_TARGET_UNKNOWN`). Verify the unit tests for `6434ff2d493c`, `0de8bd2cbe54`, `0d7c07caa518`, `10178af5c550`, `2681b7fc2880`, `c1a89d2bc235`, `e8cd45d85205`, and `975c05dc71d9` pass.
- [x] 1.3 Parse `Targets:` metadata lines in `specs.go` and keep them out of text, steps, and digests. Resolve applicable targets with the narrowing rules (`SPEC_TARGETS_WIDENED`, `SPEC_TARGETS_MISPLACED`, `SPEC_TARGETS_UNDECLARED`). Verify the unit tests for `577d723898cd`, `6616ae0098fb`, `7e8b768b3da0`, `e324fcddf99b`, and `2bec33059b75` pass.
- [x] 1.4 Compare the targets of each delta spec with its current specification (`SPEC_TARGETS_MISMATCH`, `SPEC_TARGETS_CHANGE_UNCOVERED`). Verify the unit test for `ca80f845a9cc` passes.

## 2. Plans per target

- [x] 2.1 Extend plan v2 validation in `plan.go`: the targeted evidence ID grammar, the `target` field, the digest including the target, completeness per applicable target, and `PLAN_TARGET_NOT_APPLICABLE`, with untargeted plans unchanged. Verify the unit tests for `159fc8c6d372`, `ef89652e0e06`, `8d540b6ad130`, `53e78ff15ea6`, `bb95c914ccac`, `2779f7f381d3`, `45a816cfbf7f`, `aef528d23484`, and `509b8e9fab98` pass.

## 3. Anchors and linkage per target

- [x] 3.1 Widen the TypeScript and Go anchor grammars to targeted evidence IDs. Verify the unit tests for `1a22d82ecf41`, `57dcee8c30a8`, and `560aeb5e3970` pass.
- [x] 3.2 Attribute anchors to targets by path, and add `ANCHOR_TARGET_OUTSIDE_PATHS` and `LINK_TARGET_IMPLEMENTATION_MISSING` (skipped for `evidenceOnly` targets). Apply the per-target rules in `verify.go`, and add per-target verdicts to the report (`targets`, and `evidenceTarget` on links). Verify the unit tests for `880647fbb533`, `c3f36c37e37a`, `3574271b7f35`, `1bab272a56e4`, `05d59c6b97af`, `140b21cbc3f0`, `a7ae8afade0a`, `14c6b4fe39bc`, `c4db6a432869`, and `1d0f8685d8c6` pass.

## 4. Matrix, index, and report

- [x] 4.1 Add `matrix.go` with the cell states and precedence. Verify the unit tests for `e03ff09a1cd4`, `250dbc097c43`, and `35a9719d9446` pass.
- [x] 4.2 Add the targets, per-file targets, `declaredTargets` and `applicableTargets`, evidence and anchor targets, and the matrix to `stele index`, all omitted without targets. Verify the unit tests for `34d29862cf10`, `f69cc0129044`, `4432e2c478c1`, `30ab15bc8293`, `48b3c24773d9`, `331076ad0227`, and `bda4420352ef`, and the e2e tests for `30ab15bc8293` and `c0e515c9575a`, pass.
- [x] 4.3 Render the target matrix in the human report (a capability summary, at most ten gap rows, and `--details`), and name the selected targets in the header. Verify the unit tests for `29b8400fc1b1`, `33eebddcfa33`, `00fe379ba4b4`, `99cec4c2f435`, and `5b46740a15da` pass.
- [x] 4.4 Add the ten new codes to the diagnostics catalogue with their stage, meaning, and fix (design.md, Decision 8), and extend the `PLAN_EVIDENCE_MISSING` and `PLAN_EVIDENCE_INVALID` messages to name targets. Verify the existing catalogue test (`78fda44eee93`) passes.

## 5. Target selection

- [x] 5.1 Add repeatable `--target` to `test`, `verify`, `validate`, `check`, and `index`. Intersect it with selections, allow it with `--all`, exit `2` for unknown targets or a project without targets, and filter findings, matrix columns, and tests as specified. Rename "targets" to "selections" in `stele test` help and messages. Verify the unit tests for `fc749b62bad7`, `857e3746c99d`, `094a437d7c9c`, `e5c1d7ab73d5`, `352d7120ec6c`, `1f063f3c5e49`, `49a524bcf0ac`, `e0b21aa95623`, `6265c70bed70`, `5c91d0c168f0`, `4d6ba51a47bc`, and `7c1d78728036` pass.
- [x] 5.2 Make the target filter reach batch planning, and merge only the selected outcomes. Verify the integration test for `258ffea7adf7` (real Go and Node fixtures under `ios/` and `android/`) passes.
- [x] 5.3 Verify the e2e tests for `db222c63967d` (installed `check --all --target web`) and `7c1d78728036` pass.

## 6. Keeping targets through the workflow

- [x] 6.1 Make `stele annotate --change` and `stele ids` copy a current specification's `targets` into new delta spec annotations. Verify the unit tests for `e419861e4e50`, `1aaf0a9528b5`, `c1e6a83408c6`, `6bd9c9356646`, `db2f08beff7a`, `7a12f1bf3cf0`, `f0d8e6fbaa2d`, and `a518efc1ddd8`, and the e2e test for `a518efc1ddd8`, pass.
- [x] 6.2 Add `stele annotate --specs --targets-from <archive-dir>`, and name it in the `stele-archive` template and the archive guidance merged by `openspec-extend.mjs`. Verify the unit test for `3baa32066f7a` and the integration tests for `23d533778e10`, `b7e05fe76741`, and `0020f5fc5ce4` pass.

## 7. Guidance and documentation

- [x] 7.1 Extend the `stele-plan` template (and this repository's `.agents/skills/stele-plan/SKILL.md` through `stele init`): where differences live, the targeted ID form, and a worked example of replicas, split, and journey. Verify the unit tests for `1ca6af4620e9`, `f60e81cbfa96`, `17786fd23375`, and `9dad8263a6c0` pass.
- [x] 7.2 Write `docs/guide/targets.md` with the four worked examples from design.md: `stele-editors` (vscode, zed, and jetbrains replicas), the OpenSpec checkout split (`api` and `web` with a contract), iOS and Android replicas with platform-specific scenarios, and a `system` journey. Each example has its configuration, specification, plan excerpt, and matrix, and the guide recommends an untargeted `check --all` CI job next to per-target jobs.
- [x] 7.3 Update `docs/concepts/spec-format.md` (the `targets` field and `Targets:` lines), `docs/reference/cli.md` (`--target`, `--targets-from`, and selections), `docs/reference/link-index.md` (the new fields and the matrix), the diagnostics reference, the sidebar, and the changelog (including the minimum version for targeted projects). Verify `npm run docs:build` passes.

## 8. Compatibility and close

- [x] 8.1 Add the e2e compatibility test: the built CLI and the `stele-published` binary on the same untargeted fixture, comparing `validate --specs --json`, `index --specs`, and the human report byte for byte, with only the version normalized. Verify the e2e test for `add73aad6d42` passes.
- [x] 8.2 Add the `@implements` anchors listed in design.md, run `npm run verify` and `stele check --change verification-targets`, and fix every failure before reporting the change done.
- [x] 8.3 Note the follow-up for the language server (per-target lenses and diagnostics from `matrix.go`) in the `-lsp` change or as a new change proposal. Do not implement it here.
