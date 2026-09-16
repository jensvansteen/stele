# Examples

The private `stele-examples` repository is the package consumer. It is intentionally separate from Stele so examples cannot depend on unshipped source files.

## Todo example

The first example is a plain TypeScript Todo application. Its purpose is to demonstrate the complete verification loop:

Follow the [Todo example walkthrough](/guide/inspect-example) to run it and trace one behavior from OpenSpec prose to generated evidence.

1. OpenSpec describes create, complete, filter, and delete behavior.
2. Each requirement and scenario carries a stable ID.
3. The linkage plan names expected implementation and test declarations.
4. Source and tests carry matching anchors.
5. The installed `stele-spec` npm package runs validation from the example repository.
6. Generated evidence is available as JSON files for review and CI.

## Run a local example

Install the release candidate from npm in the example repository, then run its documented commands.

```bash
# stele-examples repository
npm ci
npm run todo:check
npm run todo:dev
```

The registry installation is part of the demonstration: the example must work without importing anything from the Stele checkout.

## Add another example

For v0.1, each example should remain a TypeScript consumer and prove a new boundary, such as an API-backed service, a different TypeScript test framework, or a browser end-to-end runner. Keep application code and generated artifacts in `stele-examples`; add reusable behavior to Stele only when multiple consumers need it. Add Go or another language only after Stele defines and ships the corresponding adapters.
