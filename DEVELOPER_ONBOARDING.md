# Developer onboarding

Stele makes plain-English OpenSpec behavior traceable to real code and executable tests.

```text
OpenSpec requirement + scenarios
  → stable requirement and scenario IDs
  → planned code declaration and test selector
  → @implements and @verifies anchors
  → exact scenario test execution
  → deterministic evidence for CI and review
```

OpenSpec is the behavioral canon. Stele checks structure, planned linkage, exact test execution, and evidence freshness. It reports planning, linkage, execution, and human review separately.

To work on Stele itself:

```bash
npm install
npm run verify
npm run docs:dev
```

To use an unpublished local build in another repository:

```bash
npm pack --pack-destination /path/to/consumer/vendor
cd /path/to/consumer
npm install --save-dev ./vendor/stele-spec-0.3.0.tgz
npx stele init --change my-change
npx stele validate --json
```

Start with [Getting started](docs/guide/getting-started.md), then read [OpenSpec and Stele](docs/concepts/openspec-and-stele.md), [Plan test levels](docs/concepts/test-levels.md), and the [CLI reference](docs/reference/cli.md). The full documentation site runs with `npm run docs:dev`.
