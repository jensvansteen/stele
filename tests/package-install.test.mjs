import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const ROOT = path.resolve(import.meta.dirname, "..");

test("packed package initializes and validates a separate consumer project", async (context) => {
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), "stele-package-"));
  context.after(() => fs.rm(temp, { recursive: true }));
  const env = { ...process.env, NPM_CONFIG_CACHE: path.join(temp, "npm-cache") };
  const pack = spawnSync("npm", ["pack", "--json", "--pack-destination", temp], { cwd: ROOT, encoding: "utf8", env });
  assert.equal(pack.status, 0, pack.stderr);
  const [{ filename }] = JSON.parse(pack.stdout);
  const consumer = path.join(temp, "consumer");
  await fs.mkdir(consumer);
  await fs.writeFile(path.join(consumer, "package.json"), '{"name":"consumer","private":true,"type":"module"}\n');
  const install = spawnSync("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund", path.join(temp, filename)], { cwd: consumer, encoding: "utf8", env });
  assert.equal(install.status, 0, install.stderr);
  const init = spawnSync(path.join(consumer, "node_modules/.bin/stele"), ["init", "--change", "example"], { cwd: consumer, encoding: "utf8" });
  assert.equal(init.status, 0, init.stderr);
  const config = JSON.parse(await fs.readFile(path.join(consumer, "stele.config.json"), "utf8"));
  assert.equal(config.change, "example");
  assert.equal(await fs.stat(path.join(consumer, ".agents/skills/stele-plan/SKILL.md")).then(() => true), true);
  assert.equal(await fs.stat(path.join(consumer, ".agents/skills/stele-verify/SKILL.md")).then(() => true), true);

  const change = path.join(consumer, "openspec/changes/example");
  await fs.mkdir(path.join(change, "specs/example"), { recursive: true });
  await fs.mkdir(path.join(consumer, "src"), { recursive: true });
  await fs.mkdir(path.join(consumer, "tests"), { recursive: true });
  await fs.writeFile(path.join(change, ".openspec.yaml"), "schema: spec-driven\n");
  await fs.writeFile(path.join(change, "proposal.md"), "# Proposal: Package boundary\n\n## Why\n\nProve the installed package.\n\n## What Changes\n\n- Add a deterministic example.\n\n## Capabilities\n\n- `example`: Return a value.\n\n## Impact\n\nNo production impact.\n");
  await fs.writeFile(path.join(change, "design.md"), "# Design\n\nUse one pure function and one exact test.\n");
  await fs.writeFile(path.join(change, "tasks.md"), "# Tasks\n\n- [x] Implement the example.\n");
  await fs.writeFile(path.join(change, "specs/example/spec.md"), [
    "## ADDED Requirements",
    "",
    "### Requirement: Return a value",
    "Verification-ID: req.example.0123456789ab",
    "",
    "The system SHALL return the supplied value.",
    "",
    "#### Scenario: Value supplied",
    "Verification-ID: scn.example.abcdef012345",
    "",
    "- **WHEN** a value is supplied",
    "- **THEN** the same value is returned",
    "",
  ].join("\n"));
  await fs.writeFile(path.join(consumer, "src/example.mjs"), "// @implements req.example.0123456789ab\nexport function value(input) { return input; }\n");
  await fs.writeFile(path.join(consumer, "tests/example.test.mjs"), [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { value } from "../src/example.mjs";',
    "// @verifies scn.example.abcdef012345",
    'test("returns the supplied value", () => assert.equal(value("proof"), "proof"));',
    "",
  ].join("\n"));
  const validateEnv = { ...process.env };
  delete validateEnv.NODE_TEST_CONTEXT;
  const validate = spawnSync(path.join(consumer, "node_modules/.bin/stele"), ["validate", "--change", "example", "--json"], { cwd: consumer, encoding: "utf8", env: validateEnv });
  const evidence = await fs.readFile(path.join(consumer, "artifacts/test-results.json"), "utf8").catch(() => "no evidence file");
  assert.equal(validate.status, 0, `${validate.stderr}${validate.stdout}\n${evidence}`);
  assert.equal(JSON.parse(validate.stdout).verdict, "pass");
});
