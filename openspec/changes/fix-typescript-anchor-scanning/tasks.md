## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels in design.md, and record it in design.md before starting section 1

## 1. Scanner

- [x] 1.1 Accept `void` and `await` prefixes in the test declaration pattern; verify `TestScanAnchorsResolvesExpressionTestCalls` and `TestScanAnchorsIgnoresOtherExpressionCalls` pass
- [x] 1.2 Add the comment-aware lexer and match annotations only in comment text; verify `TestScanAnchorsIgnoresTypeScriptStringLiterals` and `TestScanAnchorsReadsEveryCommentForm` pass and existing anchor tests still pass
- [ ] 1.3 Add `@verifies` comments for this change's planned tests; verify `npm run stele -- verify --change fix-typescript-anchor-scanning` passes with the local build

## 2. Documentation and gate

- [x] 2.1 Document the accepted test forms and the comment-only rule in the IDs and anchors page and the changelog; verify `npm run docs:build` passes
- [x] 2.2 Run `npm run verify`; verify it passes with exactly 100% core coverage
- [x] 2.3 With the local build, verify `npm run stele -- test --change baseline-verification-core` records the four `scn.verify.*` scenarios as passed
