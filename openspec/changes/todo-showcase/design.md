# Design: Todo verification showcase

## Context

OpenSpec remains the source of requirements and change artifacts. Stable `req.…` and `scn.…` IDs live inside requirement/scenario bodies so headings remain compatible with OpenSpec delta matching. Source and test comments carry explicit anchors. A standalone verifier joins these records and produces JSON for the dashboard.

## Architecture

The Node HTTP server owns an in-memory Todo store and an allowlisted artifact reader. It serves a dependency-free browser application with Todo and Verification views. The verifier reads only the selected OpenSpec change, scans configured implementation/test roots, and writes a report under `artifacts/`.

Validation has two stages:

1. Proposal mode requires valid, unique IDs and planned link declarations. Planned files may not exist.
2. Implementation mode requires real requirement code anchors and scenario test anchors. Execution state is read separately from the latest test-run record.

The dashboard invokes implementation validation through a local API and reads the resulting report. It does not decide approval policy or claim semantic correctness.

The repository-owned `stele` executable exposes three deterministic operations: `verify` checks the graph and anchors, `test` executes the test declaration attached to each scenario, and `validate` combines scenario execution, strict OpenSpec validation, and implementation verification. Default machine output omits wall-clock time. An optional future presentation layer may add volatile metadata outside the signed deterministic payload.

## Decisions

- Use Node.js ESM and browser-native JavaScript to keep the showcase easy to run.
- Pin OpenSpec `1.13.0` and run its strict validator independently.
- Keep Todo state in memory so no database obscures the workflow demonstration.
- Use a small JSON linkage plan for proposal-stage targets. It stores locations, not requirement prose.
- Treat linked, executed, passed, and reviewed as independent report dimensions.
- Resolve an anchor only when it is attached to a nearby code/test declaration and matches the planned repository path plus selector.
- Run each unique scenario-anchored test declaration with an exact test-name filter; map its outcome only to the scenarios attached to that declaration.
- Keep end-to-end video files in the tailnet recordings directory. Commit only an allowlisted manifest and proxy each selected recording through the dashboard origin with byte-range support for reliable HTTPS playback.
- Derive recording placement from the manifest's covered scenario IDs and render each unique video inside the corresponding expanded requirement evidence instead of maintaining a separate gallery.

## Boundaries

- Artifact API paths are allowlisted and resolved inside the project.
- The verifier never mutates or archives OpenSpec specs.
- Dashboard-triggered validation runs local deterministic checks; it does not execute arbitrary commands.
- Review status remains `not-reviewed` until supplied by a future external review system.
- Recording metadata is review evidence, not proof that every behavior was exercised or approved.
