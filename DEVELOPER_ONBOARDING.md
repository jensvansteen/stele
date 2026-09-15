# Developer onboarding: Stele Spec

Stele Spec demonstrates a simple division of responsibility:

- **OpenSpec is the behavioral canon.** Requirements and scenarios live in `openspec/changes/<change>/specs/`.
- **Stele is the deterministic verification layer.** It checks stable IDs, code and test anchors, planned targets, and scenario execution. It emits machine-readable JSON for CI and the dashboard.
- **Tests and reviewers remain distinct evidence.** A resolved link, a passing test, and human approval are reported separately.

## Purpose

The goal is to make a codebase understandable and governable in plain English. A developer first describes the behavior people need, then plans where that behavior will be implemented and how it will be tested. Deterministic checks connect that plan to the actual code and test run so missing, misplaced, or unexecuted work cannot silently appear complete.

The intended development loop is:

```text
plain-English behavior
        ↓
OpenSpec requirement and scenarios with stable IDs
        ↓
planned code declaration and named test
        ↓
implementation and test anchors
        ↓
exact scenario execution + deterministic linkage checks
        ↓
JSON evidence, dashboard, and human review
```

## How it works

1. **Describe the behavior.** OpenSpec requirements explain what the product must do. Their scenarios give concrete inputs, actions, and expected outcomes. This is the single source of behavioral truth.
2. **Give behavior stable identity.** Each requirement receives a `req.*` ID and each scenario receives a `scn.*` ID. The wording and file location can change without breaking traceability.
3. **Plan the implementation.** `artifacts/linkage-plan.json` records the expected source declaration for each requirement and the exact named test for each scenario before coding starts.
4. **Link the real code.** `@implements` anchors sit beside relevant functions, classes, or methods. `@verifies` anchors sit beside named tests. An ID appearing in an arbitrary comment is insufficient.
5. **Check deterministically.** Stele parses the OpenSpec change, validates identity uniqueness and type, resolves anchors to nearby declarations, and confirms each file and selector matches the plan.
6. **Execute every scenario.** The runner invokes every anchored named test independently. A successful process is accepted only when TAP output proves that the exact selected test ran and passed; skipped or unmatched tests cannot count as evidence.
7. **Bind and publish evidence.** The verifier records the Git revision or dirty-tree input digest and emits stable JSON with separate proposal, linkage, execution, and review states. The dashboard renders this same report rather than calculating a second result.

For unchanged relevant inputs, the same command produces byte-for-byte identical verification JSON and stable exit codes. This makes the guardrail suitable for local development, pre-commit checks, and CI.

The checks establish that documented behavior has a planned and resolved implementation location and that its named scenarios executed successfully. They do not mathematically prove that the code fully matches the meaning of the prose. Human review remains explicit, and deeper assurance can later add language-aware analysis, mutation testing, or other semantic checks without weakening the deterministic baseline.

## Set up and run the example

Use Node.js 20.19 or newer.

```bash
npm install
npm run validate
npm run dev
```

Open <http://localhost:4173>. The default view is a plain Todo app. Select **Verification** to inspect the OpenSpec artifacts, requirement-to-code links, scenario-to-test links, execution results, and recordings embedded inside the requirements whose scenarios they cover. **Run validation** refreshes the implementation report used by the dashboard.

## Develop a change

1. Create or select a change under `openspec/changes/` and write the proposal, design, tasks, and delta specs.
2. Give each requirement a stable `req.<namespace>.<token>` ID and each scenario a stable `scn.<namespace>.<token>` ID. Keep IDs when wording or file locations change; never reuse retired IDs.
3. Add the planned code and test targets to `artifacts/linkage-plan.json`.
4. Before implementation, run:

   ```bash
   npm run verify:proposal
   ```

5. Put a requirement anchor beside the declaration that enforces the behavior:

   ```js
   // @implements req.todo.example123
   export function createTodo(title) {}
   ```

6. Put a scenario anchor beside its independently selectable named test:

   ```js
   // @verifies scn.todo.example456
   test("creates a todo", () => {});
   ```

7. Run the full guardrail before review:

   ```bash
   npm run validate
   ```

`validate` runs strict OpenSpec validation, executes every anchored scenario by exact test name, and verifies the implementation links. Exit code `0` is a pass, `1` is a policy or test failure, and `2` is an invalid invocation or tool failure.

## Useful checks

```bash
# OpenSpec only
npm run openspec:validate

# Linkage without running scenarios
npm run verify

# Scenario evidence only
npm run test:scenarios

# Stable JSON for CI or another tool
npm run stele -- validate --json

# Confirm deterministic output for unchanged inputs
npm run stele -- verify --json > /tmp/stele-a.json
npm run stele -- verify --json > /tmp/stele-b.json
cmp /tmp/stele-a.json /tmp/stele-b.json
```

Generated evidence is written to:

- `artifacts/proposal-report.json`
- `artifacts/test-results.json`
- `artifacts/verification-report.json`
- `artifacts/e2e-recordings.json`

The reports deliberately distinguish `planned`, `linked`, `executed`, `passed`, and `not-reviewed`. Stele verifies traceability and execution evidence; it does not infer that the implementation semantically satisfies the prose or that a human approved it.

## Where to start reading

- `openspec/changes/todo-showcase/` — the complete example change
- `artifacts/linkage-plan.json` — expected source and test targets
- `bin/stele.mjs` — CLI entry point and exit-code contract
- `tools/verify.mjs` — OpenSpec adapter and deterministic verifier
- `tools/run-scenarios.mjs` — exact scenario test execution
- `server.mjs` and `public/` — Todo app and artifact dashboard
- `PLAN.md` — framework boundary and longer-term direction

## Current limitations

Stele is installable from a locally produced npm tarball and initializes consumer-owned configuration plus repository-local planning and verification skills. The original Todo data is held in memory, and review remains `not-reviewed` until an external human-review source is integrated. End-to-end MP4s live under `~/recordings`; the Git-tracked manifest supplies their titles, covered scenarios, allowlisted relative paths, and private tailnet URLs. The verifier uses declaration adjacency and exact selectors for deterministic structural evidence; richer language-aware semantic analysis is future work.
