---
name: stele-plan
description: Plans deterministic verification for OpenSpec scenarios. Use after drafting or changing an OpenSpec specification and before implementation.
---

# Plan Stele verification

Read the selected OpenSpec proposal, delta specs, and design before editing code.

For every requirement `Verification-ID`, choose the TypeScript or Go declaration expected to implement it and record the exact `path#selector` target in `openspec/changes/<change>/linkage-plan.json`. For Go, the selector is a function, `Type.Method`, or type name.

For every scenario `Verification-ID`, choose the lowest test level that proves the behavior: unit for pure logic, integration for a system boundary, and end-to-end for a critical complete user journey.

Record the scenario ID, test level, boundary, exact target, and a short rationale in the OpenSpec design. Use multiple evidence entries only when they prove distinct risks. Put the exact `path#selector` target in `openspec/changes/<change>/linkage-plan.json`, with `changeId` set to that change; the v0.1 linkage schema does not yet encode or enforce the richer evidence metadata.

Run `stele verify --stage proposal --json` before implementation.
