## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels in design.md, and record it in design.md before starting section 1

## 1. Scope resolution

- [x] 1.1 Add `--specs`, reject it together with `--change`, and resolve a change or specifications scope; verify `TestRunRejectsSpecsWithChange` passes
- [x] 1.2 Load the change plan with fallback and the mismatch check; verify `TestRunVerificationPrefersChangePlan`, `TestRunVerificationFallsBackToSharedPlan`, and `TestRunVerificationRejectsPlanForOtherChange` pass
- [x] 1.3 Combine archived plans for the specifications scope; verify `TestCombinedArchivePlanPrefersLatest` passes

## 2. Verification, execution, and validation

- [x] 2.1 Check dangling anchors against every declared identity under `openspec/`; verify `TestRunVerificationIgnoresOtherChangeAnchors` and `TestRunVerificationReportsUndeclaredAnchors` pass
- [x] 2.2 Parse and verify `openspec/specs/` for the specifications scope in `verify`, `test`, and `validate`; verify `TestRunVerificationVerifiesCurrentSpecs` passes
- [x] 2.3 Run `openspec validate --specs --strict --no-interactive` for the specifications scope; verify `TestRunOpenSpecValidatesCurrentSpecs` passes
- [x] 2.4 Add the archive end-to-end test; verify `verifies current specifications after archiving` passes in `npm run test:node`

## 3. Documentation, gate, and migration

- [x] 3.1 Document `--specs`, per-change plans, and the dangling-anchor rule in the CLI reference, getting started, the Build a Todo guide, and the changelog; verify `npm run docs:build` passes
- [x] 3.2 Add anchors for this change's planned targets and run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- verify --change verification-scope` passes with the local build
- [ ] 3.3 After a release that contains this change, split this repository's plan into per-change plans, archive `baseline-verification-core`, and verify `npm run stele:published -- verify --specs` passes
