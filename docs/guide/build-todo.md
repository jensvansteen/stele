# Build a Todo feature with OpenSpec and Stele

This walkthrough starts with a behavior idea, plans it in OpenSpec, adds Stele's deterministic verification plan, and only then writes the TypeScript implementation and test. It covers one small Todo behavior so every artifact remains easy to inspect.

For a shortcut to the finished browser application and evidence files, use [Inspect the finished Todo example](/guide/inspect-example).

## 1. Create the project

Start in a new TypeScript project:

```bash
npm init -y
npm install --save-dev stele-spec@next
npx stele init
```

`stele init` initializes OpenSpec with the bundled CLI, adds the `stele` workflow schema and guidance, and installs the Stele skills. See [Getting started](/guide/getting-started#set-up-a-project) for the details.

## 2. Plan the behavior with your agent

Ask your coding agent to plan the change with the `stele-propose` skill:

```text
Use stele-propose to plan the change `todo-basics` for a small TypeScript Todo
app. The first capability lets a user add a todo from non-empty text.
Surrounding whitespace is removed, and blank text is rejected.
```

The skill uses OpenSpec to create the change under `openspec/changes/todo-basics/`, runs the Stele planning step, and stops with "Plan ready." without writing implementation code. The change contains:

| Artifact | Question it answers |
|---|---|
| `proposal.md` | Why are we changing the product, and what is in scope? |
| `specs/todo/spec.md` | What behavior must the product provide? |
| `design.md` | How will the implementation satisfy that behavior, and how will each scenario be proven? |
| `linkage-plan.json` | Which declaration and test are planned for each ID? |
| `tasks.md` | What work must be completed? |

The following steps show what to review in each artifact. You can also write them yourself.

## 3. Review the behavior and its identities

The delta specification should contain behavior like this, with a `Verification-ID` that `stele ids` inserted below every heading:

```markdown
## Purpose

Provide the basic creation behavior for a small Todo application.

## ADDED Requirements

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

These identities survive wording and file changes. They let Stele join the specification, implementation, tests, and evidence without guessing from titles. Writing specs yourself? Run `npx stele ids --change todo-basics` instead of inventing tokens.

## 4. Check the plan on its own

```bash
npx openspec validate todo-basics --strict --no-interactive
```

OpenSpec proves the plan has a valid shape; no implementation evidence exists yet.

## 5. Review how each behavior will be proven

The planning step adds a verification table to `design.md`:

```markdown
## Verification strategy

| Scenario | Level | Boundary | Rationale |
|---|---|---|---|
| `scn.todo.0a1b2c3d4e5f` | unit | pure Todo creation | No I/O is needed to prove normalization. |
| `scn.todo.1b2c3d4e5f60` | unit | pure Todo creation | Blank-input rejection is deterministic logic. |

Status: proposed, awaiting approval.
```

It also writes `openspec/changes/todo-basics/linkage-plan.json`, which records the same decisions. The plan lives with the change, so `openspec archive` keeps it:

```json
{
  "schemaVersion": 2,
  "changeId": "todo-basics",
  "scenarios": {
    "scn.todo.0a1b2c3d4e5f": {
      "evidence": [
        {
          "id": "scn.todo.0a1b2c3d4e5f.unit",
          "level": "unit",
          "rationale": "No I/O is needed to prove normalization.",
          "placement": "tests/todo.test.mts"
        }
      ]
    },
    "scn.todo.1b2c3d4e5f60": {
      "evidence": [
        {
          "id": "scn.todo.1b2c3d4e5f60.unit",
          "level": "unit",
          "rationale": "Blank-input rejection is deterministic logic.",
          "placement": "tests/todo.test.mts"
        }
      ]
    }
  }
}
```

The plan names no code or test locations. `placement` is a suggestion, and the anchors you write in step 6 and 7 decide where things live.

The planning step verified the proposal stage:

```bash
npx stele verify --stage proposal --change todo-basics --json
```

At this point it reports `PLAN_UNAPPROVED` for both entries, and nothing else: a person has not approved the levels yet.

When the levels look right, ask the agent to apply the change. `stele-apply` shows the levels again and asks "Approve these levels and start implementing?". After your explicit yes, it runs `stele approve --change todo-basics --confirmed-in-chat`, which records you as the approver, and only then implements the next two steps. You can also approve in a terminal with `npx stele approve --change todo-basics`.

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

// @verifies scn.todo.0a1b2c3d4e5f.unit
void test("adds a normalized todo", (): void => {
  assert.deepEqual(addTodo("  Ship it  "), {
    title: "Ship it",
    completed: false,
  });
});

// @verifies scn.todo.1b2c3d4e5f60.unit
void test("rejects blank todo text", (): void => {
  assert.throws((): void => {
    addTodo("   ");
  }, /required/);
});
```

Each anchor names the approved evidence ID: the scenario ID plus its level. The `void` prefix satisfies typed lint rules for floating promises, and Stele still selects the test. Stele runs each linked test by its exact name and records its own result, so a broad test-file pass cannot hide a missing scenario.

## 8. Run the complete deterministic gate

```bash
npx stele validate --change todo-basics --json
```

Validation now combines three results:

1. OpenSpec strict validation confirms the behavioral change remains structurally valid.
2. Stele implementation verification finds an `@implements` anchor for the requirement and a `@verifies` anchor on a named test for every approved evidence entry.
3. Exact test execution proves every linked scenario test was selected and passed.

The command exits with `0` only when every required check passes. It writes reproducible evidence under `artifacts/`. A reviewer still decides whether the implementation and tests adequately satisfy the English behavior.

## 9. Archive and keep verifying

When the change is done, ask the agent to archive it with `stele-archive`. The skill runs `stele validate --change todo-basics`, archives the change with OpenSpec only when validation passes, and then verifies the current specifications. The manual equivalent is:

```bash
npx stele validate --change todo-basics
npx openspec archive todo-basics
```

OpenSpec moves the requirements into `openspec/specs/` and the change, including its linkage plan, into `openspec/changes/archive/`. Keep verifying the archived behavior:

```bash
npx stele validate --specs
```

`--specs` checks every requirement in `openspec/specs/` against the plans of archived changes. Later changes can then be verified with `--change` while the archived behavior stays checked.

## 10. Continue into the complete application

This walkthrough implemented one pure behavior. The finished example adds completion, deletion, filtering, and a browser UI. Follow [Inspect the finished Todo example](/guide/inspect-example) to compare its OpenSpec artifacts, anchors, and evidence with the small project you just built.
