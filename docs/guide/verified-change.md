# Build a verified change

Use this sequence for every OpenSpec change. It keeps planning, implementation, execution, and review distinct.

## 1. Write testable behavior

Requirements define what must hold. Scenarios express observable examples with `WHEN` and `THEN`. Add stable IDs while drafting rather than deriving them from headings.

## 2. Choose the evidence boundary

For each scenario, decide the lowest test layer that proves the behavior:

- use a unit test for pure logic without I/O;
- use an integration test when the behavior crosses an API, database, filesystem, process, or component boundary;
- use an end-to-end test for a critical user journey whose assembled behavior is the risk.

Record the chosen test target in the linkage plan. The richer [verification evidence policy](/concepts/verification-evidence) is planned but not yet enforced.

## 3. Validate the proposal

```bash
npx stele verify --stage proposal --json
```

This catches missing and duplicate IDs, incomplete planned links, and invalid spec shape before code exists.

## 4. Add declaration anchors

```ts
// @implements req.recovery.22b616c90f42
export function requestRecovery(email: string) {
  // ...
}
```

```ts
// @verifies scn.recovery.20d9cd2785a4
test("accepts a registered email", () => {
  // ...
});
```

An anchor is a traceability statement. Stele resolves it to a compatible nearby declaration and checks it against the planned path and selector.

## 5. Run deterministic validation

```bash
npx stele validate --json
```

This produces test evidence in `artifacts/test-results.json` and the verification graph in `artifacts/verification-report.json` unless alternate paths are supplied.

## 6. Review meaning

A passing report proves that the declared graph is complete and that the selected tests ran. A reviewer still decides whether the code and tests adequately express the intended behavior. Keep that semantic judgment separate from the deterministic verdict.
