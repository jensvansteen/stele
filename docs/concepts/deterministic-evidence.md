# Deterministic evidence

Stele separates linkage, execution, and review so a single green badge cannot hide what was actually checked.

## Three independent states

| State | Question |
|---|---|
| Linkage | Did every ID resolve to the declaration named in the plan? |
| Execution | Did the exact scenario test execute and pass for these inputs? |
| Review | Did a person or external policy accept the evidence as adequate? |

The CLI currently computes linkage and execution. Review remains explicitly `not-reviewed` unless a future external review integration supplies it.

## Stable output

The verification report includes:

- verifier and OpenSpec versions;
- selected change and verification mode;
- repository revision and dirty state;
- a SHA-256 digest of relevant inputs;
- requirement and scenario source locations;
- planned and resolved links;
- execution outcome and diagnostics.

Wall-clock timestamps and random run identifiers are excluded from the canonical payload. Identical relevant inputs therefore produce byte-for-byte identical JSON.

```bash
npx stele verify --json > /tmp/first.json
npx stele verify --json > /tmp/second.json
cmp /tmp/first.json /tmp/second.json
```

## Exact execution

Stele runs each anchored named test independently. It accepts a pass only when the runner output confirms that exact test executed and passed. This prevents an unmatched filter, skipped test, or unrelated green suite from becoming scenario evidence.

## CI use

Run `stele validate --json`, retain the report and test evidence as CI artifacts, and gate the change on its exit code. A UI may render the JSON, but the UI must not recalculate the verdict. That preserves the same result locally, in CI, and in a hosted report.
