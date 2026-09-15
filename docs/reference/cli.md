# CLI reference

The npm package exposes the compiled Go executable directly as the `stele` command. Node remains part of the consumer toolchain because npm installs the package, OpenSpec runs on Node, and Stele can execute Node tests.

## Global commands

```text
stele init [--change ID] [--root PATH]
stele verify [--stage proposal|implementation] [--change ID] [--root PATH] [--report PATH] [--json]
stele test [--change ID] [--root PATH] [--evidence PATH] [--json]
stele validate [--change ID] [--root PATH] [--report PATH] [--evidence PATH] [--json]
stele help
stele version
```

## `stele init`

Creates `stele.config.json`, installs the `stele-plan` and `stele-verify` repository skills, and ensures `artifacts/` exists. Existing files are preserved.

| Option | Meaning |
|---|---|
| `--change ID` | Default OpenSpec change written to new configuration |
| `--root PATH` | Consumer project root; defaults to the current directory |

## `stele verify`

Parses the selected OpenSpec change and validates identities, the linkage plan, and anchors.

| Option | Meaning |
|---|---|
| `--stage proposal` | Allow planned files and declarations that do not exist yet |
| `--stage implementation` | Require real anchors that match planned declarations; default |
| `--change ID` | Override the configured OpenSpec change |
| `--report PATH` | Write the deterministic verification report to this path |
| `--json` | Print canonical machine-readable JSON to standard output |

## `stele test`

Finds scenario test anchors, selects every named test independently, and writes execution evidence.

| Option | Meaning |
|---|---|
| `--change ID` | Override the configured change |
| `--evidence PATH` | Write test evidence to this path |
| `--json` | Print the evidence document to standard output |

## `stele validate`

Runs scenario tests, OpenSpec strict validation, and implementation verification as one gate. Default outputs are `artifacts/test-results.json` and `artifacts/verification-report.json`.

## Common options

`--root PATH` sets the consumer repository root for all filesystem resolution. CLI options override `stele.config.json`; omitted values fall back to configuration.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | All selected checks passed |
| `1` | A deterministic policy, linkage, OpenSpec, or selected-test check failed |
| `2` | The invocation was invalid or a required tool could not run |

Use the exit code for gates and JSON for explanation. Do not parse the human-readable summary.
