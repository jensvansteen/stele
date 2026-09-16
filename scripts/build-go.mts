import { spawnSync, type SpawnSyncReturns } from "node:child_process";
import { copyFileSync, mkdirSync, rmSync } from "node:fs";
import path from "node:path";

const root: string = path.resolve(import.meta.dirname, "..");
const dist: string = path.join(root, "dist");
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
    ["build", "-trimpath", "-ldflags", "-s -w", "-o", output, "./cmd/stele"],
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
  if (platform === process.platform && architecture === process.arch) {
    copyFileSync(output, path.join(dist, "stele"));
  }
}
