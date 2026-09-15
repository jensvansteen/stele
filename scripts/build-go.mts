import { spawnSync, type SpawnSyncReturns } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import path from "node:path";

const root: string = path.resolve(import.meta.dirname, "..");
const output: string = path.join(root, "dist", "stele");
rmSync(path.dirname(output), { recursive: true, force: true });
mkdirSync(path.dirname(output), { recursive: true });
const result: SpawnSyncReturns<Buffer> = spawnSync(
  "go",
  ["build", "-trimpath", "-ldflags", "-s -w", "-o", output, "./cmd/stele"],
  { cwd: root, stdio: "inherit" },
);
process.exitCode = result.status ?? 2;
