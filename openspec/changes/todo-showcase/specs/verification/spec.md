## Purpose

Add deterministic stable-identity and linkage checks around OpenSpec without replacing OpenSpec's artifact or synchronization model.

## ADDED Requirements

### Requirement: Parse stable behavioral identities
Verification-ID: req.verify.a18c03ef72b6

The verifier SHALL discover uniquely formatted requirement and scenario IDs from the selected OpenSpec change and preserve their parent relationships.

#### Scenario: Parse valid change specs
Verification-ID: scn.verify.c905a1e37fd8

- **WHEN** the selected change contains valid unique requirement and scenario IDs
- **THEN** the verifier emits one normalized record for every identity

#### Scenario: Reject duplicate identity
Verification-ID: scn.verify.1ae4d6739cb0

- **WHEN** an identity is declared more than once outside a valid baseline transition
- **THEN** the verifier emits a stable duplicate-ID diagnostic

### Requirement: Verify stage-appropriate links
Verification-ID: req.verify.b6e8f421cd09

The verifier SHALL allow planned targets in proposal mode and require resolvable code/test anchors in implementation mode.

#### Scenario: Proposal accepts planned targets
Verification-ID: scn.verify.845e10cb79d6

- **WHEN** proposal mode receives complete planned requirement and scenario targets
- **THEN** verification passes even if implementation targets do not yet exist

#### Scenario: Implementation resolves anchors
Verification-ID: scn.verify.732cf49a0e18

- **WHEN** implementation mode scans the completed showcase
- **THEN** every requirement has a code anchor and every scenario has a test anchor

#### Scenario: Reject dangling anchor
Verification-ID: scn.verify.e3817b0dcf54

- **WHEN** a code or test anchor names an undeclared identity
- **THEN** the verifier emits a stable dangling-anchor diagnostic

### Requirement: Report evidence dimensions independently
Verification-ID: req.verify.d3975ac8e142

The verifier SHALL report linkage, execution, pass/fail, and review states independently in versioned JSON.

#### Scenario: Produce dashboard report
Verification-ID: scn.verify.f41ca285d706

- **WHEN** verification completes
- **THEN** a schema-versioned report contains deterministic diagnostics, requirement records, evidence dimensions, and an explicit completeness flag

### Requirement: Reproduce local validation
Verification-ID: req.verify.6b2d7904ea51

The project SHALL provide one command that runs automated tests, records revision-bound execution evidence, and performs implementation verification.

#### Scenario: Full validation succeeds
Verification-ID: scn.verify.29f07d86bc4e

- **WHEN** a contributor runs `npm run validate` on a valid checkout
- **THEN** tests and implementation verification pass and refresh their JSON artifacts

### Requirement: Provide an executable CLI
Verification-ID: req.verify.4c82d1a90fe7

The project SHALL provide a repository-owned `stele` executable with verify, test, and validate commands plus stable documented exit codes.

#### Scenario: Invoke repository CLI
Verification-ID: scn.verify.5e9a130cd7b4

- **WHEN** a contributor runs `npm run stele -- verify --json`
- **THEN** the executable prints the versioned verification report and exits successfully for a valid project

#### Scenario: Return stable failure exit
Verification-ID: scn.verify.a2c7e48b610f

- **WHEN** verification finds a policy violation
- **THEN** the CLI exits 1, while invalid invocation or tool failure exits 2

### Requirement: Emit deterministic machine output
Verification-ID: req.verify.91b3e6f04ac2

The CLI SHALL emit byte-for-byte identical default JSON for identical relevant inputs, independent of run time and file enumeration order.

#### Scenario: Repeat verification
Verification-ID: scn.verify.d6f8012b3ea5

- **WHEN** verification runs twice without relevant input changes
- **THEN** the two default JSON byte streams are identical

#### Scenario: Keep volatile metadata outside default payload
Verification-ID: scn.verify.3a70c9d1ef24

- **WHEN** the CLI emits its default deterministic report
- **THEN** it contains no wall-clock timestamp or random identifier

### Requirement: Resolve anchors to declared targets
Verification-ID: req.verify.e25a1c7b490d

The verifier SHALL associate every implementation and test anchor with a nearby declaration and ensure it matches the planned file and selector.

#### Scenario: Resolve adjacent declaration
Verification-ID: scn.verify.72d4b0e91fac

- **WHEN** an anchor immediately precedes a compatible function, method, class, or test declaration at its planned target
- **THEN** the report records that declaration as the resolved selector

#### Scenario: Reject misplaced or mismatched anchor
Verification-ID: scn.verify.b5e72a0c4d89

- **WHEN** an anchor has no nearby compatible declaration or differs from its planned path or selector
- **THEN** verification emits a stable target-resolution diagnostic and exits 1

### Requirement: Record scenario-specific execution
Verification-ID: req.verify.6fc019e4a2b8

The CLI SHALL execute each unique scenario-anchored test declaration and map its result only to the attached scenarios.

#### Scenario: Map passing test
Verification-ID: scn.verify.8d2e41b70ca3

- **WHEN** an anchored test declaration executes and passes
- **THEN** only its attached scenarios receive a passed outcome for the current input digest

#### Scenario: Map failing test
Verification-ID: scn.verify.1c7ab038e529

- **WHEN** an anchored test declaration executes and fails
- **THEN** its attached scenarios receive a failed outcome and the CLI exits 1
