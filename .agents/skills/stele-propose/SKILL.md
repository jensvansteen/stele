---
name: stele-propose
description: Proposes a new change the Stele way, from a plain-English request to specifications with Verification-IDs and a verification plan. Use when the user wants to plan a new feature or change.
---

# Propose a change with Stele

Planning only: do not write implementation code.

1. Use the `openspec-propose` skill to create the change the user describes.
2. If the change uses the `stele` schema, its `verification` artifact has run the Stele planning step. Otherwise run `stele ids --change <change>`, follow the `stele-plan` skill, and run `stele verify --stage proposal --change <change>`.
3. Run `stele verify --stage proposal --change <change>` again and fix what it reports, as the `stele-plan` skill describes.
4. Stop and finish with: "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."
