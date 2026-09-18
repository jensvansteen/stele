# CLI reference

The npm package exposes the compiled Go executable directly as the `stele` command. Node remains part of the consumer toolchain because npm installs the package, OpenSpec runs on Node, and Stele executes exact named `.ts` and `.mts` tests through Node. `.tsx` declarations can carry anchors, but TSX test execution needs a future configurable runner adapter. Go consumers also need the Go toolchain: Stele resolves Go anchors itself and runs exact Go tests through `go test`.

## Global commands

```text
stele init [--change ID] [--tools TOOLS] [--refresh-schema] [--root PATH]
stele ids [--change ID] [--check] [--root PATH] [--json]
stele verify [--stage proposal|implementation] [--change ID | --specs] [--root PATH] [--report PATH] [--json]
stele test [--change ID | --specs] [--root PATH] [--evidence PATH] [--json]
stele validate [--change ID | --specs] [--root PATH] [--report PATH] [--evidence PATH] [--json]
stele help
stele version
```

## `stele init`

Prepares a project for Stele. Existing files are preserved.

1. When the project has no `openspec/` directory, runs the bundled `openspec init` for the tools in `--tools` and, for the default `agents` target, links `.claude/skills` to `.agents/skills` unless `.claude/skills` exists.
2. Forks OpenSpec's `spec-driven` schema into `openspec/schemas/stele`, unless that schema exists. The fork adds a `verification` artifact that generates `linkage-plan.json`, requires it before `tasks` and apply, and extends the apply instruction.
3. Selects `stele` as the default schema when `openspec/config.yaml` selects `spec-driven`, and merges Stele guidance into `operations.apply.guidance`, `operations.archive.guidance`, and `rules.verification`. Existing values and comments are kept; the file is not rewritten when nothing is missing. Existing changes keep their schema.
4. Writes `stele.config.json`, installs the `stele-propose`, `stele-apply`, `stele-archive`, `stele-plan`, and `stele-verify` skills in `.agents/skills/`, and ensures `artifacts/` exists.
5. Prints the default workflow and the Stele commands to run around OpenSpec's own skills.

| Option | Meaning |
|---|---|
| `--change ID` | Default OpenSpec change written to new configuration; optional |
| `--tools TOOLS` | Tools passed to `openspec init` when OpenSpec is missing; defaults to `agents` |
| `--refresh-schema` | Fork and patch the `stele` schema again, for example after an OpenSpec upgrade |
| `--root PATH` | Consumer project root; defaults to the current directory |

## `stele ids`

Inserts a `Verification-ID` line below every requirement and scenario heading of the change's delta specs that has none. Existing IDs and every other byte, including line endings and a missing final newline, are preserved. Headings in `REMOVED` and `RENAMED` sections are skipped, and `MODIFIED` headings reuse their ID from `openspec/specs/<capability>/spec.md`.

| Option | Meaning |
|---|---|
| `--change ID` | Override the configured change |
| `--check` | Write nothing; exit with code `1` while an ID is missing |
| `--json` | Print the inserted or missing IDs with their kind, title, file, and line |

## `stele verify`

Parses the selected OpenSpec change and validates identities, the linkage plan, and anchors.

| Option | Meaning |
|---|---|
| `--stage proposal` | Allow planned files and declarations that do not exist yet |
| `--stage implementation` | Require real anchors that match planned declarations; default |
| `--change ID` | Override the configured OpenSpec change |
| `--specs` | Verify the current specifications in `openspec/specs/` instead of a change |
| `--report PATH` | Write the deterministic verification report to this path |
| `--json` | Print canonical machine-readable JSON to standard output |

## `stele test`

Finds scenario test anchors, selects every named test independently, and writes execution evidence.

| Option | Meaning |
|---|---|
| `--change ID` | Override the configured change |
| `--specs` | Run the tests linked to the current specifications |
| `--evidence PATH` | Write test evidence to this path |
| `--json` | Print the evidence document to standard output |

## `stele validate`

Runs scenario tests, OpenSpec strict validation, and implementation verification as one gate. Default outputs are `artifacts/test-results.json` and `artifacts/verification-report.json`.

With `--specs`, OpenSpec validates all specifications with `openspec validate --specs --strict`.

## Scopes and linkage plans

Each run checks one scope: a change, selected with `--change` or the configured default, or the current specifications, selected with `--specs`. The two options cannot be combined.

| Scope | Specifications | Linkage plan |
|---|---|---|
| Change | `openspec/changes/<change>/specs/` | `openspec/changes/<change>/linkage-plan.json`, or `artifacts/linkage-plan.json` when the change has none |
| Current specifications | `openspec/specs/` | Every `linkage-plan.json` under `openspec/changes/archive/`, combined in archive order; a later archive wins for the same ID |

A plan whose `changeId` names a different change fails with `PLAN_CHANGE_MISMATCH`. Anchors for IDs declared elsewhere under `openspec/`, in another change or in the current specifications, do not affect the selected scope. An anchor fails with `ANCHOR_DANGLING` only when no specification declares its ID.

A scope must contain specifications. When the selected change has no delta specs, or `--specs` finds none in `openspec/specs/`, `verify`, `test`, and `validate` stop with exit code `2` before running tests or OpenSpec. A repository without archived changes therefore verifies with `--change` until its first `openspec archive`.

## Common options

`--root PATH` sets the consumer repository root for all filesystem resolution. CLI options override `stele.config.json`; omitted values fall back to configuration.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | All selected checks passed |
| `1` | A deterministic policy, linkage, OpenSpec, or selected-test check failed |
| `2` | The invocation was invalid or a required tool could not run |

Use the exit code for gates and JSON for explanation. Do not parse the human-readable summary.
