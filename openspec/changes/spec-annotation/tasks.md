## 0. Approval

- [x] 0.1 Show the verification table and the decided questions in design.md, get explicit approval, then run `stele approve --change spec-annotation --confirmed-in-chat` and mark the design table approved; verify `stele verify --stage proposal --change spec-annotation` reports no `PLAN_UNAPPROVED`
- [x] 0.2 Resolve or defer the open questions in design.md and record the answers there before starting section 1

## 1. Grammar and recognition

- [x] 1.1 Add the annotation parser and file classification (annotated, missing, misplaced, malformed, unsupported) with field warnings; verify the unit tests for `scn.specannotation.0de8bd2cbe54`, `0d7c07caa518`, `6434ff2d493c`, `c1a89d2bc235`, `10178af5c550`, and `2681b7fc2880` pass
- [x] 1.2 Report the `SPEC_ANNOTATION_*` diagnostics during spec parsing, keep the classification per file in `ParsedSpecs`, and add `unannotatedSpecs` (`warn` by default, `error`, otherwise exit `2`) to the configuration; verify the unit tests for `0f1fab2a02b8`, `3c8b5da99a7e`, and `ee994e1a42b9` pass

## 2. Placement

- [x] 2.1 Add `stele annotate [--change | --specs | --all] [--check] [--json]` with the shared byte-preserving insertion; verify the unit tests for `c1e6a83408c6`, `6bd9c9356646`, `a518efc1ddd8`, and `db2f08beff7a`, and the e2e test for `a518efc1ddd8` in `npm run test:node`, pass
- [x] 2.2 Make `stele ids` add the annotation with correct ID line numbers, apply the policy in `--check`, and add `annotations` to `--json`; verify the unit tests for `7a12f1bf3cf0` and `f0d8e6fbaa2d` and the existing `ids` tests pass
- [x] 2.3 Make `stele init` annotate current specifications and active changes, and prepend the annotation to the `stele` schema's spec template in `openspec-extend.mjs`; verify the unit test for `dfe775967c44` and the integration test for `83d1cca317e3` pass

## 3. Archive repair

- [x] 3.1 Add `stele annotate --specs` to the `stele-archive` template and replace the archive guidance entry in `openspec-extend.mjs` idempotently; verify the unit test for `3baa32066f7a`, the integration test for `23d533778e10`, and the updated `TestArchiveSkillGatesOnValidation`, `TestLifecycleSkillsStayThin`, and merge tests pass
- [x] 3.2 Add the archive repair integration test with the real bundled `openspec archive` (one new and one merged capability); verify the integration test for `b7e05fe76741` passes

## 4. Link index

- [x] 4.1 Add `specFiles` and `specVersion` to the index within schema version 1; verify the unit test for `4432e2c478c1` and the existing index tests and e2e tests pass

## 5. Documentation, dogfooding, and gate

- [x] 5.1 Add the concept page `docs/concepts/spec-format.md` (grammar, version rule, fields, editor trigger pattern, recognition policy and the 0.2.0 cut-off, diagnostic codes, and repair after archiving), link it from the sidebar and the Stele model table, and update Getting started (annotation in the first spec, `stele init` migration), the CLI reference (`annotate`, `ids` output, `unannotatedSpecs`), the link index reference, and the OpenSpec guide (archive step); verify `npm run docs:build` passes
- [x] 5.2 Add the changelog entry under Unreleased: `stele annotate`, the annotation grammar and `SPEC_ANNOTATION_*` codes, `unannotatedSpecs` with the announced 0.2.0 default of `error`, annotation in `ids` and `init`, the schema template, the `stele-archive` step, and the additive `ids --json` and index fields; verify it names the migration command
- [x] 5.3 Run `stele annotate --all` in this repository and commit the line-1 edits separately; verify `npm run verify:self` and `npm run stele -- validate --all` report no `SPEC_ANNOTATION_*` diagnostic. Deferred: run after chore/self-verify-rc3 merges (design.md, resolved question 4).
- [x] 5.4 Add `@implements` anchors for every requirement and `@verifies` anchors for every approved evidence entry, then run `npm run verify`; verify it passes with exactly 100% core coverage and `npm run stele -- validate --change spec-annotation` passes
- [ ] 5.5 After a release that contains this change, archive it with the `stele-archive` skill; verify its new current specification starts with the annotation and `npm run verify:self` passes
