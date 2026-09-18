## Context

See proposal.md. Relevant state on main:

- **Parsing.** The OpenSpec backend (`adapter.go`) lists a scope's spec files. For a change that is every Markdown file under `openspec/changes/<id>/specs/`, and for `--specs` every `openspec/specs/**/spec.md`. `parseSpecFiles` in `specs.go` reads them line by line. A line-1 HTML comment is already ignored, so annotated specs parse the same as before (the published rc.2 reads them normally too).
- **Byte-preserving edits.** `stele ids` (`ids.go`) plans every file edit first, then writes. It keeps line endings (`splitLinesKeepEnds`, `lineTerminator`) and a missing final newline.
- **Warnings.** Diagnostics carry a severity. Warnings do not fail a verdict (`diagnosticSummary`), and `PLAN_V1_DEPRECATED` sets the pattern for a deprecation that lasts until 0.2.0.
- **Configuration.** `stele.config.json` has `schemaVersion`, `adapter`, and `change`, and `readConfig` ignores unknown fields.
- **Lifecycle.** The `stele-archive` template runs `stele validate --change`, then `openspec-archive-change`, then `stele validate --specs`. `openspec-extend.mjs` patches the forked `stele` schema and merges the archive guidance into `openspec/config.yaml`.
- **Index.** `BuildIndex` (`index.go`) is a pure function over parsed scopes, and `ParsedSpecs.Files` lists each scope's spec files.

Tested with OpenSpec 1.13 on scratch copies:

- `openspec validate --strict` accepts the annotation on line 1.
- `openspec archive` of a change that **creates** a current specification writes a fresh file that starts with `# <capability> Specification`, and the annotation is lost. The archived copy of the change keeps it.
- `openspec archive` of a change that **merges** into a current specification whose line 1 is the annotation keeps it on line 1.
- Metadata lines such as `Targets: a, b` under requirement and scenario headings also pass strict validation. This is noted for `verification-targets` and not used here.

## Goals / Non-Goals

**Goals:**

- A deterministic, versioned marker that Stele and editors can read from the first line alone.
- A grammar that later changes can extend with fields without breaking version 1 readers.
- A transition in which existing projects keep working, with a documented cut-off.
- The marker is placed automatically wherever Stele or its skills write specs, and restored after archiving.
- Tools can read the annotation state from the link index.

**Non-Goals:**

- Any field meaning, including `targets` (the `verification-targets` change).
- Per-requirement or per-scenario metadata lines.
- Editor rendering or recognition code (the `stele-editors` repository).
- Changing OpenSpec, its templates, or its archive behavior. Stele does not push changes upstream.
- Annotating archived changes.

## Decisions

### 1. Grammar

The first line, after an optional UTF-8 byte order mark, with its line ending removed:

```abnf
line     = *WSP "<!--" *WSP "stele:" *WSP kind 1*WSP version *field *WSP "-->" *WSP
kind     = "spec"
version  = "v" 1*DIGIT
field    = *WSP ";" *WSP key *WSP ":" *WSP value
key      = %x61-7A *( %x61-7A / DIGIT / "-" )      ; lowercase
value    = <any characters except ";" and "--", surrounding WSP trimmed>
WSP      = SP / HTAB
```

- **Canonical form** is `<!-- stele: spec v1 -->`. Stele always writes exactly this line.
- **Tolerance.** Whitespace is tolerated around every token, but not inside tokens. Case is not tolerated: `stele`, `spec`, `v`, and keys are lowercase. Lowercase-only keeps the matcher trivial and identical in every editor.
- **Recognizing a Stele comment.** A first line whose comment body starts with `stele:` claims to be a Stele annotation. If the rest does not match, the line is `SPEC_ANNOTATION_MALFORMED`: the author clearly meant a Stele annotation. Other first-line comments are not annotations, and the file is unannotated.
- **Kind.** `spec` is the only kind. It leaves room for other Stele-managed Markdown files later (for example a design or plan kind) without a second marker syntax. Any other kind is malformed for a specification file.
- **Version.** `v1` is the only supported version. Any other `v<digits>` is `SPEC_ANNOTATION_UNSUPPORTED`, an error. A newer format may change how the file must be read, so reading it as v1 could produce a false green. The message tells the user to upgrade Stele.
- **Versioning rule** (documented for future changes): adding an optional field is not a version change. The version changes only when an existing version 1 reader would misread the file, for example a new required field, or different meaning for headings or IDs.
- **Fields.** A field is `; key: value`. Version 1 knows no keys, so every field is ignored with a `SPEC_ANNOTATION_FIELD_IGNORED` warning, and so is a field that does not parse (such as `; owner`). Field problems are never errors, so a future field, even one mistyped, never breaks a version 1 reader. That is the forward-compatibility guarantee. When `verification-targets` adds `targets`, older Stele versions warn about it and otherwise verify normally. Newer ones parse it. A repeated key is also ignored with this warning, after its first occurrence.
- **Values** exclude `;`, which separates fields, and `--`, which HTML forbids inside comments and which would risk ending the comment early. Lists inside a value use commas, as in `targets: vscode, zed, jetbrains`.
- **Misplaced.** A line other than the first that consists only of a Stele annotation (anywhere in the file, matched by the same grammar) is `SPEC_ANNOTATION_MISPLACED`. The file still counts as unannotated.
- **Rejected alternative: YAML front matter** (`---\nstele: spec/v1\n---`). OpenSpec's parser and some Markdown renderers treat front matter differently, and a first-line HTML comment is invisible in every renderer and accepted by OpenSpec as tested.
- **Rejected alternative: a JSON-like payload** in the comment. It is harder to type and to match in editor grammars (TextMate `firstLineMatch`, Zed `first_line_pattern`, JetBrains file-type detection), and needs escaping.

A simple regular expression, `^﻿?[ \t]*<!--[ \t]*stele:`, is enough as an editor trigger. The full grammar is only needed to validate.

### 2. Recognition policy

| Situation | Code | Severity |
|---|---|---|
| No annotation on line 1 | `SPEC_ANNOTATION_MISSING` | policy (`warn` by default, `error` when configured) |
| Annotation only on a later line | `SPEC_ANNOTATION_MISPLACED` (instead of MISSING) | policy |
| Line-1 `stele:` comment that does not parse | `SPEC_ANNOTATION_MALFORMED` | error |
| Unsupported version | `SPEC_ANNOTATION_UNSUPPORTED` | error |
| Unknown, malformed, or repeated field | `SPEC_ANNOTATION_FIELD_IGNORED` | warning |

- **Policy setting.** `stele.config.json` gains `"unannotatedSpecs": "warn" | "error"`. A missing field means the release default: `warn` through 0.1.x, and `error` from 0.2.0 on. Any other value exits with code `2` for every command that reads the configuration, naming the accepted values. `schemaVersion` stays `1` because the field is optional.
- **Unannotated files are reported, never ignored.** The brief allowed "ignored or reported" after the cut-off. Ignoring would turn a forgotten or lost annotation into silently dropped requirements. The most likely way to lose one is `openspec archive` creating a new current specification. That would give a green gate over less behavior, the false green Stele exists to prevent. Reporting keeps every requirement verified and makes the fix one command. Partial adoption, where some OpenSpec capabilities are not Stele-verified, is out of scope for 0.x (resolved question 1) and not a reason to ignore files by default.
- **Cut-off 0.2.0.** This matches the other 0.x deprecations (`PLAN_V1_DEPRECATED`, `--report` and `--evidence`). Projects can opt in early with `"error"`. After 0.2.0, `"warn"` stays accepted as a permanent explicit choice (resolved question 2).
- **Messages** name the fix: `stele annotate --specs` for current specifications, `stele annotate --change <id>` or `stele ids --change <id>` for a change.
- **Where diagnostics come from.** Spec parsing produces them. `verify` and `validate` report them in each scope, `--all` in every scope, and `stele verify --stage proposal` too, so the proposal check catches a change whose delta specs were written without `stele ids`. Archived changes are never parsed as a scope, so they are never checked.
- **Approval digests** hash scenario text only, so annotating a file never makes an approval stale. The evidence input digest covers spec files, so annotating marks recorded outcomes stale, as any spec edit does.

### 3. `stele annotate`, and not an `ids` mode

`stele annotate [--change ID | --specs | --all] [--root PATH] [--check] [--json]`. Without a scope flag it uses the configured change, like `stele ids`, and exits with code `2` when there is none.

- **Why a separate command.** Archive repair must write to current specifications. `stele ids --specs` would put identity assignment in reach of current specifications, and minting IDs there bypasses change review. That is the wrong default for a repair step an agent runs unattended after every archive. `annotate` has one job, touches only line 1, and is safe to run anywhere at any time. `stele ids` reuses the same insertion routine for its change, so the rules cannot drift.
- **Rejected alternative: an automatic repair inside `stele validate --specs`.** Validation stays read-only, which the determinism and CI story depends on.

**Insertion rules** (shared by `annotate`, `ids`, and `init`):

1. The file is classified first: annotated, missing, misplaced, malformed, or unsupported.
2. Only a `missing` file is edited. The canonical line is inserted at byte 0, or right after a byte order mark. It ends with the terminator of the file's first line (`\r\n` or `\n`), or `\n` when the file has no line ending at all. An empty file becomes the annotation plus `\n`.
3. No blank line is added after the annotation: a Markdown HTML block ends at `-->`, so a heading or `## Purpose` right below renders correctly. OpenSpec's merge keeps line 1 as tested.
4. Every other byte is preserved. The whole scope is planned before anything is written, as `stele ids` does, so an error while reading leaves every file untouched.
5. Files that are misplaced, malformed, or unsupported are never edited. Stele does not guess which line a person meant, and it does not move lines. Each is reported and the command exits with code `1`, but other files are still annotated. Leaving a fixable file broken because a neighbor is broken would only make the repair step fail more often.

**Output.** Human output names each annotated file and each problem. `--json` prints `{"schemaVersion": 1, "mode": "write"|"check", "verdict": "pass"|"fail", "files": [{"scope", "path", "state", "version", "changed"}]}`, where `state` is `annotated`, `missing`, `misplaced`, `malformed`, or `unsupported`. Files are sorted by scope order, then path. There are no timestamps.

**`stele ids`.** In write mode it adds the annotation in the same write as the IDs and shifts the reported ID line numbers by one when it does. With `--check`, a missing annotation is listed but fails the check only under the `error` policy. This keeps existing `ids --check` CI gates green during the transition, as promised. A malformed or unsupported annotation fails `ids` in both modes and leaves that file unchanged, because a newer format could have different ID rules. `--json` gains an `annotations` array (`path`, `state`, `version`, `changed`). The rest of the output is unchanged.

### 4. `stele init` and the workflow schema

- **Specification files.** `init` classifies and annotates the current specifications and the delta specs of every active change (`openspec/changes/*` except `archive/`). Each annotated file is listed with the files it creates, so a repeated run lists none and still reports "already initialized". This is how existing projects migrate: upgrade, then run `stele init` (or `stele annotate --all`). `init` does not fail because of a malformed annotation. It warns and names the file, because initialization should not be blocked by content that verification will report anyway.
- **Schema template.** `openspec-extend.mjs` prepends the canonical line to `openspec/schemas/stele/templates/spec.md` when it forks or refreshes the schema, and only if it is missing, so re-running it is idempotent. New delta specs created through OpenSpec's `specs` artifact then start annotated. `stele ids` still covers agents that drop template lines.
- `init` adds no flag to skip annotation (resolved question 3).

### 5. Archive repair

- **`stele-archive`**: 1. `stele validate --change <change>`, and stop on failure. 2. `openspec-archive-change`. 3. `stele annotate --specs`. 4. `stele validate --specs`. The skill stays an ordered list of CLI steps (`lifecycle`'s "keep skills thin" rule).
- **`openspec/config.yaml` archive guidance** gains "Stele: after archiving, run `stele annotate --specs`, then `stele validate --specs`." It replaces the current "after archiving, run `stele validate --specs`" entry. The merge recognizes the older entry and replaces it rather than adding a second one, so the merge stays idempotent for projects initialized with an earlier version.
- The annotation in the change's own delta spec is kept in the archived copy. Only the new current specification needs repair, and a merged one keeps its line 1 (tested above).

### 6. Link index

This change adds fields to link index schema version 1. The version does not change because existing readers ignore new fields, and the changelog notes the additions.

```json
{
  "specFiles": [
    { "scope": "spec-annotation", "path": "openspec/changes/spec-annotation/specs/spec-annotation/spec.md",
      "annotation": "annotated", "version": "v1" },
    { "scope": "specs", "path": "openspec/specs/verify/spec.md", "annotation": "missing", "version": null }
  ],
  "requirements": [ { "id": "req.…", "specVersion": "v1", "…": "…" } ],
  "scenarios":    [ { "id": "scn.…", "specVersion": null, "…": "…" } ]
}
```

- `specFiles` follows scope order, then path. `version` is the declared version for `annotated` and `unsupported` files, otherwise `null`. `specVersion` on items is `null` unless the file is `annotated`.
- The parser keeps the classification per file in `ParsedSpecs`, next to `Files`, so `BuildIndex` stays pure and a later `stele lsp` gets it without re-reading files. Field values are not exposed. `verification-targets` will decide how `targets` appear in the index.

### 7. Why one new capability

The behavior touches `init`, `ids`, `lifecycle`, `workflow-schema`, and `link-index`. On main, only `init` has a current specification, and its text is outdated: `authoring-workflow` rewrites the same requirement and is not archived yet. `ids`, `lifecycle`, `workflow-schema`, and `link-index` exist only in unarchived changes.

- MODIFIED deltas against them would either have no current specification to apply to, or be written against text that another pending archive replaces. The result would then depend on the order of archiving, which OpenSpec does not enforce.
- One new `spec-annotation` capability keeps each requirement next to the behavior it depends on, and is consistent with the requirements it builds on. The lifecycle scenario "Archive through Stele" still holds, because the skill still ends with `stele validate --specs`.
- `stele ids` therefore had no MODIFIED headings to reuse IDs for. Every ID in this change is newly derived.

### 8. This repository

After the command exists, the repository runs `stele annotate --all`. That annotates `openspec/specs/*` and the delta specs of active changes, including this one, whose delta spec is already hand-annotated as a dogfooding check that OpenSpec strict validation and the local and published verifiers accept it. The line-1 edits in other active changes' directories can cause trivial merge conflicts with the `-lsp` and `-link-index` worktrees, so the task notes it. `npm run verify:self` then shows no annotation warnings. This step is deferred until `chore/self-verify-rc3` merges (resolved question 4).

### Verification strategy

**Status: approved by jensvansteen on 2026-09-18, before implementation started (via: agent-confirmed, chat review).**

Placement follows this repository's AGENTS.md: co-located Go tests in `internal/stele`, with the new parser and command in `annotation.go` and their tests in `annotation_test.go`, and executable contract tests of the shipped binary in `tests/cli.test.mts`. Integration tests use the real bundled OpenSpec through the helpers in `workflowschema_test.go`, as the existing schema and merge tests do. Placement is advisory.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|
| `scn.specannotation.0de8bd2cbe54` Recognize the canonical annotation | unit | `….0de8bd2cbe54.unit` | `internal/stele/annotation_test.go`, beside a new `annotation.go` | Risk: the canonical line is not recognized, so every annotated file reads as unannotated. Parsing one line is pure. |
| `scn.specannotation.0d7c07caa518` Tolerate whitespace, line endings, and a byte order mark | unit | `….0d7c07caa518.unit` | `annotation_test.go` | Risk: a line an editor reformatted (tabs, CRLF, BOM) is rejected, or read differently from what editors accept. Pure parser over byte fixtures. |
| `scn.specannotation.6434ff2d493c` Ignore fields with a warning | unit | `….6434ff2d493c.unit` | `annotation_test.go` | Risk: the future `targets` field breaks version 1 readers, or unknown fields change verification. Pure parser plus diagnostics over a fixture file. |
| `scn.specannotation.c1a89d2bc235` Reject an unsupported version | unit | `….c1a89d2bc235.unit` | `annotation_test.go` | Risk: a newer format is read as v1 and verifies with the wrong meaning. Spec parsing and the diagnostic summary are pure over fixture files. |
| `scn.specannotation.10178af5c550` Reject a malformed annotation | unit | `….10178af5c550.unit` | `annotation_test.go` | Risk: a mistyped marker is silently treated as missing or as valid. Pure. |
| `scn.specannotation.2681b7fc2880` Report an annotation below the first line | unit | `….2681b7fc2880.unit` | `annotation_test.go` | Risk: a marker a person wrote below the heading looks valid to them, but editors ignore it. Pure scan over fixture lines. |
| `scn.specannotation.0f1fab2a02b8` Warn about an unannotated specification by default | unit | `….0f1fab2a02b8.unit` | `internal/stele/cli_test.go`, beside the validate command tests | Risk: the transition breaks existing projects' gates, or skips their requirements. `validate` runs in process with the test runner and OpenSpec validation stubbed, checking the warning, the verified requirements, and exit code `0`. |
| `scn.specannotation.3c8b5da99a7e` Fail on an unannotated specification when the policy requires it | unit | `….3c8b5da99a7e.unit` | `cli_test.go` | Risk: the `error` policy is not applied, so the cut-off never takes effect. `verify` runs in process with a fixture configuration. |
| `scn.specannotation.ee994e1a42b9` Reject an unknown policy value | unit | `….ee994e1a42b9.unit` | `cli_test.go`, beside configuration handling | Risk: a typo in the policy silently falls back to `warn`. Configuration loading in process. |
| `scn.specannotation.c1e6a83408c6` Annotate the files of a scope and preserve every other byte | unit | `….c1e6a83408c6.unit` | `annotation_test.go`, beside the `ids` byte-preservation fixtures | Risk: the insertion rewrites line endings or other bytes of a person's spec. Byte comparison over temporary fixture files. |
| `scn.specannotation.6bd9c9356646` Change nothing on a second run | unit | `….6bd9c9356646.unit` | `annotation_test.go` | Risk: a repeated run duplicates the marker. Two in-process runs over one temporary project. |
| `scn.specannotation.a518efc1ddd8` Check annotations without writing | unit | `….a518efc1ddd8.unit` | `cli_test.go`, beside the `ids` check tests | Risk: check mode writes, or the JSON states and exit code are wrong. Runs in process over a temporary project. |
| | e2e | `….a518efc1ddd8.e2e` | `tests/cli.test.mts`, next to "inserts and checks verification IDs" | Distinct risk: CI and the `stele-archive` skill call the shipped binary, whose flags, JSON, and exit code together decide the gate. Only the installed executable shows that. |
| `scn.specannotation.db2f08beff7a` Leave broken annotations for a person to fix | unit | `….db2f08beff7a.unit` | `annotation_test.go` | Risk: a broken marker gets a second marker added, or one bad file stops the others from being annotated. In process over fixtures. |
| `scn.specannotation.7a12f1bf3cf0` Add the annotation together with missing IDs | unit | `….7a12f1bf3cf0.unit` | `internal/stele/ids_test.go`, beside `TestIdsInsertsMissingIdentities` | Risk: reported ID line numbers are off by one after the annotation line, or the marker is added twice. Extends the `ids` fixtures in process. |
| `scn.specannotation.f0d8e6fbaa2d` Check a missing annotation according to the policy | unit | `….f0d8e6fbaa2d.unit` | `ids_test.go`, beside `TestIdsCheckWritesNothing` | Risk: the transition breaks existing `ids --check` gates, or the `error` policy is not enforced. In process with both policies. |
| `scn.specannotation.dfe775967c44` Annotate existing specifications once | unit | `….dfe775967c44.unit` | `internal/stele/init_test.go`, beside `TestInitializeIsIdempotent` | Risk: `init` edits archived history, or re-edits files on every run. `Initialize` runs in process with the OpenSpec setup stubbed, over a fixture with specs, an active change, and an archive. |
| `scn.specannotation.83d1cca317e3` Start new delta specs from an annotated template | integration | `….83d1cca317e3.integration` | `internal/stele/workflowschema_test.go`, beside `TestVerificationInstructionsReachOpenSpec` | Risk: the template patch does not reach what OpenSpec shows agents. Needs the real bundled OpenSpec fork and instructions output, as the existing schema tests do. |
| `scn.specannotation.3baa32066f7a` Archive through Stele restores the annotation | unit | `….3baa32066f7a.unit` | `init_test.go`, beside `TestArchiveSkillGatesOnValidation` | Risk: the skill omits the repair step or runs it after validation. The skill template is embedded text, checked in process like the existing lifecycle skill tests. |
| `scn.specannotation.23d533778e10` Direct OpenSpec archiving names the repair step | integration | `….23d533778e10.integration` | `workflowschema_test.go`, beside `TestMergeKeepsUserConfiguration` | Risk: the merged guidance lacks the step, keeps the old entry next to the new one, or the merge stops being idempotent. The merge runs `openspec-extend.mjs` with OpenSpec's `yaml` package, as the existing merge tests do. |
| `scn.specannotation.b7e05fe76741` Restore the annotation on a new current specification | integration | `….b7e05fe76741.integration` | `annotation_test.go`, using the `openSpec` helper from the schema tests | Risk: OpenSpec's archive output differs from what `annotate` expects (a new file without the marker, a merged file keeping it), leaving no marker or two. Only the real bundled `openspec archive` shows its output. `annotate` itself runs in process. |
| `scn.specannotation.4432e2c478c1` Index records the annotation of each file and item | unit | `….4432e2c478c1.unit` | `internal/stele/index_test.go` | Risk: editors get the wrong state or version for a file or item. `BuildIndex` is pure over parsed fixtures. The existing index e2e already covers the shipped binary's JSON output. |

Requirement implementation anchors are checked during implementation, not planned. Suggested homes:

- `annotation.go`: grammar, classification, insertion, and the `annotate` command;
- `specs.go`: diagnostics;
- `adapter.go`: the policy in `readConfig`;
- `ids.go`, `init.go`, and `index.go`;
- `templates/stele-archive.md` and `templates/openspec-extend.mjs`.

## Risks / Trade-offs

- [Warnings on every existing project after upgrading] → They are warnings until 0.2.0, each names a one-command fix, and `stele init` fixes them on its next run.
- [Tools that assume line 1 of a spec is `# <capability> Specification` or `## Purpose`] → OpenSpec 1.13 accepts the annotation, as tested. Future OpenSpec versions are covered by the pinned-version policy and by the integration evidence for archive repair, which fails on a behavior change.
- [An OpenSpec upgrade starts keeping, or moving, the annotation when it creates a current specification] → Keeping it is harmless because `annotate` is idempotent. Moving it below the heading shows up as `SPEC_ANNOTATION_MISPLACED` and in the integration evidence.
- [The `error` default in 0.2.0 breaks projects that never re-run `init`] → It is announced in the changelog now, and `"unannotatedSpecs": "warn"` keeps the old behavior.
- [Editing other active changes' files in this repository] → Only line 1 changes. Merge conflicts with other worktrees are trivial.
- [An agent writes a delta spec without the template and never runs `stele ids`] → The proposal check reports `SPEC_ANNOTATION_MISSING`, and the `stele-propose` flow already runs `ids`.

## Decided questions (approved by jensvansteen on 2026-09-18 with the levels)

- **Grammar**: `<!-- stele: spec v<N>[; key: value]* -->`, first line only, lowercase, tolerant of whitespace, a byte order mark, and CRLF.
- **Fields**: every field problem is a warning, never an error. Adding an optional field never changes the version.
- **Unsupported version and malformed head**: errors.
- **Policy**: report and never ignore. `unannotatedSpecs` is `warn` or `error`, with default `warn` until 0.2.0 and `error` from then on.
- **Command**: `stele annotate` with `--change`, `--specs`, `--all`, `--check`, and `--json`, instead of an `ids --specs` mode.
- **`ids --check`**: fails for a missing annotation only under `error`.
- **Index**: `specFiles` and `specVersion` are added within schema version 1.
- **Specs**: one new `spec-annotation` capability, with no MODIFIED deltas.

## Resolved questions

Decided by jensvansteen on 2026-09-18, via chat, following the maintainer's recommendations.

1. **Partial adoption.** Out of scope for 0.x. Every specification file in a scope stays Stele-managed; there is no `ignore` policy or path list.
2. **Removing `warn` after 0.2.0.** `warn` stays a permanent explicit option. The default becomes `error` in 0.2.0.
3. **`init --no-annotate`.** Not added now.
4. **Other active changes.** Deferred. This change does not run `stele annotate --all` on this repository, because `chore/self-verify-rc3` is archiving changes into `openspec/specs/` in parallel and the line-1 edits would conflict. Task 5.3 stays open: run it after `chore/self-verify-rc3` merges.
5. **Language server.** Out of scope here. The `SPEC_ANNOTATION_*` diagnostics and an "Add Stele annotation" code action are a follow-up for the `editor-language-server` plan.
