## Why

Stele reads every Markdown file under a change's `specs/` directory and every current specification, and it has no way to tell its own specification files apart from other OpenSpec specs. Editors have the same problem: to recognize a Stele specification and render it richly (hover cards, run buttons, status), VS Code, Zed, and JetBrains need a cheap, deterministic trigger at the top of the file. The earlier idea of a `.stele.md` file name was rejected, because OpenSpec owns the file layout and Stele does not push format changes upstream.

A marker on the first line of the file solves both problems and stays within OpenSpec's format:

```markdown
<!-- stele: spec v1 -->
```

OpenSpec 1.13 accepts it: `openspec validate --strict` passes, and `openspec archive` keeps it when it merges into a current specification whose first line is the marker. When archiving creates a new current specification, OpenSpec writes a fresh file and drops it, so Stele has to restore it.

## What Changes

- **Annotation grammar, version `v1`.** A Stele specification starts with `<!-- stele: spec v1 -->` on its first line. Whitespace inside the comment is tolerated. After the version, optional `; key: value` fields may follow. Version 1 defines no fields, so any field is ignored with a warning, never rejected. A later change, `verification-targets`, will add fields such as `targets: vscode, zed, jetbrains` without changing the version.
- **Recognition with a transition.** Stele keeps reading unannotated specifications, so existing projects keep working, and reports each one as `SPEC_ANNOTATION_MISSING`. A new `unannotatedSpecs` setting in `stele.config.json` chooses the severity: `warn`, the default until 0.2.0, or `error`. In 0.2.0 the default becomes `error`. Unannotated files are never silently ignored. Malformed annotations, annotations on a later line, and unsupported versions have their own codes.
- **`stele annotate [--change ID | --specs | --all] [--check] [--json]`.** A new command adds the annotation as the first line of every specification in the scope that lacks one. It preserves every other byte, never adds a second annotation, and leaves files with a malformed, misplaced, or unsupported annotation unchanged, naming them instead.
- **Annotation during authoring.** `stele ids` also adds the annotation to the change's delta specs. `stele init` annotates the current specifications and the delta specs of active changes, and the `stele` workflow schema's spec template starts with the annotation.
- **Repair after archiving.** The `stele-archive` skill runs `stele annotate --specs` after `openspec-archive-change` and before `stele validate --specs`. The archive guidance that `stele init` merges into `openspec/config.yaml` names the same step for projects that use OpenSpec skills directly.
- **Link index.** `stele index` lists every specification file with its annotation state and version, and each requirement and scenario records the format version of its file, so the language server and editors can use it.
- **Documentation.** A new "Specification format" concept page, updates to Getting started, the CLI reference, the link index reference, and the OpenSpec guide, and a changelog entry.

Out of scope: `targets` and any other annotation field (the coming `verification-targets` change), per-requirement metadata lines such as `Targets: a, b`, and editor rendering (planned in the `stele-editors` repository).

## Capabilities

### New Capabilities

- `spec-annotation`: the first-line annotation grammar and version, recognition policy and diagnostics, the `stele annotate` command, annotation during `ids` and `init`, repair after archiving, and the annotation state in the link index.

### Modified Capabilities

None. The behavior this change adds to `init`, `ids`, the lifecycle skills, the workflow schema, and the link index is specified in `spec-annotation` (see design.md, "Why one new capability").

## Impact

- `internal/stele`: an annotation parser, annotation diagnostics during spec parsing, the `unannotatedSpecs` configuration field, a new `annotate` command sharing the byte-preserving line editing of `stele ids`, annotation in `ids` and `init`, the `stele-archive` skill template, the schema and configuration patch in `openspec-extend.mjs`, and new link index fields.
- JSON output is additive: `stele ids --json` gains `annotations`, and the link index gains `specFiles` and `specVersion` on requirements and scenarios, without a schema version change.
- This repository annotates its own specifications with `stele annotate --all` once the command exists.
- Documentation: the new concept page, Getting started, the CLI and link index references, the OpenSpec guide, and the changelog.
- It builds on `authoring-workflow` (`ids`, lifecycle skills, workflow schema) and `link-index` (the index), both merged on main.
