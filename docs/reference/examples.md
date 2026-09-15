# Examples

The private `stele-examples` repository is the package consumer. It is intentionally separate from Stele so examples cannot depend on unshipped source files.

## Todo example

The first example is a plain Todo application. Its purpose is to demonstrate the complete verification loop:

1. OpenSpec describes create, complete, filter, and delete behavior.
2. Each requirement and scenario carries a stable ID.
3. The linkage plan names expected implementation and test declarations.
4. Source and tests carry matching anchors.
5. The installed `stele-spec` tarball runs validation from the example repository.
6. Generated evidence can be rendered by the report UI.

## Run a local example

Pack Stele first, install the resulting archive in the example repository, then use that repository’s documented development command.

```bash
# Stele repository
npm pack --pack-destination ../stele-examples/vendor

# stele-examples repository
npm install --save-dev ./vendor/stele-spec-0.3.0.tgz
npx stele validate --json
```

The tarball installation is part of the demonstration: the example must work without importing anything from the Stele checkout.

## Add another example

Each example should prove a new boundary, such as a Go consumer, an API-backed service, or a browser end-to-end runner. Keep application code and generated artifacts in `stele-examples`; add reusable behavior to Stele only when multiple consumers need it.
