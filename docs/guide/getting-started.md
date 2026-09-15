# Getting started

Stele verifies that plain-English OpenSpec behavior is connected to its planned implementation and executable tests. It is installed per project and runs locally or in CI.

## Prerequisites

- Node.js 24 or newer, with native `.mts` execution
- Go 1.24 or newer when developing Stele itself
- An OpenSpec change with requirements and scenarios

Consumers execute the compiled Go binary carried by the npm package. Node is still required for npm, the bundled OpenSpec CLI, and JavaScript tests. Consumers do not need Go for normal verification.

## Install

```bash
npm install --save-dev stele-spec
npx stele init --change account-recovery
```

Initialization creates:

```text
stele.config.json
.agents/skills/stele-plan/SKILL.md
.agents/skills/stele-verify/SKILL.md
artifacts/
```

The generated configuration selects the OpenSpec adapter and a default change:

```json
{
  "schemaVersion": 1,
  "adapter": "openspec",
  "change": "account-recovery"
}
```

Running `init` again is safe. Existing configuration and skill files are preserved.

## Add the two identities

Give each OpenSpec requirement one `req` identity and each scenario one `scn` identity:

```markdown
### Requirement: Request a recovery link
Verification-ID: req.recovery.22b616c90f42

The system SHALL accept a registered account email.

#### Scenario: Registered email
Verification-ID: scn.recovery.20d9cd2785a4

- **WHEN** the user submits a registered email
- **THEN** the system accepts the request
```

## Plan before implementation

Map each requirement to the declaration expected to enforce it and each scenario to an independently selectable test:

```json
{
  "schemaVersion": 1,
  "changeId": "account-recovery",
  "requirements": {
    "req.recovery.22b616c90f42": "src/recovery.ts#requestRecovery"
  },
  "scenarios": {
    "scn.recovery.20d9cd2785a4": "test/recovery.test.ts#accepts a registered email"
  }
}
```

Save this as `artifacts/linkage-plan.json`, then check the plan:

```bash
npx stele verify --stage proposal
```

Proposal verification allows targets that have not been created yet.

## Implement and verify

Attach `@implements` to the code declaration and `@verifies` to the named test. Then run:

```bash
npx stele validate
```

Validation runs the selected scenario tests, OpenSpec strict validation, and implementation linkage verification. Exit code `0` means all selected checks passed; see the [CLI reference](/reference/cli) for the complete contract.

## Commit inputs, ignore outputs

Commit the specifications, `stele.config.json`, linkage plan, source anchors, and test anchors. Generated reports under `artifacts/` can be uploaded by CI and ignored locally when your review process does not require committed evidence.
