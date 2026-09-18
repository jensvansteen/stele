## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels, the decided questions, and the open question in design.md, and record it in design.md before starting section 1
- [x] 0.2 Confirm `verification-strategy` is implemented before starting section 1

## 1. Index

- [x] 1.1 Keep requirement body text, and scenario raw text and structured steps (including multi-line bullets and missing or malformed text), on the parsed specs; verify `TestParseSpecsKeepsRequirementTextAndScenarioSteps` passes
- [x] 1.2 Build the index with requirements, scenarios, anchors, evidence, approval, and anchor status; verify `TestIndexListsLinksForAChange`, `TestIndexIncludesCurrentSpecifications`, and `TestIndexFlagsUndeclaredAnchors` pass
- [x] 1.3 Move evidence to schema 3 with per-execution digests and mark stale outcomes; verify `TestIndexMarksStaleOutcomes` passes
- [x] 1.4 Add `stele index`; verify `prints a link index for editors` and `emits an identical link index for identical inputs` pass in `npm run test:node`

## 2. Running one behavior

- [x] 2.1 Add positional targets to `stele test` (requirement, scenario, evidence, and specification file) with evidence merging; verify `TestRunSelectedScenarioMergesEvidence`, `TestRunSelectedEvidenceOnly`, `TestRunSelectedRequirementScenarios`, `TestRunSelectedSpecFileScenarios`, `TestRunSelectedTargetsCombine`, and `TestTestRejectsUnknownTargets` pass
- [x] 2.2 Leave interpolated TypeScript test names unresolved; verify `TestScanAnchorsLeavesParameterizedTestsUnresolved` passes

## 2b. Scopes and output files

- [x] 2b.1 Add `--all` to `test`, `verify`, `validate`, and `index`, reporting each scope separately with one combined exit code; verify `TestAllScopesReportSeparately`, `TestIndexCoversEveryScope`, `TestAllRejectsConflictingScopes`, and `checks every scope with --all` pass
- [x] 2b.2 Add `--evidence-file`, `--report-file`, and `--output-file`, keeping `--evidence` and `--report` as deprecated aliases with a warning; verify `TestOutputFileFlags` and `TestDeprecatedOutputFlagsWarn` pass

## 3. Verdicts

- [x] 3.1 Add `verdicts` to reports and validation output, redefine `verdict` as overall with a changelog breaking-change note, and add the execution line to human verify output; verify `TestReportSeparatesExecutionVerdict` and `TestReportMarksMissingEvidenceIncomplete` pass

## 4. Documentation and gate

- [x] 4.1 Document `stele index`, its schema for editor authors, positional test targets, `--all`, the new and deprecated file flags, evidence schema 3, and report verdicts in the CLI reference and changelog; verify `npm run docs:build` passes
- [x] 4.2 Add anchors for this change's planned targets and run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- validate --change link-index` passes with the local build
- [x] 4.3 After a release that contains this change, archive it and verify `npm run verify:self` passes
