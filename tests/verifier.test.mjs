import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { parseSpecs, runVerification, scanAnchors } from "../tools/verify.mjs";

// @verifies scn.verify.c905a1e37fd8
test("parses all stable identities and parent relationships", async () => {
  const parsed = await parseSpecs();
  assert.equal(parsed.diagnostics.length, 0);
  assert.equal(parsed.requirements.length, 17);
  assert.equal(parsed.requirements.flatMap((requirement) => requirement.scenarios).length, 26);
  assert.ok(parsed.requirements.every((requirement) => requirement.id.startsWith("req.") && requirement.scenarios.every((scenario) => scenario.id.startsWith("scn."))));
});

// @verifies scn.verify.1ae4d6739cb0
test("reports duplicate identities with a stable code", async (context) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "trace-duplicate-"));
  context.after(() => fs.rm(root, { recursive: true }));
  const specRoot = path.join(root, "openspec/changes/todo-showcase/specs/example");
  await fs.mkdir(specRoot, { recursive: true });
  await fs.writeFile(path.join(specRoot, "spec.md"), `### Requirement: First\nVerification-ID: req.demo.aaaaaaaaaaaa\n#### Scenario: One\nVerification-ID: scn.demo.bbbbbbbbbbbb\n### Requirement: Second\nVerification-ID: req.demo.aaaaaaaaaaaa\n#### Scenario: Two\nVerification-ID: scn.demo.cccccccccccc\n`);
  const parsed = await parseSpecs({ root });
  assert.ok(parsed.diagnostics.some((item) => item.code === "ID_DUPLICATE"));
});

// @verifies scn.verify.845e10cb79d6
test("proposal mode accepts complete planned targets", async () => {
  const report = await runVerification({ mode: "proposal" });
  assert.equal(report.verdict, "pass", JSON.stringify(report.diagnostics));
  assert.equal(report.stages.linkage.status, "planned");
  assert.ok(report.requirements.every((requirement) => requirement.linkage === "planned"));
});

// @verifies scn.verify.732cf49a0e18
test("implementation mode resolves every requirement and scenario anchor", async () => {
  const report = await runVerification({ mode: "implementation" });
  assert.equal(report.verdict, "pass", JSON.stringify(report.diagnostics));
  assert.equal(report.summary.linkedRequirements, report.summary.requirements);
  assert.equal(report.summary.linkedScenarios, report.summary.scenarios);
});

// @verifies scn.verify.e3817b0dcf54
test("detects an anchor that names no declared identity", async (context) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "trace-dangling-"));
  context.after(() => fs.rm(root, { recursive: true }));
  await fs.mkdir(path.join(root, "src"), { recursive: true });
  await fs.mkdir(path.join(root, "tests"), { recursive: true });
  await fs.mkdir(path.join(root, "openspec/changes/todo-showcase/specs/example"), { recursive: true });
  await fs.writeFile(path.join(root, "openspec/changes/todo-showcase/specs/example/spec.md"), `### Requirement: Example\nVerification-ID: req.demo.bbbbbbbbbbbb\n#### Scenario: Example works\nVerification-ID: scn.demo.cccccccccccc\n`);
  const annotation = ["@implements", "req.demo.aaaaaaaaaaaa"].join(" ");
  const validCodeAnchor = ["@implements", "req.demo.bbbbbbbbbbbb"].join(" ");
  const validTestAnchor = ["@verifies", "scn.demo.cccccccccccc"].join(" ");
  await fs.writeFile(path.join(root, "src/example.mjs"), `// ${validCodeAnchor}\n// ${annotation}\nexport const value = 1;\n`);
  await fs.writeFile(path.join(root, "tests/example.test.mjs"), `// ${validTestAnchor}\n`);
  const anchors = await scanAnchors({ root });
  assert.ok(anchors.some((anchor) => anchor.id === "req.demo.aaaaaaaaaaaa"));
  const report = await runVerification({ root, mode: "implementation" });
  assert.ok(report.diagnostics.some((item) => item.code === "ANCHOR_DANGLING" && item.identityId === "req.demo.aaaaaaaaaaaa"));
});

// @verifies scn.verify.f41ca285d706
test("report preserves independent linkage, execution, and review states", async () => {
  const report = await runVerification({ mode: "implementation" });
  assert.equal(report.schemaVersion, "2.0");
  assert.equal(report.stages.linkage.status, "pass");
  assert.ok(["not-run", "stale", "passed", "failed"].includes(report.stages.execution.status));
  assert.equal(report.stages.review.status, "not-reviewed");
  assert.equal(report.complete, true);
});

// @verifies scn.verify.72d4b0e91fac
test("resolves anchors to nearby planned declarations", async () => {
  const anchors = await scanAnchors();
  assert.ok(anchors.some((anchor) => anchor.id === "req.todo.7a92c1e8b304" && anchor.path === "src/todo-store.mjs" && anchor.selector === "list"));
  assert.ok(anchors.some((anchor) => anchor.id === "scn.todo.f98c1437a6d2" && anchor.path === "tests/todo-store.test.mjs" && anchor.selector === "creates a trimmed open task"));
});

// @verifies scn.verify.b5e72a0c4d89
test("rejects anchors without their planned declaration target", async (context) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "trace-target-"));
  context.after(() => fs.rm(root, { recursive: true }));
  await fs.mkdir(path.join(root, "src"), { recursive: true });
  await fs.mkdir(path.join(root, "tests"), { recursive: true });
  await fs.mkdir(path.join(root, "artifacts"), { recursive: true });
  await fs.mkdir(path.join(root, "openspec/changes/todo-showcase/specs/example"), { recursive: true });
  await fs.writeFile(path.join(root, "openspec/changes/todo-showcase/specs/example/spec.md"), `### Requirement: Example\nVerification-ID: req.demo.bbbbbbbbbbbb\n#### Scenario: Example works\nVerification-ID: scn.demo.cccccccccccc\n`);
  const codeAnchor = ["@implements", "req.demo.bbbbbbbbbbbb"].join(" ");
  const testAnchor = ["@verifies", "scn.demo.cccccccccccc"].join(" ");
  await fs.writeFile(path.join(root, "src/example.mjs"), `// ${codeAnchor}\nexport function actualTarget() {}\n`);
  await fs.writeFile(path.join(root, "tests/example.test.mjs"), `import test from "node:test";\n// ${testAnchor}\ntest("example works", () => {});\n`);
  await fs.writeFile(path.join(root, "artifacts/linkage-plan.json"), JSON.stringify({ requirements: { "req.demo.bbbbbbbbbbbb": "src/example.mjs#differentTarget" }, scenarios: { "scn.demo.cccccccccccc": "tests/example.test.mjs#example works" } }));
  const report = await runVerification({ root, mode: "implementation" });
  assert.ok(report.diagnostics.some((item) => item.code === "LINK_TARGET_MISMATCH" && item.identityId === "req.demo.bbbbbbbbbbbb"));
});
