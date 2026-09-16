## Purpose

Define how Stele finds annotations in TypeScript sources and which test declarations a TypeScript test anchor can resolve to.

## ADDED Requirements

### Requirement: Resolve TypeScript tests called as expressions
Verification-ID: req.tsanchors.3529ec7b6931

A test anchor in a TypeScript test file SHALL resolve to the title of the next named `test` or `it` call, whether the call stands alone or is prefixed with `void` or `await`.

#### Scenario: Bind a test anchor to a void or awaited test call
Verification-ID: scn.tsanchors.f4c1eb23c2ab

- **WHEN** `@verifies` comments directly precede `void test("adds a todo", ...)`, `await it("removes a todo", ...)`, and `test("lists todos", ...)`
- **THEN** the anchors resolve to `adds a todo`, `removes a todo`, and `lists todos`

#### Scenario: Leave other expressions unresolved
Verification-ID: scn.tsanchors.f5c807c92cad

- **WHEN** a `@verifies` comment directly precedes `void run("adds a todo")`
- **THEN** the anchor has no selector

### Requirement: Read TypeScript annotations only from comments
Verification-ID: req.tsanchors.c02ad141ee6a

The verifier SHALL record an `@implements` or `@verifies` annotation in a TypeScript file only when it appears inside a line comment, block comment, or JSDoc comment, and SHALL ignore annotation text inside string and template literals.

#### Scenario: Ignore annotations inside string and template literals
Verification-ID: scn.tsanchors.e3be49f14532

- **WHEN** a TypeScript file contains annotation text only inside single-quoted, double-quoted, and template literals, including a template literal that spans lines
- **THEN** the verifier records no anchor for that text

#### Scenario: Keep annotations in every comment form
Verification-ID: scn.tsanchors.e6114f49ffdc

- **WHEN** annotations appear in a `//` comment, a `/* */` comment, and a JSDoc line inside `/** */`
- **THEN** the verifier records an anchor for each of them
