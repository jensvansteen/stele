# IDs and anchors

Stele traces behavior through two identity types.

| Identity | Format | Placed in | Anchored by |
|---|---|---|---|
| Requirement | `req.<namespace>.<12 hex>` | OpenSpec requirement body | `@implements` |
| Scenario | `scn.<namespace>.<12 hex>` | OpenSpec scenario body | `@verifies` |

## Stable behavior IDs

An ID names behavior, not a heading, ticket, function, file, or test framework. Preserve it when the behavior is reworded, moved, or renamed. Create a new ID for new behavior and do not reuse a retired identity.

```markdown
### Requirement: Delete a task
Verification-ID: req.todo.ea4d4f29a1c7

#### Scenario: Delete an existing task
Verification-ID: scn.todo.591a3b429cf0
```

## Code anchors

Place the requirement anchor beside the declaration that enforces the behavior:

```go
// @implements req.todo.ea4d4f29a1c7
func (store *Store) Delete(id string) error {
    // ...
}
```

## Test anchors

Place the scenario anchor beside an independently selectable named test:

```go
// @verifies scn.todo.591a3b429cf0
func TestDeleteExistingTask(t *testing.T) {
    // ...
}
```

The current runner selects exact named Node and Go tests. Multiple scenario IDs may point to one test declaration when the test truly exercises each scenario, though smaller evidence units are easier to diagnose.

## Why nearby declarations matter

A matching string in a comment is not enough. Stele checks that the anchor is attached to a compatible declaration and that its path and selector match the linkage plan. This rejects stale copy-pasted comments and anchors placed in unrelated files.

## Common failures

| Failure | Meaning |
|---|---|
| Unknown ID | An anchor names behavior absent from the selected OpenSpec change |
| Missing anchor | Planned behavior has no corresponding declaration |
| Duplicate ID | The spec defines one behavior identity more than once |
| Target mismatch | The real path or selector differs from the linkage plan |
| Unselected test | A process exited successfully but the named test did not run |
