# Use a local package

You can test Stele in another repository without publishing to npm. `npm pack` creates the same package archive that a registry release would contain.

## Pack Stele

From the Stele checkout:

```bash
npm ci
npm pack --pack-destination /path/to/consumer/vendor
```

The `prepack` script compiles the Go binaries before npm creates the archive.

## Install the archive

From the consuming repository:

```bash
npm install --save-dev ./vendor/stele-spec-0.1.0-rc.2.tgz
npx openspec init .
npx openspec new change todo-basics
npx stele init --change todo-basics
npx stele verify --stage proposal --json
```

The native OpenSpec steps create the specification workspace and feature change. `stele init` then configures Stele for that existing change. Continue with the [complete project setup](/guide/getting-started#build-it-yourself-complete-project-setup).

This is preferable to a filesystem link for the package-boundary test. It reveals missing packaged files, incorrect relative paths, and executable assumptions that a symlink can hide.

## Refresh after a change

Create a new archive and reinstall that file in the consumer. The archive filename contains the package version, so remove an older local dependency entry when changing versions.

## What the package carries

- the compiled Go executable exposed directly as the npm `stele` binary;
- the pinned OpenSpec dependency;
- the repository-local skill templates embedded in the Go binary;
- the documentation source.

The consumer owns its OpenSpec Markdown, configuration, linkage plan, anchors, tests, and evidence outputs. Consumer source and test anchors can be TypeScript or Go; other languages require future adapters.

The archive includes macOS and Linux binaries for ARM64 and x64. Installation selects the matching binary; unsupported platforms fail with a clear error.
