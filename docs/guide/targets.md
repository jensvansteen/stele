# Targets

Many products build one behavior in several places: an iOS and an Android app, a VS Code, Zed, and JetBrains extension, a web front end and its API, and an end-to-end suite that proves the whole flow. Targets let one specification describe that behavior once, while every place proves it with its own evidence.

**Targets are optional.** Without them, a specification describes the project itself, and Stele works exactly as it did before targets existed: the same evidence IDs, approvals, and output, byte for byte. You opt in only where one specification covers several places.

## Concepts

- A **target** is a place that produces evidence: a codebase, an app, or a test suite, such as `ios`, `android`, `web`, `api`, `vscode`, or `system`.
- `stele.config.json` is the only registry of targets. Each target has the `paths` where its code and tests live, so `andriod` in a specification is an error instead of a silent fourth target.
- A specification declares its targets on its first line: `<!-- stele: spec v1; targets: ios, android -->`. Every requirement and scenario applies to every declared target.
- A requirement or scenario narrows with a `Targets:` line directly below its heading. It may only narrow, never widen.
- **Behavior differences go in the specification**, as scenarios narrowed with `Targets:`. **Testing differences go in the plan**, as evidence levels per target.
- A specification without `targets` still describes the whole project, also in a project that uses targets.

```json
{
  "schemaVersion": 1,
  "targets": {
    "ios": { "paths": ["ios/**", "shared/**"] },
    "android": { "paths": ["android/**", "shared/**"] },
    "system": { "paths": ["e2e/**"], "evidenceOnly": true }
  }
}
```

In `paths`, `*` matches within one path segment and `**` any number of segments. Paths of different targets may overlap, for shared code such as a Kotlin Multiplatform module. A target name is lowercase, and `unit`, `integration`, and `e2e` are reserved for levels. `evidenceOnly` marks a target that proves behavior without implementing it, such as an end-to-end suite.

## What Stele checks per target

| Rule | Diagnostic |
|---|---|
| Every scenario has one or more plan entries for every target it applies to | `PLAN_EVIDENCE_MISSING`, naming the target |
| An entry's `target` matches its evidence ID, and the scenario applies to it | `PLAN_EVIDENCE_INVALID`, `PLAN_TARGET_NOT_APPLICABLE` |
| A `@verifies` anchor lies within its target's `paths` | `ANCHOR_TARGET_OUTSIDE_PATHS` |
| A requirement has an `@implements` anchor within the paths of every target it applies to, except `evidenceOnly` targets | `LINK_TARGET_IMPLEMENTATION_MISSING` |

A targeted evidence ID puts the target between the scenario and the level: `scn.share.3c4d5e6f7a8b.android.integration`. The anchor says which target it proves, so a test copied from the iOS app into the Android app with its ID unchanged is caught.

The report shows a scenario by target matrix, and the [link index](/reference/link-index#targets) contains it in full:

```text
  Target matrix          android    ios
  share                  1/2        3/3
    Share an empty list  ✗ missing  ✓ passed
```

## Example 1: editor extensions as replicas

The `stele-editors` repository builds the same editor features three times. Every listed target needs its own evidence, and the matrix shows which replica is behind.

```json
"targets": {
  "jetbrains": { "paths": ["jetbrains/**"] },
  "vscode": { "paths": ["vscode/**"] },
  "zed": { "paths": ["zed/**"] }
}
```

```markdown
<!-- stele: spec v1; targets: vscode, zed, jetbrains -->
## ADDED Requirements

### Requirement: Show a code lens above every anchored declaration
Verification-ID: req.lens.1a2b3c4d5e6f

The editor SHALL show a code lens naming the requirement above each `@implements` anchor.

#### Scenario: Lens on an implemented function
Verification-ID: scn.lens.0a1b2c3d4e5f

- **WHEN** a TypeScript function has `@implements req.todo.…`
- **THEN** a lens above it names the requirement and opens its specification
```

```json
"scn.lens.0a1b2c3d4e5f": {
  "evidence": [
    { "id": "scn.lens.0a1b2c3d4e5f.vscode.e2e", "target": "vscode", "level": "e2e", "rationale": "…" },
    { "id": "scn.lens.0a1b2c3d4e5f.zed.integration", "target": "zed", "level": "integration", "rationale": "…" },
    { "id": "scn.lens.0a1b2c3d4e5f.jetbrains.integration", "target": "jetbrains", "level": "integration", "rationale": "…" }
  ]
}
```

Each replica's tests carry their own `@verifies`, and each replica's code its own `@implements`. With the JetBrains plugin not started yet:

```text
  Target matrix                      jetbrains  vscode    zed
  lens                               0/1        1/1       1/1
    Lens on an implemented function  ✗ missing  ✓ passed  ✓ passed
```

## Example 2: a checkout split over an API and a web front end

This is OpenSpec's `add-checkout-promo` example in one repository. The requirements are split by responsibility, and a contract requirement lists both sides.

```json
"targets": {
  "api": { "paths": ["api/**"] },
  "web": { "paths": ["web/**"] }
}
```

```markdown
<!-- stele: spec v1; targets: api, web -->
## ADDED Requirements

### Requirement: Validate promo codes
Verification-ID: req.checkout.6f7a8b9c0d1e
Targets: api

The checkout API SHALL reject expired promo codes with `410` and the code `PROMO_EXPIRED`.

#### Scenario: Reject an expired code
Verification-ID: scn.checkout.7a8b9c0d1e2f

- **WHEN** an expired code is posted
- **THEN** the API answers `410` with `PROMO_EXPIRED`

### Requirement: Show the promo discount
Verification-ID: req.checkout.8b9c0d1e2f3a
Targets: web

The checkout page SHALL show the discount line once a code is accepted.

#### Scenario: Show the discount line
Verification-ID: scn.checkout.1f2a3b4c5d6e

- **WHEN** the API accepts a code
- **THEN** the page shows the discount line

### Requirement: Promo contract
Verification-ID: req.checkout.9c0d1e2f3a4b

`POST /checkout/promo` SHALL accept `{ "code": string }` and return `{ "discount": { "amount": integer, "currency": string } }`.

#### Scenario: Apply a valid code
Verification-ID: scn.checkout.0d1e2f3a4b5c

- **WHEN** a valid code is submitted
- **THEN** the response has the discount amount and currency, and the page shows it
```

Each side proves its own part of the contract: `api` shows that the handler serves the documented shape, and `web` shows that the client parses and renders it, against a stub built from the same contract.

```json
"scn.checkout.0d1e2f3a4b5c": {
  "evidence": [
    { "id": "scn.checkout.0d1e2f3a4b5c.api.integration", "target": "api", "level": "integration", "rationale": "…" },
    { "id": "scn.checkout.0d1e2f3a4b5c.web.unit", "target": "web", "level": "unit", "rationale": "…" }
  ]
}
```

```text
  Target matrix             api  web
  checkout                  2/2  1/2
    Show the discount line  n/a  ✗ failed
```

The `api` requirement needs an `@implements` anchor under `api/`, the `web` requirement one under `web/`, and the contract one under each.

## Example 3: iOS and Android with platform-specific scenarios

One shared requirement, one shared scenario, and one platform-specific scenario per target. The platform difference is behavior, so it lives in the specification.

```markdown
<!-- stele: spec v1; targets: ios, android -->
## ADDED Requirements

### Requirement: Share a list
Verification-ID: req.share.2b3c4d5e6f7a

The app SHALL let the user share a list as plain text.

#### Scenario: Share text contains every item
Verification-ID: scn.share.3c4d5e6f7a8b

- **WHEN** the user shares a list with three items
- **THEN** the shared text lists the three items in order

#### Scenario: Share through the iOS share sheet
Verification-ID: scn.share.4d5e6f7a8b9c
Targets: ios

- **WHEN** the user taps Share
- **THEN** the system share sheet opens with the list text

#### Scenario: Share through an Android share intent
Verification-ID: scn.share.5e6f7a8b9c0d
Targets: android

- **WHEN** the user taps Share
- **THEN** an `ACTION_SEND` chooser opens with the list text as `EXTRA_TEXT`
```

The testing difference lives in the plan: iOS proves the shared scenario with a unit test, Android with an instrumented integration test.

```json
"scn.share.3c4d5e6f7a8b": {
  "evidence": [
    { "id": "scn.share.3c4d5e6f7a8b.ios.unit", "target": "ios", "level": "unit", "rationale": "…" },
    { "id": "scn.share.3c4d5e6f7a8b.android.integration", "target": "android", "level": "integration", "rationale": "…" }
  ]
}
```

```text
  Target matrix                            android    ios
  share                                    0/2        2/2
    Share text contains every item         ✗ missing  ✓ passed
    Share through an Android share intent  ✗ missing  n/a
```

## Example 4: a `system` journey across everything

A flow across the API and the web front end is proven once, by an end-to-end suite. Its target is `evidenceOnly`, because it implements nothing.

```json
"system": { "paths": ["e2e/**"], "evidenceOnly": true }
```

```markdown
<!-- stele: spec v1; targets: api, web, system -->
### Requirement: Complete a purchase with a promo code
Verification-ID: req.checkout.1e2f3a4b5c6d
Targets: system

A shopper SHALL be able to apply a promo code and pay the discounted total.

#### Scenario: Pay the discounted total
Verification-ID: scn.checkout.2f3a4b5c6d7e

- **WHEN** a shopper adds an item, applies a valid code, and pays
- **THEN** the order total is the discounted amount
```

The plan has one entry, `scn.checkout.2f3a4b5c6d7e.system.e2e`, anchored in `e2e/`. The requirement still needs one `@implements` anchor somewhere in the project, as every requirement does, but no implementation per target.

```text
  Target matrix  api  system  web
  checkout       0/0  1/1     0/0
```

## Check one target at a time

Every command that verifies or runs tests accepts `--target`:

```bash
npx stele test --target android          # run only the Android evidence
npx stele verify --change share --target ios
npx stele check --all --target web       # one CI job per target
```

`--target` covers the scenarios and requirements that apply to the selected targets, with only their plan entries, anchors, and tests. Specification, identity, and annotation findings are reported for the whole scope, and specifications without targets stay in scope. See [Targets in the CLI reference](/reference/cli#targets).

In CI, run one job per target next to one job without `--target`. The per-target jobs let each platform fail on its own gaps; the untargeted job keeps every specification, including shared tooling, checked as a whole:

```yaml
jobs:
  stele:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-node@v5
        with: { node-version: 24 }
      - run: npm ci
      - run: npx stele check --all
  android:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-node@v5
        with: { node-version: 24 }
      - run: npm ci
      - run: npx stele check --all --target android
```

## Keep targets through the OpenSpec workflow

OpenSpec copies requirement blocks, not first lines. Three rules keep targets intact:

1. **New delta specs inherit.** `stele ids` and `stele annotate --change` write the current specification's `targets` into a new delta spec's annotation. A delta spec that has lost or gained them is `SPEC_TARGETS_MISMATCH`.
2. **Changing a capability's targets is explicit.** A delta spec with a different list must list, as modified, renamed, or removed, every current requirement whose targets change (`SPEC_TARGETS_CHANGE_UNCOVERED`). Adding `web` to an `ios, android` capability therefore means modifying its requirements, narrowing those that stay mobile-only, and planning `web` evidence, in one reviewable change.
3. **Archiving restores.** Right after `openspec archive`, run `stele annotate --specs --targets-from openspec/changes/archive/<date>-<change>`. It copies each capability's `targets` from the archived delta spec onto its current specification. The `stele-archive` skill and the archive guidance that `stele init` adds to `openspec/config.yaml` do this for you.

## Adopt targets in an existing project

1. Add `targets` to `stele.config.json`. Nothing changes until a specification declares targets.
2. In a change, add `; targets: …` to the first line of the specifications that several targets implement, and narrow the requirements and scenarios that are platform-specific.
3. Plan per-target evidence with the `stele-plan` skill, approve it, and rename the anchors to the targeted IDs, such as `@verifies scn.share.3c4d5e6f7a8b.ios.unit`.

Targeted projects need the Stele release that ships targets. Stele 0.1.0-rc.5 and older warn about the `targets` field and read `@verifies scn.x.ios.unit` as a bare scenario anchor.
