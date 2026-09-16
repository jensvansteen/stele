## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels in design.md, and record it in design.md before starting section 1

## 1. Go anchor scanning

- [ ] 1.1 Classify `.go` and `_test.go` files under the source roots, `pkg`, and the repository root; verify with `TestScanAnchorsResolvesGoTestFunctions` and by removing the `.go` case from `TestScanAnchorsIgnoresUnsupportedConsumerLanguages`
- [ ] 1.2 Parse Go files and extract annotations only from comments; verify `TestScanAnchorsIgnoresGoStringLiterals` passes
- [ ] 1.3 Bind annotations to adjacent functions, methods, types, and test functions with the documented selectors; verify `TestScanAnchorsResolvesGoDeclarations` and `TestScanAnchorsLeavesUnattachedGoAnchorsUnresolved` pass
- [ ] 1.4 Stop verification with exit code `2` for unparsable Go files; verify `TestVerifyRejectsInvalidGoSource` passes

## 2. Go test execution

- [ ] 2.1 Dispatch `_test.go` targets to a Go runner that calls `go test -json -count=1 -run '^Name$'` from the nearest `go.mod`; verify `TestExecuteExactGoTestPasses` and `TestExecuteExactGoTestFails` pass
- [ ] 2.2 Map skip events to `test-skipped` and missing events to `test-not-executed`; verify `TestExecuteExactGoTestReportsSkip` and `TestExecuteExactGoTestDetectsNoMatchingExecution` pass
- [ ] 2.3 Derive `-tags` from the file's `//go:build` constraint; verify `TestExecuteExactGoTestAppliesBuildConstraints` passes
- [ ] 2.4 Add the mixed TypeScript and Go end-to-end test; verify `runs TypeScript and Go scenario tests together` passes in `npm run test:node`

## 3. Documentation and gate

- [ ] 3.1 Update the CLI reference, the architecture page, the IDs and anchors page, and the changelog to describe Go consumers; verify `npm run docs:build` passes
- [ ] 3.2 Add `@implements` and `@verifies` comments for this change's planned targets; verify `npm run stele -- verify --change add-go-support` passes with the local build
- [ ] 3.3 Run `npm run verify`; verify it passes with exactly 100% core coverage
- [ ] 3.4 After a release that contains this change, verify `npm run stele:published -- validate --change add-go-support` exits with `0`
