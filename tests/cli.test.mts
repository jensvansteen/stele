import assert from "node:assert/strict";
import { spawnSync, type SpawnSyncReturns } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test, { type TestContext } from "node:test";

interface VerificationReport {
  readonly schemaVersion: string;
  readonly verdict: string;
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
  return {
    schemaVersion: parsed.schemaVersion,
    verdict: parsed.verdict,
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
  await fs.writeFile(path.join(root, "src/todo.mts"), "// @implements req.todo.0123456789ab\nexport function addTodo(text: string): { text: string } { return { text }; }\n");
  await fs.writeFile(path.join(root, "tests/todo.test.mts"), [
    'import assert from "node:assert/strict";',
    'import test from "node:test";',
    'import { addTodo } from "../src/todo.mts";',
    "// @verifies scn.todo.abcdef012345",
    'test("saves entered text", () => assert.equal(addTodo("ship").text, "ship"));',
    "",
  ].join("\n"));
  return root;
}

// @verifies scn.verify.5e9a130cd7b4
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
  assert.equal(report.schemaVersion, "2.0");
  assert.equal(report.verdict, "pass");
});

// @verifies scn.verify.a2c7e48b610f
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

// @verifies scn.verify.d6f8012b3ea5
void test("emits identical default JSON for identical inputs", async (context: TestContext): Promise<void> => {
  const root: string = await passingFixture(context);
  const args: readonly string[] = ["verify", "--root", root, "--change", "example", "--json"];
  const first: SpawnSyncReturns<string> = cli(args);
  const second: SpawnSyncReturns<string> = cli(args);
  assert.equal(first.status, 0, first.stderr);
  assert.equal(second.status, 0, second.stderr);
  assert.equal(first.stdout, second.stdout);
});

// @verifies scn.verify.3a70c9d1ef24
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
