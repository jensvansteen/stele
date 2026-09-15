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
npm install --save-dev ./vendor/stele-spec-0.3.0.tgz
npx stele init --change todo-basics
npx stele verify --stage proposal --json
```

This is preferable to a filesystem link for the package-boundary test. It reveals missing packaged files, incorrect relative paths, and executable assumptions that a symlink can hide.

## Refresh after a change

Create a new archive and reinstall that file in the consumer. The archive filename contains the package version, so remove an older local dependency entry when changing versions.

## What the package carries

- the compiled Go executable exposed directly as the npm `stele` binary;
- the pinned OpenSpec dependency;
- the repository-local skill templates embedded in the Go binary;
- the documentation source.

The consumer owns its OpenSpec Markdown, configuration, linkage plan, anchors, tests, and evidence outputs.

The current local archive contains a binary for the machine that creates it. Cross-platform registry distribution will require platform-specific npm artifacts or release downloads.
