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
| `npm run test:coverage` | Run every Go test once with the race detector, and require 100% core coverage |
| `npm run test:node` | Test the installed CLI from Node |
| `npm run test:package` | Test the packed-package boundary in Go |
| `npm run verify` | Run the complete local CI gate |
| `npm run docs:build` | Build the documentation site |

Keep `cmd/stele/main.go` as a small composition root. Put private product code in `internal/stele` while it remains one cohesive verifier package. Place Go tests beside their implementation using the standard `foo_test.go` naming convention. Split packages only when a component has an independent responsibility and dependency direction, such as a future report server or another specification adapter.

The package distribution smoke test uses the `integration` build tag because it runs npm and builds a separate consumer project.

`npm ci` installs the repository's Git hooks. Use Conventional Commit subjects such as `ci(release): add npm publishing workflow`, and branch names such as `feat/my-change` or `release/0.1.0-rc.1`. The commit hook checks the subject, the push hook checks the branch name, and the required CI lint job checks branch name, pull request title, and commit subjects. The hooks provide early feedback; CI is the merge gate.

Changes to verification behavior need meaningful tests for both success and failure paths. The coverage gate requires 100% statement coverage for the Go core, while assertions and review remain responsible for test quality. `golangci-lint` 2.13.2 enforces Go formatting, correctness, modernization, and readability rules. ESLint, the TypeScript compiler, and `type-coverage` enforce the corresponding MTS quality gates.

Consumer fixtures use TypeScript or Go. Preserve their existing behavioral IDs. Code that implements a requirement uses `@implements req.…` beside a compatible declaration; a test that verifies a scenario uses `@verifies scn.…` beside an independently selectable named test, a TypeScript `test(...)` call or a Go `TestXxx` function. Stele verifies its own implementation this way, using the published package pinned as the `stele-published` development dependency:

1. Plan a change under `openspec/changes/<change>/`: proposal, delta specs with Verification-IDs, a design, tasks, and the change's own version 2 `linkage-plan.json` whose evidence entries the maintainer approved with `stele approve`. Check it with `npm run stele:published -- verify --stage proposal --change <change>`.
2. Implement it on the same branch, and merge the change complete: approved, implemented, and passing. Review the plan on the branch or in a draft pull request. CI checks every active change at the implementation stage, so a plan-only or partly implemented change fails every pull request once it is on `main`. Until a release contains the behavior the change relies on, check it with the local build: `npm run stele -- check --change <change>`.
3. Archive the change with the `stele-archive` skill once all its tasks are complete, usually in the pull request that finishes it: `npm run stele:published -- validate --change <change>`, then `npx openspec archive <change> --yes`, then `npm run stele:published -- annotate --specs --targets-from openspec/changes/archive/<date>-<change>` (OpenSpec drops the annotation line from newly created specifications, and keeps only the first line of merged ones, so this restores the annotation and each capability's targets), then `npm run stele:published -- validate --specs`. OpenSpec merges its specs into `openspec/specs/` and moves the change, including its linkage plan, to `openspec/changes/archive/`.
4. CI runs two Stele gates on Linux, for pull requests and pushes to `main`, each as its own job in parallel with the tests, so one gate never hides the other:
   - `npm run verify:self` builds the CLI under test and runs `stele check --specs` with the published package: IDs, annotations, and validation of all archived behavior, judged by a verifier the pull request cannot change.
   - `npm run check:changes` runs `stele check --change <change>` with the Stele built from the same commit for every active change: plan completeness and approval, IDs, annotations, anchors, removed behavior, and evidence. Only the local build understands a change that uses features the published package lacks; it grades itself until the next release moves the change under `verify:self`. The current specifications are left to `verify:self`, so the two gates do not run the same tests twice.

   Run both locally before you push. In GitHub Actions, findings of both gates appear as annotations on the pull request diff.

`stele.config.json` has no default change, so plain `stele verify` requires `--change` or `--specs`.

See [Build a verified change](docs/guide/verified-change.md) for the complete workflow.

## Releases

Release source changes through a reviewed pull request into `main`. The [release workflow](.github/workflows/release.yml) runs when a version tag such as `v0.1.0-rc.1` points to a commit on `main`. It checks that the tag matches `package.json`, runs the complete verification and documentation build, stages the npm package, and then creates a GitHub Release for the same tag. Prerelease versions use npm's `next` tag and a GitHub prerelease; a stable version uses `latest` and a normal GitHub Release. The package build includes macOS and Linux binaries for ARM64 and x64.

The workflow publishes through npm trusted publishing: its GitHub OIDC identity (`id-token: write`) is the trusted publisher configured on npmjs.com for `jensvansteen/stele` and the workflow filename `release.yml`, allowing staged publishing only. It stages the package with `npm stage publish` from npm 12, so no npm token is stored in GitHub. A staged version is not installable until a maintainer approves it with 2FA, on npmjs.com or with `npm stage list stele-spec` followed by `npm stage approve <stage-id>`.

A release that changes the bundled OpenSpec version follows the [OpenSpec upgrade checklist](docs/reference/versions.md#upgrading-openspec-is-a-stele-release) first.

Never reuse or move a published version tag. Update both `package.json` and `package-lock.json` for each new candidate or final release, merge the change, then create the matching tag on `main`. After the workflow succeeds, approve the staged version, then confirm the GitHub release and the npm `next` or `latest` dist-tag with `npm view stele-spec dist-tags`.

Until 0.1.0 is final, no stable version exists, so also point `latest` at the new candidate after approving it: `npm dist-tag add stele-spec@<version> latest`. Otherwise a plain `npm install stele-spec` keeps resolving to an older candidate. Once a stable version is published, the workflow moves `latest` itself and this step is no longer needed.
