---
name: stele-verify
description: Runs deterministic Stele checks for OpenSpec IDs, code anchors, test anchors, and exact scenario execution. Use before review or when auditing spec-to-code fulfillment.
---

# Verify with Stele

Use the project's configured OpenSpec change. Requirements use `req.<namespace>.<token>` IDs and code anchors use `@implements <id>`. Scenarios use `scn.<namespace>.<token>` IDs and named tests use `@verifies <id>`.

Run `stele validate --json`. Treat exit code `0` as pass, `1` as a deterministic policy or test failure, and `2` as invalid invocation or tool failure.

Report linkage, execution, and human review as separate states. A resolved anchor is not evidence that a test executed or passed. Do not describe verification as successful unless every required scenario was selected and passed.
