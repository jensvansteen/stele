## Context

See proposal.md. Today `RunVerification` reads `artifacts/linkage-plan.json` for every change and ignores its `changeId`. `validateLinkage` knows only the selected change's identities, so every other anchor is dangling. `ParseSpecs` reads only `openspec/changes/<change>/specs`. `runOpenSpec` validates only a single change.

## Goals / Non-Goals

**Goals:**

- Verify any active change in a repository that has several.
- Keep archived behavior verifiable after `openspec archive`.

**Non-Goals:**

- Verifying several active changes in one run.
- Changing the linkage plan schema; multiple evidence entries and approvals belong to `verification-strategy`.
- Anchors that name retired identities. A retired identity is one that no specification declares any more; its anchors stay dangling, which is the desired signal.

## Decisions

### A verification scope

The CLI resolves a scope before running: either a change (from `--change` or the configuration) or the current specifications (`--specs`). The scope provides the spec files to parse, the linkage plan, and the OpenSpec validation arguments. `--specs` together with `--change` is an invocation error. When `--specs` is given, the configured default change is ignored.

### Plan location and precedence

For a change scope, the plan is `openspec/changes/<change>/linkage-plan.json`, falling back to `artifacts/linkage-plan.json`. Keeping the plan inside the change directory lets `openspec archive` carry it into `openspec/changes/archive/<date>-<change>/`. A plan whose `changeId` differs from the selected change produces `PLAN_CHANGE_MISMATCH` and no linkage from that plan. Alternative considered: one shared plan keyed by change. It does not move with archives, and it hides which change owns an entry.

For the specifications scope, the plan combines every `linkage-plan.json` under `openspec/changes/archive/`, in ascending directory-name order, and later entries replace earlier ones for the same identity. OpenSpec prefixes archive directories with the archive date, so this order is chronological and deterministic. The combined plan has no `changeId`, and no mismatch check applies.

### Identity universe for dangling anchors

`ANCHOR_DANGLING` checks against every identity declared in any Markdown file under `openspec/changes/` (archived changes included) and `openspec/specs/`. Only anchors for identities in the selected scope take part in linkage, kind, and target checks. Alternative considered: ignoring every out-of-scope anchor. That would hide typos that happen to match no identity.

### Evidence and validation per scope

`stele test --specs` runs the tests linked to scenarios in `openspec/specs/`. `stele validate --specs` runs `openspec validate --specs --strict --no-interactive`. Evidence and report formats are unchanged; the report's `openspec.changeId` is empty for the specifications scope.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-16, before implementation started. One scenario has a second level, for a distinct risk.

Levels, defined by what the test reaches: **unit** calls code directly, in process, possibly with temporary files or stubbed dependencies. **integration** exercises our code together with one real outside tool, such as Node, OpenSpec, or the Go toolchain. **e2e** uses the real product through its user-facing entry point: the UI for applications, and the installed executable or packed package for command-line tools such as Stele.

Evidence IDs extend the scenario's single behavior ID with the level, for example `scn.todo.591a3b429cf0.e2e`. A second entry at the same level adds a number, as in `.e2e.2`. The spec keeps only the behavior ID. Anchors name the evidence ID, as in `// @verifies scn.todo.591a3b429cf0.e2e`. The published verifier reads such an anchor as the plain scenario ID, and the v1 linkage plan stays keyed by that ID until `verification-strategy` adds per-entry plans.

| Scenario | Level | Evidence ID | Boundary | Target | Risk and why this level is the lowest convincing one | Checkable now |
|---|---|---|---|---|---|---|
| `scn.verificationscope.20fde975089b` Read the plan stored with the change | unit ★ | `scn.verificationscope.20fde975089b.unit` | `RunVerification` on fixtures | `internal/stele/scope_test.go#TestRunVerificationPrefersChangePlan` | Risk: the shared plan silently overrides the change's own plan. File precedence is deterministic. | No, Go |
| `scn.verificationscope.d477fd8e9980` Fall back to the shared plan | unit ★ | `scn.verificationscope.d477fd8e9980.unit` | `RunVerification` on fixtures | `internal/stele/scope_test.go#TestRunVerificationFallsBackToSharedPlan` | Risk: existing consumers break. Pure precedence. | No, Go |
| `scn.verificationscope.784db68ce025` Reject a plan for another change | unit ★ | `scn.verificationscope.784db68ce025.unit` | `RunVerification` on fixtures | `internal/stele/scope_test.go#TestRunVerificationRejectsPlanForOtherChange` | Risk: a copied plan vouches for the wrong change. Pure check. | No, Go |
| `scn.verificationscope.ee5d7b261d62` Ignore anchors of another active change | unit ★ | `scn.verificationscope.ee5d7b261d62.unit` | `RunVerification` on fixtures | `internal/stele/scope_test.go#TestRunVerificationIgnoresOtherChangeAnchors` | Risk: several active changes cannot pass together, which is this repository's situation today. Pure. | No, Go |
| `scn.verificationscope.101e06b07f39` Report an anchor that no specification declares | unit ★ | `scn.verificationscope.101e06b07f39.unit` | `RunVerification` on fixtures | `internal/stele/scope_test.go#TestRunVerificationReportsUndeclaredAnchors` | Risk: widening the identity universe hides real typos. Pure. | No, Go |
| `scn.verificationscope.8b577e9712e7` Verify archived behavior | unit ★ | `scn.verificationscope.8b577e9712e7.unit` | `RunVerification` on an archived fixture | `internal/stele/scope_test.go#TestRunVerificationVerifiesCurrentSpecs` | Risk: archived behavior loses its evidence. Spec and plan loading is deterministic over files. | No, Go |
| | e2e | `scn.verificationscope.8b577e9712e7.e2e` | built `dist/stele` after a real `openspec archive` | `tests/cli.test.mts#verifies current specifications after archiving` | Second level for a distinct risk: OpenSpec's real archive layout, such as the directory naming and merged spec text, differs from the fixture. Only the real tool shows that. | Resolves once `fix-typescript-anchor-scanning` ships |
| `scn.verificationscope.70c3ee060846` Prefer the most recent archived plan | unit ★ | `scn.verificationscope.70c3ee060846.unit` | plan combination | `internal/stele/scope_test.go#TestCombinedArchivePlanPrefersLatest` | Risk: nondeterministic or oldest-wins targets. Ordering is pure. | No, Go |
| `scn.verificationscope.f8ab6e2e1812` Run OpenSpec validation for the current specifications | integration ★ | `scn.verificationscope.f8ab6e2e1812.integration` | real bundled OpenSpec CLI | `internal/stele/scope_test.go#TestRunOpenSpecValidatesCurrentSpecs` | Risk: wrong OpenSpec arguments, which a stub cannot catch. Needs the real CLI; the Stele binary is not needed. | No, Go |
| `scn.verificationscope.79a83cf82aba` Reject a conflicting selection | unit ★ | `scn.verificationscope.79a83cf82aba.unit` | CLI `Run` | `internal/stele/scope_test.go#TestRunRejectsSpecsWithChange` | Risk: an ambiguous scope runs silently. `Run` returns the exit code in process. | No, Go |

Requirement implementation targets:

| Requirement | Target |
|---|---|
| `req.verificationscope.713ceb31377c` Use the selected change's own linkage plan | `internal/stele/scope.go#loadChangePlan` |
| `req.verificationscope.9c81618619df` Scope anchors to declared identities | `internal/stele/scope.go#declaredIdentities` |
| `req.verificationscope.bee89d6750ed` Verify the current specifications | `internal/stele/scope.go#resolveScope` |

## Risks / Trade-offs

- [A shared plan that names another change is now rejected] → Marked BREAKING. The error names both change IDs, and the changelog explains the move to per-change plans.
- [Scanning every spec under `openspec/` on each run costs time in large repositories] → Only headings and ID lines are parsed; this is linear and small next to test execution.
- [This repository's own plan must move once a release contains this change] → A task moves each change's entries into its directory, then archives `baseline-verification-core` and verifies it with `--specs`.

## Migration Plan

1. Release this change.
2. In this repository, split `artifacts/linkage-plan.json` into one `linkage-plan.json` per change directory, and delete the shared file.
3. Archive `baseline-verification-core` and verify it with `npm run stele:published -- verify --specs`.
