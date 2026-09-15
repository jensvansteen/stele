---
name: stele-plan
description: Plans deterministic verification for OpenSpec scenarios. Use after drafting or changing an OpenSpec specification and before implementation.
---

# Plan Stele verification

Read the selected OpenSpec proposal, delta specs, and design before editing code.

For every scenario `Verification-ID`, add one planned test target to `artifacts/linkage-plan.json`. Choose the lowest test level that proves the behavior:

- `unit` for pure logic without I/O or side effects
- `integration` when behavior crosses an API, database, filesystem, process, or component boundary
- `e2e` for a critical complete user journey

Record a short, reviewable rationale based on the boundary and risk. Do not require every scenario to have every test level. Use multiple evidence entries only when they prove distinct risks or facets.

Run `stele verify --stage proposal --json`. Do not begin implementation until it exits successfully.
