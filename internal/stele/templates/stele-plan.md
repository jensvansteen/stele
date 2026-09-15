---
name: stele-plan
description: Plans deterministic verification for OpenSpec scenarios. Use after drafting or changing an OpenSpec specification and before implementation.
---

# Plan Stele verification

Read the selected OpenSpec proposal, delta specs, and design before editing code.

For every scenario `Verification-ID`, add a planned test target. Choose the lowest test level that proves the behavior: unit for pure logic, integration for a system boundary, and end-to-end for a critical complete user journey.

Record the test level, boundary, required evidence, selector, and a short rationale. Use multiple evidence entries only when they prove distinct risks.

Run `stele verify --stage proposal --json` before implementation.
