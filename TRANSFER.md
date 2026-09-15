# Project split

The product and showcase are intentionally separate:

- [`jensvansteen/stele`](https://github.com/jensvansteen/stele) contains the reusable verifier, CLI, package, skills, tests, and documentation.
- [`jensvansteen/stele-examples`](https://github.com/jensvansteen/stele-examples) contains independent consuming applications, beginning with the Todo example and artifact dashboard.
- `stele-legacy` preserves the earlier methodology repository for historical reference.

The examples repository installs Stele from an npm tarball and does not import source files from the product checkout.
