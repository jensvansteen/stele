# OpenSpec and Stele

OpenSpec and Stele solve different parts of spec-driven development.

| Component | Owns |
|---|---|
| OpenSpec | Requirements, scenarios, proposals, designs, tasks, and change lifecycle |
| Stele | Stable IDs, planned links, anchor resolution, exact test execution, and deterministic evidence |
| Product tests | Executable examples of selected behavior |
| Human reviewer | Semantic adequacy of the implementation and its evidence |

OpenSpec remains the only behavioral canon. Stele reads it through an adapter and builds an evidence graph around it.

## The dependency direction

```text
proposal explains why
        ↓
delta specs define the behavioral change
        ↓
design selects an implementation and verification approach
        ↓
tasks sequence the work and its completion checks
```

Design and tasks must satisfy the target behavior; they do not redefine it. When OpenSpec archives a completed change, its delta updates the main capability spec. Stable IDs should move with that behavior so later changes modify the same identity.

## The Stele layer

Stele adds three records without copying the prose:

1. A stable identity inside each requirement and scenario block.
2. A linkage plan that names expected code and test targets.
3. A deterministic report that records resolution and execution outcomes.

This separation makes the verifier reusable. A future adapter can read another specification format while preserving the Stele evidence model.

## What a pass means

A passing implementation report establishes that:

- the selected OpenSpec identities are valid and unique;
- every required identity has a planned target;
- every implementation anchor resolves to the planned declaration;
- every scenario anchor resolves to the planned named test;
- recorded scenario evidence is current for the relevant inputs.

It does not prove that prose and implementation are mathematically equivalent. Test quality and semantic coverage remain review questions.
