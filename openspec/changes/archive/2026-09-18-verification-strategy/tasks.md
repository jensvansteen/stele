## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels, the proposed answers, and the open questions in design.md, and record it in design.md before starting section 1
- [x] 0.2 Confirm `authoring-workflow` is implemented before starting section 1

## 1. Evidence anchors and plan schema

- [x] 1.1 Keep the full annotated ID in TypeScript and Go anchors and split it into base identity, evidence ID, and level; verify the existing anchor tests pass and new cases cover `.unit`, `.e2e.2`, and bare IDs
- [x] 1.2 Parse v1 and v2 plans, including advisory placement, and validate v2 evidence entries; verify `TestProposalAcceptsApprovedEvidencePlan`, `TestProposalRejectsMalformedEvidence`, `TestProposalRequiresEvidencePerScenario`, and `TestProposalRejectsUnknownPlanIDs` pass
- [x] 1.3 Add the `PLAN_V1_DEPRECATED` warning naming 0.2.0; verify `TestV1PlanStillVerifiesWithWarning` and the existing v1 tests pass

## 2. Approvals

- [x] 2.1 Compute whitespace-normalized digests and report unapproved and stale entries in both stages; verify `TestProposalReportsUnapprovedEntries` and `TestProposalReportsStaleApprovals` pass
- [x] 2.2 Add `stele approve` with the interactive review (injected terminal), `--all --yes`, `--confirmed-in-chat`, `--by`, and the refusal without confirmation; verify `TestApproveReviewsEntriesInteractively`, `TestApproveRecordsConversationConfirmation`, and `TestApproveRefusesWithoutConfirmation` pass

## 3. Evidence verification and execution

- [x] 3.1 Add the v2 implementation rules; verify `TestImplementationAcceptsEvidenceAnywhere`, `TestImplementationReportsMissingEvidence`, `TestImplementationRejectsUnplannedEvidence`, and `TestV2VerificationIgnoresLocation` pass
- [x] 3.2 Record evidence IDs and levels in execution evidence and reports; verify `TestScenarioFailsWhenOneEvidenceFails` passes
- [x] 3.3 Add the end-to-end workflow test; verify `runs the approved evidence workflow end to end` passes in `npm run test:node`

## 4. Migration and guidance

- [x] 4.1 Add `stele plan migrate`; verify `TestPlanMigrateConvertsV1Plans` passes
- [x] 4.2 Rewrite `stele-plan` (levels, advisory placement, v2 format, conversational approval step) and `stele-verify` (evidence rules); verify `TestPlanningSkillContainsStrategyGuidance`, `TestPlanningSkillDescribesConversationalApproval`, and `TestInitializeKeepsProjectSkills` pass
- [x] 4.3 Document level definitions, v2 plans, the approval flow (conversation, terminal review, `--confirmed-in-chat`, optional PR review of plan files), `plan migrate`, and advisory placement in the CLI reference, IDs and anchors, test levels, the OpenSpec guide, and the changelog; verify `npm run docs:build` passes
- [x] 4.4 Record the product vision ("Stele becomes your code in English": links for verification and for review and navigation, placement left to project conventions) in `docs/intent/`, and link it from the documentation index; verify `npm run docs:build` passes

## 5. Gate and dogfooding

- [x] 5.1 Add anchors for this change's planned targets and run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- validate --change verification-strategy` passes with the local build
- [ ] 5.2 After a release that contains this change, migrate this repository's plans to v2, have the maintainer approve them, update the self-verification docs, and verify `npm run verify:self` passes
- [x] 5.3 Archive the change after 5.2, and verify `npm run verify:self` passes
