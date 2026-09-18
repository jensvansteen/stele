import { spawnSync, type SpawnSyncReturns } from "node:child_process";
import { copyFileSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import path from "node:path";

const root: string = path.resolve(import.meta.dirname, "..");
const dist: string = path.join(root, "dist");
function readPackageVersion(): string {
  const manifest: unknown = JSON.parse(readFileSync(path.join(root, "package.json"), "utf8"));
  if (typeof manifest !== "object" || manifest === null || !("version" in manifest)) {
    throw new Error("package.json has no version");
  }
  const { version }: { version: unknown } = manifest;
  if (typeof version !== "string" || version === "") {
    throw new Error("package.json has no version");
  }
  return version;
}

const packageVersion: string = readPackageVersion();
const linkerFlags = `-s -w -X github.com/jensvansteen/stele/internal/stele.Version=${packageVersion}`;
const hostArchitecture: string = process.arch === "x64" ? "amd64" : process.arch;
const targets: readonly (readonly [string, string])[] = [
  ["darwin", "arm64"],
  ["darwin", "amd64"],
  ["linux", "arm64"],
  ["linux", "amd64"],
];

rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });

for (const [platform, architecture] of targets) {
  const output: string = path.join(dist, `stele-${platform}-${architecture}`);
  const result: SpawnSyncReturns<Buffer> = spawnSync(
    "go",
    ["build", "-trimpath", "-ldflags", linkerFlags, "-o", output, "./cmd/stele"],
    {
      cwd: root,
      stdio: "inherit",
      env: { ...process.env, GOOS: platform, GOARCH: architecture, CGO_ENABLED: "0" },
    },
  );
  if (result.status !== 0) {
    process.exitCode = result.status ?? 2;
    break;
  }
  if (platform === process.platform && architecture === hostArchitecture) {
    copyFileSync(output, path.join(dist, "stele"));
  }
}
