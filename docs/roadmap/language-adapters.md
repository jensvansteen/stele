# Language adapter roadmap

Stele should add deeper code inspection after the installable package and anchor-to-test workflow are proven in real consumer projects. The adapter boundary is documented now so the deterministic core can grow without mixing language-specific behavior into its policy engine.

## What exists today

The current verification path is intentionally small:

1. The OpenSpec adapter reads requirements and scenarios.
2. The anchor scanner finds explicit `@implements` and `@verifies` references.
3. The verifier resolves those anchors against the linkage plan.
4. The exact test runner executes each named TypeScript test through Node.
5. The evidence writer emits the canonical result.

This v0.1 path is supported for TypeScript consumers only. Go is used to implement the Stele executable, but Go source and tests in a consumer repository are not part of the current product contract.

This proves that a declared scenario has an implementation link and passing test evidence. It does not prove that every public API in the repository was declared in a specification.

## Two adapter responsibilities

Language audit adapters and test-runner adapters solve different problems.

| Adapter | Input | Output | Question answered |
| --- | --- | --- | --- |
| Language audit | Source files and build metadata | Normalized public-surface inventory | Is public implementation present without a specification contract? |
| Test runner | Test path and exact selector | Normalized execution evidence | Did this exact linked test run and pass? |

The OpenSpec adapter remains a third, separate boundary. It provides the expected behavioral contract and lifecycle; language adapters must not reinterpret that behavior.

## Deterministic surface comparison

A language adapter should use a parser, compiler metadata, or a framework's stable route table to extract the actual public surface. Stele compares that result with an explicit expected contract, never with an AI interpretation of prose.

Examples of expected and actual inventories include:

| Surface | Expected contract | Actual extraction |
| --- | --- | --- |
| REST API | OpenAPI or structured OpenSpec metadata for methods, paths, schemas, and responses | Framework route definitions and handler signatures |
| Library | Declared exported names and signatures | Compiler or syntax-tree export data |
| CLI | Commands, flags, arguments, and exit behavior | Command registration metadata |

Both sides are converted to a versioned schema, normalized, and sorted. The comparison reports missing, changed, and undocumented items. Given the same files, configuration, and adapter version, it must produce byte-for-byte identical canonical output.

Tests remain necessary. A surface comparison proves that an endpoint or symbol matches the declared shape; linked tests prove its behavior.

The verification plan may require behavioral tests, static or runtime conformance checks, and measurements for different facets of one scenario. The shared policy is defined in [Verification evidence](/concepts/verification-evidence); adapters only collect the named evidence.

## Core boundary

The Stele core should own:

- claim and scenario identity;
- verification policy;
- normalized contract and evidence schemas;
- deterministic comparison and ordering;
- diagnostics, exit codes, and report generation.

An adapter should only discover a surface or execute an exact test and return normalized data. It must not decide whether a requirement is adequate, change OpenSpec files, calculate the final verdict, or use an LLM in the verification path.

Start with a typed Go interface and a registry of compiled built-in adapters. Avoid Go runtime plugins because they complicate portability and distribution. If external adapters become necessary, add a versioned subprocess protocol that exchanges canonical JSON and validates the adapter version and output schema.

## Repository strategy

Keep the first official adapters in the Stele repository while the interface and evidence schema are still changing. One repository gives them the same fixtures, determinism tests, coverage gate, release version, and compatibility checks as the core.

Do not make the main binary the permanent home for every ecosystem integration. Move an adapter to its own repository or package when it has a heavy language runtime, framework-specific dependencies, a different release cadence, or an independent maintainer. External adapters should be standalone executables that implement the versioned canonical-JSON subprocess protocol; the Stele core should never load arbitrary Go plugins into its process.

This produces a staged hybrid model:

1. build and stabilize the TypeScript and first contract-surface adapters in this repository;
2. freeze a small adapter input/output protocol with compatibility fixtures;
3. extract specialized adapters only when their dependency or ownership boundary is real;
4. keep protocol conformance tests in Stele and run them against every official adapter release.

## Priority after TypeScript

Prioritize adapters by user reach, exact test-selection support, parser quality, and the new product boundary each one proves.

1. **JavaScript** extends the current TypeScript and Node family with the smallest new surface. It can share Node test execution and most declaration semantics while proving that Stele can support an untyped codebase deliberately.
2. **Python** is the first truly separate ecosystem and the strongest next reach into AI, data, automation, and backend projects. Python exposes a standard syntax tree, and pytest provides exact node IDs for selecting a module, class, method, function, or parameterized case.
3. **Go** provides a clean compiled-language adapter using the standard parser and syntax-tree packages plus exact `go test -run` selection. It also lets Stele dogfood its adapter against its own implementation.
4. **Swift** adds iOS, macOS, and server-side Swift projects. SwiftSyntax provides a source-accurate syntax tree, and Xcode can select one test by identifier. The adapter should support Swift Package Manager first, then add Xcode schemes, test plans, XCTest, and Swift Testing on macOS runners.
5. **JVM**, beginning with Java and then Kotlin, reaches large enterprise systems. Java and Kotlin need separate declaration parsers but can share build-tool discovery, classpath handling, and JUnit Platform execution evidence.
6. **C# and .NET** follow the JVM adapter. Roslyn supplies a strong compiler-backed surface, but project discovery, target frameworks, and test-provider differences make it a larger integration.

Rust is a strong later candidate once the core adapter protocol is stable. Its deterministic compiler and Cargo test model are attractive, but the earlier adapters cover broader immediate demand and more varied integration boundaries.

The target language does not dictate the implementation language of the adapter. Use the most authoritative deterministic interface available: a compiler API, standard syntax tree, framework metadata, or a small target-runtime helper. Every adapter still returns the same versioned canonical JSON to the Go core.

## Delivery order

1. Keep the current TypeScript anchor and exact-test verification path stable.
2. Finalize the normalized contract inventory and adapter interfaces.
3. Add JavaScript within the existing Node adapter family.
4. Prove the external adapter protocol end to end with Python.
5. Add fixtures proving stable output and drift detection.
6. Expose the audit separately from the fast verification command.

The first deeper adapter should extend the TypeScript path around a demonstrated surface, such as framework API routes. A later consumer-language adapter may support exported Go symbols and exact Go tests. Stele should not build parsers for every language before a consumer establishes the required surface and framework semantics.
