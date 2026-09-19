<!-- stele: spec v1 -->
## MODIFIED Requirements

### Requirement: Export a deterministic link index
Verification-ID: req.linkindex.78a6abc9c59d

The `stele index` command SHALL print a JSON document for the selected scope to standard output, or with `--output-file PATH` write it to that file. The index SHALL be built by a reusable in-process component, so a later language server can serve the same data without running the command. When a change is selected, the document SHALL also include the current specifications. Every item SHALL be labelled with its scope. The document SHALL list every requirement and scenario with its ID, title, specification text, and source location. A requirement's text SHALL be the Markdown under its heading without its `Verification-ID` line and without its scenarios. A scenario SHALL carry its raw Markdown text and its structured steps, one per `- **KEYWORD** …` bullet, with the keyword and the bullet's full text. It SHALL also list the scenarios of each requirement, every code and test anchor with its location, selector, and evidence level, the planned evidence with its approval state, and the last known execution outcome of each piece of evidence. When the scope has targeted specifications, the document SHALL also list the configured targets with their paths and `evidenceOnly` values, give each targeted requirement and scenario its declared `Targets:` list, or `null` when it has none, and its applicable targets, give each targeted evidence entry and test anchor its target, and include the scenario by target matrix of each scope, as the `verification-targets` capability defines it. Without targeted specifications, none of these fields SHALL appear. With `--target`, the document SHALL contain only the selected targets' columns, evidence, and anchors. Identical inputs SHALL produce identical bytes.

#### Scenario: Index a change
Verification-ID: scn.linkindex.30ab15bc8293

- **WHEN** `stele index --change <change> --json` runs for a change with anchored code, anchored unit and e2e tests, and an approved v2 plan
- **THEN** the index lists the requirement with its scenarios and specification text, the code anchor, both test anchors with their levels and selectors, and the approval state of each evidence entry

#### Scenario: Keep requirement text and scenario steps
Verification-ID: scn.linkindex.2bec33059b75

- **WHEN** a requirement has body text and a scenario whose `WHEN`, `THEN`, and `AND` bullets include one that continues on the next line, another scenario has no text, and another has a bullet without a bold keyword
- **THEN** the requirement keeps its body text without the `Verification-ID` line or its scenarios, the first scenario keeps its raw text and one step per bullet with the keyword and the joined multi-line text, the scenario without text has empty text and no steps, and the bullet without a keyword becomes a step with an empty keyword

#### Scenario: Include the current specifications
Verification-ID: scn.linkindex.48b3c24773d9

- **WHEN** `stele index --change <change>` runs in a project with archived behavior in `openspec/specs`
- **THEN** the index also lists the current requirements and scenarios, each labelled with scope `specs`, while the change's items are labelled with the change ID

#### Scenario: Emit identical bytes for identical inputs
Verification-ID: scn.linkindex.c0e515c9575a

- **WHEN** `stele index` runs twice on unchanged inputs, once printing to standard output and once with `--output-file`
- **THEN** both runs produce identical bytes without timestamps

#### Scenario: Mark stale execution outcomes
Verification-ID: scn.linkindex.331076ad0227

- **WHEN** the stored evidence for a test was recorded before one of the verified inputs changed
- **THEN** the index reports that evidence's last outcome with state `stale`

#### Scenario: Flag anchors that no specification declares
Verification-ID: scn.linkindex.560aeb5e3970

- **WHEN** a code or test anchor names an ID that no specification under `openspec/` declares
- **THEN** the index lists that anchor with status `undeclared`

#### Scenario: Index targets and the matrix
Verification-ID: scn.linkindex.34d29862cf10
- **WHEN** `stele index --change <change>` runs for a change whose specification declares `targets: ios, android`, with one scenario narrowed to `ios` and one scenario with passing `ios` evidence and no `android` entry
- **THEN** the index lists both targets with their paths, gives the narrowed scenario the declared targets `["ios"]` and the other the declared targets `null` and applicable targets `["android", "ios"]`, labels each evidence entry and anchor with its target, and its matrix has `n/a` for `android` on the narrowed scenario and `missing` for `android` on the other

### Requirement: Run the tests of selected behavior
Verification-ID: req.linkindex.860a4d91b9fe

The `stele test` command SHALL accept zero or more positional selections. A selection that starts with `req.` or `scn.` SHALL be an identity: a requirement ID selects the evidence tests of all its scenarios, a scenario ID selects the evidence tests of that scenario, and an evidence ID (`<scenario>.<level>[.<n>]`, or `<scenario>.<target>.<level>[.<n>]` for a targeted scenario) selects the tests of that entry. Any other selection SHALL be the path of a specification file under `openspec/`, in the current specifications or in a change, and selects the tests of every scenario in that file. Several selections SHALL select the union of their tests, and `--target` SHALL narrow that union to the selected targets' entries. The command SHALL run only the selected tests, merge their outcomes into the stored evidence without discarding other outcomes, and, before running any test, exit with code `2` when a selection names an identity the scope does not declare or a file that does not exist. Without selections it SHALL run every test of the scope, as before. Version 1 supports exact test names only.

#### Scenario: Run the tests of one scenario
Verification-ID: scn.linkindex.352d7120ec6c

- **WHEN** `stele test <scenario-id>` runs for a scenario with unit and e2e evidence tests in a scope with other scenarios
- **THEN** only that scenario's tests run, both outcomes are recorded, and the outcomes of other scenarios in the stored evidence are unchanged

#### Scenario: Run the tests of one evidence entry
Verification-ID: scn.linkindex.1f063f3c5e49

- **WHEN** `stele test <scenario-id>.e2e` runs for a scenario that also has unit evidence
- **THEN** only the e2e test runs and only its outcome is updated

#### Scenario: Run the tests of one requirement
Verification-ID: scn.linkindex.49a524bcf0ac

- **WHEN** `stele test <requirement-id>` runs for a requirement with two scenarios
- **THEN** the tests of both scenarios run and no other tests run

#### Scenario: Run the tests of one specification file
Verification-ID: scn.linkindex.e0b21aa95623

- **WHEN** `stele test openspec/changes/<change>/specs/<capability>/spec.md` runs for a change with two specification files
- **THEN** the tests of every scenario in that file run and no tests of the other file run

#### Scenario: Run the tests of one targeted evidence entry
Verification-ID: scn.linkindex.e5c1d7ab73d5
- **WHEN** `stele test <scenario-id>.android.unit` runs for a scenario with `ios` and `android` unit evidence
- **THEN** only the `android` unit test runs and only its outcome is updated

#### Scenario: Combine several targets
Verification-ID: scn.linkindex.6265c70bed70

- **WHEN** `stele test` receives a scenario ID from one requirement and the ID of another requirement
- **THEN** the tests of that scenario and of every scenario of the other requirement run, each test once

#### Scenario: Reject an unknown target
Verification-ID: scn.linkindex.5c91d0c168f0

- **WHEN** a selection names an identity that the scope does not declare, or a specification file that does not exist
- **THEN** the command exits with code `2`, names the selection, and runs no tests

#### Scenario: Leave parameterized test names unresolved
Verification-ID: scn.linkindex.57dcee8c30a8

- **WHEN** a test anchor precedes a test whose name contains template interpolation, or a `test.each` table
- **THEN** the anchor has no selector and the test is reported as not selectable instead of being run by a partial name

### Requirement: Check every scope at once
Verification-ID: req.linkindex.0c06109d8d29

With `--all`, the `stele test`, `stele verify`, `stele validate`, and `stele index` commands SHALL check the current specifications and every active change, each as its own scope. They SHALL report the result of each scope separately and exit with one combined code: `0` when every scope passes, `2` when any scope cannot be checked, and `1` otherwise. `--all` SHALL NOT be combined with `--change`, `--specs`, or test selections, and MAY be combined with `--target`.

#### Scenario: Verify every scope separately
Verification-ID: scn.linkindex.7c1d78728036

- **WHEN** `stele verify --all` runs in a project with current specifications and two active changes, one of which fails
- **THEN** the output reports the current specifications and each change separately, the failure names its change, and the command exits with code `1`

#### Scenario: Index every scope
Verification-ID: scn.linkindex.bda4420352ef

- **WHEN** `stele index --all` runs in a project with current specifications and two active changes
- **THEN** the index lists the items of all three scopes, each labelled with its scope

#### Scenario: Reject conflicting scope options
Verification-ID: scn.linkindex.4d6ba51a47bc

- **WHEN** `--all` is combined with `--change`, `--specs`, or a test selection
- **THEN** the command exits with code `2` and checks nothing

#### Scenario: Combine every scope with a target
Verification-ID: scn.linkindex.094a437d7c9c
- **WHEN** `stele verify --all --target web` runs in a project with current specifications and one active change that both declare the targets `api` and `web`
- **THEN** both scopes are verified for `web` only, each reported separately, and the command exits with one combined code
