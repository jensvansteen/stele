---
name: stele-verify
description: Runs deterministic Stele checks for OpenSpec IDs, code anchors, test anchors, and exact scenario execution. Use before review or when auditing spec-to-code fulfillment.
---

# Verify with Stele

Requirements use `req.<namespace>.<token>` IDs and code anchors use `@implements <id>`. Scenarios use `scn.<namespace>.<token>` IDs and named tests use `@verifies <id>`.

Run `stele validate --json`. Exit code `0` means pass, `1` means a policy or selected-test failure, and `2` means an invocation or tool failure.

Keep linkage, execution, and human review as separate states. Never report success unless every required scenario was selected and passed.
