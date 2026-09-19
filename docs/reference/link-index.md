# Link index

`stele index` prints one JSON document with every link between specifications, code, and tests, for editors and review tools. An editor can show a requirement's text and a scenario's steps when the cursor rests on an ID, like a doc comment, jump from a scenario to its implementation and tests, and offer to run a scenario with `stele test <id>`.

```bash
npx stele index --change todo-basics            # the change and the current specifications
npx stele index --specs                         # the current specifications only
npx stele index --all --output-file index.json  # every scope, written to a file
```

The document is deterministic: identical inputs give identical bytes, with no timestamps. Run the command again whenever you need fresh data; Stele does not watch files.

## Document

```json
{
  "schemaVersion": 1,
  "scopes": [
    { "id": "todo-basics", "kind": "change" },
    { "id": "specs", "kind": "specs" }
  ],
  "specFiles": [
    {
      "scope": "todo-basics",
      "path": "openspec/changes/todo-basics/specs/todo/spec.md",
      "annotation": "annotated",
      "version": "v1"
    },
    { "scope": "specs", "path": "openspec/specs/todo/spec.md", "annotation": "missing", "version": null }
  ],
  "requirements": [
    {
      "id": "req.todo.22b616c90f42",
      "scope": "todo-basics",
      "title": "Add a todo",
      "text": "The system SHALL add a todo with the entered title.",
      "source": { "path": "openspec/changes/todo-basics/specs/todo/spec.md", "line": 4 },
      "specVersion": "v1",
      "scenarios": ["scn.todo.20d9cd2785a4"],
      "implementations": [{ "path": "src/todo.mts", "line": 7, "selector": "addTodo" }]
    }
  ],
  "scenarios": [
    {
      "id": "scn.todo.20d9cd2785a4",
      "scope": "todo-basics",
      "requirement": "req.todo.22b616c90f42",
      "title": "Non-empty title",
      "text": "- **WHEN** the user enters a non-empty title\n- **THEN** a todo with that title is added",
      "steps": [
        { "keyword": "WHEN", "text": "the user enters a non-empty title" },
        { "keyword": "THEN", "text": "a todo with that title is added" }
      ],
      "source": { "path": "openspec/changes/todo-basics/specs/todo/spec.md", "line": 9 },
      "specVersion": "v1",
      "evidence": [
        {
          "id": "scn.todo.20d9cd2785a4.unit",
          "level": "unit",
          "approval": "approved",
          "placement": "tests/todo.test.mts",
          "tests": [{ "path": "tests/todo.test.mts", "line": 5, "selector": "adds a todo" }],
          "execution": { "state": "executed", "outcome": "passed" }
        }
      ]
    }
  ],
  "anchors": [
    {
      "id": "req.todo.22b616c90f42",
      "scope": "todo-basics",
      "annotation": "implements",
      "kind": "code",
      "path": "src/todo.mts",
      "line": 7,
      "selector": "addTodo",
      "status": "linked"
    },
    {
      "id": "scn.todo.20d9cd2785a4",
      "scope": "todo-basics",
      "annotation": "verifies",
      "kind": "test",
      "path": "tests/todo.test.mts",
      "line": 5,
      "selector": "adds a todo",
      "evidenceId": "scn.todo.20d9cd2785a4.unit",
      "level": "unit",
      "status": "linked"
    }
  ]
}
```

## Scopes

`scopes` lists the scopes in order: with `--change`, the change and then the current specifications when there are any; with `--specs`, only `specs`; with `--all`, `specs` and then every active change in directory order. Every requirement, scenario, and anchor carries the `scope` it belongs to. A requirement that a change modifies appears once per scope with the same ID, so an editor can show the current and the proposed text side by side.

## Specification files

`specFiles` lists each specification file once per scope, in scope order and then by path, with its [annotation](/concepts/spec-format) state: `annotated`, `missing`, `misplaced`, `malformed`, or `unsupported`. `version` is the declared version of an annotated or unsupported file, and otherwise `null`. Field values are not listed.

`specFiles` and `specVersion` were added within schema version 1; readers that do not know them can ignore them.

## Requirements and scenarios

Requirements and scenarios follow spec order within a scope. Lines are 1-based.

- A requirement's `text` is the Markdown under its heading, without the `Verification-ID` line and without its scenarios.
- A scenario's `text` is its raw Markdown below the heading, without the `Verification-ID` line. `steps` has one entry per `- **KEYWORD** …` bullet, with the keyword in upper case (`GIVEN`, `WHEN`, `THEN`, `AND`, …). A bullet that continues on following lines is one step with the lines joined by spaces. A bullet without a bold keyword is a step with an empty `keyword`, and a scenario without text has an empty `text` and no steps.
- `implementations` lists the requirement's `@implements` anchors.
- `specVersion` is the format version of the item's file, such as `v1`, or `null` unless the file is annotated with a supported version.

## Evidence

Each scenario lists its evidence:

- Under a version 2 plan, one entry per planned evidence ID, with its `level`, advisory `placement`, and `approval`: `approved`, `unapproved`, or `stale` (the scenario or entry changed since it was approved).
- Otherwise one entry per evidence ID that its test anchors name, or the scenario ID for bare anchors. `approval` is `v1` when a version 1 plan targets the scenario, with that plan's `target`, and `unplanned` when no plan does.

`tests` lists the test anchors of an entry. `execution` is the last known outcome from `artifacts/test-results.json`:

| `state` | `outcome` | Meaning |
|---|---|---|
| `executed` | `passed` or `failed` | Every test of the entry ran with the current inputs |
| `stale` | `passed` or `failed` | The last outcome, recorded before an input changed |
| `not-run` | `not-run` | A test of the entry has no recorded execution |

The input digest covers the whole repository, so any edit to an input marks every outcome stale until the tests run again.

## Targets

These fields appear only when a scope has [targeted](/guide/targets) specifications, so the index of a project without targets is unchanged:

- `targets` at the top level lists the configured targets by name, with their `paths` and `evidenceOnly`.
- A specification file lists its `targets` field in the written order.
- Each requirement and scenario of a targeted specification has `declaredTargets`, its own `Targets:` line or `null` without one, and `applicableTargets`, the targets it applies to after narrowing, ordered by name.
- A targeted evidence entry has its `target`. For a version 1 entry, `target` is still the planned path, until 0.2.0.
- A test anchor with a targeted evidence ID has its `target`.
- Each scope in `scopes` has a `matrix`: `targets`, the columns ordered by name, and `rows`, one per targeted scenario in specification order, with its `scenario`, `requirement`, `title`, `capability`, and one cell per target.

```json
{
  "target": "android",
  "state": "missing",
  "evidence": []
}
```

A cell's `state` is `n/a` when the scenario does not apply to the target, `missing` without a plan entry or with an approved entry that no `@verifies` anchor resolves, `unapproved` while an entry is unapproved or stale, and otherwise the worst execution state of its entries: `failed`, `stale`, `not-run`, or `passed`. When the entries of a cell differ, the order `failed`, `missing`, `unapproved`, `stale`, `not-run`, `passed` decides. `evidence` lists the cell's evidence IDs.

With `--target`, the index contains only the selected targets' columns, evidence entries, and anchors, the scenarios and requirements that apply to them, and `selectedTargets` names the selection. Untargeted items stay.

## Anchors

Anchors are sorted by scope, path, and line. An anchor appears once per scope whose specifications declare its ID, and otherwise once without a `scope`, after the others.

| `status` | Meaning |
|---|---|
| `linked` | An indexed scope declares the ID and the anchor is attached to a declaration |
| `unresolved` | An indexed scope declares the ID, but the anchor has no `selector`: no declaration follows it, or the test has no exact name |
| `other-scope` | Only a change or specification outside the index declares the ID |
| `undeclared` | No specification under `openspec/` declares the ID, often a typo |

## Using the index in process

The index is built by one function over parsed specifications, anchors, plans, and evidence, without reading files itself. A future `stele lsp` language server will call it in memory to serve hover cards, go-to-definition, run buttons, and diagnostics without starting a process for every request.
