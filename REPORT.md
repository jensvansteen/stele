# Version 0.3 implementation report

Stele now uses a statically typed Go core for specification parsing, anchor resolution, exact scenario-test execution, evidence generation, OpenSpec validation orchestration, initialization, and CLI behavior. npm remains the installer but exposes the native executable directly, avoiding a Node process on every command.

The verifier core has an enforced 100.0% statement-coverage gate. The complete check also runs `golangci-lint`, formatting verification, `go vet`, race-enabled tests, native compilation, CLI integration tests, and a tarball installation test in a clean consumer repository. GitHub Actions runs lint separately and verifies Linux and macOS.

The previous JavaScript verifier completed the standalone Todo linkage check in 60.41 ms median. Direct Go completed the same check in 17.18 ms median, a 3.52-times improvement. See [the benchmark report](docs/reference/performance.md) for method and limitations.

The VitePress documentation site covers onboarding, OpenSpec ownership, IDs and anchors, test-level planning, deterministic evidence, CLI behavior, package architecture, examples, measured performance, and the dashboard roadmap. It builds statically and was verified in Safari against a live local server; that pass also corrected code-block contrast.

The Todo application, its OpenSpec change, artifact dashboard, and recordings now belong to the independent `stele-examples` consumer repository. Stele contains only reusable product code, package assets, quality tooling, and documentation.
