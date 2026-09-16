# Changelog

## 0.1.0-rc.1 — Release candidate

- Reimplemented deterministic verification, anchor scanning, report generation, scenario execution, OpenSpec orchestration, initialization, and the CLI in Go.
- Packaged native macOS and Linux binaries for ARM64 and x64, selected at installation without a runtime Node launcher.
- Added exact named TypeScript test selection for v0.1 consumers, race-enabled tests for the Go verifier, a strict coverage gate, `golangci-lint`, and Linux/macOS GitHub Actions.
- Added repository-local planning and verification skills to the package initializer.
- Added a VitePress documentation site with guides, concepts, CLI and architecture reference, and examples.
- Kept a plain Todo showcase in the independent `stele-examples` consumer repository; the verification dashboard is deferred.
- Defined v0.1 consumer support as TypeScript-first; Go and other consumer languages remain future adapter work.

## Prototype history

- Proved the OpenSpec and stable-ID workflow with the original Todo showcase, dashboard, recordings, deterministic JavaScript verifier, and local npm tarball installation.
