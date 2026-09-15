import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const ROOT = path.resolve(import.meta.dirname, "..");

function cli(args) {
  return spawnSync(process.execPath, ["bin/stele.mjs", ...args], { cwd: ROOT, encoding: "utf8" });
}

// @verifies scn.verify.5e9a130cd7b4
test("exposes an executable verify command", () => {
  const result = cli(["verify", "--json"]);
  assert.equal(result.status, 0, result.stderr);
  const report = JSON.parse(result.stdout);
  assert.equal(report.schemaVersion, "2.0");
  assert.equal(report.verdict, "pass");
});

// @verifies scn.verify.a2c7e48b610f
test("returns stable failure exit codes", async (context) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-fail-"));
  context.after(() => fs.rm(root, { recursive: true }));
  const specRoot = path.join(root, "openspec/changes/todo-showcase/specs/example");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), "### Requirement: Missing identity\n#### Scenario: Missing too\n");
  const policyFailure = cli(["verify", "--root", root, "--json"]);
  const invocationFailure = cli(["unknown-command"]);
  assert.equal(policyFailure.status, 1);
  assert.equal(invocationFailure.status, 2);
});

// @verifies scn.verify.d6f8012b3ea5
test("emits identical default JSON for identical inputs", () => {
  const first = cli(["verify", "--json"]);
  const second = cli(["verify", "--json"]);
  assert.equal(first.status, 0, first.stderr);
  assert.equal(second.status, 0, second.stderr);
  assert.equal(first.stdout, second.stdout);
});

// @verifies scn.verify.3a70c9d1ef24
test("omits volatile metadata from the deterministic payload", () => {
  const result = cli(["verify", "--json"]);
  const report = JSON.parse(result.stdout);
  assert.equal("generatedAt" in report, false);
  assert.equal("recordedAt" in report.stages.execution, false);
});
