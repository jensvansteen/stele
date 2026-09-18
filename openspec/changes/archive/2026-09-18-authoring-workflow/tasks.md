## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels, the proposed answers, and the open questions in design.md, and record it in design.md before starting section 1

## 1. `stele ids`

- [x] 1.1 Implement insertion, `--check`, and `--json` in `internal/stele/ids.go` (the prototype is inspiration only); verify `TestIdsInsertsMissingIdentities`, `TestIdsPreservesBytesAndExistingIdentities`, `TestIdsCheckWritesNothing`, and `TestIdsReportsJSON` pass
- [x] 1.2 Implement derivation with collisions against `openspec/` and anchors; verify `TestIdsAreReproducible` and `TestIdsAvoidDeclaredAndAnchoredIdentities` pass
- [x] 1.3 Implement base-ID reuse and section handling; verify `TestIdsReuseLivingSpecIdentities` and `TestIdsSkipRemovedAndRenamedSections` pass
- [x] 1.4 Add the end-to-end test; verify `inserts and checks verification IDs` passes in `npm run test:node`

## 2. Lifecycle skills

- [x] 2.1 Add the `stele-propose`, `stele-apply`, and `stele-archive` templates as ordered steps only, and install all five skills; verify `TestProposeSkillOrdersPlanningSteps`, `TestApplySkillConfirmsBeforeDelegating`, `TestArchiveSkillGatesOnValidation`, `TestLifecycleSkillsStayThin`, and the extended `TestInitializeWritesConfigAndSkills` pass

## 3. Workflow schema and configuration (support for direct OpenSpec use)

- [x] 3.1 Add the embedded `openspec-extend.mjs` script and its Go runner, resolving `yaml` from the bundled OpenSpec; verify `TestMergeKeepsUserConfiguration` and `TestMergeIsIdempotent` pass
- [x] 3.2 Fork, patch, and select the `stele` schema from `stele init`; verify `TestInitInstallsStelePipelineSchema`, `TestVerificationInstructionsReachOpenSpec`, `TestApplyWaitsForVerificationPlan`, and `TestInitKeepsExistingSchemas` pass
- [x] 3.3 Merge apply and archive guidance and verification rules; verify `TestOpenSpecInstructionsShowGuidance` passes
- [x] 3.4 Run the bundled `openspec init` when `openspec/` is missing (`--tools`, `.claude/skills` link), make `--change` optional, add `--refresh-schema`, and print the workflow and fallback; verify `TestInitInitializesOpenSpecWhenMissing`, `TestRunHandlesHelpVersionAndInitialization`, `TestInitPrintsDefaultWorkflow`, and the package install test in an empty consumer pass
- [x] 3.5 Point the `stele-plan` skill at the `verification` artifact and add the approval step used by `stele-apply` and the schema (recording in design.md until `verification-strategy`), without changing the plan format; verify `TestInitializeWritesConfigAndSkills` passes

## 4. Documentation

- [x] 4.1 Make `npm install` → `npx stele init` → `stele-propose` the default path in the README, getting started, and Build a Todo; add the "Using Stele with OpenSpec" guide: the credit "Stele currently builds on OpenSpec (MIT, Fission AI)", extension through schemas, `openspec-verify-change` after `stele validate`, explore and sync-specs, moving an existing change, and the fallback table. Link it from the navigation; verify `npm run docs:build` passes
- [x] 4.2 Update the CLI reference (`stele ids`, `init` options), and the changelog; verify `npm run docs:build` passes

## 5. Gate and dogfooding

- [x] 5.1 Add anchors for this change's planned targets and run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- validate --change authoring-workflow` passes with the local build
- [x] 5.2 Run `stele init` in this repository with the local build; verify the lifecycle skills, the `stele` schema, and the configuration guidance are installed, `.claude/skills` still links to `.agents/skills`, and a scratch `openspec new change` uses the `stele` schema
- [x] 5.3 After a release that contains this change, use the published `stele ids` instead of the prototype, archive the change, and verify `npm run verify:self` passes
- [ ] 5.4 Delete the `feat/stele-ids` prototype branch once the maintainer consents
