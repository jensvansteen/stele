# Dashboard roadmap

The dashboard should make the evidence graph easy to inspect without becoming a second verification engine.

## Phase 1: static HTML report

Model the first release after Playwright’s HTML reporter:

```bash
stele validate --json
stele report
stele show-report
```

`stele report` should combine OpenSpec Markdown with deterministic evidence into a static directory. `stele show-report` should serve that directory locally. The same output can be uploaded as a CI artifact or hosted without the application runtime.

Each requirement should expand to show:

- the OpenSpec requirement and scenarios;
- stable IDs and planned test level;
- linked implementation declarations;
- linked test source and exact selector;
- execution result, logs, screenshots, and recordings;
- input revision and evidence freshness;
- separate linkage, execution, and review states.

## Phase 2: local control surface

A later `stele dashboard` command can add a local service with a fixed operation set:

- run proposal or implementation verification;
- run selected or full validation;
- stream command output;
- propose linkage or test-policy edits;
- show exact Markdown or JSON diffs;
- apply reviewed edits and validate again.

Project-changing controls should remain local. Hosted CI reports stay read-only.

## Architectural boundary

The Go CLI computes every verdict and writes the canonical evidence. The report renderer reads that evidence. Interactive commands call the same CLI operations and write normal OpenSpec or Stele files before validating again.

This boundary keeps the result reproducible in CI and prevents browser code from changing what “passed” means.

## Delivery order

1. Finalize and version the evidence schema.
2. Add test-level policy to the linkage plan and verifier.
3. Generate a self-contained static report.
4. Serve and watch the report locally.
5. Add reviewed local commands and edits.
6. Publish reports as CI artifacts.
