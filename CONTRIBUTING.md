# Contributing

Start with `docs/intent/journeys.md` and the relevant OpenSpec change or baseline spec. Preserve existing `Verification-ID` values when behavior is renamed or modified; allocate a new ID for a new obligation.

Code that enforces a requirement needs `@implements req.…` near the enforcement point. A test exercising a scenario needs `@verifies scn.…` near the test. Run `npm run validate` before review.

Artifact, code, test, execution, and review states are reported independently. Do not describe a linked requirement as passed unless matching revision-bound execution evidence exists.
