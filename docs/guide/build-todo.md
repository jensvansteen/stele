# Build a Todo feature with OpenSpec and Stele

This walkthrough starts with a behavior idea, plans it in OpenSpec, adds Stele's deterministic verification plan, and only then writes the TypeScript implementation and test. It covers one small Todo behavior so every artifact remains easy to inspect.

For a shortcut to the finished browser application and evidence files, use [Inspect the finished Todo example](/guide/inspect-example).

## 1. Create the project and OpenSpec change

Start in a new TypeScript project:

```bash
npm init -y
npm install --save-dev stele-spec@next
npx openspec init .
npx openspec new change todo-basics
```

OpenSpec now owns the planning workflow under `openspec/changes/todo-basics/`. Do not write implementation code yet.

## 2. Plan the behavior with OpenSpec

Ask your coding agent to create the OpenSpec artifacts from a plain-English request:

```text
Use OpenSpec to plan the change `todo-basics` for a small TypeScript Todo app.
The first capability lets a user add a todo from non-empty text. Surrounding
whitespace is removed, and blank text is rejected. Create the proposal, delta
specification, design, and implementation tasks. Do not implement the feature yet.
```

You can also write the Markdown yourself. Either path should produce these artifacts:

| Artifact | Question it answers |
|---|---|
| `proposal.md` | Why are we changing the product, and what is in scope? |
| `specs/todo/spec.md` | What behavior must the product provide? |
| `design.md` | How will the implementation satisfy that behavior? |
| `tasks.md` | What work must be completed? |

Review the generated files before continuing. The delta specification should contain behavior like this:

```markdown
## Purpose

Provide the basic creation behavior for a small Todo application.

## ADDED Requirements

### Requirement: Create a todo

The application SHALL create an open todo from non-empty text.

#### Scenario: Add non-empty todo

- **WHEN** the user submits non-empty todo text
- **THEN** an open todo is added with surrounding whitespace removed

#### Scenario: Reject blank todo

- **WHEN** the user submits blank or whitespace-only text
- **THEN** no todo is created and validation fails
```

Check the OpenSpec plan on its own:

```bash
npx openspec validate todo-basics --strict --no-interactive
```

Fix the proposal or specifications until OpenSpec accepts the change. At this point OpenSpec proves the plan has a valid shape; no implementation evidence exists yet.

## 3. Add Stele to the planned change

Initialize Stele after the OpenSpec change exists:

```bash
npx stele init --change todo-basics
```

This creates `stele.config.json`, installs the repository-local planning and verification skills, and remembers `todo-basics` as the default change. It does not rewrite the OpenSpec artifacts.

## 4. Give the behavior stable identities

Add one requirement ID and one ID to each scenario in `specs/todo/spec.md`:

```markdown
### Requirement: Create a todo
Verification-ID: req.todo.1a2b3c4d5e6f

The application SHALL create an open todo from non-empty text.

#### Scenario: Add non-empty todo
Verification-ID: scn.todo.0a1b2c3d4e5f

- **WHEN** the user submits non-empty todo text
- **THEN** an open todo is added with surrounding whitespace removed

#### Scenario: Reject blank todo
Verification-ID: scn.todo.1b2c3d4e5f60

- **WHEN** the user submits blank or whitespace-only text
- **THEN** no todo is created and validation fails
```

These identities survive wording and file changes. They let Stele join the specification, implementation, tests, and evidence without guessing from titles.

## 5. Plan how each behavior will be proven

Before implementation, add the verification strategy to `design.md`:

```markdown
## Verification strategy

| Scenario | Level | Boundary | Rationale |
|---|---|---|---|
| `scn.todo.0a1b2c3d4e5f` | unit | pure Todo creation | No I/O is needed to prove normalization. |
| `scn.todo.1b2c3d4e5f60` | unit | pure Todo creation | Blank-input rejection is deterministic logic. |
```

Then create `artifacts/linkage-plan.json` with the exact declarations and tests you intend to write:

```json
{
  "schemaVersion": 1,
  "changeId": "todo-basics",
  "requirements": {
    "req.todo.1a2b3c4d5e6f": "src/todo.mts#addTodo"
  },
  "scenarios": {
    "scn.todo.0a1b2c3d4e5f": "tests/todo.test.mts#adds a normalized todo",
    "scn.todo.1b2c3d4e5f60": "tests/todo.test.mts#rejects blank todo text"
  }
}
```

Verify the proposal stage:

```bash
npx stele verify --stage proposal --json
```

This stage accepts targets that do not exist yet. It fails when an OpenSpec identity is missing from the plan or the target is malformed.

## 6. Implement the planned declaration

Create `src/todo.mts`:

```typescript
export interface Todo {
  readonly title: string;
  readonly completed: boolean;
}

// @implements req.todo.1a2b3c4d5e6f
export function addTodo(input: string): Todo {
  const title = input.trim();
  if (title.length === 0) {
    throw new Error("Todo title is required");
  }
  return { title, completed: false };
}
```

The implementation anchor answers where the requirement is owned. It supports navigation and change-impact review; it is not behavioral proof by itself.

## 7. Write independently selectable tests

Create `tests/todo.test.mts`:

```typescript
import assert from "node:assert/strict";
import test from "node:test";
import { addTodo } from "../src/todo.mts";

// @verifies scn.todo.0a1b2c3d4e5f
void test("adds a normalized todo", (): void => {
  assert.deepEqual(addTodo("  Ship it  "), {
    title: "Ship it",
    completed: false,
  });
});

// @verifies scn.todo.1b2c3d4e5f60
void test("rejects blank todo text", (): void => {
  assert.throws((): void => {
    addTodo("   ");
  }, /required/);
});
```

Each title exactly matches the selector in the linkage plan. Stele runs each linked scenario independently, so a broad test-file pass cannot hide a missing scenario.

## 8. Run the complete deterministic gate

```bash
npx stele validate --json
```

Validation now combines three results:

1. OpenSpec strict validation confirms the behavioral change remains structurally valid.
2. Stele implementation verification resolves every ID to its planned declaration or named test.
3. Exact test execution proves every linked scenario test was selected and passed.

The command exits with `0` only when every required check passes. It writes reproducible evidence under `artifacts/`. A reviewer still decides whether the implementation and tests adequately satisfy the English behavior.

## 9. Continue into the complete application

This walkthrough implemented one pure behavior. The finished example adds completion, deletion, filtering, and a browser UI. Follow [Inspect the finished Todo example](/guide/inspect-example) to compare its OpenSpec artifacts, anchors, and evidence with the small project you just built.
