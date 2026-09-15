#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const temporaryDirectory = mkdtempSync(path.join(tmpdir(), "stele-coverage-"));
const profile = path.join(temporaryDirectory, "core.coverage");

function run(command, commandArguments) {
  const result = spawnSync(command, commandArguments, { encoding: "utf8", stdio: ["ignore", "pipe", "inherit"] });
  if (result.status !== 0) {
    const error = new Error(`${command} exited with status ${result.status ?? 1}`);
    error.exitCode = result.status ?? 1;
    throw error;
  }
  return result.stdout;
}

try {
  process.stdout.write(run("go", ["test", "./internal/stele", `-coverprofile=${profile}`]));
  const blocks = readFileSync(profile, "utf8").trim().split("\n").slice(1);
  let covered = 0;
  let total = 0;
  for (const block of blocks) {
    const fields = block.trim().split(/\s+/);
    const statements = Number(fields.at(-2));
    const executions = Number(fields.at(-1));
    total += statements;
    if (executions > 0) covered += statements;
  }
  if (total === 0 || covered !== total) {
    const percentage = total === 0 ? 0 : (covered / total) * 100;
    console.error(`Go verifier core statement coverage is ${percentage.toFixed(2)}% (${covered}/${total}); required: 100%.`);
    process.exitCode = 1;
  } else {
    console.log(`Go verifier core statement coverage: 100.0% (${covered}/${total}).`);
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = error.exitCode ?? 1;
} finally {
  rmSync(temporaryDirectory, { recursive: true, force: true });
}
