# Getting started

Stele verifies that plain-English [OpenSpec](https://github.com/Fission-AI/OpenSpec) behavior is connected to its planned implementation and executable tests. It is installed per project and runs locally or in CI.

## Choose a path

- **Build a feature step by step:** follow [Build a Todo feature with OpenSpec and Stele](/guide/build-todo) to set up a project, plan a change from a plain-English prompt, approve its verification plan, implement the code, and produce evidence.
- **Add verification to an existing project:** continue with the concise setup and implementation steps on this page.
- **Inspect a finished result:** open the [Todo example walkthrough](/guide/inspect-example) to trace a complete OpenSpec change through its linkage plan, TypeScript anchors, exact tests, and generated evidence.

## Prerequisites

- Node.js 24 or newer, with native `.mts` execution
- npm 11 or newer
- A TypeScript or Go project; Go projects also need the Go toolchain

The npm package carries Stele's compiled verifier and the compatible OpenSpec CLI. Node runs the package tooling, OpenSpec, and exact TypeScript tests. Exact Go tests run through `go test`.

Stele scans declarations in `.ts`, `.tsx`, `.mts`, and `.go` files, executes exact named `.ts` and `.mts` tests through Node, and executes exact `Test` functions in `_test.go` files. TSX test execution needs a later configurable runner adapter.

OpenSpec itself is not limited to TypeScript. Its Markdown workflow can describe projects in any programming language. TypeScript and Go are the code and test environments supported by Stele's deterministic integration.

## Set up a project

Run the setup from the root of the project you want to verify:

```bash
npm install --save-dev stele-spec@next
npx stele init
```

1. `npm install --save-dev stele-spec@next` installs the release candidate and its compatible OpenSpec CLI for local development and CI. Pin the exact candidate version for reproducible builds.
2. `npx stele init` prepares the project:
   - When the project has no `openspec/` directory, it runs the bundled `openspec init` for the tool-neutral `.agents/skills` directory and links `.claude/skills` to it. Pass `--tools claude,cursor` to choose OpenSpec's tools instead. An existing `.claude/skills` is never replaced.
   - It forks OpenSpec's `spec-driven` schema into a project schema named `stele`, which adds a `verification` planning step before `tasks`, and selects it for new changes.
   - It merges Stele's apply and archive guidance into `openspec/config.yaml`, keeping your own settings.
   - It writes `stele.config.json`, installs the `stele-propose`, `stele-apply`, `stele-archive`, `stele-plan`, and `stele-verify` skills, and creates `artifacts/`.
   - It adds the [Stele annotation](/concepts/spec-format) to the first line of every current specification and every delta spec of an active change that lacks it. Archived changes are left alone.

An existing OpenSpec setup is left alone apart from these additions. Running `init` again is safe: existing files are preserved.

Upgrading a project that already uses Stele? Run `npx stele init` again, or `npx stele annotate --all`, to annotate its specifications. Until then, verification warns with `SPEC_ANNOTATION_MISSING`; from Stele 0.2.0 that is an error by default.

Use `npx stele` so local development and CI execute the pinned version. Contributors testing an unreleased Stele checkout can follow [Use a local package](/guide/local-package); the workflow after installation is identical.

## Plan a change with your agent

Ask your coding agent to use the `stele-propose` skill:

```text
Use stele-propose to plan a change `todo-basics` that lets a user add a todo
from non-empty text.
```

The agent creates the OpenSpec change under `openspec/changes/todo-basics/`, then runs the Stele planning step and stops with "Plan ready." Review the result before anything is implemented:

- the specifications, where `stele ids` inserted a `Verification-ID` below every requirement and scenario;
- the verification table in `design.md`, with a level, a rationale, and an advisory placement for every scenario;
- `linkage-plan.json`, which names the declaration and test planned for every ID.

When you ask the agent to apply the change, `stele-apply` first shows the verification levels and asks you to confirm them. It records your approval in `design.md`, implements the tasks with OpenSpec's apply skill, adds the anchors, and runs `stele validate --change todo-basics`. When the change is done, `stele-archive` archives it only after validation passes.

The sections below show what the agent produces, so you can also write or review each piece yourself. [Use Stele with OpenSpec](/guide/openspec) covers working with OpenSpec's own skills and commands.

## Choose the change to verify

OpenSpec groups a proposed feature under `openspec/changes/<change-id>`. Pass that directory name with `--change`:

```bash
npx stele validate --change todo-basics
```

`stele init --change todo-basics` saves a default change in `stele.config.json`, so later commands can omit `--change`. Without a default, `verify`, `test`, and `validate` need `--change` or `--specs` and otherwise exit with code `2`.

## Add the two identities

Each OpenSpec requirement needs one `req` identity and each scenario one `scn` identity. Let Stele insert them instead of writing tokens by hand:

```bash
npx stele ids --change todo-basics
```

`stele ids` also starts each delta spec with the annotation that marks it as a Stele specification, if it lacks one:

```markdown
<!-- stele: spec v1 -->
## ADDED Requirements

### Requirement: Add a todo
Verification-ID: req.todo.22b616c90f42

The system SHALL add a todo with the entered title.

#### Scenario: Non-empty title
Verification-ID: scn.todo.20d9cd2785a4

- **WHEN** the user enters a non-empty title
- **THEN** a todo with that title is added
```

`stele ids --check` writes nothing and exits with code `1` while an ID is missing, which suits CI and pre-commit hooks.

## Plan before implementation

Plan, for every scenario, which kinds of evidence prove it and why. The plan names no files: anchors in the code say where things live.

```json
{
  "schemaVersion": 2,
  "changeId": "todo-basics",
  "scenarios": {
    "scn.todo.20d9cd2785a4": {
      "evidence": [
        {
          "id": "scn.todo.20d9cd2785a4.unit",
          "level": "unit",
          "rationale": "Adding a todo is pure logic; no I/O is needed.",
          "placement": "tests/todo.test.ts, beside the other Todo tests"
        }
      ]
    }
  }
}
```

Save this as `openspec/changes/todo-basics/linkage-plan.json`, next to the change it plans. The evidence ID is the scenario ID plus the level. `placement` is advisory and never checked. See [Test levels](/concepts/test-levels) for the level definitions.

## Approve the levels

A person approves the planned levels before implementation starts. Until then, the proposal check reports every entry as `PLAN_UNAPPROVED`:

```bash
npx stele verify --stage proposal --change todo-basics
```

Approve in one of three ways:

- In the conversation: `stele-apply` shows the levels and asks "Approve these levels and start implementing?". After your explicit yes, the agent runs `stele approve --change todo-basics --confirmed-in-chat`.
- In a terminal: `npx stele approve --change todo-basics` shows each entry and asks approve, reject, or skip.
- In bulk: `npx stele approve --change todo-basics --all --yes`.

Each approval records the approver (your `git config user.name`, or `--by`), the date, how it was given, and a digest of the entry and the scenario's wording. Rewording the scenario or changing the level or rationale makes the approval stale; whitespace-only edits do not. Teams that want a second check can require pull-request review of plan files.

## Implement and verify

Attach `@implements` to the code declaration and `@verifies scn.todo.20d9cd2785a4.unit` to the named test, wherever your project's conventions place them. Then run:

```bash
npx stele validate --change todo-basics
```

Validation runs every evidence test, OpenSpec strict validation, and implementation verification. Every approved entry needs a matching `@verifies` anchor on a test that passes, and every requirement needs an `@implements` anchor. Exit code `0` means all selected checks passed; see the [CLI reference](/reference/cli) for the complete contract.

While you work on one behavior, run only its tests. A target is a requirement, scenario, or evidence ID, or a spec file, and the outcomes merge into the stored evidence:

```bash
npx stele test scn.todo.20d9cd2785a4
npx stele test openspec/changes/todo-basics/specs/todo/spec.md
```

`npx stele verify` then reports linkage, execution, and an overall verdict separately; it stays `incomplete` until every scenario's tests have run with the current inputs. `npx stele index` prints the links as JSON for editors, including each scenario's `WHEN`/`THEN` steps; see [Link index](/reference/link-index). In CI, `npx stele validate --all` checks the current specifications and every active change in one step.

Plans with `schemaVersion` 1, which map each ID to a `path#selector` target, still work with a `PLAN_V1_DEPRECATED` warning until Stele 0.2.0. Convert one with `npx stele plan migrate --change <change>`.

## Commit inputs, ignore outputs

Commit the specifications, `stele.config.json`, linkage plan, source anchors, and test anchors. Generated reports under `artifacts/` can be uploaded by CI and ignored locally when your review process does not require committed evidence.
