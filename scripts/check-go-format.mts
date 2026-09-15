import { spawnSync } from "node:child_process";

const result = spawnSync("gofmt", ["-l", "cmd", "internal"], { encoding: "utf8" });
if (result.error) {
  console.error(result.error.message);
  process.exitCode = 2;
} else if (result.stdout.trim()) {
  console.error(`Go files need formatting:\n${result.stdout.trim()}`);
  process.exitCode = 1;
} else {
  process.exitCode = result.status ?? 0;
}
