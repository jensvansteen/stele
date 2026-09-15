# Verification evidence

One rule organizes this model: **every scenario needs adequate verification evidence**.

A test is one evidence kind. Unit, integration, and end-to-end describe the level of that test. A conformance check compares something actual with an expected contract. A measurement compares an observed metric with a threshold. The verification plan declares the required evidence, adapters collect it, and the Stele core validates the results.

::: warning Current support
The current release resolves `@verifies` anchors to exact named TypeScript tests executed through Node. Go and other consumer languages need future adapters. The evidence kinds and policy schema on this page define the next compatible extension and are not enforced yet.
:::

## The model

Evidence kind describes **how** a claim is checked. Test level describes **where** a behavioral test runs.

The policy uses three short machine values: `test`, `conformance`, and `measurement`. In the prose below, `test` means a behavioral test.

| Evidence kind | Proves | Typical metadata |
| --- | --- | --- |
| Behavioral test | The system produces an expected outcome | level, boundary, runner, path, selector |
| Conformance check | An artifact, source surface, or running provider matches an expected contract | mode, adapter, contract, subject, rule set |
| Measurement | An observed metric satisfies a declared threshold | adapter, workload, metric, threshold, samples, environment |

Conformance mode can be `static` or `runtime`. Static mode covers checks such as an OpenAPI document, exported interface, schema, or compiler rule. Runtime mode covers a running API provider. These are variations of the same actual-versus-expected comparison, not separate top-level concepts.

Unit, integration, and end-to-end are valid levels for behavioral tests. They are not levels for a conformance check or performance measurement.

```text
scenario
└── required evidence
    ├── test → unit, integration, or end-to-end
    ├── conformance → static or runtime
    └── measurement → performance, capacity, or another metric
```

## When the evidence is a test

Choose the lowest level that convincingly proves the behavior:

| Level | Use when | Typical boundary |
| --- | --- | --- |
| Unit | Pure logic can prove the behavior | Function or class without I/O |
| Integration | The risk exists where components meet | API, database, filesystem, process, or subsystem |
| End-to-end | The assembled critical journey is the behavior at risk | Browser or full application flow |

Do not require all three levels for every scenario. Add another evidence item only when it proves a different facet or boundary. The test pyramid is a suite-level cost heuristic, not a quota for each feature.

## OpenAPI example

Consider this scenario: a client creates a todo through `POST /todos`.

Three checks can support it without claiming to prove the same fact:

1. **Static conformance evidence** checks that `POST /todos`, its request body, the `201` response, and the Todo response schema are declared. A compatibility rule can also reject forbidden breaking changes.
2. **Runtime conformance evidence** starts or addresses the service, sends requests derived from the contract, and confirms that real responses conform to the OpenAPI shapes.
3. **Behavioral integration evidence** checks product meaning: a valid todo is persisted, an empty title is rejected, and the returned identity can be retrieved.

The static contract proves interface shape. Provider conformance proves that the running boundary honors that shape. The behavioral test proves the intended outcome. A project can require one or all three according to the scenario's risks.

## Proposed policy shape

```yaml
scenario: scn.todo.20d9cd2785a4
evidence:
  - kind: conformance
    mode: static
    role: required
    facet: public API shape
    adapter: openapi
    target: openapi.yaml#/paths/~1todos/post
    ruleset: backwards-compatible

  - kind: conformance
    mode: runtime
    role: required
    facet: provider response shape
    adapter: openapi-provider
    contract: openapi.yaml
    endpoint: local-test-service

  - kind: test
    role: required
    facet: todo persistence
    level: integration
    boundary: HTTP API and todo store
    runner: node-test
    path: tests/todo-api.test.mts
    selector: persists a valid todo

  - kind: measurement
    role: supporting
    facet: API latency
    adapter: load-test
    workload: create-todo-standard
    metric: response-time-p95
    threshold: 200ms
    samples: 100
    environment: ci-performance-runner
```

Each entry names one facet so reviewers can see why multiple checks exist. Required evidence gates the verdict. Supporting evidence adds review context but cannot compensate for missing or failed required evidence.

## Deterministic contract

Every evidence adapter must return a versioned, normalized result. For the same inputs, configuration, and adapter version, the canonical result must be identical.

The Stele core should:

1. validate that every scenario has the evidence required by project policy;
2. resolve each target without guessing;
3. invoke the named adapter or runner with an exact target;
4. validate the returned evidence schema;
5. bind the result to the relevant input digest and repository revision;
6. pass only when every required result is current and successful.

An adapter may parse source, compiler metadata, an OpenAPI document, runner output, or performance measurements. It must not use an LLM to decide the deterministic verdict or reinterpret the plain-English requirement.

Measurements vary between runs, so their observation is not byte-for-byte deterministic. The evaluation remains deterministic: given the same recorded measurements and policy, Stele must produce the same verdict. Measurement evidence therefore records the workload, environment or runner class, warm-up, sample count, tool version, metric, threshold, and allowed variance.

## Review presentation

The dashboard should label evidence by the fact it establishes. It should never display an implementation link, contract shape check, test anchor, passing execution, and human review as one undifferentiated green state.

For each scenario, reviewers should be able to inspect:

- the required evidence policy and rationale;
- the adapter, target, and checked facet;
- the normalized result and diagnostics;
- the input revision and freshness;
- attached logs, screenshots, or recordings;
- the separate human-review state.
