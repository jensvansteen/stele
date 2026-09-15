# Proposal: Todo verification showcase

## Why

The stable-ID verification plan needs a complete, reviewable example that proves the workflow across requirements, implementation, tests, execution evidence, and a consuming dashboard.

## What Changes

- Add a local Todo application with create, complete, filter, and delete behavior.
- Add an artifact dashboard that renders OpenSpec documents and requirement evidence.
- Add a read-only verification layer with proposal and implementation modes.
- Add a versioned JSON report and local validation command.
- Add a repository-owned `stele` CLI with deterministic output and scenario-specific test execution.

## Capabilities

- `todo-management`: A small browser Todo workflow backed by an in-memory API.
- `artifact-dashboard`: A review interface for OpenSpec and verification artifacts.
- `verification`: Stable identity, anchors, stage-aware checks, and reporting.

## Impact

The project gains a Node server, dependency-free browser UI, tests, and one pinned OpenSpec development dependency. It does not publish a package, deploy a service, or change the preserved Stele reference.
