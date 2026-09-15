import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const ROOT = path.resolve(import.meta.dirname, "..");

function cli(args) {
  return spawnSync("dist/stele", args, { cwd: ROOT, encoding: "utf8" });
}

async function passingFixture(context) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-pass-"));
  context.after(() => fs.rm(root, { recursive: true }));
  const specRoot = path.join(root, "openspec/changes/example/specs/todo");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.mkdir(path.join(root, "src"), { recursive: true });
  await fs.mkdir(path.join(root, "tests"), { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), [
    "### Requirement: Add a todo",
    "Verification-ID: req.todo.0123456789ab",
    "#### Scenario: Save entered text",
    "Verification-ID: scn.todo.abcdef012345",
    "- **WHEN** a user enters a todo",
    "- **THEN** the todo is saved",
    "",
  ].join("\n"));
  await fs.writeFile(path.join(root, "src/todo.mjs"), "// @implements req.todo.0123456789ab\nexport function addTodo(text) { return { text }; }\n");
  await fs.writeFile(path.join(root, "tests/todo.test.mjs"), [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../src/todo.mjs";',
    "// @verifies scn.todo.abcdef012345",
    'test("saves entered text", () => assert.equal(addTodo("ship").text, "ship"));',
    "",
  ].join("\n"));
  return root;
}

// @verifies scn.verify.5e9a130cd7b4
test("exposes an executable verify command", async (context) => {
  const root = await passingFixture(context);
  const result = cli(["verify", "--root", root, "--change", "example", "--json"]);
  assert.equal(result.status, 0, result.stderr);
  const report = JSON.parse(result.stdout);
  assert.equal(report.schemaVersion, "2.0");
  assert.equal(report.verdict, "pass");
});

// @verifies scn.verify.a2c7e48b610f
test("returns stable failure exit codes", async (context) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-fail-"));
  context.after(() => fs.rm(root, { recursive: true }));
  const specRoot = path.join(root, "openspec/changes/example/specs/example");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), "### Requirement: Missing identity\n#### Scenario: Missing too\n");
  const policyFailure = cli(["verify", "--root", root, "--change", "example", "--json"]);
  const invocationFailure = cli(["unknown-command"]);
  assert.equal(policyFailure.status, 1);
  assert.equal(invocationFailure.status, 2);
});

// @verifies scn.verify.d6f8012b3ea5
test("emits identical default JSON for identical inputs", async (context) => {
  const root = await passingFixture(context);
  const args = ["verify", "--root", root, "--change", "example", "--json"];
  const first = cli(args);
  const second = cli(args);
  assert.equal(first.status, 0, first.stderr);
  assert.equal(second.status, 0, second.stderr);
  assert.equal(first.stdout, second.stdout);
});

// @verifies scn.verify.3a70c9d1ef24
test("omits volatile metadata from the deterministic payload", async (context) => {
  const root = await passingFixture(context);
  const result = cli(["verify", "--root", root, "--change", "example", "--json"]);
  const report = JSON.parse(result.stdout);
  assert.equal("generatedAt" in report, false);
  assert.equal("recordedAt" in report.stages.execution, false);
});
