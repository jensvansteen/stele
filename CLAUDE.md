## Stele-derived OpenSpec project

This project follows the OpenSpec lifecycle and adds Stele-derived stable behavioral IDs. OpenSpec requirement and scenario blocks are the single canon. Requirements use `req.<namespace>.<token>` IDs; scenarios use `scn.<namespace>.<token>` IDs. Code anchors requirements with `@implements <id>` and tests anchor scenarios with `@verifies <id>`.

Before implementing behavior, read the selected OpenSpec change under `openspec/changes/`. The Stele reference methodology lives at `docs/.stele/METHODOLOGY.md`; project-specific adaptations are defined in `PLAN.md` and take precedence where the old methodology assumes `docs/spec` claim tables.

Run `npm run verify:proposal` before implementation planning, then `npm run validate` before review. Use `npm run stele -- verify --json` for deterministic machine output. Proposal mode allows planned links. Implementation mode requires anchors attached to compatible declarations at the planned file and selector. Scenario execution selects every anchored named test independently. A resolved link is not proof of execution, passing behavior, or human review; keep those states separate.
