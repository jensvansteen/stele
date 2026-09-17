---
name: stele-apply
description: Implements a planned change the Stele way, after the user confirms the verification levels. Use when the user wants to apply, implement, or continue a change.
---

# Apply a change with Stele

1. Run `stele verify --stage proposal --change <change>`.
2. Follow the approval step in the `stele-plan` skill: show the verification levels, ask the user to confirm them, and record the confirmation only after an explicit yes. Never treat the request to apply as confirmation.
3. Use the `openspec-apply-change` skill to implement the tasks.
4. Add `@implements` and `@verifies` anchors while writing code and tests, as the `stele-verify` skill describes.
5. Run `stele validate --change <change>` and fix failures before reporting the change done.
