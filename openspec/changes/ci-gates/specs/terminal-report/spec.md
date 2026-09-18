<!-- stele: spec v1 -->
## ADDED Requirements

### Requirement: Annotate findings in GitHub Actions
Verification-ID: req.terminalreport.b475f4f66880

`stele validate`, `stele verify`, `stele test`, and `stele check` SHALL accept `--annotations=auto|github|never`. With `auto`, the default, Stele SHALL write annotations only when the environment variable `GITHUB_ACTIONS` is `true`. `github` and `never` SHALL override the environment, and any other value SHALL exit with code `2` before running anything.

When it writes annotations, Stele SHALL write to standard error, after the run, one GitHub Actions workflow command per finding that the report shows and per failed test:

- `::error` for errors and failed tests, and `::warning` for warnings;
- `file` and `line` properties when the location is known, and a `title` property naming the diagnostic code, or `Test failed` for a failed test;
- a message with the one-line meaning of the code, and the evidence ID or identity with its requirement or scenario title when known.

A group that the report truncates SHALL add one annotation without a location that gives the number of further findings and names `--details`. File paths SHALL be relative to `GITHUB_WORKSPACE` when the project root lies inside it, and relative to the project root otherwise. Property values and messages SHALL be escaped as GitHub Actions requires. Annotations SHALL follow the order of the report, repeat no identical line within one command, and be identical for identical inputs. Standard output, JSON output, report files, evidence files, and exit codes SHALL be the same whatever `--annotations` says, and `--quiet` and `--json` SHALL NOT suppress annotations.

#### Scenario: Annotate a finding at its location
Verification-ID: scn.terminalreport.d9386a6ce84d

- **WHEN** `stele validate --change example` runs with `GITHUB_ACTIONS=true` and a test at `tests/todo.test.mts:14` has an anchor for an identity that no specification declares
- **THEN** standard error contains a line starting with `::error file=tests/todo.test.mts,line=14,title=ANCHOR_DANGLING::`, and standard output and the exit code are the same as for the run without `GITHUB_ACTIONS`

#### Scenario: Annotate a failed test
Verification-ID: scn.terminalreport.447957e12de7

- **WHEN** a command runs with `GITHUB_ACTIONS=true` and the test at `tests/todo.test.mts:20` fails
- **THEN** standard error contains a line starting with `::error file=tests/todo.test.mts,line=20,title=Test failed::` whose message names the test's evidence ID

#### Scenario: Annotate only in GitHub Actions unless asked
Verification-ID: scn.terminalreport.e045d1b2d55c

- **WHEN** `stele validate` fails with findings, once without `GITHUB_ACTIONS`, once with `--annotations=github` and without `GITHUB_ACTIONS`, and once with `GITHUB_ACTIONS=true` and `--annotations=never`
- **THEN** only the run with `--annotations=github` writes lines starting with `::` to standard error

#### Scenario: Reject an unknown annotation mode
Verification-ID: scn.terminalreport.50fd0b8c9244

- **WHEN** `stele validate --annotations=gitlab` runs
- **THEN** the command exits with code `2` before running anything and names the accepted values

#### Scenario: Follow the truncation of the report
Verification-ID: scn.terminalreport.be5c922ef009

- **WHEN** the current specifications have 123 `PLAN_UNAPPROVED` findings and `stele validate --specs` runs with `GITHUB_ACTIONS=true`
- **THEN** standard error has five `PLAN_UNAPPROVED` annotations with their locations and one annotation without a location that says 118 more findings exist and names `--details`, and with `--details` it has all 123 and no such annotation

#### Scenario: Locate files from the workspace root
Verification-ID: scn.terminalreport.fbadf694cd04

- **WHEN** `stele validate --root app` runs with `GITHUB_ACTIONS=true`, `GITHUB_WORKSPACE` names the directory that contains `app`, and a finding is at `src/todo.ts:3` in the project
- **THEN** its annotation has the property `file=app/src/todo.ts`

#### Scenario: Escape workflow command values
Verification-ID: scn.terminalreport.f466c27970e9

- **WHEN** an annotation's message contains `%` and a line break, and its file path contains a comma
- **THEN** the message has `%25` and `%0A` in their place, the path has `%2C` in place of the comma, and the annotation is one line
