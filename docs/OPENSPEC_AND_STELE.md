# How OpenSpec and Stele work together

## Purpose

The goal is to describe product behavior in plain English, plan how it will be implemented, and deterministically check that the plan is connected to real code and executed tests.

OpenSpec and Stele have separate responsibilities:

| Component | Responsibility |
|---|---|
| OpenSpec | The authoritative requirements, scenarios, proposals, designs, tasks, and change lifecycle |
| Stele | Stable behavioral IDs, code and test linkage, exact scenario execution, deterministic diagnostics, and evidence reports |
| Product tests | Evidence that selected examples of the behavior executed and passed |
| Human reviewer | Judgment that the implementation and tests adequately satisfy the intended behavior |
| Devin | Reads the plan, implements the change, runs the guardrail, and returns the resulting evidence |

OpenSpec remains the only specification canon. Stele does not create a second copy of the requirements.

## The development loop

```text
Plain-English product intent
            ↓
OpenSpec requirement and scenarios
            ↓
Stable requirement and scenario IDs
            ↓
Planned code declaration and named test
            ↓
Implementation with @implements anchors
Tests with @verifies anchors
            ↓
Stele deterministic validation
            ↓
Revision-bound JSON evidence
            ↓
Dashboard and human review
```

### 1. Describe the behavior in OpenSpec

A requirement states what the system must do. Scenarios describe concrete outcomes in plain English.

```markdown
### Requirement: Create a task
Verification-ID: req.todo.c6148d2fa790

The application SHALL create a task from valid text.

#### Scenario: Add a valid task
Verification-ID: scn.todo.f98c1437a6d2

- **WHEN** the user submits valid task text
- **THEN** an open task is created
```

The IDs remain stable when wording or file locations change. They identify behavior rather than headings, tickets, functions, or test-framework names.

### 2. Plan implementation and validation

Before coding, a small linkage plan records where each requirement should be enforced and which named test should verify each scenario.

```json
{
  "requirements": {
    "req.todo.c6148d2fa790": "src/todo-store.mjs#create"
  },
  "scenarios": {
    "scn.todo.f98c1437a6d2": "tests/todo-store.test.mjs#creates a trimmed open task"
  }
}
```

Proposal validation permits planned files that do not exist yet. This allows plan review before implementation begins.

### 3. Link the real implementation

The developer or agent attaches the requirement ID to the declaration that enforces it:

```js
// @implements req.todo.c6148d2fa790
export function create(title) {
  // implementation
}
```

The scenario ID is attached to an independently selectable named test:

```js
// @verifies scn.todo.f98c1437a6d2
test("creates a trimmed open task", () => {
  // assertions
});
```

Stele rejects unknown IDs, wrong anchor types, missing declarations, ambiguous selectors, and anchors that do not match the planned file and symbol.

### 4. Execute and report deterministically

```bash
npm run verify:proposal
npm run validate
npm run stele -- validate --json
```

Full validation performs three operations:

1. Run OpenSpec strict validation.
2. Select and execute every anchored named scenario test independently.
3. Resolve every requirement and scenario anchor against the implementation plan.

For identical relevant inputs, Stele produces byte-for-byte identical JSON and stable exit codes:

- `0` means the selected checks passed.
- `1` means a deterministic policy or test failed.
- `2` means the invocation or tool failed.

The report keeps proposal, linkage, execution, recordings, and human review as separate evidence dimensions.

## How Devin fits

Devin does not require a custom integration service for the first pilot. The repository supplies the operating contract:

1. Read the selected OpenSpec change and repository instructions.
2. Run proposal validation before implementation.
3. Implement the planned declarations and named tests with anchors.
4. Run full validation until it passes.
5. Return the deterministic report and any browser recordings for review.

A minimal Devin setup needs:

- OpenSpec and Stele Spec installed in the repository;
- an `AGENTS.md` or equivalent instruction file;
- one setup command;
- one proposal-check command;
- one full validation command;
- CI storage for the JSON report and optional recordings.

The verifier should remain local and provider-independent. Replacing Devin with another agent runner should not change the specifications or evidence format.

## What is already proven here

The current Todo showcase demonstrates:

- OpenSpec as the single behavioral canon;
- stable requirement and scenario IDs;
- proposal-stage planned targets;
- declaration-aware code and test anchors;
- independently executed scenario tests;
- deterministic JSON and exit codes;
- revision or dirty-tree input binding;
- a dashboard showing requirements, code, tests, results, artifacts, and recordings together.

## What remains before a real pilot

The next release should focus on a small reusable package rather than a larger platform:

1. Extract the repository verifier into an installable `stele-spec` package while keeping the `stele` command.
2. Replace the hard-coded example change with project configuration and change selection.
3. Verify OpenSpec add, modify, rename, remove, sync, and archive round trips.
4. Support configurable source roots and the first real project test runner.
5. Publish the deterministic report in CI and connect it to the review workflow.
6. Pilot one bounded feature in a Devin-managed repository.

Language-aware symbol resolution, mutation testing, multiple test-runner adapters, historical identity transitions, and broader dashboards can follow after the pilot proves value.

## What deterministic verification does and does not prove

Stele proves that declared behavior has stable identity, planned implementation and test targets, resolvable anchors, and exact tests that executed with a known result for the current inputs.

It does not mathematically prove that the implementation fully captures the meaning of the prose. Test quality and semantic adequacy still require human review, with deeper analysis added later where it provides measurable value.
