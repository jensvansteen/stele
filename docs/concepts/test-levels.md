# Test levels

A behavioral test is one kind of [verification evidence](/concepts/verification-evidence). Unit, integration, and end-to-end describe where that test runs; they are not separate evidence systems.

## Level definitions

Levels are defined by what the test reaches, not by the tool that runs it:

| Level | The test reaches |
|---|---|
| `unit` | Code called directly, in process, possibly with temporary files or stubbed dependencies |
| `integration` | The code working together with one real outside tool or service |
| `e2e` | The real product through its user-facing entry point: the UI for applications, the installed executable or package for command-line tools |

Choose the lowest level that convincingly proves a scenario. Plan a second level only for a distinct risk that the first cannot cover, and say which risk in the rationale.

## Evidence IDs

Each planned level is one evidence entry with its own ID: the scenario ID plus the level, such as `scn.todo.591a3b429cf0.unit`. A second entry of the same level adds an ordinal: `.unit.2`, then `.unit.3`. The test that supplies the evidence carries that ID:

```ts
// @verifies scn.todo.591a3b429cf0.e2e
test("deletes a task from the list", () => {
  // ...
});
```

A scenario passes only when every one of its evidence tests passes, and a failed scenario names the evidence that failed.

The broader evidence model, performance measurements, and the OpenAPI example live on the [Verification evidence](/concepts/verification-evidence) page.
