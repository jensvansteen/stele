# The Stele model

Stele describes a codebase in plain English and keeps that description connected to the code and tests. Its model has six parts. None of them depends on a particular specification tool.

| Concept | What it is | Where it lives |
|---|---|---|
| Behavior specification | Requirements and their concrete scenarios, in plain English | The specification backend |
| Verification-ID | A stable identity for each requirement (`req.…`) and scenario (`scn.…`) | Below each heading in the specification |
| Evidence plan | Per scenario, the approved levels of evidence, why each is enough, and who approved them | `linkage-plan.json` beside the change |
| Anchors | `@implements` and `@verifies` comments that link code and tests to identities and evidence IDs | Next to the code and tests, wherever the project places them |
| Evidence | The recorded outcome of every exact test run, bound to the current inputs | `artifacts/test-results.json` |
| Verdict | The deterministic result of checking the plan, the anchors, and the evidence | `artifacts/verification-report.json` and the exit code |

Together, the identities and anchors form a link index: from a sentence in the specification to its code and tests, and back. The same links serve verification and review.

## How the parts fit

1. A change describes new or modified behavior. `stele ids` gives every requirement and scenario an identity.
2. The evidence plan proposes, per scenario, one or more levels (unit, integration, e2e) with a rationale. A person approves them before implementation.
3. Code and tests carry anchors. The anchors, not the plan, say where things live.
4. Stele runs every planned test by its exact name and records the evidence.
5. The verdict passes only when every identity is linked, every approved piece of evidence ran and passed, and nothing changed since approval.

## The specification backend

Stele currently builds on OpenSpec (MIT, Fission AI). Stele reaches it through an adapter, selected with the `adapter` field of `stele.config.json`; `openspec` is the only adapter today and the default. The adapter locates, parses, and validates specifications and installs the backend's files, so Stele's workflow does not depend on OpenSpec's internals.

| Stele concept | OpenSpec file |
|---|---|
| Change | `openspec/changes/<change>/` |
| Behavior specification of a change | `openspec/changes/<change>/specs/**/*.md` (delta specs) |
| Current behavior specification | `openspec/specs/**/*.md` |
| Evidence plan | `openspec/changes/<change>/linkage-plan.json`, archived with the change |
| Planning step | The `verification` artifact of the `stele` workflow schema in `openspec/schemas/stele/` |
| Specification validation | `openspec validate --strict`, run with the bundled OpenSpec CLI |

The OpenSpec version is pinned; see [Versions](/reference/versions).
