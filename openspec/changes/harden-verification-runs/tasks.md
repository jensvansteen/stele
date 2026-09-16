## 0. Approval

- [ ] 0.1 Get reviewer approval for the proposed verification levels in design.md, and record it in design.md before starting section 1

## 1. Empty scopes

- [ ] 1.1 Add `requireScopeSpecs` and call it at the start of verification and scenario execution; verify `TestRunRejectsMissingChange`, `TestRunRejectsChangeWithoutSpecs`, and `TestRunRejectsEmptyCurrentSpecs` pass
- [ ] 1.2 Update existing fixtures that relied on empty scopes; verify `npm run verify` passes with exactly 100% core coverage

## 2. Node test isolation

- [ ] 2.1 Start linked Node tests with `nodeTestEnvironment`, which drops `NODE_TEST_CONTEXT`; verify `TestExecuteNodeTestIgnoresEnclosingTestRunner` passes
- [ ] 2.2 Remove the environment workaround from `tests/cli.test.mts`; verify `runs TypeScript and Go scenario tests together` still passes in `npm run test:node`

## 3. Documentation and gate

- [ ] 3.1 Document the empty-scope exit code in the CLI reference and add changelog entries; verify `npm run docs:build` passes
- [ ] 3.2 Add `@implements` and `@verifies` comments for this change's planned targets; verify `npm run stele -- validate --change harden-verification-runs` passes with the local build
- [ ] 3.3 Archive the change once it is released, and verify `npm run verify:self` passes
