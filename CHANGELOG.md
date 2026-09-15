# Changelog

## 0.1.0 — Unreleased

- Reimplemented deterministic verification, anchor scanning, report generation, scenario execution, OpenSpec orchestration, initialization, and the CLI in Go.
- Exposed the native Go executable directly through the npm package without a runtime Node launcher.
- Added exact named TypeScript test selection for v0.1 consumers, race-enabled tests for the Go verifier, a strict coverage gate, `golangci-lint`, and Linux/macOS GitHub Actions.
- Added repository-local planning and verification skills to the package initializer.
- Added a VitePress documentation site with guides, concepts, CLI and architecture reference, measured performance, examples, and the dashboard roadmap.
- Moved the Todo showcase and artifact dashboard into the independent `stele-examples` consumer repository.
- Defined v0.1 consumer support as TypeScript-first; Go and other consumer languages remain future adapter work.

## Prototype history

- Proved the OpenSpec and stable-ID workflow with the original Todo showcase, dashboard, recordings, deterministic JavaScript verifier, and local npm tarball installation.
