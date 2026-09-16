# Changelog

## Unreleased

- Added Go consumer support: anchors in Go comments resolve to functions, methods, types, and `TestXxx` functions, and linked Go tests run individually with `go test -json -count=1`, honoring `//go:build` tags. Skipped Go tests count as failed with reason `test-skipped`, and unparsable Go files stop verification.
- A scenario linked to several tests now passes only when every one of them passes; previously the last execution decided.
- The evidence digest now also covers `pkg/` and Go files in the repository root.
- Added `--specs` to `verify`, `test`, and `validate` to keep verifying behavior after `openspec archive`, using the combined plans of archived changes.
- Each change now reads its own `openspec/changes/<change>/linkage-plan.json`, falling back to `artifacts/linkage-plan.json`. **Breaking:** a plan whose `changeId` names another change now fails with `PLAN_CHANGE_MISMATCH`; move each change's entries into its own directory.
- Anchors for IDs declared by other changes or by the current specifications no longer count as `ANCHOR_DANGLING`.
- Fixed test anchors on `void test(...)` and `await test(...)` calls, the form used in the Build a Todo guide, which previously reported `ANCHOR_TARGET_MISSING`.
- TypeScript annotations are now read only from comments. Anchor text inside string and template literals no longer creates anchors.

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
