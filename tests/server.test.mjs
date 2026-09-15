import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { startServer } from "../server.mjs";

let instance;
let baseUrl;
let recordingsRoot;
let recordingManifestPath;

before(async () => {
  recordingsRoot = await fs.mkdtemp(path.join(os.tmpdir(), "openspec-recordings-"));
  recordingManifestPath = path.join(recordingsRoot, "manifest.json");
  await fs.writeFile(path.join(recordingsRoot, "todo.mp4"), Buffer.from("recording-evidence"));
  await fs.writeFile(recordingManifestPath, JSON.stringify({ recordings: [{ id: "todo-workflow", title: "Todo workflow", description: "Recorded flow", relativePath: "todo.mp4", tailnetUrl: "http://100.64.0.1:7780/todo.mp4", coveredScenarios: ["scn.todo.f98c1437a6d2"] }] }));
  instance = await startServer({ port: 0, seed: [{ id: "todo-1", title: "Seed task", completed: false }], recordingsRoot, recordingManifestPath });
  baseUrl = `http://127.0.0.1:${instance.port}`;
});

after(async () => {
  if (!instance) return;
  await new Promise((resolve, reject) => instance.server.close((error) => error ? reject(error) : resolve()));
});

// @verifies scn.todo.82e61fc5d9a7
test("serves the initial task list and accurate summary", async () => {
  const response = await fetch(`${baseUrl}/api/todos`);
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), {
    items: [{ id: "todo-1", title: "Seed task", completed: false }],
    summary: { total: 1, open: 1, done: 0 }
  });
});

test("supports Todo creation, toggle, deletion, and validation over HTTP", async () => {
  const createdResponse = await fetch(`${baseUrl}/api/todos`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ title: "API task" })
  });
  const { todo } = await createdResponse.json();
  assert.equal(createdResponse.status, 201);
  assert.equal(todo.title, "API task");

  const toggled = await fetch(`${baseUrl}/api/todos/${todo.id}`, { method: "PATCH", headers: { "content-type": "application/json" }, body: "{}" });
  assert.equal((await toggled.json()).todo.completed, true);

  const removed = await fetch(`${baseUrl}/api/todos/${todo.id}`, { method: "DELETE" });
  assert.equal(removed.status, 200);

  const invalid = await fetch(`${baseUrl}/api/todos`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ title: " " }) });
  assert.equal(invalid.status, 400);
});

// @verifies scn.dashboard.60b4ae93f7c1
// @verifies scn.dashboard.b1c6f804d3e9
test("serves a verification summary with requirement links", async () => {
  const response = await fetch(`${baseUrl}/api/verification`);
  const report = await response.json();
  assert.equal(response.status, 200);
  assert.equal(report.schemaVersion, "2.0");
  assert.equal(report.summary.requirements, 17);
  assert.ok(report.requirements.every((requirement) => requirement.codeLinks.length > 0));
});

// @verifies scn.dashboard.73fa18c5b2d0
test("runs implementation verification from the dashboard endpoint", async () => {
  const response = await fetch(`${baseUrl}/api/verify`, { method: "POST", headers: { "content-type": "application/json" }, body: "{}" });
  const report = await response.json();
  assert.equal(response.status, 200, JSON.stringify(report.diagnostics));
  assert.equal(report.verdict, "pass");
});

// @verifies scn.dashboard.54a702fd89c3
test("serves allowlisted artifacts and rejects arbitrary paths", async () => {
  const response = await fetch(`${baseUrl}/api/artifacts`);
  const { artifacts } = await response.json();
  assert.ok(artifacts.some((artifact) => artifact.path.endsWith("proposal.md") && artifact.content.includes("Todo verification showcase")));
  assert.ok(artifacts.some((artifact) => artifact.path === "PLAN.md"));

  const page = await fetch(baseUrl);
  assert.match(await page.text(), /The artifact room/);
  assert.equal((await fetch(`${baseUrl}/../package.json`)).status, 404);
});

// @verifies scn.dashboard.0c4e91ab73f2
test("serves allowlisted end-to-end recordings with range support", async () => {
  const manifestResponse = await fetch(`${baseUrl}/api/recordings`);
  const { recordings } = await manifestResponse.json();
  assert.equal(manifestResponse.status, 200);
  assert.deepEqual(recordings[0], {
    id: "todo-workflow",
    title: "Todo workflow",
    description: "Recorded flow",
    tailnetUrl: "http://100.64.0.1:7780/todo.mp4",
    coveredScenarios: ["scn.todo.f98c1437a6d2"],
    streamUrl: "/api/recordings/todo-workflow"
  });

  const videoResponse = await fetch(`${baseUrl}/api/recordings/todo-workflow`, { headers: { range: "bytes=0-3" } });
  assert.equal(videoResponse.status, 206);
  assert.equal(videoResponse.headers.get("content-range"), "bytes 0-3/18");
  assert.equal(Buffer.from(await videoResponse.arrayBuffer()).toString(), "reco");
  assert.equal((await fetch(`${baseUrl}/api/recordings/not-allowlisted`)).status, 404);
});
