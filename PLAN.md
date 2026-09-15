# Stele roadmap

Version 0.3 establishes the product boundary: a Go verification core, direct native CLI, npm installation, pinned OpenSpec adapter, embedded planning and verification skills, deterministic evidence, exact Node and Go test execution, 100% core coverage, CI, and documentation.

The next milestones are:

1. Add a versioned verification-policy schema that records test level, boundary, role, facet, runner, selector, and rationale for every scenario.
2. Produce cross-platform release binaries and platform-specific npm packages so public installation never compiles Go locally.
3. Generate a self-contained HTML report from OpenSpec Markdown and Stele evidence.
4. Serve that report locally and publish it as a CI artifact.
5. Add local-only dashboard controls for running verification and reviewing file changes.
6. Prove OpenSpec lifecycle compatibility across add, modify, rename, remove, sync, and archive fixtures.

The deterministic CLI remains the verdict authority. Report and dashboard layers only render its evidence or invoke its fixed operations.
