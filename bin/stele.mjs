#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { runScenarioTests } from "../tools/run-scenarios.mjs";
import { runVerification, serializeReport } from "../tools/verify.mjs";

const PACKAGE_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const DEFAULT_ROOT = process.cwd();
const HELP = `stele 0.2.0 — deterministic OpenSpec implementation verification

Usage:
  stele init [--change ID] [--root PATH]
  stele verify [--stage proposal|implementation] [--change ID] [--root PATH] [--report PATH] [--json]
  stele test [--change ID] [--root PATH] [--evidence PATH] [--json]
  stele validate [--change ID] [--root PATH] [--report PATH] [--evidence PATH] [--json]

Exit codes:
  0  selected checks passed
  1  deterministic policy or test failure
  2  invalid invocation or tool failure
`;

function parseOptions(args) {
  const options = { root: DEFAULT_ROOT, changeId: null, stage: "implementation", json: false };
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--json") options.json = true;
    else if (["--root", "--change", "--stage", "--report", "--evidence"].includes(argument)) {
      const value = args[index + 1];
      if (!value || value.startsWith("--")) throw new Error(`Missing value for ${argument}.`);
      index += 1;
      if (argument === "--root") options.root = path.resolve(value);
      if (argument === "--change") options.changeId = value;
      if (argument === "--stage") options.stage = value;
      if (argument === "--report") options.reportPath = value;
      if (argument === "--evidence") options.evidencePath = value;
    } else throw new Error(`Unknown option: ${argument}`);
  }
  if (!["proposal", "implementation"].includes(options.stage)) throw new Error(`Unknown stage: ${options.stage}`);
  return options;
}

async function configuredOptions(options) {
  const configPath = path.join(options.root, "stele.config.json");
  const config = await fs.readFile(configPath, "utf8").then(JSON.parse).catch(() => ({}));
  return { ...options, changeId: options.changeId ?? config.change ?? "todo-showcase" };
}

async function copyTemplate(root, relativePath, destination, created) {
  const source = path.join(PACKAGE_ROOT, "templates", relativePath);
  const existing = await fs.readFile(destination, "utf8").catch(() => null);
  const content = await fs.readFile(source, "utf8");
  if (existing !== null) return;
  await fs.mkdir(path.dirname(destination), { recursive: true });
  await fs.writeFile(destination, content);
  created.push(path.relative(root, destination));
}

async function initCommand(options) {
  const created = [];
  const configPath = path.join(options.root, "stele.config.json");
  const existingConfig = await fs.readFile(configPath, "utf8").catch(() => null);
  if (existingConfig === null) {
    await fs.writeFile(configPath, `${JSON.stringify({ schemaVersion: 1, adapter: "openspec", change: options.changeId ?? "todo-showcase" }, null, 2)}\n`);
    created.push(path.relative(options.root, configPath));
  }
  await copyTemplate(options.root, "skills/stele-plan/SKILL.md", path.join(options.root, ".agents/skills/stele-plan/SKILL.md"), created);
  await copyTemplate(options.root, "skills/stele-verify/SKILL.md", path.join(options.root, ".agents/skills/stele-verify/SKILL.md"), created);
  await fs.mkdir(path.join(options.root, "artifacts"), { recursive: true });
  console.log(created.length ? `Initialized Stele: ${created.join(", ")}` : "Stele is already initialized.");
  return 0;
}

function openSpecCliPath() {
  const require = createRequire(import.meta.url);
  const entry = require.resolve("@fission-ai/openspec");
  return path.resolve(path.dirname(entry), "..", "bin", "openspec.js");
}

function printVerification(report, json) {
  if (json) process.stdout.write(serializeReport(report));
  else {
    const mark = report.verdict === "pass" ? "✓" : "✗";
    console.log(`${mark} ${report.mode} verification ${report.verdict}: ${report.summary.requirements} requirements, ${report.summary.scenarios} scenarios, ${report.summary.errors} errors`);
    for (const diagnostic of report.diagnostics) console.log(`  ${diagnostic.severity.toUpperCase()} ${diagnostic.code}: ${diagnostic.message}`);
  }
}

async function verifyCommand(options) {
  const report = await runVerification({ root: options.root, changeId: options.changeId, mode: options.stage, reportPath: options.reportPath });
  printVerification(report, options.json);
  return report.verdict === "pass" ? 0 : 1;
}

async function testCommand(options) {
  const evidence = await runScenarioTests({ root: options.root, changeId: options.changeId, evidencePath: options.evidencePath });
  if (options.json) process.stdout.write(`${JSON.stringify(evidence, null, 2)}\n`);
  else {
    const passed = evidence.scenarios.filter((scenario) => scenario.outcome === "passed").length;
    console.log(`${evidence.outcome === "passed" ? "✓" : "✗"} scenario execution ${evidence.outcome}: ${passed}/${evidence.scenarios.length} passed`);
    for (const scenario of evidence.scenarios.filter((item) => item.outcome !== "passed")) console.log(`  ${scenario.id}: ${scenario.outcome}`);
  }
  return evidence.outcome === "passed" ? 0 : 1;
}

// @implements req.verify.6b2d7904ea51
async function validateCommand(options) {
  const evidencePath = options.evidencePath ?? "artifacts/test-results.json";
  const reportPath = options.reportPath ?? "artifacts/verification-report.json";
  const evidence = await runScenarioTests({ root: options.root, changeId: options.changeId, evidencePath });
  const openspec = spawnSync(process.execPath, [openSpecCliPath(), "validate", options.changeId, "--strict", "--no-interactive"], { cwd: options.root, encoding: "utf8", env: { ...process.env, OPENSPEC_TELEMETRY: "0" } });
  const report = await runVerification({ root: options.root, changeId: options.changeId, mode: "implementation", reportPath });
  const passed = evidence.outcome === "passed" && openspec.status === 0 && report.verdict === "pass";
  if (options.json) process.stdout.write(`${JSON.stringify({ schemaVersion: 1, verdict: passed ? "pass" : "fail", openspec: openspec.status === 0 ? "pass" : "fail", execution: evidence.outcome, verification: report.verdict }, null, 2)}\n`);
  else {
    console.log(`${openspec.status === 0 ? "✓" : "✗"} OpenSpec strict validation ${openspec.status === 0 ? "passed" : "failed"}`);
    console.log(`${evidence.outcome === "passed" ? "✓" : "✗"} scenario execution ${evidence.outcome}: ${evidence.scenarios.filter((scenario) => scenario.outcome === "passed").length}/${evidence.scenarios.length} passed`);
    printVerification(report, false);
    console.log(`${passed ? "✓" : "✗"} deterministic validation ${passed ? "passed" : "failed"}`);
  }
  return passed ? 0 : 1;
}

// @implements req.verify.4c82d1a90fe7
export async function main(argv = process.argv.slice(2)) {
  const [command = "help", ...args] = argv;
  if (["help", "--help", "-h"].includes(command)) {
    process.stdout.write(HELP);
    return 0;
  }
  if (["version", "--version", "-v"].includes(command)) {
    console.log("0.2.0");
    return 0;
  }
  const parsedOptions = parseOptions(args);
  if (command === "init") return initCommand(parsedOptions);
  const options = await configuredOptions(parsedOptions);
  if (command === "verify") return verifyCommand(options);
  if (command === "test") return testCommand(options);
  if (command === "validate") return validateCommand(options);
  throw new Error(`Unknown command: ${command}`);
}

const invokedPath = process.argv[1]
  ? await fs.realpath(process.argv[1]).catch(() => path.resolve(process.argv[1]))
  : "";

if (fileURLToPath(import.meta.url) === invokedPath) {
  main().then((exitCode) => { process.exitCode = exitCode; }).catch((error) => {
    console.error(`stele: ${error.message}`);
    process.exitCode = 2;
  });
}
