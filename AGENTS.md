# Stele product repository

This repository contains the reusable Stele verifier, npm distribution boundary, embedded project skills, tests, and documentation. Example applications and their OpenSpec changes belong in `stele-examples`.

The CLI implementation lives in `internal/stele`; `cmd/stele/main.go` stays a minimal composition root. Go tests use the standard co-located `*_test.go` convention. Do not commit generated `dist`, coverage, artifact, or documentation build output.

Run `npm run verify` before review. This gate runs the pinned Go lint policy, formatting, `go vet`, race-enabled tests, the exact 100% core coverage check, native build, CLI tests, and the external packed-package smoke test. Run `npm run docs:build` after documentation changes.

Preserve the boundary that OpenSpec owns behavior and Stele owns deterministic traceability and evidence. A resolved anchor, passing execution, and human review are separate states.
