# ids Specification

## Purpose
Give every requirement and scenario a stable Verification-ID without hand-written tokens, and let CI check that none is missing.

## Requirements

### Requirement: Insert missing verification IDs
Verification-ID: req.ids.5cd09e44308f

The `stele ids [--change <id>]` command SHALL insert a `Verification-ID` line directly below every requirement and scenario heading of the change's delta specs that has none. It SHALL leave existing IDs unchanged and preserve every other byte, including line endings and a missing final newline. With `--check` it SHALL write nothing and exit with code `1` while any ID is missing. With `--json` it SHALL print the inserted or missing IDs with their locations.

#### Scenario: Insert IDs below headings that lack them
Verification-ID: scn.ids.83b2e0efd623

- **WHEN** `stele ids` runs for a change whose requirement and scenario headings have no IDs
- **THEN** each heading gets a `Verification-ID` line directly below it, the command reports each ID with its file and line, and proposal verification then finds no missing ID

#### Scenario: Preserve existing IDs and every other byte
Verification-ID: scn.ids.1f44342aea82

- **WHEN** a delta spec uses CRLF line endings, has no final newline, and already has some IDs, including an invalid one
- **THEN** only missing IDs are added, existing lines are unchanged, and a second run changes nothing

#### Scenario: Check for missing IDs without writing
Verification-ID: scn.ids.9211a14a8ec9

- **WHEN** `stele ids --check` runs while a heading lacks an ID
- **THEN** no file changes, the command lists each missing heading, and it exits with code `1`, or with code `0` once none is missing

#### Scenario: Report IDs as JSON
Verification-ID: scn.ids.0db6a666b9d4

- **WHEN** `stele ids --json` runs
- **THEN** it prints deterministic JSON listing each ID with its kind, title, file, and line

### Requirement: Derive stable and unique IDs
Verification-ID: req.ids.320cb32c3b8a

Derived IDs SHALL be reproducible from the change, capability path, heading kind, titles, and occurrence. They SHALL use the namespace of the first valid ID in the file, or else the capability path reduced to lowercase letters and digits. They SHALL differ from every ID declared under `openspec/` and from every ID named by a code or test anchor, including anchors with evidence-ID suffixes.

#### Scenario: Derive the same IDs for the same draft
Verification-ID: scn.ids.ed0d3d9619a5

- **WHEN** two copies of the same draft change are processed
- **THEN** both receive identical IDs

#### Scenario: Avoid IDs that already exist
Verification-ID: scn.ids.d83366faeaf7

- **WHEN** the first derived ID for a heading already appears in an archived change, in `openspec/specs`, or in a `@verifies <id>.e2e` anchor
- **THEN** the command derives a different, unused ID

### Requirement: Keep the identity of modified behavior
Verification-ID: req.ids.877eabdee0b4

For headings in a `MODIFIED Requirements` section, the command SHALL reuse the ID of the same requirement or scenario in `openspec/specs/<capability>/spec.md`, following `RENAMED` pairs, unless the change already uses that ID. It SHALL insert nothing in `REMOVED` and `RENAMED` sections.

#### Scenario: Reuse the current ID of a modified requirement
Verification-ID: scn.ids.11a49d3afd32

- **WHEN** a `MODIFIED` requirement, renamed in the same change, and one of its scenarios lack IDs, and the living spec declares both under the old requirement title
- **THEN** both receive their IDs from the living spec

#### Scenario: Leave removed and renamed sections alone
Verification-ID: scn.ids.03c9b0ba7536

- **WHEN** a delta spec has `REMOVED` and `RENAMED` sections
- **THEN** no ID is inserted in them
