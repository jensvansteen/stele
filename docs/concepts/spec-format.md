# Specification format

A Stele specification is an OpenSpec specification whose first line declares it:

```markdown
<!-- stele: spec v1 -->
## Purpose

Let a user keep a list of todos.
```

The annotation tells Stele and editors, from the first line alone, that the file is a Stele specification and which format version it uses. It is an HTML comment, so every Markdown renderer hides it, and OpenSpec accepts it: `openspec validate --strict` passes, and `openspec archive` keeps it when it merges a change into an annotated current specification.

Stele writes the annotation for you. `stele ids`, `stele init`, and the `stele` workflow schema's specification template add it, and `stele annotate` adds it to any file that lacks it. For a delta spec of a capability whose current specification declares targets, `stele ids` and `stele annotate` copy its `targets` field too.

## Grammar

The annotation is the first line of the file, after an optional UTF-8 byte order mark, with its line ending removed:

```text
line     = *WSP "<!--" *WSP "stele:" *WSP kind 1*WSP version *field *WSP "-->" *WSP
kind     = "spec"
version  = "v" 1*DIGIT
field    = *WSP ";" *WSP key *WSP ":" *WSP value
key      = %x61-7A *( %x61-7A / DIGIT / "-" )      ; lowercase
value    = <any characters except ";" and "--", surrounding WSP trimmed>
WSP      = SP / HTAB
```

- The canonical form is `<!-- stele: spec v1 -->`. Stele always writes exactly this line.
- Spaces and tabs are allowed around every token, and either line ending, LF or CRLF, is fine. `stele`, `spec`, `v`, and keys are lowercase.
- `spec` is the only kind. Other Stele-managed Markdown files may get their own kind later, with the same syntax.
- `v1` is the only supported version.

A first-line comment whose body starts with `stele:` claims to be an annotation. When the rest does not match the grammar, the file is malformed. Any other first-line comment is not an annotation.

### Fields

After the version, an annotation may carry fields, each written as `; key: value`. A value may contain commas for lists, but not `;`, which separates fields, or `--`, which HTML does not allow inside a comment:

```markdown
<!-- stele: spec v1; targets: vscode, zed, jetbrains -->
```

Version 1 defines one field, `targets`. Stele ignores every other field with a `SPEC_ANNOTATION_FIELD_IGNORED` warning, including a field that does not follow the form, such as `; owner`, and a repeated key. An ignored field never makes verification fail, so a field that a later Stele release adds never breaks an older one.

### The `targets` field

`targets` lists the [targets](/guide/targets) a specification applies to, as a comma-separated list of names that `stele.config.json` configures:

```markdown
<!-- stele: spec v1; targets: ios, android -->
```

Targets are optional. A specification without the field describes the project itself, exactly as before targets existed. With the field, every requirement and scenario applies to every listed target, unless it narrows them with a `Targets:` line:

```markdown
#### Scenario: Share through the iOS share sheet
Verification-ID: scn.share.4d5e6f7a8b9c
Targets: ios
```

- The `Targets:` line belongs in the metadata block directly below a heading, next to `Verification-ID:`, in either order.
- It may only narrow: a requirement's list is a subset of the file's targets, and a scenario's list a subset of its requirement's.
- It is metadata, so it is not part of the requirement or scenario text, the scenario steps, or an approval digest.

An empty list, a repeated name, an invalid name, or a second `targets` field or `Targets:` line is `SPEC_TARGETS_MALFORMED`. A name that the configuration does not know is `SPEC_TARGET_UNKNOWN`. Stele 0.1.0-rc.5 and older warn about the field and read targeted evidence IDs wrongly, so targeted projects need the release that ships targets.

### Versioning rule

Adding an optional field does not change the version. The version changes only when a version 1 reader would read the file wrongly, for example because of a new required field, or a different meaning for headings or IDs. A newer version is an error for an older Stele, because reading it as version 1 could produce a false pass.

## Recognizing the annotation in an editor

To recognize a Stele specification, an editor only needs to test the first line:

```text
^\uFEFF?[ \t]*<!--[ \t]*stele:
```

This pattern works as a TextMate `firstLineMatch`, a Zed `first_line_pattern`, or a JetBrains file-type check. Only validation needs the full grammar. `stele index` reports each file's annotation state and version, so an editor can also read them from the [link index](/reference/link-index).

## Diagnostics

Stele keeps reading and verifying every specification file of a scope, annotated or not, and never skips a file because it lacks an annotation. Spec parsing reports these diagnostics in `verify`, `validate`, and the proposal stage:

| Situation | Code | Severity |
|---|---|---|
| No annotation on the first line | `SPEC_ANNOTATION_MISSING` | The `unannotatedSpecs` policy |
| An annotation only on a later line | `SPEC_ANNOTATION_MISPLACED`, instead of `MISSING` | The `unannotatedSpecs` policy |
| A first-line `stele:` comment that does not match the grammar | `SPEC_ANNOTATION_MALFORMED` | Error |
| A version other than `v1` | `SPEC_ANNOTATION_UNSUPPORTED` | Error |
| An unknown, malformed, or repeated field other than `targets` | `SPEC_ANNOTATION_FIELD_IGNORED` | Warning |
| A malformed target list, or a second `targets` field or `Targets:` line | `SPEC_TARGETS_MALFORMED` | Error |
| A target name that `stele.config.json` does not configure | `SPEC_TARGET_UNKNOWN` | Error |
| A `Targets:` line that widens its parent, lies outside a metadata block, or appears without a `targets` field | `SPEC_TARGETS_WIDENED`, `SPEC_TARGETS_MISPLACED`, `SPEC_TARGETS_UNDECLARED` | Error |
| A delta spec whose targets differ from its current specification's | `SPEC_TARGETS_MISMATCH`, `SPEC_TARGETS_CHANGE_UNCOVERED` | Error |

A missing annotation names the command that adds it: `stele annotate --specs` for current specifications, and `stele annotate --change <change>` or `stele ids --change <change>` for a change. Archived changes are never checked.

Known limitation: detection is not yet aware of fenced code blocks. A line inside a ```` ``` ```` or `~~~` block that consists only of a Stele annotation, such as an example in the documentation of a specification, is still reported as `SPEC_ANNOTATION_MISPLACED`. A follow-up change makes the check fence-aware; until then, indent the example or add text to the line.

## The `unannotatedSpecs` policy

The `unannotatedSpecs` field of `stele.config.json` sets the severity of missing and misplaced annotations:

```json
{
  "schemaVersion": 1,
  "adapter": "openspec",
  "unannotatedSpecs": "error"
}
```

| Value | Effect |
|---|---|
| `warn` | A warning, which does not change the verdict |
| `error` | An error, which fails verification |

Without the field, Stele uses the release default: `warn` in 0.1.x. **From Stele 0.2.0 the default is `error`.** `warn` stays accepted after that as an explicit choice. Any other value makes every command that reads the configuration exit with code `2`.

To migrate an existing project, upgrade Stele and run `stele init`, or `stele annotate --all`. Then set `"unannotatedSpecs": "error"` to opt in to the 0.2.0 behavior early.

## Restoring the annotation after archiving

When archiving a change creates a new current specification, OpenSpec writes a fresh file that starts with `# <capability> Specification`, and the annotation is lost. A current specification that archiving merges into keeps its first line.

`stele annotate --specs` restores the lost annotations and changes nothing else. With `--targets-from openspec/changes/archive/<date>-<change>`, it also copies each capability's `targets` field from the archived delta spec, because OpenSpec keeps only the first line of a merged specification. The `stele-archive` skill runs it between `openspec-archive-change` and `stele validate --specs`, and the archive guidance that `stele init` adds to `openspec/config.yaml` names the same step for projects that use OpenSpec's skills directly. See [Use Stele with OpenSpec](/guide/openspec#run-the-stele-gates-around-openspec-s-skills).
