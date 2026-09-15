#!/usr/bin/env node
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { promises as fs } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const ROOT = process.cwd();
const ID_PATTERN = /^(req|scn)\.[a-z0-9]+\.[a-f0-9]{12}$/;
const ANCHOR_PATTERN = /@(implements|verifies)\s+((?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12})/g;

async function walk(directory, predicate = () => true) {
  const entries = await fs.readdir(directory, { withFileTypes: true }).catch(() => []);
  const files = [];
  for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...await walk(absolute, predicate));
    else if (entry.isFile() && predicate(absolute)) files.push(absolute);
  }
  return files;
}

// @implements req.verify.a18c03ef72b6
export async function parseSpecs({ root = ROOT, changeId = "todo-showcase" } = {}) {
  const specsRoot = path.join(root, "openspec", "changes", changeId, "specs");
  const files = await walk(specsRoot, (file) => file.endsWith(".md"));
  const requirements = [];
  const diagnostics = [];

  for (const file of files) {
    const relativePath = path.relative(root, file);
    const lines = (await fs.readFile(file, "utf8")).split(/\r?\n/);
    let requirement = null;
    let scenario = null;
    let target = null;

    lines.forEach((line, index) => {
      const requirementMatch = line.match(/^### Requirement:\s*(.+)$/);
      const scenarioMatch = line.match(/^#### Scenario:\s*(.+)$/);
      const idMatch = line.match(/^Verification-ID:\s*(\S+)\s*$/);

      if (requirementMatch) {
        requirement = {
          id: null,
          title: requirementMatch[1].trim(),
          source: { path: relativePath, line: index + 1 },
          scenarios: []
        };
        requirements.push(requirement);
        scenario = null;
        target = "requirement";
      } else if (scenarioMatch && requirement) {
        scenario = {
          id: null,
          title: scenarioMatch[1].trim(),
          source: { path: relativePath, line: index + 1 }
        };
        requirement.scenarios.push(scenario);
        target = "scenario";
      } else if (idMatch && target) {
        const entity = target === "scenario" ? scenario : requirement;
        if (entity.id) {
          diagnostics.push(diag("ID_MULTIPLE", "error", `Multiple IDs declared for ${entity.title}.`, relativePath, index + 1));
        } else {
          entity.id = idMatch[1];
          if (!ID_PATTERN.test(entity.id) || (target === "scenario") !== entity.id.startsWith("scn.")) {
            diagnostics.push(diag("ID_FORMAT", "error", `Invalid ${target} ID: ${entity.id}.`, relativePath, index + 1));
          }
        }
      }
    });
  }

  for (const requirement of requirements) {
    if (!requirement.id) diagnostics.push(diag("ID_REQUIREMENT_MISSING", "error", `Requirement “${requirement.title}” has no Verification-ID.`, requirement.source.path, requirement.source.line));
    if (requirement.scenarios.length === 0) diagnostics.push(diag("SCENARIO_MISSING", "error", `Requirement ${requirement.id ?? requirement.title} has no scenarios.`, requirement.source.path, requirement.source.line));
    for (const item of requirement.scenarios) {
      if (!item.id) diagnostics.push(diag("ID_SCENARIO_MISSING", "error", `Scenario “${item.title}” has no Verification-ID.`, item.source.path, item.source.line));
    }
  }

  const identities = requirements.flatMap((requirement) => [requirement, ...requirement.scenarios]);
  const byId = new Map();
  for (const identity of identities.filter((item) => item.id)) {
    if (byId.has(identity.id)) diagnostics.push(diag("ID_DUPLICATE", "error", `Identity ${identity.id} is declared more than once.`, identity.source.path, identity.source.line, identity.id));
    byId.set(identity.id, identity);
  }

  return { requirements, diagnostics, files: files.map((file) => path.relative(root, file)) };
}

// @implements req.verify.e25a1c7b490d
export async function scanAnchors({ root = ROOT } = {}) {
  const codeRoots = ["bin", "src", "public", "tools", "scripts"];
  const codeFiles = [path.join(root, "server.mjs")];
  for (const directory of codeRoots) codeFiles.push(...await walk(path.join(root, directory), (file) => /\.(?:mjs|js)$/.test(file)));
  const testFiles = await walk(path.join(root, "tests"), (file) => /\.test\.mjs$/.test(file));
  const anchors = [];

  for (const [kind, files] of [["code", codeFiles], ["test", testFiles]]) {
    for (const file of files) {
      const content = await fs.readFile(file, "utf8").catch(() => "");
      const lines = content.split(/\r?\n/);
      let match;
      ANCHOR_PATTERN.lastIndex = 0;
      while ((match = ANCHOR_PATTERN.exec(content))) {
        const line = content.slice(0, match.index).split(/\r?\n/).length;
        const target = findAdjacentDeclaration(lines, line - 1, kind);
        anchors.push({
          id: match[2],
          annotation: match[1],
          kind,
          path: path.relative(root, file),
          line,
          selector: target?.selector ?? null,
          declarationLine: target?.line ?? null
        });
      }
    }
  }
  return anchors.sort((a, b) => `${a.id}:${a.path}:${a.line}`.localeCompare(`${b.id}:${b.path}:${b.line}`));
}

function findAdjacentDeclaration(lines, anchorIndex, kind) {
  for (let index = anchorIndex + 1; index < Math.min(lines.length, anchorIndex + 7); index += 1) {
    const line = lines[index].trim();
    if (!line || line.startsWith("//") || line.startsWith("/*") || line.startsWith("*")) continue;

    if (kind === "test") {
      const testMatch = line.match(/^(?:test|it)\(\s*["'`]([^"'`]+)["'`]/);
      return testMatch ? { selector: testMatch[1], line: index + 1 } : null;
    }

    const functionMatch = line.match(/^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)/);
    if (functionMatch) return { selector: functionMatch[1], line: index + 1 };
    const classMatch = line.match(/^(?:export\s+)?class\s+([A-Za-z_$][\w$]*)/);
    if (classMatch) return { selector: classMatch[1], line: index + 1 };
    const methodMatch = line.match(/^(?:async\s+)?#?([A-Za-z_$][\w$]*)\s*\(/);
    if (methodMatch && !["if", "for", "while", "switch", "catch"].includes(methodMatch[1])) return { selector: methodMatch[1], line: index + 1 };
    return null;
  }
  return null;
}

function plannedTarget(value) {
  const separator = value?.indexOf("#") ?? -1;
  return separator < 0 ? { path: value ?? null, selector: null } : { path: value.slice(0, separator), selector: value.slice(separator + 1) };
}

function matchesPlanned(anchor, value) {
  const planned = plannedTarget(value);
  return anchor.path === planned.path && anchor.selector === planned.selector;
}

function diag(code, severity, message, sourcePath = null, line = null, identityId = null) {
  return { code, severity, message, identityId, source: sourcePath ? { path: sourcePath, line } : null };
}

export async function computeInputDigest(root = ROOT) {
  const files = [];
  for (const item of ["openspec", "bin", "src", "public", "tools", "tests", "scripts"]) {
    files.push(...await walk(path.join(root, item), (file) => /\.(?:md|ya?ml|json|mjs|js)$/.test(file)));
  }
  for (const item of ["package.json", "package-lock.json", "docs/.stele/config.toml", "artifacts/linkage-plan.json"]) {
    const file = path.join(root, item);
    if (await fs.stat(file).then((stat) => stat.isFile()).catch(() => false)) files.push(file);
  }
  const hash = createHash("sha256");
  for (const file of files.sort()) {
    hash.update(path.relative(root, file));
    hash.update("\0");
    hash.update(await fs.readFile(file));
    hash.update("\0");
  }
  return hash.digest("hex");
}

function gitState(root) {
  try {
    const revision = execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
    const dirty = Boolean(execFileSync("git", ["status", "--porcelain"], { cwd: root, encoding: "utf8" }).trim());
    return { revision, dirty };
  } catch {
    return { revision: "uncommitted", dirty: true };
  }
}

async function readJson(file, fallback) {
  try {
    return JSON.parse(await fs.readFile(file, "utf8"));
  } catch {
    return fallback;
  }
}

// @implements req.verify.b6e8f421cd09
export async function runVerification({ root = ROOT, changeId = "todo-showcase", mode = "implementation", reportPath } = {}) {
  const parsed = await parseSpecs({ root, changeId });
  const anchors = await scanAnchors({ root });
  const plan = await readJson(path.join(root, "artifacts", "linkage-plan.json"), { requirements: {}, scenarios: {} });
  const diagnostics = [...parsed.diagnostics];
  const known = new Set(parsed.requirements.flatMap((requirement) => [requirement.id, ...requirement.scenarios.map((scenario) => scenario.id)]).filter(Boolean));

  for (const anchor of anchors) {
    if (!known.has(anchor.id)) diagnostics.push(diag("ANCHOR_DANGLING", "error", `${anchor.annotation} names undeclared identity ${anchor.id}.`, anchor.path, anchor.line, anchor.id));
    if ((anchor.annotation === "implements") !== anchor.id.startsWith("req.")) diagnostics.push(diag("ANCHOR_KIND", "error", `${anchor.annotation} cannot target ${anchor.id}.`, anchor.path, anchor.line, anchor.id));
    if (mode === "implementation" && known.has(anchor.id) && !anchor.selector) diagnostics.push(diag("ANCHOR_TARGET_MISSING", "error", `${anchor.annotation} ${anchor.id} is not attached to a nearby compatible declaration.`, anchor.path, anchor.line, anchor.id));
  }

  for (const requirement of parsed.requirements) {
    const codeLinks = anchors.filter((anchor) => anchor.id === requirement.id && anchor.kind === "code");
    if (mode === "proposal" && !plan.requirements?.[requirement.id]) diagnostics.push(diag("PLAN_CODE_MISSING", "error", `No planned code target for ${requirement.id}.`, requirement.source.path, requirement.source.line, requirement.id));
    if (mode === "implementation" && codeLinks.length === 0) diagnostics.push(diag("LINK_CODE_MISSING", "error", `No code anchor resolves for ${requirement.id}.`, requirement.source.path, requirement.source.line, requirement.id));
    if (mode === "implementation" && codeLinks.length > 0 && plan.requirements?.[requirement.id] && !codeLinks.some((anchor) => matchesPlanned(anchor, plan.requirements[requirement.id]))) diagnostics.push(diag("LINK_TARGET_MISMATCH", "error", `${requirement.id} does not resolve to planned target ${plan.requirements[requirement.id]}.`, requirement.source.path, requirement.source.line, requirement.id));

    for (const scenario of requirement.scenarios) {
      const testLinks = anchors.filter((anchor) => anchor.id === scenario.id && anchor.kind === "test");
      if (mode === "proposal" && !plan.scenarios?.[scenario.id]) diagnostics.push(diag("PLAN_TEST_MISSING", "error", `No planned test target for ${scenario.id}.`, scenario.source.path, scenario.source.line, scenario.id));
      if (mode === "implementation" && testLinks.length === 0) diagnostics.push(diag("LINK_TEST_MISSING", "error", `No test anchor resolves for ${scenario.id}.`, scenario.source.path, scenario.source.line, scenario.id));
      if (mode === "implementation" && testLinks.length > 0 && plan.scenarios?.[scenario.id] && !testLinks.some((anchor) => matchesPlanned(anchor, plan.scenarios[scenario.id]))) diagnostics.push(diag("LINK_TARGET_MISMATCH", "error", `${scenario.id} does not resolve to planned target ${plan.scenarios[scenario.id]}.`, scenario.source.path, scenario.source.line, scenario.id));
    }
  }

  diagnostics.sort((a, b) => `${a.code}:${a.source?.path}:${a.source?.line}`.localeCompare(`${b.code}:${b.source?.path}:${b.source?.line}`));
  const inputDigest = await computeInputDigest(root);
  const execution = await readJson(path.join(root, "artifacts", "test-results.json"), null);
  const report = buildReport({ parsed, anchors, plan, diagnostics, mode, changeId, inputDigest, execution, root });

  if (reportPath) {
    const absolute = path.resolve(root, reportPath);
    await fs.mkdir(path.dirname(absolute), { recursive: true });
    await fs.writeFile(absolute, serializeReport(report));
  }
  return report;
}

// @implements req.verify.d3975ac8e142
export function buildReport({ parsed, anchors, plan, diagnostics, mode, changeId, inputDigest, execution, root }) {
  const git = gitState(root);
  const executionIsCurrent = execution?.inputDigest === inputDigest;
  const outcomeByScenario = new Map((execution?.scenarios ?? []).map((entry) => [entry.id, executionIsCurrent ? entry.outcome : "stale"]));
  const errorCount = diagnostics.filter((item) => item.severity === "error").length;

  const requirements = parsed.requirements.map((requirement) => {
    const codeLinks = anchors.filter((anchor) => anchor.id === requirement.id && anchor.kind === "code");
    return {
      id: requirement.id,
      title: requirement.title,
      status: "proposed",
      source: requirement.source,
      codeLinks: mode === "proposal" ? [{ kind: "code", state: "planned", target: plan.requirements?.[requirement.id] ?? null }] : codeLinks.map((anchor) => ({ ...anchor, state: "resolved" })),
      linkage: mode === "proposal" ? (plan.requirements?.[requirement.id] ? "planned" : "missing") : (codeLinks.length ? "linked" : "missing"),
      scenarios: requirement.scenarios.map((scenario) => {
        const testLinks = anchors.filter((anchor) => anchor.id === scenario.id && anchor.kind === "test");
        const outcome = outcomeByScenario.get(scenario.id) ?? "not-run";
        return {
          id: scenario.id,
          title: scenario.title,
          source: scenario.source,
          testLinks: mode === "proposal" ? [{ kind: "test", state: "planned", target: plan.scenarios?.[scenario.id] ?? null }] : testLinks.map((anchor) => ({ ...anchor, state: "resolved" })),
          linkage: mode === "proposal" ? (plan.scenarios?.[scenario.id] ? "planned" : "missing") : (testLinks.length ? "linked" : "missing"),
          execution: { state: outcome === "not-run" ? "not-run" : outcome === "stale" ? "stale" : "executed", outcome }
        };
      })
    };
  });

  const scenarios = requirements.flatMap((requirement) => requirement.scenarios);
  const executionStatus = scenarios.length && scenarios.every((scenario) => scenario.execution.outcome === "passed") ? "passed" : scenarios.some((scenario) => scenario.execution.outcome === "failed") ? "failed" : scenarios.some((scenario) => scenario.execution.outcome === "stale") ? "stale" : "not-run";
  return {
    schemaVersion: "2.0",
    verifier: { name: "stele", version: "0.2.0" },
    openspec: { version: "1.13.0", changeId },
    mode,
    repository: { revision: git.revision, dirty: git.dirty, inputDigest },
    complete: true,
    verdict: errorCount === 0 ? "pass" : "fail",
    stages: {
      proposal: { status: parsed.diagnostics.length === 0 ? "pass" : "fail" },
      linkage: { status: errorCount === 0 ? (mode === "proposal" ? "planned" : "pass") : "fail" },
      execution: { status: executionStatus, testedRevision: execution?.testedRevision ?? null },
      review: { status: "not-reviewed", reviewedRevision: null }
    },
    summary: {
      requirements: requirements.length,
      scenarios: scenarios.length,
      linkedRequirements: requirements.filter((item) => ["linked", "planned"].includes(item.linkage)).length,
      linkedScenarios: scenarios.filter((item) => ["linked", "planned"].includes(item.linkage)).length,
      passedScenarios: scenarios.filter((item) => item.execution.outcome === "passed").length,
      errors: errorCount,
      warnings: diagnostics.filter((item) => item.severity === "warning").length
    },
    requirements,
    diagnostics
  };
}

// @implements req.verify.91b3e6f04ac2
export function serializeReport(report) {
  return `${JSON.stringify(report, null, 2)}\n`;
}

async function main() {
  const args = process.argv.slice(2);
  const modeIndex = args.indexOf("--mode");
  const reportIndex = args.indexOf("--report");
  const mode = modeIndex >= 0 ? args[modeIndex + 1] : "implementation";
  const reportPath = reportIndex >= 0 ? args[reportIndex + 1] : undefined;
  if (!['proposal', 'implementation'].includes(mode)) throw new Error(`Unknown mode: ${mode}`);
  const report = await runVerification({ mode, reportPath });
  const mark = report.verdict === "pass" ? "✓" : "✗";
  console.log(`${mark} ${mode} verification ${report.verdict}: ${report.summary.requirements} requirements, ${report.summary.scenarios} scenarios, ${report.summary.errors} errors`);
  for (const diagnostic of report.diagnostics) console.log(`  ${diagnostic.severity.toUpperCase()} ${diagnostic.code}: ${diagnostic.message}`);
  process.exitCode = report.verdict === "pass" ? 0 : 1;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) main().catch((error) => {
  console.error(error.stack ?? error.message);
  process.exitCode = 2;
});
