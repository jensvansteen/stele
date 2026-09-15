import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { runScenarioTests } from "../tools/run-scenarios.mjs";

async function fixture(context, { failing = false } = {}) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "stele-scenario-"));
  context.after(() => fs.rm(root, { recursive: true }));
  const specRoot = path.join(root, "openspec/changes/todo-showcase/specs/example");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.mkdir(path.join(root, "tests"), { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), `### Requirement: Example\nVerification-ID: req.demo.bbbbbbbbbbbb\n#### Scenario: Probe\nVerification-ID: scn.demo.cccccccccccc\n`);
  const anchor = ["@verifies", "scn.demo.cccccccccccc"].join(" ");
  await fs.writeFile(path.join(root, "tests/probe.test.mjs"), `import assert from "node:assert/strict";\nimport test from "node:test";\n// ${anchor}\ntest("probe", () => assert.equal(1, ${failing ? 2 : 1}));\n`);
  return root;
}

// @verifies scn.verify.8d2e41b70ca3
test("maps a passing test to only its anchored scenario", async (context) => {
  const root = await fixture(context);
  const evidence = await runScenarioTests({ root });
  assert.equal(evidence.outcome, "passed");
  assert.deepEqual(evidence.scenarios, [{ id: "scn.demo.cccccccccccc", outcome: "passed" }]);
  assert.deepEqual(evidence.executions[0].scenarioIds, ["scn.demo.cccccccccccc"]);
});

// @verifies scn.verify.1c7ab038e529
test("maps a failing test and returns a failed outcome", async (context) => {
  const root = await fixture(context, { failing: true });
  const evidence = await runScenarioTests({ root });
  assert.equal(evidence.outcome, "failed", JSON.stringify(evidence));
  assert.deepEqual(evidence.scenarios, [{ id: "scn.demo.cccccccccccc", outcome: "failed" }]);
});
