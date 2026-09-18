import assert from "node:assert/strict";
import { spawnSync, type SpawnSyncReturns } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test, { type TestContext } from "node:test";

interface VerificationReport {
  readonly schemaVersion: string;
  readonly verdict: string;
  readonly linkage: string;
  readonly stages: {
    readonly execution: Readonly<Record<string, unknown>>;
  };
}

const ROOT: string = path.resolve(import.meta.dirname, "..");

function cli(args: readonly string[]): SpawnSyncReturns<string> {
  return spawnSync("dist/stele", args, { cwd: ROOT, encoding: "utf8" });
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function parseVerificationReport(value: string): VerificationReport {
  const parsed: unknown = JSON.parse(value);
  assert.ok(isRecord(parsed));
  assert.ok(typeof parsed.schemaVersion === "string");
  assert.ok(typeof parsed.verdict === "string");
  assert.ok(isRecord(parsed.stages));
  assert.ok(isRecord(parsed.stages.execution));
  assert.ok(isRecord(parsed.verdicts));
  assert.ok(typeof parsed.verdicts.linkage === "string");
  return {
    schemaVersion: parsed.schemaVersion,
    verdict: parsed.verdict,
    linkage: parsed.verdicts.linkage,
    stages: { execution: parsed.stages.execution },
  };
}

async function passingFixture(context: TestContext): Promise<string> {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-pass-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const specRoot: string = path.join(root, "openspec/changes/example/specs/todo");
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
  await fs.writeFile(path.join(root, "src/todo.mts"), "// @" + "implements req.todo.0123456789ab\nexport function addTodo(text: string): { text: string } { return { text }; }\n");
  await fs.writeFile(path.join(root, "tests/todo.test.mts"), [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../src/todo.mts";',
    "// @" + "verifies scn.todo.abcdef012345",
    'test("saves entered text", () => assert.equal(addTodo("ship").text, "ship"));',
    "",
  ].join("\n"));
  return root;
}

// @verifies scn.verify.5e9a130cd7b4.e2e
void test("exposes an executable verify command", async (context: TestContext): Promise<void> => {
  const root: string = await passingFixture(context);
  const result: SpawnSyncReturns<string> = cli([
    "verify",
    "--root",
    root,
    "--change",
    "example",
    "--json",
  ]);
  assert.equal(result.status, 0, result.stderr);
  const report: VerificationReport = parseVerificationReport(result.stdout);
  assert.equal(report.schemaVersion, "2.1");
  assert.equal(report.linkage, "pass");
  assert.equal(report.verdict, "incomplete");
});

// @verifies scn.verify.a2c7e48b610f.e2e
void test("returns stable failure exit codes", async (context: TestContext): Promise<void> => {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-fail-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const specRoot: string = path.join(root, "openspec/changes/example/specs/example");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), "### Requirement: Missing identity\n#### Scenario: Missing too\n");
  const policyFailure: SpawnSyncReturns<string> = cli([
    "verify",
    "--root",
    root,
    "--change",
    "example",
    "--json",
  ]);
  const invocationFailure: SpawnSyncReturns<string> = cli(["unknown-command"]);
  assert.equal(policyFailure.status, 1);
  assert.equal(invocationFailure.status, 2);
});

// @verifies scn.verify.d6f8012b3ea5.e2e
void test("emits identical default JSON for identical inputs", async (context: TestContext): Promise<void> => {
  const root: string = await passingFixture(context);
  const args: readonly string[] = ["verify", "--root", root, "--change", "example", "--json"];
  const first: SpawnSyncReturns<string> = cli(args);
  const second: SpawnSyncReturns<string> = cli(args);
  assert.equal(first.status, 0, first.stderr);
  assert.equal(second.status, 0, second.stderr);
  assert.equal(first.stdout, second.stdout);
});

// @verifies scn.verify.3a70c9d1ef24.e2e
void test("omits volatile metadata from the deterministic payload", async (context: TestContext): Promise<void> => {
  const root: string = await passingFixture(context);
  const result: SpawnSyncReturns<string> = cli([
    "verify",
    "--root",
    root,
    "--change",
    "example",
    "--json",
  ]);
  const report: VerificationReport = parseVerificationReport(result.stdout);
  assert.equal("generatedAt" in report, false);
  assert.equal("recordedAt" in report.stages.execution, false);
});

const OPENSPEC_CLI: string = path.join(ROOT, "node_modules/@fission-ai/openspec/bin/openspec.js");

async function writeProjectFile(root: string, relative: string, lines: readonly string[]): Promise<void> {
  const target: string = path.join(root, relative);
  await fs.mkdir(path.dirname(target), { recursive: true });
  await fs.writeFile(target, lines.join("\n"));
}

// @verifies scn.verificationscope.8b577e9712e7.e2e
void test("verifies current specifications after archiving", async (context: TestContext): Promise<void> => {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-archive-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const change = "openspec/changes/todo-basics";
  await writeProjectFile(root, "package.json", ['{ "type": "module" }', ""]);
  await writeProjectFile(root, "openspec/config.yaml", ["schema: spec-driven", ""]);
  await fs.mkdir(path.join(root, "openspec/specs"), { recursive: true });
  await writeProjectFile(root, `${change}/proposal.md`, [
    "## Why", "", "Users need to record todos before they can manage their work.", "",
    "## What Changes", "", "- Add a todo from entered text.", "",
  ]);
  await writeProjectFile(root, `${change}/tasks.md`, ["## 1. Work", "", "- [x] 1.1 Add todos", ""]);
  await writeProjectFile(root, `${change}/specs/todo/spec.md`, [
    "## Purpose", "", "Let users keep a small list of todos in the example application.", "",
    "## ADDED Requirements", "",
    "### Requirement: Add a todo", "Verification-ID: req.todo.0123456789ab", "",
    "The application SHALL add a todo from entered text.", "",
    "#### Scenario: Save entered text", "Verification-ID: scn.todo.abcdef012345", "",
    "- **WHEN** a user enters a todo", "- **THEN** the todo is saved", "",
  ]);
  await writeProjectFile(root, `${change}/linkage-plan.json`, [JSON.stringify({
    schemaVersion: 1,
    changeId: "todo-basics",
    requirements: { "req.todo.0123456789ab": "src/todo.mts#addTodo" },
    scenarios: { "scn.todo.abcdef012345": "tests/todo.test.mts#saves entered text" },
  }), ""]);
  await writeProjectFile(root, "src/todo.mts", [
    "// @" + "implements req.todo.0123456789ab",
    "export function addTodo(text: string): { text: string } { return { text }; }",
    "",
  ]);
  await writeProjectFile(root, "tests/todo.test.mts", [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../src/todo.mts";',
    "// @" + "verifies scn.todo.abcdef012345.e2e",
    'void test("saves entered text", (): void => { assert.equal(addTodo("ship").text, "ship"); });',
    "",
  ]);

  const archived: SpawnSyncReturns<string> = spawnSync(
    "node",
    [OPENSPEC_CLI, "archive", "todo-basics", "--yes"],
    { cwd: root, encoding: "utf8", env: { ...process.env, OPENSPEC_TELEMETRY: "0" } },
  );
  assert.equal(archived.status, 0, archived.stdout + archived.stderr);

  const verified: SpawnSyncReturns<string> = cli(["verify", "--specs", "--root", root, "--json"]);
  assert.equal(verified.status, 0, verified.stdout);
  assert.equal(parseVerificationReport(verified.stdout).linkage, "pass");

  const validated: SpawnSyncReturns<string> = cli(["validate", "--specs", "--root", root]);
  assert.equal(validated.status, 0, validated.stdout + validated.stderr);
});

interface ScenarioEvidence {
  readonly outcome: string;
  readonly scenarios: readonly { readonly id: string; readonly outcome: string }[];
}

function parseScenarioEvidence(value: string): ScenarioEvidence {
  const parsed: unknown = JSON.parse(value);
  assert.ok(isRecord(parsed));
  assert.ok(typeof parsed.outcome === "string");
  const scenarios: unknown = parsed.scenarios;
  assert.ok(isUnknownArray(scenarios));
  return {
    outcome: parsed.outcome,
    scenarios: scenarios.map((entry: unknown): { id: string; outcome: string } => {
      assert.ok(isRecord(entry));
      assert.ok(typeof entry.id === "string");
      assert.ok(typeof entry.outcome === "string");
      return { id: entry.id, outcome: entry.outcome };
    }),
  };
}

function isUnknownArray(value: unknown): value is readonly unknown[] {
  return Array.isArray(value);
}

// @verifies scn.gosupport.2c88ed381429.e2e
// @verifies scn.execution.9cbf5cc6d03d.e2e
void test("runs TypeScript and Go scenario tests together", async (context: TestContext): Promise<void> => {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-mixed-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  await writeProjectFile(root, "package.json", ['{ "type": "module" }', ""]);
  await writeProjectFile(root, "go.mod", ["module example.com/mixed", "", "go 1.22", ""]);
  await writeProjectFile(root, "openspec/changes/mixed/specs/mixed/spec.md", [
    "### Requirement: Mix languages", "Verification-ID: req.mixed.0123456789ab",
    "#### Scenario: TypeScript behavior", "Verification-ID: scn.mixed.111111111111",
    "#### Scenario: Go behavior", "Verification-ID: scn.mixed.222222222222",
    "",
  ]);
  await writeProjectFile(root, "openspec/changes/mixed/linkage-plan.json", [JSON.stringify({
    schemaVersion: 1,
    changeId: "mixed",
    requirements: { "req.mixed.0123456789ab": "internal/mixed/mixed.go#Double" },
    scenarios: {
      "scn.mixed.111111111111": "tests/mixed.test.mts#runs in node",
      "scn.mixed.222222222222": "internal/mixed/mixed_test.go#TestDouble",
    },
  }), ""]);
  await writeProjectFile(root, "internal/mixed/mixed.go", [
    "package mixed", "",
    "// @" + "implements req.mixed.0123456789ab",
    "func Double(value int) int { return value * 2 }", "",
  ]);
  await writeProjectFile(root, "internal/mixed/mixed_test.go", [
    "package mixed", "", 'import "testing"', "",
    "// @" + "verifies scn.mixed.222222222222.unit",
    "func TestDouble(t *testing.T) {",
    "\tif Double(2) != 4 {",
    '\t\tt.Fatal("double failed")',
    "\t}",
    "}", "",
  ]);
  await writeProjectFile(root, "tests/mixed.test.mts", [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    "// @" + "verifies scn.mixed.111111111111.unit",
    'void test("runs in node", (): void => { assert.equal(1 + 1, 2); });',
    "",
  ]);

  const tested: SpawnSyncReturns<string> = cli(["test", "--root", root, "--change", "mixed", "--json"]);
  assert.equal(tested.status, 0, tested.stdout + tested.stderr);
  const evidence: ScenarioEvidence = parseScenarioEvidence(tested.stdout);
  assert.equal(evidence.outcome, "passed");
  assert.deepEqual(evidence.scenarios, [
    { id: "scn.mixed.111111111111", outcome: "passed" },
    { id: "scn.mixed.222222222222", outcome: "passed" },
  ]);

  const verified: SpawnSyncReturns<string> = cli(["verify", "--root", root, "--change", "mixed", "--json"]);
  assert.equal(verified.status, 0, verified.stdout);
});

// @verifies scn.ids.83b2e0efd623.e2e
void test("inserts and checks verification IDs", async (context: TestContext): Promise<void> => {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-ids-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const specPath: string = path.join(root, "openspec/changes/draft/specs/todo/spec.md");
  await fs.mkdir(path.dirname(specPath), { recursive: true });
  await fs.writeFile(specPath, [
    "## ADDED Requirements",
    "",
    "### Requirement: Add a todo",
    "The application SHALL add a todo.",
    "",
    "#### Scenario: Save entered text",
    "- **WHEN** a user enters a todo",
    "- **THEN** the todo is saved",
    "",
  ].join("\n"));
  const scope: readonly string[] = ["--root", root, "--change", "draft"];

  const missing: SpawnSyncReturns<string> = cli(["ids", ...scope, "--check"]);
  assert.equal(missing.status, 1, missing.stderr);
  assert.match(missing.stdout, /2 headings lack a Verification-ID/v);

  const inserted: SpawnSyncReturns<string> = cli(["ids", ...scope]);
  assert.equal(inserted.status, 0, inserted.stderr);
  const content: string = await fs.readFile(specPath, "utf8");
  assert.match(content, /### Requirement: Add a todo\nVerification-ID: req\.todo\.[a-f0-9]{12}\n/v);
  assert.match(content, /#### Scenario: Save entered text\nVerification-ID: scn\.todo\.[a-f0-9]{12}\n/v);

  const checked: SpawnSyncReturns<string> = cli(["ids", ...scope, "--check"]);
  assert.equal(checked.status, 0, checked.stdout);

  const proposal: SpawnSyncReturns<string> = cli(["verify", ...scope, "--stage", "proposal", "--json"]);
  assert.equal(parseVerificationReport(proposal.stdout).schemaVersion.length > 0, true);
  assert.doesNotMatch(proposal.stdout, /ID_(?:REQUIREMENT|SCENARIO)_MISSING/v);
});

// @verifies scn.verificationstrategy.122427afc2f9.e2e
// @verifies scn.verificationstrategy.1b1070d81978.e2e
void test("runs the approved evidence workflow end to end", async (context: TestContext): Promise<void> => {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-evidence-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const change = "openspec/changes/todo-basics";
  const scenario = "scn.todo.abcdef012345";
  await writeProjectFile(root, "package.json", ['{ "type": "module" }', ""]);
  await writeProjectFile(root, "openspec/config.yaml", ["schema: spec-driven", ""]);
  await writeProjectFile(root, `${change}/proposal.md`, [
    "## Why", "", "Users need to record todos before they can manage their work.", "",
    "## What Changes", "", "- Add a todo from entered text.", "",
  ]);
  await writeProjectFile(root, `${change}/tasks.md`, ["## 1. Work", "", "- [x] 1.1 Add todos", ""]);
  await writeProjectFile(root, `${change}/specs/todo/spec.md`, [
    "## ADDED Requirements", "",
    "### Requirement: Add a todo", "Verification-ID: req.todo.0123456789ab", "",
    "The application SHALL add a todo from entered text.", "",
    "#### Scenario: Save entered text", `Verification-ID: ${scenario}`, "",
    "- **WHEN** a user enters a todo", "- **THEN** the todo is saved", "",
  ]);
  await writeProjectFile(root, `${change}/linkage-plan.json`, [JSON.stringify({
    schemaVersion: 2,
    changeId: "todo-basics",
    scenarios: {
      [scenario]: {
        evidence: [
          { id: `${scenario}.unit`, level: "unit", rationale: "Pure logic.", placement: "tests/todo.test.mts" },
          { id: `${scenario}.e2e`, level: "e2e", rationale: "The user journey.", placement: "tests/e2e/todo.test.mts" },
        ],
      },
    },
  }, null, 2), ""]);
  await writeProjectFile(root, "src/todo.mts", [
    "// @" + "implements req.todo.0123456789ab",
    "export function addTodo(text: string): { text: string } { return { text }; }",
    "",
  ]);
  const testFile = (anchor: string, title: string): readonly string[] => [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../../src/todo.mts";',
    "// @" + `verifies ${anchor}`,
    `void test("${title}", (): void => { assert.equal(addTodo("ship").text, "ship"); });`,
    "",
  ];
  await writeProjectFile(root, "src/checks/todo.test.mts", testFile(`${scenario}.unit`, "saves entered text"));
  await writeProjectFile(root, "scripts/journeys/todo.test.mts", testFile(`${scenario}.e2e`, "records a todo"));
  const scope: readonly string[] = ["--root", root, "--change", "todo-basics"];

  const pending: SpawnSyncReturns<string> = cli(["verify", ...scope, "--stage", "proposal", "--json"]);
  assert.equal(pending.status, 1, pending.stderr);
  assert.match(pending.stdout, /PLAN_UNAPPROVED/v);

  const refused: SpawnSyncReturns<string> = cli(["approve", ...scope]);
  assert.equal(refused.status, 2, refused.stdout);
  assert.match(refused.stderr, /--confirmed-in-chat/v);

  const approvedPlan: SpawnSyncReturns<string> = cli(["approve", ...scope, "--all", "--yes", "--by", "Tester"]);
  assert.equal(approvedPlan.status, 0, approvedPlan.stderr);
  assert.match(approvedPlan.stdout, /approved scn\.todo\.abcdef012345\.unit by Tester \(via cli\)/v);
  assert.match(approvedPlan.stdout, /approved scn\.todo\.abcdef012345\.e2e by Tester \(via cli\)/v);

  const planned: SpawnSyncReturns<string> = cli(["verify", ...scope, "--stage", "proposal"]);
  assert.equal(planned.status, 0, planned.stdout);

  const validated: SpawnSyncReturns<string> = cli(["validate", ...scope, "--json"]);
  assert.equal(validated.status, 0, validated.stdout + validated.stderr);
  const report: string = await fs.readFile(path.join(root, "artifacts/verification-report.json"), "utf8");
  assert.match(report, /"evidenceId": "scn\.todo\.abcdef012345\.e2e",\n\s+"level": "e2e"/v);
  assert.match(report, /"path": "scripts\/journeys\/todo\.test\.mts"/v);
  const evidence: ScenarioEvidence = parseScenarioEvidence(
    await fs.readFile(path.join(root, "artifacts/test-results.json"), "utf8"),
  );
  assert.deepEqual(evidence.scenarios, [{ id: scenario, outcome: "passed" }]);
});

interface LinkIndex {
  readonly scopes: readonly string[];
  readonly scenarioSteps: readonly string[];
  readonly evidence: readonly string[];
  readonly anchors: readonly string[];
}

function field(record: Record<string, unknown>, key: string): string {
  const value: unknown = record[key];
  assert.ok(typeof value === "string" || value === null || value === undefined, key);
  return value ?? "";
}

function records(value: unknown): readonly Record<string, unknown>[] {
  assert.ok(isUnknownArray(value));
  return value.map((item: unknown): Record<string, unknown> => {
    assert.ok(isRecord(item));
    return item;
  });
}

function parseLinkIndex(value: string): LinkIndex {
  const parsed: unknown = JSON.parse(value);
  assert.ok(isRecord(parsed));
  const scenarios: readonly Record<string, unknown>[] = records(parsed.scenarios);
  return {
    scopes: records(parsed.scopes).map((scope: Record<string, unknown>): string => `${field(scope, "kind")}:${field(scope, "id")}`),
    scenarioSteps: scenarios.flatMap((scenario: Record<string, unknown>): string[] =>
      records(scenario.steps).map((step: Record<string, unknown>): string => `${field(step, "keyword")} ${field(step, "text")}`)),
    evidence: scenarios.flatMap((scenario: Record<string, unknown>): string[] =>
      records(scenario.evidence).map((item: Record<string, unknown>): string => `${field(item, "id")}:${field(item, "approval")}`)),
    anchors: records(parsed.anchors).map((anchor: Record<string, unknown>): string =>
      `${field(anchor, "kind")}:${field(anchor, "level")}:${field(anchor, "selector")}:${field(anchor, "status")}`),
  };
}

async function evidencePlanFixture(context: TestContext): Promise<string> {
  const root: string = await fs.mkdtemp(path.join(os.tmpdir(), "stele-cli-index-"));
  context.after((): Promise<void> => fs.rm(root, { recursive: true }));
  const change = "openspec/changes/todo-basics";
  const scenario = "scn.todo.abcdef012345";
  await writeProjectFile(root, "package.json", ['{ "type": "module" }', ""]);
  await writeProjectFile(root, `${change}/specs/todo/spec.md`, [
    "## ADDED Requirements", "",
    "### Requirement: Add a todo", "Verification-ID: req.todo.0123456789ab", "",
    "The application SHALL add a todo from entered text.", "",
    "#### Scenario: Save entered text", `Verification-ID: ${scenario}`, "",
    "- **WHEN** a user enters a todo", "- **THEN** the todo is saved", "",
  ]);
  await writeProjectFile(root, `${change}/linkage-plan.json`, [JSON.stringify({
    schemaVersion: 2,
    changeId: "todo-basics",
    scenarios: {
      [scenario]: {
        evidence: [
          { id: `${scenario}.unit`, level: "unit", rationale: "Pure logic." },
          { id: `${scenario}.e2e`, level: "e2e", rationale: "The user journey." },
        ],
      },
    },
  }, null, 2), ""]);
  await writeProjectFile(root, "src/todo.mts", [
    "// @" + "implements req.todo.0123456789ab",
    "export function addTodo(text: string): { text: string } { return { text }; }",
    "",
  ]);
  const testFile = (anchor: string, title: string): readonly string[] => [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../src/todo.mts";',
    "// @" + `verifies ${anchor}`,
    `void test("${title}", (): void => { assert.equal(addTodo("ship").text, "ship"); });`,
    "",
  ];
  await writeProjectFile(root, "tests/todo.test.mts", testFile(`${scenario}.unit`, "saves entered text"));
  await writeProjectFile(root, "tests/journey.test.mts", testFile(`${scenario}.e2e`, "records a todo"));
  const approved: SpawnSyncReturns<string> = cli(["approve", "--root", root, "--change", "todo-basics", "--all", "--yes", "--by", "Tester"]);
  assert.equal(approved.status, 0, approved.stderr);
  return root;
}

// @verifies scn.linkindex.30ab15bc8293.e2e
void test("prints a link index for editors", async (context: TestContext): Promise<void> => {
  const root: string = await evidencePlanFixture(context);
  const result: SpawnSyncReturns<string> = cli(["index", "--root", root, "--change", "todo-basics", "--json"]);
  assert.equal(result.status, 0, result.stderr);
  const index: LinkIndex = parseLinkIndex(result.stdout);
  assert.deepEqual(index.scopes, ["change:todo-basics"]);
  assert.deepEqual(index.scenarioSteps, ["WHEN a user enters a todo", "THEN the todo is saved"]);
  assert.deepEqual(index.evidence, ["scn.todo.abcdef012345.unit:approved", "scn.todo.abcdef012345.e2e:approved"]);
  assert.deepEqual(index.anchors, [
    "code::addTodo:linked",
    "test:e2e:records a todo:linked",
    "test:unit:saves entered text:linked",
  ]);

  const tested: SpawnSyncReturns<string> = cli(["test", "scn.todo.abcdef012345.e2e", "--root", root, "--change", "todo-basics"]);
  assert.equal(tested.status, 0, tested.stdout + tested.stderr);
  assert.match(tested.stdout, /selected tests: 1\/1 passed/v);
  const after: SpawnSyncReturns<string> = cli(["index", "--root", root, "--change", "todo-basics"]);
  assert.match(after.stdout, /"state": "executed",\n\s+"outcome": "passed"/v);
});

// @verifies scn.linkindex.c0e515c9575a.e2e
void test("emits an identical link index for identical inputs", async (context: TestContext): Promise<void> => {
  const root: string = await evidencePlanFixture(context);
  const printed: SpawnSyncReturns<string> = cli(["index", "--root", root, "--change", "todo-basics", "--json"]);
  const written: SpawnSyncReturns<string> = cli(["index", "--root", root, "--change", "todo-basics", "--output-file", "out/index.json"]);
  assert.equal(printed.status, 0, printed.stderr);
  assert.equal(written.status, 0, written.stderr);
  assert.equal(written.stdout, "");
  assert.equal(await fs.readFile(path.join(root, "out/index.json"), "utf8"), printed.stdout);
  assert.doesNotMatch(printed.stdout, /generatedAt|recordedAt|timestamp/v);
});

// @verifies scn.linkindex.7c1d78728036.e2e
void test("checks every scope with --all", async (context: TestContext): Promise<void> => {
  const root: string = await passingFixture(context);
  await writeProjectFile(root, "openspec/specs/todo/spec.md", [
    "### Requirement: Add a todo", "Verification-ID: req.todo.0123456789ab",
    "#### Scenario: Save entered text", "Verification-ID: scn.todo.abcdef012345", "",
  ]);
  await writeProjectFile(root, "openspec/changes/broken/specs/broken/spec.md", [
    "### Requirement: Broken", "Verification-ID: req.broken.0123456789ab",
    "#### Scenario: Broken", "Verification-ID: scn.broken.abcdef012345", "",
  ]);
  const result: SpawnSyncReturns<string> = cli(["verify", "--all", "--root", root]);
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.match(result.stdout, /== current specifications ==\n✓ implementation verification pass/v);
  assert.match(result.stdout, /== change broken ==\n✗ implementation verification fail/v);
  assert.match(result.stdout, /== change example ==\n✓ implementation verification pass/v);
  assert.match(result.stdout, /✗ 1 of 3 scopes failed: change broken/v);

  const conflicting: SpawnSyncReturns<string> = cli(["verify", "--all", "--change", "example", "--root", root]);
  assert.equal(conflicting.status, 2);
});
