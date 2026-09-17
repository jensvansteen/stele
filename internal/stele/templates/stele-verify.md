---
name: stele-verify
description: Reference for Stele anchors and deterministic checks of OpenSpec IDs, code anchors, test anchors, and exact scenario execution. Used by stele-apply and stele-archive, and before review.
---

# Verify with Stele

Requirements use `req.<namespace>.<token>` IDs and code anchors use `@implements <id>`. Scenarios use `scn.<namespace>.<token>` IDs and named tests use `@verifies <evidence-id>`, where the evidence ID is the scenario ID plus the planned level, such as `scn.todo.591a3b429cf0.unit`.

Put each anchor in a comment directly above the declaration or named test it belongs to, wherever the project's conventions place that code.

Run `stele validate --change <change>` while a change is in progress, and `stele validate --specs` for archived behavior. Exit code `0` means pass, `1` means a policy or selected-test failure, and `2` means an invocation or tool failure.

Keep linkage, execution, and human review as separate states. Never report success unless every required scenario was selected and passed.
