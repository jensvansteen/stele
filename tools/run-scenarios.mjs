import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import path from "node:path";
import { computeInputDigest, parseSpecs, scanAnchors } from "./verify.mjs";

const ROOT = process.cwd();

function gitRevision(root) {
  const result = spawnSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" });
  return result.status === 0 ? result.stdout.trim() : "uncommitted";
}

function escapeRegex(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function deterministicJson(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

function testEnvironment() {
  const environment = { ...process.env, STELE_CHILD_TEST: "1" };
  delete environment.NODE_TEST_CONTEXT;
  return environment;
}

function exactTestPassed(output, selector) {
  const escaped = escapeRegex(selector);
  return new RegExp(`(?:^|\\n)ok \\d+ - ${escaped}(?:\\n|$)`).test(output);
}

export async function discoverScenarioTests({ root = ROOT, changeId = "todo-showcase" } = {}) {
  const parsed = await parseSpecs({ root, changeId });
  const scenarioIds = new Set(parsed.requirements.flatMap((requirement) => requirement.scenarios.map((scenario) => scenario.id)).filter(Boolean));
  const anchors = (await scanAnchors({ root })).filter((anchor) => anchor.kind === "test" && scenarioIds.has(anchor.id));
  return anchors.sort((a, b) => `${a.path}:${a.selector}:${a.id}`.localeCompare(`${b.path}:${b.selector}:${b.id}`));
}

// @implements req.verify.6fc019e4a2b8
export async function runScenarioTests({ root = ROOT, changeId = "todo-showcase", evidencePath } = {}) {
  const inputDigest = await computeInputDigest(root);
  const anchors = await discoverScenarioTests({ root, changeId });
  const groups = new Map();
  for (const anchor of anchors) {
    const key = `${anchor.path}\0${anchor.selector ?? ""}`;
    if (!groups.has(key)) groups.set(key, { path: anchor.path, selector: anchor.selector, ids: [] });
    groups.get(key).ids.push(anchor.id);
  }

  const outcomeById = new Map();
  const executions = [];
  for (const group of [...groups.values()].sort((a, b) => `${a.path}:${a.selector}`.localeCompare(`${b.path}:${b.selector}`))) {
    let outcome = "failed";
    let reason = "target-not-resolved";
    if (group.selector) {
      const result = spawnSync(process.execPath, [
        "--test",
        "--test-reporter=tap",
        `--test-name-pattern=^${escapeRegex(group.selector)}$`,
        group.path
      ], { cwd: root, encoding: "utf8", env: testEnvironment() });
      const selectedTestPassed = result.status === 0 && exactTestPassed(result.stdout, group.selector);
      outcome = selectedTestPassed ? "passed" : "failed";
      reason = selectedTestPassed ? null : result.status === 0 ? "test-not-executed" : "test-process-failed";
    }
    for (const id of group.ids.sort()) outcomeById.set(id, outcome);
    executions.push({ path: group.path, selector: group.selector, scenarioIds: [...group.ids].sort(), outcome, reason });
  }

  const parsed = await parseSpecs({ root, changeId });
  const scenarios = parsed.requirements
    .flatMap((requirement) => requirement.scenarios)
    .map((scenario) => ({ id: scenario.id, outcome: outcomeById.get(scenario.id) ?? "not-run" }))
    .sort((a, b) => a.id.localeCompare(b.id));
  const outcome = scenarios.length > 0 && scenarios.every((scenario) => scenario.outcome === "passed") ? "passed" : "failed";
  const evidence = {
    schemaVersion: 2,
    runner: "node:test/exact-scenario",
    testedRevision: gitRevision(root),
    inputDigest,
    outcome,
    scenarios,
    executions
  };

  if (evidencePath) {
    const absolute = path.resolve(root, evidencePath);
    await fs.mkdir(path.dirname(absolute), { recursive: true });
    await fs.writeFile(absolute, deterministicJson(evidence));
  }
  return evidence;
}
