# Package architecture

Stele is one npm package with a compiled core and a narrow npm installation boundary.

```text
stele-spec npm package
├── dist/stele             npm-exposed Go executable
├── internal/stele/        parser, scanner, runner, report model, CLI
├── cmd/stele/             Go executable entry point
├── OpenSpec dependency    pinned adapter runtime
└── embedded templates     stele-plan and stele-verify skills
```

## Deterministic Go core

The Go implementation owns parsing, identity validation, anchor scanning, linkage resolution, exact test selection, evidence generation, and exit codes. Static typing and compile-time checks make changes to the evidence model visible across the codebase, while the compiled binary keeps repeated local checks fast.

`cmd/stele/main.go` is intentionally a small composition root. The implementation remains one cohesive private package under `internal/stele`; Go convention places each `*_test.go` file beside the source it verifies, and those files are excluded from normal builds. Splitting parser, runner, report, and CLI packages now would expose internal plumbing and introduce shared-model dependencies without an independent reuse boundary. A dashboard server or second specification adapter would create a useful boundary and trigger that split.

## npm distribution

npm provides familiar installation and executable linking for TypeScript projects. The package exposes the compiled binary directly, removing a JavaScript process from every command invocation. Node remains necessary for npm, the bundled OpenSpec tool, and TypeScript test execution.

Version 0.1 supports TypeScript consumers. The Go core is an implementation and distribution choice; it does not imply support for anchors or tests in Go consumer repositories. Each additional consumer language needs an explicit declaration scanner and exact test-runner adapter.

The current `prepack` step builds for the machine creating the archive. A public cross-platform release should move each operating-system and architecture build into its own optional npm package or release artifact.

## OpenSpec adapter

The first adapter reads the standard OpenSpec change layout and invokes the pinned OpenSpec strict validator. OpenSpec remains the authority for proposal, design, task, requirement, scenario, and archive semantics.

This specification adapter is separate from language and test-runner adapters. It explains where behavior is declared; it does not inspect implementation code or execute tests.

## Current anchor scanner

`internal/stele/anchors.go` implements the fast TypeScript linkage pass for v0.1. It:

- walks configured source and test directories for supported TypeScript files;
- finds `@implements` and `@verifies` annotations containing a stable requirement or scenario ID;
- classifies each annotation as a code or test anchor from its file location;
- recognizes a nearby TypeScript declaration to capture a code symbol or exact test selector;
- normalizes paths and sorts the result before verification.

The scanner proves that an explicit anchor resolves to a nearby TypeScript declaration. It does not parse a complete abstract syntax tree, discover every exported symbol or route, infer behavior from source code, or prove that an unanchored implementation has a specification. Those broader checks and support for other consumer languages belong to planned language adapters.

## Skills

`stele init` writes versioned planning and verification skills into the consuming repository. Agents can read the same project-local workflow instructions without requiring a provider-specific service. The CLI remains the authority for deterministic results.

## Consumer boundary

Stele resolves all project paths from the consumer root. A packed-package integration test installs the archive into a fresh repository to detect accidental imports from the Stele source checkout.

## Why one package

Keeping the core, OpenSpec adapter, distribution metadata, and skill templates together avoids version skew while the model is still evolving. A second independent spec adapter would provide concrete evidence for extracting an adapter package later.
