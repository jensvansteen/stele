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

`npm ci` installs the repository's Git hooks. Use Conventional Commit subjects such as `ci(release): add npm publishing workflow`, and branch names such as `feat/my-change` or `release/0.1.0-rc.1`. The commit hook checks the subject, the push hook checks the branch name, and the required CI lint job checks branch name, pull request title, and commit subjects. The hooks provide early feedback; CI is the merge gate.

Changes to verification behavior need meaningful tests for both success and failure paths. The coverage gate requires 100% statement coverage for the Go core, while assertions and review remain responsible for test quality. `golangci-lint` 2.13.2 enforces Go formatting, correctness, modernization, and readability rules. ESLint, the TypeScript compiler, and `type-coverage` enforce the corresponding MTS quality gates.

Version 0.1 consumer fixtures use TypeScript. Preserve their existing behavioral IDs. TypeScript code that implements a requirement uses `@implements req.…` beside a compatible declaration; a TypeScript test that verifies a scenario uses `@verifies scn.…` beside an independently selectable named test. Go remains the implementation language of Stele itself, not a supported consumer language in this release.

See [Build a verified change](docs/guide/verified-change.md) for the complete workflow.

## Releases

Release source changes through a reviewed pull request into `main`. The [release workflow](.github/workflows/release.yml) runs when a version tag such as `v0.1.0-rc.1` points to a commit on `main`. It checks that the tag matches `package.json`, runs the complete verification and documentation build, publishes the npm package, and then creates a GitHub Release for the same tag. Prerelease versions use npm's `next` tag and a GitHub prerelease; a stable version uses `latest` and a normal GitHub Release. The package build includes macOS and Linux binaries for ARM64 and x64.

The first publication of a new npm package needs a one-time npm publishing credential in the GitHub repository secret `NPM_PUBLISH_TOKEN`: npm only lets maintainers configure a trusted publisher once the package exists. Do not commit or print the token. After that first GitHub-managed release, configure npm's GitHub Actions trusted publisher for `jensvansteen/stele` and the workflow filename `release.yml`, allowing direct publishing. Run a subsequent candidate through it and remove the temporary secret after OIDC succeeds. The workflow already requests `id-token: write` for that transition.

Never reuse or move a published version tag. Update both `package.json` and `package-lock.json` for each new candidate or final release, merge the change, then create the matching tag on `main`. Confirm the GitHub release and the npm `next` or `latest` dist-tag after the workflow succeeds.
