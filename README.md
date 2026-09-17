# Stele

Stele lets teams describe and maintain a codebase in plain English, then deterministically checks that every described behavior stays connected to implemented code and current, passing evidence.

[Read the published Stele documentation](https://jensvansteen.github.io/stele/)

[OpenSpec](https://github.com/Fission-AI/OpenSpec) is Stele's first specification foundation. Stele extends its requirements and scenarios with stable identities, planned code and test targets, exact execution, and reproducible evidence for CI and review. OpenSpec owns the behavioral source of truth; Stele verifies the evidence graph around it.

The verifier is written in Go and distributed through an npm package. npm handles installation and pins the bundled OpenSpec CLI; the installed `stele` command invokes the native binary directly.
The package includes binaries for macOS and Linux on ARM64 and x64; installation selects the matching binary.

[OpenSpec](https://github.com/Fission-AI/OpenSpec) can describe a codebase in any programming language. Stele integrates deterministically with TypeScript and Go: it resolves anchors in `.ts`, `.tsx`, `.mts`, and `.go` source, executes exact named `.ts` and `.mts` tests through Node, and runs exact Go tests through `go test`. Additional execution environments will arrive through later adapters.

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

Install the release candidate and let Stele set up the project:

```bash
npm install --save-dev stele-spec@next
npx stele init
```

`stele init` initializes OpenSpec with the bundled, pinned CLI when the project has none, adds the Stele workflow schema and guidance to OpenSpec, and installs the Stele skills. Then ask your coding agent to plan a change with the `stele-propose` skill:

```text
Use stele-propose to plan a change that lets a user add a todo from non-empty text.
```

The agent writes the OpenSpec proposal, specifications with Verification-IDs (`stele ids`), a design with a verification table, and a linkage plan, and stops for your review. `stele-apply` asks you to confirm the verification levels before it implements anything and finishes with `stele validate --change <change>`. `stele-archive` archives the change only after validation passes.

Prefer OpenSpec's own skills? They keep working: the `stele` schema adds the planning step to new changes. See [Use Stele with OpenSpec](docs/guide/openspec.md).

The installed dependency pins Stele and its compatible OpenSpec CLI for both local use and CI. For reproducible CI builds, pin the exact release candidate version. To test an unreleased checkout without publishing it, create and install a tarball as described in [Use a local package](docs/guide/local-package.md).

The independent [`stele-examples`](https://github.com/jensvansteen/stele-examples) repository proves this package boundary with a Todo application. It imports no source files from this checkout.

## Verification loop

1. Write the feature as OpenSpec requirements and concrete scenarios.
2. Run `stele ids` to give each requirement an immutable `req.<namespace>.<token>` ID and each scenario an `scn.<namespace>.<token>` ID.
3. Plan the source declaration and test selector for every ID.
4. Put `@implements <requirement-id>` beside the code declaration and `@verifies <scenario-id>` beside the named test.
5. Run `stele validate`. Stele checks OpenSpec, resolves the anchors, runs every scenario test by its exact selector, and binds the result to the relevant input digest.

A resolved anchor proves traceability. A passing execution proves the selected test ran. Human review still decides whether the code and test adequately satisfy the prose.

Continue with [Getting started](docs/guide/getting-started.md), or follow [Build a Todo feature](docs/guide/build-todo.md) from an OpenSpec prompt through implementation and evidence. [OpenSpec and Stele](docs/concepts/openspec-and-stele.md), [Plan verification evidence](docs/concepts/verification-evidence.md), and the [CLI reference](docs/reference/cli.md) explain the model in depth. The independent [Todo example](docs/guide/inspect-example.md) shows a completed application without requiring you to build it first.

Development setup and repository architecture live in [Contributing](CONTRIBUTING.md).

## Current scope

Version 0.1 provides the Go verifier, OpenSpec adapter, native CLI, project initializer, repository-local skills, deterministic reports, and exact test selection for TypeScript and Go consumers.
