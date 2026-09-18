## Context

See proposal.md. OpenSpec knowledge is spread across the verifier:

- `specs.go` (`openspec/changes/<id>/specs` and `openspec/specs` paths, and the `### Requirement:` and `#### Scenario:` grammar);
- `scope.go` (plan locations and the archive directory);
- `openspec.go` (the bundled CLI and strict validation);
- `init.go`, and later `workflowschema.go` (backend artifacts).

`Config.Adapter` is written as `"openspec"` and never read. The OpenSpec version is pinned by `package.json` (`"@fission-ai/openspec": "1.13.0"`, bundled) and by `OpenSpecVersion` in `cli.go`. OpenSpec skills record `metadata.generatedBy: "<version>"`, and the Stele schema fork records its versions in a header comment (see `authoring-workflow`).

## Goals / Non-Goals

**Goals:**

- One internal interface through which every command reaches the specification backend.
- A visible, cheap warning when a project's backend files drift from the pinned version.
- Documentation that presents Stele's concepts first and OpenSpec as the current backend.

**Non-Goals:**

- A second backend, or a plugin system. The seam is internal, with one implementation plus a test double.
- Changing OpenSpec behavior or file formats.
- Making the pin configurable. Bumping OpenSpec remains a Stele release step.

## Decisions

### Backend interface

```go
type specificationBackend interface {
    Name() string
    ScopeSpecFiles(root string, scope verificationScope) ([]string, error)
    ParseIdentities(root string, files []string) (ParsedSpecs, error)
    DeclaredIdentities(root string) (map[string]bool, error)
    PlanPaths(root string, scope verificationScope) []string
    ArchivedPlanPaths(root string) []string
    Validate(root string, scope verificationScope) (bool, error)
    Install(root string, change string) ([]string, error)
    VersionDrift(root string, lookPath bool) []string
}
```

- **Resolution.** `resolveBackend(config)` returns the OpenSpec implementation for `""` or `"openspec"`, and an error listing the supported names otherwise. The CLI maps that error to exit code `2`. Commands receive the backend through `options`.
- **Test double.** Tests can register an in-memory backend through an unexported variable, as other seams in this package do.
- **Scope of the move.** Existing functions move behind the OpenSpec implementation unchanged. All existing tests keep passing, which proves behavior is unchanged.
- **Rejected alternative:** an exported plugin API. It has no second consumer yet and would freeze internals.

### Drift checks

- **Skills.** `VersionDrift` reads `.agents/skills/openspec-*/SKILL.md` frontmatter for `generatedBy`, and the first comment line of `openspec/schemas/stele/schema.yaml`. These are file reads only, so `stele verify` and `stele validate` can afford them.
- **The `openspec` binary.** `stele init` alone looks up `openspec` with `exec.LookPath`. If it resolves outside the Stele package, `init` runs it with `--version` and a short timeout. That starts a Node process, so it stays out of the frequently run commands.
- **Output.** Warnings go to standard error as `stele: warning: …`. They name both versions and the fix:
  - the project's OpenSpec: run `npx openspec update` with the bundled version;
  - a different global `openspec`: prefer `npx openspec`, which uses the version pinned with Stele.
- **Exit codes** never change, unless `--strict-versions` is given. That flag, on `init`, `verify`, and `validate`, prints the same findings as `stele: error: …` and makes the command exit with code `1`.
- **Cost.** The check is cheap: a few small file reads per command and one process at init. It is worth adding.

### Documentation

- **Concepts page** (`docs/concepts/model.md`):
  - behavior specification with Verification-IDs;
  - evidence plan with levels and approvals;
  - anchors;
  - evidence;
  - verdicts;
  - the link index.

  It closes with "Stele currently builds on OpenSpec (MIT, Fission AI)" and a map of Stele concepts to OpenSpec files.
- **Version policy page** (`docs/reference/versions.md`):
  - why OpenSpec is pinned exactly;
  - what the drift warnings mean;
  - that an OpenSpec bump is a deliberate Stele release: update the dependency and `OpenSpecVersion`, re-run the schema experiment, re-check the forked schema patch, the lifecycle skills, and the configuration guidance, and release.
- **CONTRIBUTING** release steps link to that checklist.

### Verification strategy

**Status: approved** by jensvansteen on 2026-09-17, before implementation started (via: agent-confirmed, chat review). The review added the optional `--strict-versions` flag with its own unit-level scenario; drift still only warns by default.

Placement follows this repository's AGENTS.md and is advisory. This change's linkage plan is v1 for the published verifier and records the ★ row.

| Scenario | Level | Evidence ID | Advisory placement (reason) | Target | Risk and why this level is the lowest convincing one |
|---|---|---|---|---|---|
| `scn.backend.17bc5c526b79` Use OpenSpec by default | unit ★ | `….17bc5c526b79.unit` | `internal/stele/adapter_test.go`, beside `adapter.go` | `internal/stele/adapter_test.go#TestBackendDefaultsToOpenSpec` | Risk: existing projects break after the refactor. Resolution is pure; the existing test suite covers the unchanged behavior. |
| `scn.backend.e20f52b594b9` Reject an unknown adapter | unit ★ | `….e20f52b594b9.unit` | `adapter_test.go` | `internal/stele/adapter_test.go#TestUnknownAdapterIsRejected` | Risk: a typo silently falls back to OpenSpec. `Run` returns the exit code in process. |
| `scn.backend.ad74565b613f` Route commands through the backend | unit ★ | `….ad74565b613f.unit` | `adapter_test.go` | `internal/stele/adapter_test.go#TestCommandsUseSelectedBackend` | Risk: OpenSpec paths bypass the seam, so a future backend cannot work. An in-memory backend with no `openspec/` directory proves every command goes through it. |
| `scn.backend.c077e67d1e58` Warn about skills from another OpenSpec version | unit ★ | `….c077e67d1e58.unit` | `adapter_test.go` | `internal/stele/adapter_test.go#TestDriftWarnsForOtherSkillVersions` | Risk: an OpenSpec update silently changes agent behavior. Fixture skill files. |
| `scn.backend.124f95bf55a3` Warn about another openspec on PATH | integration ★ | `….124f95bf55a3.integration` | `adapter_test.go` | `internal/stele/adapter_test.go#TestInitWarnsForOtherOpenSpecOnPath` | Risk: users run a different global `openspec` than the pinned one. It needs a real process lookup and execution; a fake executable on `PATH` stands in for the other version. |
| `scn.backend.08f190c98ed9` Fail on drift when strict versions are requested | unit ★ | `….08f190c98ed9.unit` | `adapter_test.go` | `internal/stele/adapter_test.go#TestStrictVersionsFailOnDrift` | Risk: the flag does not fail CI, or changes the default. `Run` in process with fixture skill files, with and without the flag. |
| `scn.backend.0ee7b914816a` Stay silent when versions match | unit ★ | `….0ee7b914816a.unit` | `adapter_test.go` | `internal/stele/adapter_test.go#TestDriftSilentWhenVersionsMatch` | Risk: noisy warnings train users to ignore them. |

Requirement implementation targets (v1 plan today):

| Requirement | Target |
|---|---|
| `req.backend.b5ff52883615` Select the specification backend | `internal/stele/adapter.go#resolveBackend` |
| `req.backend.17e80d964275` Warn about backend version drift | `internal/stele/adapter.go#openSpecVersionDrift` |

## Risks / Trade-offs

- [The refactor touches every command] → Moves happen without behavior changes, under the existing tests and the 100% coverage gate. Self-verification (`npm run verify:self`) checks the archived behavior too.
- [The `generatedBy` metadata format could change in a future OpenSpec] → A missing field produces no warning, and the pin keeps the format stable until a deliberate bump.
- [Node startup for the PATH check] → It runs only during `init`.

## Decided questions

- Should `stele validate` fail, rather than warn, when the Stele schema fork was produced by another OpenSpec version? Decided: warn by default; the optional `--strict-versions` flag makes any drift fail.
