# Vision: OpenSpec stable-ID showcase

## What this is

A local Todo product and artifact dashboard that prove OpenSpec requirements can carry stable IDs and connect to implementation, tests, and execution evidence.

## Who it is for

An engineering lead or product engineer evaluating a spec-driven delivery workflow before adopting it in a larger product.

## Why now

The framework plan needs a concrete end-to-end proof. OpenSpec manages change artifacts well, while the Stele reference supplies stable identity and anchor ideas; this showcase tests the combination in a small application.

## What v1 must do

- Let a user create, complete, filter, and delete Todo items.
- Show OpenSpec proposal, design, task, and delta-spec artifacts in a dashboard.
- Trace every adopted requirement and scenario to code, tests, and execution state.
- Run proposal-stage and implementation-stage checks with a versioned JSON result.
- Start and validate locally with documented commands.

## Non-goals

- Production authentication, synchronization, multi-user storage, or cloud deployment.
- Personal identity verification.
- Daytona orchestration, PR approval automation, or package publication.
- Semantic proof that implementation behavior is correct solely because links resolve.

## Constraints

- Node.js 20.19 or newer, with OpenSpec pinned to 1.13.0.
- One OpenSpec canon; no second Stele requirement store.
- Local-first and dependency-light, with no database required.
- Requirement evidence must keep linked, executed, passed, and reviewed as separate states.

## Open questions

None for the showcase. Production package identity, dashboard hosting, and runner integrations remain later decisions in `PLAN.md`.
