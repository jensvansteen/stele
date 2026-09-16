## 1. Self-verification setup

- [x] 1.1 Initialize OpenSpec with shared `.agents` skills and link `.claude/skills` to them; verify `openspec/config.yaml` and `.agents/skills/openspec-propose/SKILL.md` exist
- [x] 1.2 Pin the published `stele-spec` as the `stele-published` development dependency with a `stele:published` script; verify `npm exec -c 'command -v stele'` resolves to `node_modules/.bin/stele`
- [x] 1.3 Run `stele init --change baseline-verification-core` with the published package; verify `stele.config.json` and both Stele skills exist
- [x] 1.4 Track `artifacts/linkage-plan.json` while keeping other artifacts ignored; verify `git status` lists only the plan under `artifacts/`

## 2. Baseline plan

- [x] 2.1 Write delta specs for `init`, `verify`, `execution`, and `validate`, keeping the four existing `scn.verify.*` identities; verify `openspec validate baseline-verification-core --strict` passes
- [x] 2.2 Assemble the fixture annotations in `tests/cli.test.mts` from string parts; verify the published verifier reports no `ANCHOR_DANGLING`
- [x] 2.3 Write the proposed verification strategy and linkage plan; verify `npm run stele:published -- verify --stage proposal` passes
- [x] 2.4 Get reviewer approval for the proposed verification levels in design.md, and record the approval in design.md

## 3. Evidence after dependencies ship

- [ ] 3.1 After a published release selects `void test(...)` declarations, confirm the four Node scenarios execute with `npm run stele:published -- test`
- [ ] 3.2 After `add-go-support` ships in a published release, add `@implements` comments to the planned Go declarations and `@verifies` comments to the planned Go tests; verify `npm run stele:published -- verify` passes
- [ ] 3.3 Close the test gap for `scn.validate.d9553f1a4c1c` by asserting that the failing check is named; verify the test fails when the name is removed
- [ ] 3.4 Run `npm run stele:published -- validate`; verify it exits with `0`
