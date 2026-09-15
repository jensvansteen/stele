# How OpenSpec and Stele work together

OpenSpec owns the plain-English requirements, scenarios, proposals, designs, tasks, and lifecycle of a change. Stele adds stable behavioral IDs, planned links to implementation and tests, exact scenario execution, and deterministic evidence.

```text
product intent
    ↓
OpenSpec requirement and scenarios
    ↓
stable requirement and scenario IDs
    ↓
planned declarations and test selectors
    ↓
@implements and @verifies anchors
    ↓
Stele validation and revision-bound evidence
    ↓
report UI and human review
```

There is one specification canon: OpenSpec. Stele does not copy requirement prose into another format. Its IDs and linkage plan join behavior to evidence while allowing wording, files, and implementation details to evolve.

The normal loop is:

1. Write testable requirements and scenarios in an OpenSpec change.
2. Assign a stable `req.…` ID to each requirement and `scn.…` ID to each scenario.
3. Plan a code declaration for every requirement and a named test for every scenario.
4. Run proposal verification before implementation.
5. Attach `@implements` and `@verifies` anchors to the real declarations.
6. Run deterministic validation and review the resulting evidence.
7. Preserve the behavior IDs when OpenSpec applies or archives the change.

Read the maintained guide for the complete model:

- [OpenSpec and Stele](./concepts/openspec-and-stele.md)
- [Build a verified change](./guide/verified-change.md)
- [Plan test levels](./concepts/test-levels.md)
- [Deterministic evidence](./concepts/deterministic-evidence.md)
