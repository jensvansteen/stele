# Stele Spec implementation report

Date: 2026-09-14

## Outcome

Stele Spec now contains a working Todo product, a live artifact dashboard, an OpenSpec change, and an executable `stele` CLI. OpenSpec remains the only behavioral canon. Requirement IDs connect specs to named code declarations; scenario IDs connect specs to exact named test executions.

## What works

- Todo list, create, blank-input validation, completion/reopen, filters, and deletion.
- Artifact dashboard with proposal, design, tasks, three delta specs, verification report, test run, and framework plan.
- Expandable matrix for 17 requirements and 26 scenarios.
- Two playable end-to-end browser recordings with covered scenario IDs, same-origin range streaming, and private tailnet links.
- Proposal mode for validating IDs and planned targets before implementation exists.
- Implementation mode that resolves each actual `@implements` and `@verifies` anchor to a nearby declaration and its planned file/selector.
- Repository-owned `stele verify`, `stele test`, and `stele validate` commands with stable exit codes.
- Scenario-specific execution that requires TAP proof that each exact named test ran and passed.
- Byte-for-byte deterministic JSON for identical relevant inputs; volatile timestamps are excluded.
- Independent proposal, linkage, execution, pass/fail, and human-review states.
- A local pre-commit hook that runs implementation verification when dependencies are installed.
- OpenSpec 1.13.0 pinned in the lockfile and strict OpenSpec validation.

## Validation result

`npm run validate` completed successfully:

- OpenSpec strict validation: passed.
- Automated framework/product scenarios: 26 passed, 0 failed.
- Stable identity: 17 requirements and 26 scenarios parsed, 0 errors.
- Linkage: all 17 requirements and all 26 scenarios linked to planned declarations.
- Execution: all 26 scenarios were selected and passed independently for the current input digest.
- Review: intentionally `not-reviewed`; the verifier never invents human approval.

The generated machine-readable outputs are `artifacts/proposal-report.json`, `artifacts/test-results.json`, and `artifacts/verification-report.json`. The implementation report uses schema 2.0 and records the verifier version, OpenSpec version, revision state, input digest, links, execution state, diagnostics, and review state.

## Deterministic CLI verification

The executable lives at `bin/stele.mjs` and is exposed through the package `bin` field. Two consecutive `stele verify --json` invocations produce identical bytes when relevant inputs do not change. Policy/test failures return exit code 1; invalid commands return exit code 2. Tests cover both paths.

The scenario runner also has an explicit negative fixture. A deliberately failing anchored test is mapped to its scenario as `failed`; a process that exits successfully without TAP evidence for the exact selected test is reported as `test-not-executed`.

The earlier Simdock checkout used Stele-style specs and anchors, but its hook skipped verification when a global `stele` command was absent. This showcase closes that gap with a versioned repository executable and npm entry points.

## Browser walkthrough

The running application was exercised in a real browser at `http://localhost:4173`:

1. Created “Review verification report”.
2. Marked it complete and confirmed the remaining count changed.
3. Filtered the workspace to completed tasks.
4. Opened the Verification dashboard.
5. Confirmed Proposal = pass, Linkage = pass, Execution = passed, Review = not-reviewed.
6. Inspected the expandable evidence matrix and proposal artifact.
7. Ran implementation validation from the dashboard and observed `Validation pass: 0 errors`.

Deletion is covered through the Todo store and HTTP integration tests.

## Responsive UI audit

The interface was re-checked in the built-in browser at 1280×720 and 390×844 after applying the frontend-design and web-interface guideline review:

- Created “Browser verification pass”, marked it complete, and confirmed the Done filter showed it.
- Ran validation from the dashboard and observed `Validation pass: 0 errors`.
- Selected the schema-2 verification report from the artifact explorer and confirmed its 17 requirements, 26 scenarios, and pass state.
- Moved from `verification-report` to `test-results` with the Right Arrow key and confirmed the selected tab and content changed together.
- Confirmed every artifact tab remains visible through wrapping at desktop and mobile widths.
- Confirmed no page-level horizontal overflow, no unlabeled buttons, and no browser console warnings or errors.

## End-to-end recording evidence

The Verification view embeds two browser recordings directly inside the expanded requirements whose scenarios they cover: a Todo create/complete/filter journey and a dashboard traceability/artifact/validation journey. Manifest scenario IDs determine placement, and a deterministic mapping test rejects unrelated requirement associations. Both MP4s are stored outside the repository under `~/recordings`, registered through `artifacts/e2e-recordings.json`, and streamed through an allowlisted same-origin endpoint with HTTP byte-range support. A real Chromium playback check confirmed inline placement, finite video duration, successful `206` range responses, removal of the separate gallery, and no page-level horizontal overflow.

The UI now includes semantic hash links, a skip link, visible keyboard focus, tab-list semantics and arrow-key navigation, pressed states for Todo filters, reduced-motion handling, and visible delete controls on touch devices.

## Product and dashboard separation

The final product check removed OpenSpec copy, active-change metadata, verification calls to action, and verification-themed seed tasks from the Todo workspace. The Todo surface now presents only an ordinary task workflow with generic examples. The separate Verification view retains the traceability matrix and directly embeds the proposal, design, tasks, capability specs, verification report, scenario evidence, and framework plan.

## Reproduce end to end

```bash
npm install
npm run validate
npm run dev
```

Open `http://localhost:4173`, exercise the Todo workspace, then open **Verification** and click **Run validation**. For deterministic CLI output, run `npm run stele -- verify --json` twice and compare the files as shown in the README. For the proposal stage alone, run `npm run verify:proposal`. For OpenSpec independently, run `npm run openspec:validate`.

## Deliberate limits

Todo data is session-only and resets when the server restarts. The dashboard is local and does not implement PR labels, cloud runners, or Daytona. Test execution evidence is bound to a digest of the relevant inputs because this initial repository has no commit yet. Declaration adjacency and exact test execution provide deterministic structural and behavioral evidence; they are not formal proof that a linked function semantically satisfies the prose. Semantic review and human approval remain separate inputs.
