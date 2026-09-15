# Plan test levels

Test level belongs in the verification plan because it describes how a scenario will be proven. The OpenSpec scenario continues to describe what the product must do.

::: warning Roadmap status
The current linkage plan records a test path and selector. Stele does not yet enforce the richer test-level fields on this page. They define the next compatible schema extension.
:::

## Choose the lowest convincing level

| Level | Use when | Typical boundary |
|---|---|---|
| Unit | Pure logic can prove the behavior | Function or single class, no I/O |
| Integration | The risk exists where components meet | API, database, filesystem, process, or subsystem contract |
| End-to-end | The assembled critical journey is the behavior at risk | Browser or full application flow |

Do not require all three levels for every scenario. Add more than one evidence item only when each proves a distinct facet or boundary. The test pyramid is a suite-level cost heuristic, not a quota attached to every feature.

## Proposed verification policy

Each scenario plan should eventually declare:

```yaml
scenario: scn.recovery.20d9cd2785a4
evidence:
  - level: integration
    boundary: HTTP API and recovery service
    character: contract
    role: required
    facet: registered-account request
    runner: node-test
    path: test/recovery-api.test.ts
    selector: accepts a registered email
    rationale: The observable behavior begins at the API boundary.
```

Useful fields are:

- **level** — unit, integration, or end-to-end;
- **boundary** — the concrete interface under test;
- **character** — example, property, snapshot, or contract;
- **role** — required or supporting evidence;
- **facet** — the part of the scenario this evidence proves;
- **runner, path, selector** — the independently executable target;
- **rationale** — why this is the lowest convincing test level.

## Deterministic enforcement

The planning skill should require a declared policy for every scenario. The verifier can then reject missing fields, incompatible runner/level combinations, missing required anchors, selectors that resolve to the wrong test, and required evidence that is absent, stale, skipped, or failed.

Recordings should attach to end-to-end evidence as review material. A recording alone should not turn an unexecuted automated check green.
