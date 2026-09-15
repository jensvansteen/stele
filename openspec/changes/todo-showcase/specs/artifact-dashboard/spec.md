## Purpose

Give reviewers one local interface for inspecting the change artifacts, traceability links, and honest evidence states produced by the showcase.

## ADDED Requirements

### Requirement: Summarize verification state
Verification-ID: req.dashboard.9f5bca7132d8

The dashboard SHALL summarize proposal, linkage, execution, and review state without collapsing them into a single correctness claim.

#### Scenario: View verification summary
Verification-ID: scn.dashboard.60b4ae93f7c1

- **WHEN** the reviewer opens the Verification view
- **THEN** stage status and counts are shown with missing or unknown evidence explicit

### Requirement: Trace requirements to evidence
Verification-ID: req.dashboard.2e6d391a0b47

The dashboard SHALL show each requirement, its scenarios, and their resolved code/test links.

#### Scenario: Inspect requirement evidence
Verification-ID: scn.dashboard.b1c6f804d3e9

- **WHEN** the reviewer expands a requirement in the matrix
- **THEN** its source, code anchors, scenario test anchors, and execution outcome are visible

### Requirement: Run local validation
Verification-ID: req.dashboard.e14fc08b6739

The dashboard SHALL let the reviewer run the local implementation verifier and refresh the displayed report.

#### Scenario: Re-run from dashboard
Verification-ID: scn.dashboard.73fa18c5b2d0

- **WHEN** the reviewer activates Run validation
- **THEN** the dashboard shows progress and then renders the new report or actionable failure

### Requirement: Browse source artifacts
Verification-ID: req.dashboard.48d012ace679

The dashboard SHALL display an allowlisted set of proposal, design, task, specification, and report artifacts.

#### Scenario: Select an artifact
Verification-ID: scn.dashboard.54a702fd89c3

- **WHEN** the reviewer selects an artifact
- **THEN** its current repository content is displayed with its relative path

### Requirement: Replay end-to-end evidence
Verification-ID: req.dashboard.3d8a2f7c91e4

The dashboard SHALL embed allowlisted end-to-end recordings alongside the requirements and scenarios they cover, with a title, covered flow, playable video, and tailnet URL while keeping video files outside the repository.

#### Scenario: Play a recorded workflow
Verification-ID: scn.dashboard.0c4e91ab73f2

- **WHEN** the reviewer expands a requirement with covered recording evidence
- **THEN** the dashboard loads the corresponding recording inside that requirement through the application origin and exposes its direct tailnet URL
