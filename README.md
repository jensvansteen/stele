# Stele

Stele lets teams describe and maintain a codebase in plain English, then deterministically checks that every described behavior stays connected to implemented code and current, passing evidence.

[OpenSpec](https://github.com/Fission-AI/OpenSpec) is Stele's first specification foundation. Stele extends its requirements and scenarios with stable identities, planned code and test targets, exact execution, and reproducible evidence for CI and review. OpenSpec owns the behavioral source of truth; Stele verifies the evidence graph around it.

The verifier is written in Go and distributed through an npm package. npm handles installation and pins the bundled OpenSpec CLI; the installed `stele` command invokes the native binary directly.

[OpenSpec](https://github.com/Fission-AI/OpenSpec) can describe a codebase in any programming language. Stele v0.1 narrows its deterministic integration to TypeScript: it resolves anchors in `.ts`, `.tsx`, and `.mts` source and executes exact named `.ts` and `.mts` tests through Node. Additional execution environments will arrive through later adapters.

```text
OpenSpec requirement and scenarios
  → stable requirement and scenario IDs
  → planned code declaration and test selector
  → @implements and @verifies anchors
  → exact scenario test execution
  → deterministic evidence for CI and review
```

OpenSpec remains the behavioral source of truth. Stele reports planning, linkage, execution, and human review as separate states, so a resolved anchor is never mistaken for proof that the behavior works.

## Get started

Install Stele in the project that owns the OpenSpec change:

```bash
npm install --save-dev stele-spec
npx openspec init .
npx openspec new change my-change
npx stele init --change my-change
npx stele verify --stage proposal --json
# Add IDs, a linkage plan, implementation anchors, and test anchors.
npx stele validate
```

OpenSpec initialization and change creation are required native OpenSpec steps. `stele init` then adds Stele configuration and repository-local skills and selects the existing change; it does not replace OpenSpec's authoring workflow. `stele verify --stage proposal` checks that the proposed verification plan is complete before implementation starts. After the anchors and tests exist, `stele validate` runs the complete deterministic check.

The installed dependency pins Stele and its compatible OpenSpec CLI for both local use and CI. To test an unreleased checkout without publishing it, create and install a tarball as described in [Use a local package](docs/guide/local-package.md).

The independent [`stele-examples`](https://github.com/jensvansteen/stele-examples) repository proves this package boundary with a Todo application. It imports no source files from this checkout.

## Verification loop

1. Write the feature as OpenSpec requirements and concrete scenarios.
2. Add one immutable `req.<namespace>.<token>` ID to each requirement and one `scn.<namespace>.<token>` ID to each scenario.
3. Plan the source declaration and test selector for every ID.
4. Put `@implements <requirement-id>` beside the code declaration and `@verifies <scenario-id>` beside the named test.
5. Run `stele validate`. Stele checks OpenSpec, resolves the anchors, runs every scenario test by its exact selector, and binds the result to the relevant input digest.

A resolved anchor proves traceability. A passing execution proves the selected test ran. Human review still decides whether the code and test adequately satisfy the prose.

Continue with [Getting started](docs/guide/getting-started.md), or follow [Build a Todo feature](docs/guide/build-todo.md) from an OpenSpec prompt through implementation and evidence. [OpenSpec and Stele](docs/concepts/openspec-and-stele.md), [Plan verification evidence](docs/concepts/verification-evidence.md), and the [CLI reference](docs/reference/cli.md) explain the model in depth. The independent [Todo example](docs/guide/inspect-example.md) shows the completed application and dashboard without requiring you to build it first.

Development setup and repository architecture live in [Contributing](CONTRIBUTING.md).

## Current scope

Version 0.1 provides the Go verifier, OpenSpec adapter, native CLI, project initializer, repository-local skills, deterministic reports, and exact TypeScript test selection for TypeScript consumers. Cross-platform npm release packaging, additional language adapters, and the generated artifact dashboard are the next product milestones.
