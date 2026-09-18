# Versions

## OpenSpec is pinned

Each Stele release bundles one exact OpenSpec version, currently 1.13.0. The `stele-spec` package depends on that exact `@fission-ai/openspec` version, and Stele always runs the bundled CLI. A project that installs Stele therefore gets the OpenSpec version Stele was tested with.

The pin is deliberate. Stele relies on OpenSpec's file layout, its strict validation, its schema commands, its instructions, and the skills it generates. A different OpenSpec version can change any of them.

## Drift warnings

Files that OpenSpec generated keep the version that generated them. Stele compares them with the pinned version:

| Check | When | Fix |
|---|---|---|
| OpenSpec skills record another `generatedBy` version | `init`, `verify`, `validate` | `npx openspec update`, which uses the pinned version |
| The `stele` schema was forked from another OpenSpec version | `init`, `verify`, `validate` | `npx stele init --refresh-schema` |
| The `openspec` on `PATH` reports another version | `init` only, because it starts a process | Use `npx openspec`, which runs the pinned version |

Findings are printed to standard error as `stele: warning: …` and do not change the exit code. Pass `--strict-versions` to `init`, `verify`, or `validate` to print them as `stele: error: …` and exit with code `1` instead, for example in CI.

## Upgrading OpenSpec is a Stele release

Projects do not choose their OpenSpec version separately; a new OpenSpec version reaches them through a Stele release. To bump it:

1. Update the exact `@fission-ai/openspec` dependency in `package.json` and `package-lock.json`, and `OpenSpecVersion` in `internal/stele/cli.go`.
2. Re-run the schema experiment: `openspec schema fork spec-driven`, the Stele patch, `openspec new change`, `openspec status`, and `openspec instructions` for the `verification` artifact and for apply and archive.
3. Re-check the forked schema patch, the configuration guidance, the lifecycle skills (`stele-propose`, `stele-apply`, `stele-archive`) against the regenerated OpenSpec skills, and this repository's own `.agents/skills`.
4. Regenerate this repository's OpenSpec skills with `npx openspec update` and its schema with `npm run stele -- init --refresh-schema`.
5. Run `npm run verify`, `npm run docs:build`, and `npm run verify:self`, then release as described in [Contributing](https://github.com/jensvansteen/stele/blob/main/CONTRIBUTING.md#releases).
