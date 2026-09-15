# Stele

Stele is a deterministic verification layer for OpenSpec. OpenSpec owns plain-English requirements and scenarios. Stele gives that behavior stable IDs, connects it to planned code and named tests, executes each scenario test independently, and writes reproducible JSON evidence for CI and review.

The verifier is written in Go and distributed through an npm package. npm handles installation and pins the bundled OpenSpec CLI; the installed `stele` command invokes the native binary directly.

## Install locally

The package is not published to npm yet. Build and install the same tarball npm will eventually publish:

```bash
# In this repository
npm install
npm pack --pack-destination /path/to/consumer/vendor

# In the consumer repository
npm install --save-dev ./vendor/stele-spec-0.3.0.tgz
npx stele init --change my-change
npx stele verify --stage proposal --json
```

The independent [`stele-examples`](https://github.com/jensvansteen/stele-examples) repository proves this package boundary with a Todo application. It imports no source files from this checkout.

## Verification loop

1. Write the feature as OpenSpec requirements and concrete scenarios.
2. Add one immutable `req.<namespace>.<token>` ID to each requirement and one `scn.<namespace>.<token>` ID to each scenario.
3. Plan the source declaration and test selector for every ID.
4. Put `@implements <requirement-id>` beside the code declaration and `@verifies <scenario-id>` beside the named test.
5. Run `stele validate`. Stele checks OpenSpec, resolves the anchors, runs every scenario test by its exact selector, and binds the result to the relevant input digest.

A resolved anchor proves traceability. A passing execution proves the selected test ran. Human review still decides whether the code and test adequately satisfy the prose.

## Develop Stele

Requirements: Node.js 24 or newer and Go 1.24 or newer. Node 24 lets the package tooling and integration tests run directly as native `.mts` files.

```bash
npm install
npm run verify
npm run docs:dev
```

Useful commands:

| Command | Purpose |
|---|---|
| `npm run build` | Compile the native `dist/stele` executable |
| `npm run lint` | Run the pinned Go lint policy |
| `npm run test:go` | Run Go tests with the race detector |
| `npm run test:node` | Test the CLI and packed-package boundary |
| `npm run verify` | Run the complete local CI gate |
| `npm run docs:build` | Build the documentation site |

The Go entry point is [`cmd/stele/main.go`](cmd/stele/main.go). The private verifier package is under [`internal/stele`](internal/stele). Go convention keeps each `*_test.go` file beside the implementation it tests; test files are excluded from normal builds.

Read the [documentation site source](docs/index.md) or start with [Getting started](docs/guide/getting-started.md). [OpenSpec and Stele](docs/concepts/openspec-and-stele.md) explains the ownership boundary, and [Performance](docs/reference/performance.md) contains the benchmark method and results.

## Current scope

Version 0.3 provides the Go verifier, OpenSpec adapter, native CLI, project initializer, repository-local skills, deterministic reports, exact Node and Go test selection, linting, CI, and documentation site. Cross-platform npm release packaging and the generated artifact dashboard are the next product milestones.
