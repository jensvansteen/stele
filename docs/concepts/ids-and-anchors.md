# IDs and anchors

Stele traces behavior through two identity types.

| Identity | Format | Placed in | Anchored by |
|---|---|---|---|
| Requirement | `req.<namespace>.<12 hex>` | OpenSpec requirement body | `@implements` |
| Scenario | `scn.<namespace>.<12 hex>` | OpenSpec scenario body | `@verifies` |

## Stable behavior IDs

An ID names behavior, not a heading, ticket, function, file, or test framework. Preserve it when the behavior is reworded, moved, or renamed. Create a new ID for new behavior and do not reuse a retired identity.

`stele ids --change <change>` inserts the missing IDs of a change. It derives each new token from the change, capability, and heading, avoids every ID already declared under `openspec/` or named by an anchor, and reuses the current ID for headings in a `MODIFIED Requirements` section. `stele ids --check` only reports missing IDs.

```markdown
### Requirement: Delete a task
Verification-ID: req.todo.ea4d4f29a1c7

#### Scenario: Delete an existing task
Verification-ID: scn.todo.591a3b429cf0
```

## Code anchors

Place the requirement anchor beside the declaration that enforces the behavior:

```ts
// @implements req.todo.ea4d4f29a1c7
export function deleteTodo(id: string): void {
  // ...
}
```

The code anchor is a deterministic navigation and ownership link, not proof that the behavior works. It lets a review UI place the plain-English requirement beside the relevant production declaration, open the exact source or pull-request diff, show every component that implements the same requirement, and identify code affected by a future spec change.

This distinction matters when a passing user journey crosses several internal components: the test shows that the journey works, while the implementation anchors show reviewers where each part is owned. A project concerned only with executable acceptance tests may make code anchors optional; Stele requires them when bidirectional spec-to-code traceability is part of the project policy.

## Test anchors

Place the scenario anchor beside an independently selectable named test:

```ts
// @verifies scn.todo.591a3b429cf0.unit
test("deletes an existing task", () => {
  // ...
});
```

The runner selects exact named TypeScript tests through Node and exact Go test functions through `go test`. Multiple scenario IDs may point to one test declaration when the test truly exercises each scenario, though smaller evidence units are easier to diagnose. Other consumer languages need dedicated declaration and test-runner adapters.

With a version 2 plan, the anchor names an evidence ID: the scenario ID plus the planned level, such as `.unit` or `.e2e.2`. A bare scenario ID, or a level the plan does not list, fails with `ANCHOR_EVIDENCE_UNPLANNED`. Version 1 plans also accept bare scenario IDs.

The test anchor identifies the intended evidence unit. It becomes behavioral evidence only after Stele confirms that the exact test executed and passed for the current inputs.

A test anchor resolves to the next `test(...)` or `it(...)` call, including calls written as `void test(...)` or `await test(...)`, which typed lint rules require for floating promises. `describe` blocks, `test.each`, and `test.only` are not selectable.

## Go anchors

In Go, put the anchor in the comment directly above the declaration:

```go
// DeleteTodo removes a task.
//
// @implements req.todo.ea4d4f29a1c7
func DeleteTodo(id string) error {
	// ...
}

// @verifies scn.todo.591a3b429cf0.unit
func TestDeleteExistingTask(t *testing.T) {
	// ...
}
```

A code anchor resolves to a function (`DeleteTodo`), a method (`Store.Delete`, without pointer or type parameters), or a single type declaration. A test anchor in a `_test.go` file resolves only to a top-level `TestXxx(t *testing.T)` function. Subtests, examples, benchmarks, and fuzz targets are not selectable. Stele runs each linked test with `go test -json -count=1 -run '^Name$'` in its package, from the nearest `go.mod`, and passes the custom tags named by the file's `//go:build` line. A skipped test is not a pass. A Go file that does not parse stops verification with exit code `2`.

## Anchors live in comments

Stele reads annotations only from `//` comments, `/* */` comments, and JSDoc lines. Text inside string and template literals is ignored, so test fixtures can contain anchor text without creating anchors. The scanner does not recognize regular-expression literals; a quote or `//` inside one can hide a comment that follows it on the same line.

## Why nearby declarations matter

A matching string in a comment is not enough. Stele checks that the anchor is attached to a compatible declaration, so a stale copy-pasted comment does not count. With a version 2 plan, the anchors are the only record of where code and tests live, and Stele never judges their location. A version 1 plan additionally requires the path and selector to match its targets.

## Common failures

| Failure | Meaning |
|---|---|
| Unknown ID | An anchor names behavior absent from the selected OpenSpec change |
| Missing anchor | Planned behavior has no corresponding declaration |
| Duplicate ID | The spec defines one behavior identity more than once |
| Target mismatch | The real path or selector differs from a version 1 plan (`LINK_TARGET_MISMATCH`) |
| Missing evidence | An approved evidence entry has no `@verifies` anchor on a named test (`LINK_EVIDENCE_MISSING`) |
| Unplanned evidence | A test claims an evidence ID the plan does not list (`ANCHOR_EVIDENCE_UNPLANNED`) |
| Unapproved or stale plan | An entry was never approved, or changed since approval (`PLAN_UNAPPROVED`, `PLAN_APPROVAL_STALE`) |
| Unselected test | A process exited successfully but the named test did not run |
