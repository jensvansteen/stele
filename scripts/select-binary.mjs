import { chmodSync, copyFileSync, existsSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const dist = path.join(root, "dist");
const architecture = process.arch === "x64" ? "amd64" : process.arch;
const source = path.join(dist, `stele-${process.platform}-${architecture}`);

// Source checkouts are installed before their first build. Packed packages must
// contain the selected binary, and installs fail visibly if they do not.
if (!existsSync(source) && existsSync(path.join(root, "go.mod"))) {
  process.exit(0);
}
if (!existsSync(source)) {
  throw new Error(`Stele does not include a binary for ${process.platform}/${process.arch}`);
}

const executable = path.join(dist, "stele");
copyFileSync(source, executable);
chmodSync(executable, 0o755);
