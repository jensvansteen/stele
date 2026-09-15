# Stele

Stele connects plain-English OpenSpec behavior to implementation and test evidence through a deterministic `stele` CLI. OpenSpec owns the requirements and change artifacts; Stele adds stable requirement/scenario IDs, resolvable code and test anchors, scenario-specific execution evidence, deterministic JSON, and stable exit codes.

The purpose is to describe a codebase and proposed behavior in plain English, plan where each feature will be implemented and tested, and then run deterministic checks that connect those statements to real code declarations and exact test executions. The resulting evidence makes omissions and broken links visible while keeping human semantic review as a separate decision.

“ID” means behavioral requirement identity, not verification of a person.

## Install it in another local repository

Stele does not need to be published to npm during development. Build the same tarball npm would publish, copy it into the consumer repository, and install it as a development dependency:

```bash
# In this repository
npm pack --pack-destination /path/to/consumer/vendor

# In the consumer repository
npm install --save-dev ./vendor/stele-spec-0.2.0.tgz
npx stele init --change my-open-spec-change
npx stele verify --stage proposal --json
```

`stele init` writes `stele.config.json` and installs the `stele-plan` and `stele-verify` skills under `.agents/skills/`. The package bundles its pinned OpenSpec runtime, so this flow works from the tarball without a registry install.

The separate `stele-examples` repository proves this package boundary with a newly implemented Todo app. It imports no files from this checkout.

## Try the original prototype

```bash
npm install
npm run validate
npm run dev
```

Open <http://localhost:4173>. Use the Todo workspace, then open **Verification** and click **Run validation**. The dashboard reads the same JSON report that the CLI produces for CI or another consumer. Expand a covered requirement to play its allowlisted end-to-end recording alongside the scenario and test evidence.

## Useful commands

| Command | Purpose |
|---|---|
| `npm run dev` | Start the Todo app and dashboard |
| `npm test` | Run unit and integration tests |
| `npm run verify:proposal` | Validate IDs and planned links before implementation |
| `npm run verify` | Validate IDs and real code/test anchors |
| `npm run validate` | Execute every anchored scenario, run OpenSpec strict validation, and verify implementation links |
| `npm run openspec:validate` | Run OpenSpec's own strict validation |

## Use the CLI directly

The package exposes `bin/stele.mjs` as the `stele` executable. Inside this repository, invoke it without a global install:

```bash
npm run stele -- verify --stage proposal --json
npm run stele -- verify --stage implementation --json
npm run stele -- test --json
npm run stele -- validate --json
```

Exit code `0` means the selected checks passed, `1` means a deterministic policy or selected test failed, and `2` means the invocation or tool failed.

Implementation verification checks more than whether an ID string appears somewhere. Each `@implements` or `@verifies` anchor must:

- name a declared OpenSpec identity of the correct kind;
- sit next to a compatible function, class, method, or named test declaration;
- resolve to the repository-relative path and selector declared in `artifacts/linkage-plan.json`;
- remain unique and complete for the selected requirement/scenario set.

`stele test` runs each anchored named test independently with an exact name selector. A zero process exit is accepted only when TAP output confirms that exact test ran and passed, preventing unmatched or skipped tests from being reported as passing evidence.

The JSON report is byte-for-byte stable for identical relevant inputs. It contains revision or dirty-tree identity and a SHA-256 input digest, but excludes timestamps and run IDs. You can reproduce the check with:

```bash
npm run stele -- verify --json > /tmp/stele-first.json
npm run stele -- verify --json > /tmp/stele-second.json
cmp /tmp/stele-first.json /tmp/stele-second.json
```

Start with [the product journeys](docs/intent/journeys.md), inspect the active change under [`openspec/changes/todo-showcase`](openspec/changes/todo-showcase), and read [the framework plan](PLAN.md) for the longer-term direction. Generated evidence is in [`artifacts/test-results.json`](artifacts/test-results.json) and [`artifacts/verification-report.json`](artifacts/verification-report.json). Recording metadata and covered scenario IDs live in [`artifacts/e2e-recordings.json`](artifacts/e2e-recordings.json); the MP4 files remain outside Git under `~/recordings`.

For the shortest contributor workflow, read [Developer onboarding](DEVELOPER_ONBOARDING.md).

For the architecture and Devin adoption model, read [How OpenSpec and Stele work together](docs/OPENSPEC_AND_STELE.md).

The previous methodology repository is now `stele-legacy`. This repository is the installable Stele product; `stele-examples` contains independent consuming applications.
