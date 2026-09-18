# Continuous integration

Run one command in CI to gate every pull request on your specifications, plans, anchors, and evidence:

```bash
npx stele check --all
```

`stele check --all` checks Verification-IDs and annotations, and runs `stele validate` over the current specifications and every active change. It exits with one code for all of them. See [`stele check`](/reference/cli#stele-check) for its steps.

## GitHub Actions

Copy this workflow to `.github/workflows/stele.yml`:

```yaml
name: Stele

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  stele:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
      # Go projects: Stele runs Go evidence tests with the Go toolchain.
      # - uses: actions/setup-go@v6
      #   with:
      #     go-version-file: go.mod
      #     cache: true
      - run: npm ci
      # Projects with browser e2e evidence, such as Playwright:
      # - uses: actions/cache@v4
      #   with:
      #     path: ~/.cache/ms-playwright
      #     key: playwright-${{ runner.os }}-${{ hashFiles('package-lock.json') }}
      # - run: npx playwright install --with-deps chromium
      - name: Check specifications, plans, anchors, and evidence
        run: npx stele check --all
      - name: Keep the verification report and evidence
        if: ${{ !cancelled() }}
        uses: actions/upload-artifact@v4
        with:
          name: stele-report
          path: |
            artifacts/verification-report.json
            artifacts/test-results.json
```

- **Node 24** is the Node.js version Stele requires. `cache: npm` caches the npm download cache between runs.
- **Go projects** uncomment `actions/setup-go`, because Stele runs Go evidence tests with `go test`.
- **Browser e2e evidence**, such as Playwright tests, needs the browser on the runner. Uncomment the cache and the install step, and install only the browsers your tests use.
- **The report upload** runs even when the check fails, so the verification report and the evidence of the failing run can be downloaded from the workflow run.

### Findings on the pull request

In GitHub Actions, Stele writes each finding and each failed test as a workflow annotation to standard error, such as:

```text
::error file=tests/todo.test.mts,line=14,title=ANCHOR_DANGLING::An anchor names an ID no specification declares. scn.todo.abcdef012345
```

GitHub shows these inline on the pull request diff, so reviewers see a missing ID, an unapproved plan entry, or a failed test at its line. Stele detects GitHub Actions from `GITHUB_ACTIONS=true`, and Gitea and Forgejo Actions set it too. Paths are relative to the checkout, also when you run Stele with `--root` in a subdirectory.

The annotations follow the report: the first five findings of each code, and one annotation that counts the rest; `--details` annotates every finding. GitHub itself shows only the first 10 errors and 10 warnings of a step inline; the job log and the report file keep the full list.

Annotations change nothing else: standard output, `--json`, report and evidence files, and exit codes are the same. `--quiet` and `--json` keep them. Turn them off with `--annotations=never`, or write them outside GitHub Actions with `--annotations=github`.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | Every check passed |
| `1` | A check failed: a finding such as an unapproved plan entry or a missing anchor, or a failed test |
| `2` | The command could not run: a bad option, an empty scope, or OpenSpec failing to start |

Gate on the exit code. The step fails on `1` and on `2`.

## Output in CI

- **Plain output.** Without a terminal, Stele prints plain, uncolored lines. It respects `NO_COLOR`, and `--color=never` forces plain output anywhere.
- **Progress** goes to standard error, one line per stage, per finished test file, and per failed test. Standard output holds only the report.
- **`--quiet`** prints only the verdict line and no progress. Annotations still appear.
- **`--json`** prints one deterministic document on standard output for tools. Read it, or the report file, for automation; never parse the human report, which may change between releases. See [Determinism in CI](/reference/cli#determinism-in-ci).

## Pinning

`stele-spec` is a development dependency pinned in `package-lock.json`, so `npm ci` installs the locked version and `npx stele` runs it. CI and every developer check with the same Stele and the same bundled OpenSpec. Upgrade Stele in a pull request, like any other dependency.

## Merge changes complete

`--all` checks every active change under `openspec/changes/` at the implementation stage. A change that is only planned, or implemented in part, fails the gate: its requirements have no anchors yet, or its plan is not approved. Every later pull request would fail too once such a change is on `main`.

So a change reaches `main` approved, implemented, and passing, in one pull request or at the end of a branch:

1. Plan the change on its branch, or in a draft pull request, and get its verification plan approved.
2. Implement it on the same branch until `npx stele check --change <change>` passes.
3. Merge. Archive the change afterwards, or in the same pull request.

Run `npx stele check --all` locally before you push to see what CI will report.
