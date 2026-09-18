# Product intent

Stele becomes your code in English.

A codebase should be readable as plain-English behavior. Every requirement and scenario in the specification links to the code that implements it and to the tests that prove it. Those links serve two equal purposes:

- **Verification.** Stele checks that every planned piece of evidence exists and that its tests pass for the current inputs.
- **Review and navigation.** People move between a sentence in the specification, the code that owns it, and the tests that prove it, in both directions. An editor extension can show the English behavior next to the code, and a review can show which behavior a change touches.

## Principles

- **Anchors are the source of truth for links.** `@implements` and `@verifies` comments sit next to the code and tests they describe. Stele reads them; it does not keep a second copy of where things live.
- **People decide how behavior is proven.** For every scenario, a person approves which levels of evidence prove it and why. Stele records who approved what, and notices when the behavior or the plan changes afterwards.
- **Stele never prescribes where code lives.** Placement follows the project's own conventions, from its `AGENTS.md`, `CLAUDE.md`, skills, and existing layout. Suggestions are advisory and never verified.
- **Deterministic results.** The same inputs produce the same reports and exit codes, so CI and people can trust them.
- **The specification backend is replaceable.** Stele builds on OpenSpec today, through an adapter, and keeps its own model independent of it.

## What a pass means

A pass says that every behavior is linked, every approved piece of evidence ran and passed, and nothing changed since approval. It does not say that the tests are good enough. That judgment stays with the people who review the change.
