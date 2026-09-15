# Contributing

Use Node.js 24 or newer, npm 11 or newer, and Go 1.24 or newer. This repository uses npm and `package-lock.json`; do not mix pnpm or Yarn into the checkout. Install the exact dependency tree with `npm ci`, then run `npm run verify` before opening a pull request.

Node 24 lets the package tooling and integration tests run directly as native `.mts` files. If another package manager moved or replaced `node_modules`, restore the committed dependency tree with:

```bash
npm ci
npm run verify
npm run docs:dev
```

To preview the production documentation build locally:

```bash
npm run docs:build
npm run docs:preview
```

Useful contributor commands:

| Command | Purpose |
|---|---|
| `npm run build` | Compile the native `dist/stele` executable |
| `npm run lint` | Run the pinned Go and TypeScript lint policies |
| `npm run typecheck` | Run Go vet and strict TypeScript checking |
| `npm run type-coverage` | Require 100% TypeScript type coverage |
| `npm run test:go` | Run Go tests with the race detector |
| `npm run test:node` | Test the installed CLI from Node |
| `npm run test:package` | Test the packed-package boundary in Go |
| `npm run verify` | Run the complete local CI gate |
| `npm run docs:build` | Build the documentation site |

Keep `cmd/stele/main.go` as a small composition root. Put private product code in `internal/stele` while it remains one cohesive verifier package. Place Go tests beside their implementation using the standard `foo_test.go` naming convention. Split packages only when a component has an independent responsibility and dependency direction, such as a future report server or another specification adapter.

The package distribution smoke test uses the `integration` build tag because it runs npm and builds a separate consumer project.

Changes to verification behavior need meaningful tests for both success and failure paths. The coverage gate requires 100% statement coverage for the Go core, while assertions and review remain responsible for test quality. `golangci-lint` 2.13.2 enforces Go formatting, correctness, modernization, and readability rules. ESLint, the TypeScript compiler, and `type-coverage` enforce the corresponding MTS quality gates.

Version 0.1 consumer fixtures use TypeScript. Preserve their existing behavioral IDs. TypeScript code that implements a requirement uses `@implements req.…` beside a compatible declaration; a TypeScript test that verifies a scenario uses `@verifies scn.…` beside an independently selectable named test. Go remains the implementation language of Stele itself, not a supported consumer language in this release.

See [Build a verified change](docs/guide/verified-change.md) for the complete workflow.
