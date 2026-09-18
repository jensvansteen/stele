## 0. Approval

- [x] 0.1 Get reviewer approval for the proposed verification levels and open question in design.md, and record it in design.md before starting section 1

## 1. Backend seam

- [x] 1.1 Add `specificationBackend`, `resolveBackend`, and the OpenSpec implementation, and pass the backend through `options`; verify `TestBackendDefaultsToOpenSpec` and `TestUnknownAdapterIsRejected` pass
- [x] 1.2 Move OpenSpec paths, parsing, plan locations, validation, and installation behind the backend without behavior changes; verify `TestCommandsUseSelectedBackend` and the full existing test suite pass

## 2. Version drift

- [x] 2.1 Add file-based drift checks to `init`, `verify`, and `validate`; verify `TestDriftWarnsForOtherSkillVersions` and `TestDriftSilentWhenVersionsMatch` pass
- [x] 2.2 Add the `PATH` check to `init`; verify `TestInitWarnsForOtherOpenSpecOnPath` passes
- [x] 2.3 Add `--strict-versions` to `init`, `verify`, and `validate`; verify `TestStrictVersionsFailOnDrift` passes

## 3. Documentation

- [x] 3.1 Add `docs/concepts/model.md` with Stele's concepts, the OpenSpec mapping, and the credit "Stele currently builds on OpenSpec (MIT, Fission AI)"; verify `npm run docs:build` passes
- [x] 3.2 Add `docs/reference/versions.md` with the pin policy, drift warnings, and the OpenSpec bump checklist, link it from CONTRIBUTING's release steps, and add changelog entries; verify `npm run docs:build` passes

## 4. Gate

- [x] 4.1 Add anchors for this change's planned targets and run `npm run verify`; verify it passes with exactly 100% core coverage, `npm run stele -- validate --change specification-adapter` passes with the local build, and `npm run verify:self` still passes
- [x] 4.2 After a release that contains this change, archive it and verify `npm run verify:self` passes
