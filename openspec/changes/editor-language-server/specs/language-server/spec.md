<!-- stele: spec v1 -->
## Purpose

Serve Stele's specification, code, and test links to any editor through the Language Server Protocol, so people can read, navigate, run, and check behavior from where they edit code.

## ADDED Requirements

### Requirement: Serve the Language Server Protocol over standard streams
Verification-ID: req.languageserver.950acbe48c80
The `stele lsp` command SHALL run a Language Server Protocol server that reads JSON-RPC messages with `Content-Length` framing from standard input and writes them to standard output. It SHALL write nothing else to standard output; logs SHALL go to standard error. It SHALL serve the workspace folders named in `initialize`, each resolved as a Stele project root, and advertise only the features this specification defines. Its `initialize` result SHALL name the server `stele` with the same version that `stele --version` prints. It SHALL answer requests received before `initialize` with the protocol's "server not initialized" error, answer a malformed message with a JSON-RPC error without stopping, and exit with code `0` on `exit` after `shutdown`, or code `1` on `exit` without `shutdown`.

#### Scenario: Complete a session through the installed binary
Verification-ID: scn.languageserver.3a0c2fb46bad
- **WHEN** a client starts the installed `stele lsp` executable, sends `initialize` with a workspace folder that contains a Stele project, then `initialized`, a hover request, `shutdown`, and `exit`
- **THEN** standard output contains only framed JSON-RPC responses, the `initialize` result advertises hover, definition, references, CodeLens, code actions, command execution, and diagnostics and names the version that `stele --version` prints, the hover returns the specification text, and the process exits with code `0`

#### Scenario: Survive malformed input
Verification-ID: scn.languageserver.16fcd1dc9b41
- **WHEN** the server receives a message whose body is not valid JSON, followed by a valid request
- **THEN** it answers the first with a JSON-RPC parse error and the second normally

#### Scenario: Refuse requests before initialization
Verification-ID: scn.languageserver.be03d984234b
- **WHEN** a hover request arrives before `initialize`
- **THEN** the server answers with the "server not initialized" error and serves no data

#### Scenario: Exit without shutdown
Verification-ID: scn.languageserver.e311dc0148f1
- **WHEN** the client sends `exit` without a preceding `shutdown`
- **THEN** the process exits with code `1`

### Requirement: Show specification text and status on hover
Verification-ID: req.languageserver.c8c10808862b
Hovering an `@implements` or `@verifies` anchor SHALL show a card for the linked requirement or scenario, the way an editor shows a doc comment: its title, its scope, and its specification text. A requirement's text SHALL be its body from the link index. A scenario's text SHALL be its steps from the link index, one line per step in specification order, with the keyword (`GIVEN`, `WHEN`, `THEN`, `AND`, …) emphasized and the step's full text; a step without a keyword SHALL show its text alone, and a scenario without steps SHALL show its raw text. For an evidence anchor the card SHALL also show the evidence level, the approval state, and the last execution outcome, marked stale when it is. Hovering a requirement or scenario heading or its `Verification-ID` line in a `spec.md` SHALL list where that behavior is implemented and tested, with each evidence entry's level, approval, and outcome. When a requirement exists in more than one scope, the hover SHALL show each scope's text, labelled. The hover SHALL use Markdown when the client declares Markdown support and plain text otherwise.

#### Scenario: Hover an implementation anchor
Verification-ID: scn.languageserver.59068d53a952
- **WHEN** the cursor is on the ID of an `// @implements req.…` comment for a requirement declared in the current specifications
- **THEN** the hover shows the requirement title, the scope `specs`, and the requirement body text

#### Scenario: Hover an evidence anchor
Verification-ID: scn.languageserver.015bb7af18f8
- **WHEN** the cursor is on the ID of a `// @verifies scn.….unit` comment whose evidence is approved and whose last outcome passed before an input changed
- **THEN** the hover shows the scenario title, its steps, the level `unit`, the approval state `approved`, and the outcome `passed` marked stale

#### Scenario: Render scenario steps as a card
Verification-ID: scn.languageserver.5c3579497b2b
- **WHEN** the cursor is on an evidence anchor of a scenario whose steps are a `WHEN` bullet that continues on the next line, a `THEN` bullet, an `AND` bullet, and a bullet without a bold keyword
- **THEN** the hover shows four step lines in that order, the first with its joined text, the keywords `WHEN`, `THEN`, and `AND` emphasized, and the last step's text without a keyword

#### Scenario: Hover a scenario heading in a specification
Verification-ID: scn.languageserver.aa36d54d66ec
- **WHEN** the cursor is on a `#### Scenario:` heading whose scenario has a unit test anchor and a planned e2e entry without an anchor
- **THEN** the hover lists the unit test location with its outcome and the e2e entry as planned without a test

#### Scenario: Show both scopes of a modified requirement
Verification-ID: scn.languageserver.cd81e42944ee
- **WHEN** an active change modifies a requirement that also exists in the current specifications and the cursor is on an anchor for its ID
- **THEN** the hover shows the current text and the proposed text, each labelled with its scope

#### Scenario: Fall back to plain text
Verification-ID: scn.languageserver.62861d0e5875
- **WHEN** the client does not declare Markdown in its hover content formats
- **THEN** the hover content is plain text with the same information

### Requirement: Navigate between specification, code, and tests
Verification-ID: req.languageserver.488754628741
Go to definition SHALL work in both directions. From an anchor ID it SHALL return the heading of the requirement or scenario that declares it, in every scope that declares it. From a requirement heading or `Verification-ID` line it SHALL return every `@implements` anchor location of that requirement. From a scenario heading or `Verification-ID` line it SHALL return every `@verifies` anchor location of that scenario's evidence. Locations SHALL be ordered by path and line. An ID no specification declares SHALL return no location.

#### Scenario: Jump from an anchor to its specification
Verification-ID: scn.languageserver.08b3c3878af0
- **WHEN** definition is requested on the ID of an `@verifies scn.….e2e` anchor
- **THEN** the result is the location of that scenario's heading in its `spec.md`

#### Scenario: Jump from a requirement to its implementations
Verification-ID: scn.languageserver.3f9633e42fd4
- **WHEN** definition is requested on a requirement heading that has two `@implements` anchors in different files
- **THEN** the result is both anchor locations, ordered by path and line

#### Scenario: Jump from a scenario to its tests
Verification-ID: scn.languageserver.28e4bd385efa
- **WHEN** definition is requested on a scenario heading with unit and e2e test anchors
- **THEN** the result is both test anchor locations

### Requirement: Find every use of a requirement or scenario
Verification-ID: req.languageserver.70e72541715e
Find references SHALL answer "where is this behavior used?" from a requirement or scenario heading, its `Verification-ID` line, or the ID in an anchor. For a requirement it SHALL return every `@implements` anchor of the requirement and every `@verifies` anchor of its scenarios' evidence. For a scenario, or an evidence ID in an anchor, it SHALL return every `@verifies` anchor of that scenario's evidence. When the request asks to include the declaration, the result SHALL also contain the declaring heading in every scope that declares the ID. Locations SHALL be ordered by path and line, each once. An ID no specification declares SHALL return the anchors that name it and no heading.

#### Scenario: Find the uses of a requirement
Verification-ID: scn.languageserver.100060104a1b
- **WHEN** references are requested with the declaration included on a requirement heading that exists in the current specifications and in an active change, with one `@implements` anchor and two scenarios that each have a unit test anchor
- **THEN** the result is both headings, the implementation anchor, and both test anchors, ordered by path and line

#### Scenario: Find the tests of a scenario from one of its tests
Verification-ID: scn.languageserver.e42ad028df53
- **WHEN** references are requested without the declaration on the ID of an `@verifies scn.….unit` anchor whose scenario also has an e2e test anchor
- **THEN** the result is the unit and e2e test anchors and no heading

### Requirement: Offer run actions, status, and summaries as CodeLens
Verification-ID: req.languageserver.a798154d936e
The server SHALL provide CodeLens items in `spec.md` files on every requirement and scenario heading that has planned evidence or test anchors: a "Run all" action, one run action per evidence level present, and a status summary of each level's last outcome, such as `unit ✓ · e2e ✗ · stale`. A requirement's summary SHALL combine its scenarios. A combined status SHALL follow the rule `stele verify` uses for its execution verdict: failed when any outcome failed, otherwise stale when any is stale, otherwise not run when any has no current outcome, and passed only when every outcome passed. Levels without an execution SHALL show as not run, and unapproved entries SHALL be marked as such. Each code and test anchor SHALL get a summary lens with the linked behavior's title and its first `WHEN` and `THEN` steps, shortened to a fixed length, whose action shows the specification heading. Each test anchor SHALL also get a run action for its own evidence entry. Every action SHALL be a command the server itself executes, and every run action's arguments SHALL be the positional targets that `stele test` accepts. When the client can show documents, the show action SHALL ask the client to show the heading; otherwise it SHALL show the card as a message.

#### Scenario: Show run actions and status on a scenario
Verification-ID: scn.languageserver.f584d3ec6c44
- **WHEN** a scenario has unit evidence that passed and e2e evidence that failed, both current
- **THEN** its heading shows "Run all", "Run unit", "Run e2e", and the status `unit ✓ · e2e ✗`

#### Scenario: Summarize a requirement
Verification-ID: scn.languageserver.f1cf94d0c1d6
- **WHEN** a requirement has two scenarios, one passed and one never run
- **THEN** its heading shows "Run all" with the requirement ID as target and a status that shows one scenario passed and one not run, combined as not run

#### Scenario: Run one evidence entry from a test
Verification-ID: scn.languageserver.cdca0ea9f34c
- **WHEN** a test is anchored with `@verifies scn.….unit`
- **THEN** the anchor line shows a run action whose target is `scn.….unit`

#### Scenario: Summarize the linked scenario above a test
Verification-ID: scn.languageserver.28e44f61a51f
- **WHEN** a test is anchored with `@verifies scn.….unit` for a scenario whose steps are a long `WHEN`, a `THEN`, and an `AND`
- **THEN** the anchor line shows a lens with the scenario title and the `WHEN` and `THEN` steps, shortened to the fixed length with an ellipsis and without the `AND` step

#### Scenario: Show the specification from a summary lens
Verification-ID: scn.languageserver.53ed0e506ce6
- **WHEN** the client executes a summary lens's action, once in a session whose client declares support for showing documents and once in a session whose client does not
- **THEN** the first session asks the client to show the scenario heading in its `spec.md`, and the second shows a message with the plain-text card

### Requirement: Run selected tests and merge their results
Verification-ID: req.languageserver.e3ab377dc59f
Executing a run command SHALL run the selected tests through the same runner as `stele test <targets...>`, with the same selection, batching, and merging: only the selected tests run, in the batches the command uses (one test process per Go package and build tag set, or per Node test file, one batch after another), their outcomes are merged into the stored evidence without discarding other outcomes, and the command's result reports each selected evidence entry's outcome, every batch that did not complete, and the verdicts `linkage`, `execution`, and `overall` of each affected scope, as `stele verify` computes them after the run. While tests run, the server SHALL report progress when the client supports it: a start with the number of selected tests, one report per finished test with the counts run, passed, and failed, the percentage, and the failed test's name, one report for each batch that did not complete naming it and how its process ended, and an end with the summary. When the client cancels, the server SHALL stop the run: it stops the running test process and every process that process started, starts no further batch, writes no evidence, and ends the command as cancelled. After a run, the server SHALL refresh its CodeLens items and diagnostics. An unknown target SHALL fail the command with an error that names it before any test runs. While one run is in progress in a project, another run command for that project SHALL fail with an error that names the running targets.

#### Scenario: Run a scenario from its heading
Verification-ID: scn.languageserver.074136f1f47f
- **WHEN** the client executes the run command with one scenario ID in a project where other scenarios have stored outcomes
- **THEN** only that scenario's tests run, its outcomes are stored, the other stored outcomes are unchanged, and the refreshed status shows the new outcomes

#### Scenario: Reject an unknown target
Verification-ID: scn.languageserver.a12e36dfdfb8
- **WHEN** the client executes the run command with an ID the workspace does not declare
- **THEN** the command fails with an error naming the ID and no test runs

#### Scenario: Cancel a running command
Verification-ID: scn.languageserver.3c56123372c1
- **WHEN** the client cancels a run while a batch's test process, which has started a child process of its own, is still running and another batch has not started yet
- **THEN** the test process and its child are stopped, the other batch never starts, the command ends as cancelled, and the stored evidence file is unchanged

#### Scenario: Refuse a second concurrent run
Verification-ID: scn.languageserver.ddba3d8c9626
- **WHEN** the client executes a run command while another run is in progress
- **THEN** the second command fails with an error that names the running targets, and the first run continues

#### Scenario: Report progress while tests run
Verification-ID: scn.languageserver.159800f00b8d
- **WHEN** a client that supports work-done progress runs three tests and the second one fails
- **THEN** the server sends a progress start naming three tests, three reports whose run counts rise from one to three with the passed and failed counts so far and the failed test's name, and a progress end with the summary

#### Scenario: Report the verdicts of the scope after a run
Verification-ID: scn.languageserver.dfffd1e421b4
- **WHEN** the client runs one scenario whose tests pass, in a scope whose other tests passed before and that has approved evidence without a test anchor
- **THEN** the command's result lists the scenario's outcomes as passed and the scope's verdicts as linkage `fail`, execution `passed`, and overall `fail`

#### Scenario: Report a batch that did not complete
Verification-ID: scn.languageserver.40c7e3ae9243
- **WHEN** the client runs two tests of one batch whose process exits after reporting the first test's result and before the second's
- **THEN** progress reports the batch with how its process ended, the result lists the first test's outcome and the second test as failed because its process failed, and the result names the batch as not completed

### Requirement: Report Stele findings as diagnostics
Verification-ID: req.languageserver.c4e7f3210359
The server SHALL publish, for every file that has a problem, the findings that `stele verify` reports for each served scope, with the same code and severity, at their location or, without one, at the heading of the identity they name. The current specifications SHALL be checked at the implementation stage, and an active change at the implementation stage once its linkage plan has an approved entry and at the proposal stage before that. `PLAN_UNAPPROVED` SHALL NOT be published. The server SHALL also publish `EXECUTION_FAILED` warnings for failed last outcomes and `EXECUTION_STALE` information for stale ones, on the scenario heading and on the test anchor. Every diagnostic's message SHALL contain the finding's own message, the meaning and the fix step from the diagnostic catalogue that the terminal report uses, with the fix naming the finding's scope. A finding that several scopes report at the same place SHALL be published once. When a problem is fixed, its diagnostic SHALL be cleared.

#### Scenario: Flag an unknown ID in an anchor
Verification-ID: scn.languageserver.e6a664740343
- **WHEN** a source file contains `// @implements req.todo.000000000000` and no specification declares that ID
- **THEN** that line has an error diagnostic with the code `stele verify` reports for an undeclared anchor

#### Scenario: Flag approved evidence without a test
Verification-ID: scn.languageserver.e33a998bdf85
- **WHEN** a scenario's e2e evidence entry is approved and no test carries `@verifies` for it
- **THEN** the scenario heading has a `LINK_EVIDENCE_MISSING` diagnostic with the severity `stele verify` reports, naming the missing evidence ID

#### Scenario: Flag stale results
Verification-ID: scn.languageserver.72a3e3ef9ecb
- **WHEN** a scenario's stored outcome was recorded before one of the verified inputs changed
- **THEN** the scenario heading and its test anchor have an informational diagnostic that says the result is stale

#### Scenario: Clear a fixed problem
Verification-ID: scn.languageserver.106cfbc87c5b
- **WHEN** the unknown ID in an anchor is corrected to a declared ID
- **THEN** the diagnostic on that line is removed

#### Scenario: Explain a finding with the diagnostic catalogue
Verification-ID: scn.languageserver.3efdcf7c5cf2
- **WHEN** an active change has approved evidence without a test anchor
- **THEN** the diagnostic's message contains the finding's message, the catalogue's meaning for `LINK_EVIDENCE_MISSING`, and its fix step, and the fix names `--change` with the change ID

#### Scenario: Show annotation problems in a specification
Verification-ID: scn.languageserver.2d35637c302f
- **WHEN** one delta spec of an active change has no Stele annotation and another starts with `<!-- stele: spec v2 -->`, in a project without an `unannotatedSpecs` policy
- **THEN** the first file has a `SPEC_ANNOTATION_MISSING` warning on line 1 whose fix names `stele annotate --change` with the change ID, and the second has a `SPEC_ANNOTATION_UNSUPPORTED` error on line 1

#### Scenario: Flag a test anchored to removed behavior
Verification-ID: scn.languageserver.8431050348d1
- **WHEN** an active change with an approved plan entry removes a requirement, and a test still carries `@verifies` for one of that requirement's scenarios
- **THEN** the anchor line has a `LINK_REMOVED_BEHAVIOR_ANCHORED` error

#### Scenario: Check a change in planning at the proposal stage
Verification-ID: scn.languageserver.5df656e62280
- **WHEN** an active change has requirements without code anchors and a linkage plan with no approved entry, and then one entry is approved
- **THEN** before the approval neither `LINK_CODE_MISSING` nor `PLAN_UNAPPROVED` is published for the change, and after it the requirement headings have `LINK_CODE_MISSING` diagnostics and still no `PLAN_UNAPPROVED`

### Requirement: Add the Stele annotation from the editor
Verification-ID: req.languageserver.c37c196d0a2e
The server SHALL offer a quick fix named "Add Stele annotation" for every `SPEC_ANNOTATION_MISSING` diagnostic of an open specification file. Its edit SHALL produce exactly the bytes that `stele annotate` writes for the same content: `<!-- stele: spec v1 -->` inserted as the first line, after a byte order mark if there is one, ended with the file's first line ending, or with a line feed when the file has none. It SHALL be the preferred fix and name the diagnostic it resolves. The server SHALL NOT offer it for a file with a malformed, unsupported, or misplaced annotation. When the client cannot receive code action literals, the server SHALL offer the same fix as a command that applies the edit through the client.

#### Scenario: Offer the annotation quick fix
Verification-ID: scn.languageserver.4edc5f1a06b4
- **WHEN** code actions are requested for line 1 of an open delta spec that has no annotation
- **THEN** the result contains one preferred quick fix "Add Stele annotation" linked to the `SPEC_ANNOTATION_MISSING` diagnostic, whose edit inserts the annotation at the start of the file

#### Scenario: Write the same bytes as stele annotate
Verification-ID: scn.languageserver.fe68c635aa85
- **WHEN** the quick fix's edit is applied to a specification with CRLF line endings and no final newline, and to one that starts with a byte order mark
- **THEN** each result is byte-for-byte what `stele annotate` writes for the same file

#### Scenario: Leave broken annotations to a person
Verification-ID: scn.languageserver.4b2f05995999
- **WHEN** code actions are requested for a specification whose first line is `<!-- stele: spec -->`, one whose first line is `<!-- stele: spec v2 -->`, and one with the annotation on line 3
- **THEN** no "Add Stele annotation" fix is offered for any of them

#### Scenario: Apply the fix through a command
Verification-ID: scn.languageserver.53c81eb42bf4
- **WHEN** the client does not declare support for code action literals and executes the offered "Add Stele annotation" command
- **THEN** the server asks the client to apply the same edit to the file

### Requirement: Keep the index current as files change
Verification-ID: req.languageserver.ab7fbe14be03
The server SHALL reflect the content of open documents, including unsaved edits, and of files on disk. It SHALL update its index when a document is opened, changed, saved, or closed, and when specifications, sources, tests, linkage plans, the project configuration, or the evidence file are created, changed, or deleted on disk, including by a `stele test` run in a terminal. It SHALL watch these files through the client when the client supports dynamic file-watch registration, and otherwise by checking the files itself. An update SHALL re-read only the changed files, and the result SHALL equal a full rebuild of the same state. Whether a stored outcome is stale SHALL be decided from the saved files, because tests run against them.

#### Scenario: Reflect an unsaved edit
Verification-ID: scn.languageserver.ba15e715dc8b
- **WHEN** a user types a new `@implements` anchor for a declared requirement into an open file without saving
- **THEN** hovering the new anchor shows the requirement, and definition on the requirement heading includes the new location

#### Scenario: Pick up results from a terminal run
Verification-ID: scn.languageserver.4aa8ef7297c3
- **WHEN** `stele test` runs in a terminal and rewrites the evidence file while the server is running
- **THEN** the CodeLens status and diagnostics reflect the new outcomes without restarting the server

#### Scenario: Pick up a new specification file
Verification-ID: scn.languageserver.39afeb90a2e9
- **WHEN** a new `spec.md` with a requirement and scenario is created in an active change
- **THEN** its headings get CodeLens items and anchors for its IDs stop being reported as unknown

#### Scenario: Watch files without client support
Verification-ID: scn.languageserver.a8fc0b7b6625
- **WHEN** the client does not support dynamic file-watch registration and a test file changes on disk
- **THEN** the server still updates its index for that change

#### Scenario: Match a full rebuild after incremental updates
Verification-ID: scn.languageserver.82c1741b92aa
- **WHEN** a sequence of edits, creations, and deletions is applied incrementally
- **THEN** the resulting index equals the index built from scratch for the final state

### Requirement: Stay deterministic and editor-neutral
Verification-ID: req.languageserver.2bab83c5c10f
For identical workspace content and identical requests, the server SHALL send identical responses and diagnostics, with lists in a stable order and without timestamps. Only progress notifications of a run MAY follow the order in which tests finish; a run's result SHALL NOT. Its behavior SHALL depend only on the capabilities the client declares, never on the client's name or version. The server SHALL answer a `stele/index` request with the same document that `stele index --json --all` prints for the same saved files.

#### Scenario: Answer identically for identical state
Verification-ID: scn.languageserver.8d66c5680555
- **WHEN** two server sessions receive the same sequence of requests for the same workspace content
- **THEN** every response is byte-for-byte identical

#### Scenario: Match the command-line index
Verification-ID: scn.languageserver.bc6723089290
- **WHEN** the installed binary answers `stele/index` for a workspace with saved files, including an unannotated specification, and `stele index --json --all` runs in the same workspace
- **THEN** both documents are byte-for-byte identical

#### Scenario: Ignore the client identity
Verification-ID: scn.languageserver.300e9d84bbbd
- **WHEN** two sessions declare identical capabilities but different `clientInfo` names
- **THEN** their responses to the same requests are identical
