# Contributing

Use Node.js 20.19 or newer and Go 1.24 or newer. Install dependencies with `npm install`, then run `npm run verify` before opening a pull request.

Keep `cmd/stele/main.go` as a small composition root. Put private product code in `internal/stele` while it remains one cohesive verifier package. Place Go tests beside their implementation using the standard `foo_test.go` naming convention. Split packages only when a component has an independent responsibility and dependency direction, such as a future report server or another specification adapter.

Changes to verification behavior need meaningful tests for both success and failure paths. The coverage gate requires 100% statement coverage for the Go core, while assertions and review remain responsible for test quality. `golangci-lint` 2.13.2 enforces formatting, correctness, modernization, and readability rules.

Preserve existing behavioral IDs in consumer fixtures. Code that implements a requirement uses `@implements req.…` beside a compatible declaration; a test that verifies a scenario uses `@verifies scn.…` beside an independently selectable named test.

See [Build a verified change](docs/guide/verified-change.md) for the complete workflow.
