import { createServer } from "node:http";
import { createReadStream, promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { TodoStore } from "./src/todo-store.mjs";
import { runVerification } from "./tools/verify.mjs";

const ROOT = path.dirname(fileURLToPath(import.meta.url));
const PUBLIC_ROOT = path.join(ROOT, "public");
const RECORDING_MANIFEST = path.join(ROOT, "artifacts", "e2e-recordings.json");
const DEFAULT_RECORDINGS_ROOT = path.join(os.homedir(), "recordings");
const ARTIFACT_PATHS = [
  "openspec/changes/todo-showcase/proposal.md",
  "openspec/changes/todo-showcase/design.md",
  "openspec/changes/todo-showcase/tasks.md",
  "openspec/changes/todo-showcase/specs/todo-management/spec.md",
  "openspec/changes/todo-showcase/specs/artifact-dashboard/spec.md",
  "openspec/changes/todo-showcase/specs/verification/spec.md",
  "artifacts/verification-report.json",
  "artifacts/test-results.json",
  "artifacts/e2e-recordings.json",
  "PLAN.md"
];

function json(response, status, body) {
  response.writeHead(status, { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" });
  response.end(JSON.stringify(body));
}

async function bodyJson(request) {
  let body = "";
  for await (const chunk of request) body += chunk;
  return body ? JSON.parse(body) : {};
}

// @implements req.dashboard.48d012ace679
async function readArtifact(relativePath) {
  if (!ARTIFACT_PATHS.includes(relativePath)) throw Object.assign(new Error("Artifact is not allowlisted."), { statusCode: 404 });
  const absolute = path.resolve(ROOT, relativePath);
  if (!absolute.startsWith(`${ROOT}${path.sep}`)) throw Object.assign(new Error("Artifact path escaped the project."), { statusCode: 400 });
  const content = await fs.readFile(absolute, "utf8").catch(() => "Artifact has not been generated yet. Run validation first.\n");
  return { path: relativePath, kind: path.extname(relativePath) === ".json" ? "json" : "markdown", content };
}

async function loadRecordingManifest(manifestPath = RECORDING_MANIFEST) {
  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
  return manifest.recordings ?? [];
}

function publicRecording(recording) {
  return {
    id: recording.id,
    title: recording.title,
    description: recording.description,
    tailnetUrl: recording.tailnetUrl,
    coveredScenarios: recording.coveredScenarios ?? [],
    streamUrl: `/api/recordings/${encodeURIComponent(recording.id)}`
  };
}

async function sendRecording(request, response, recording, recordingsRoot) {
  const absoluteRoot = path.resolve(recordingsRoot);
  const absolute = path.resolve(absoluteRoot, recording.relativePath);
  if (!absolute.startsWith(`${absoluteRoot}${path.sep}`)) throw Object.assign(new Error("Recording path escaped the recordings directory."), { statusCode: 400 });
  const stat = await fs.stat(absolute);
  const range = request.headers.range?.match(/^bytes=(\d+)-(\d*)$/);
  if (!range) {
    response.writeHead(200, { "content-type": "video/mp4", "content-length": stat.size, "accept-ranges": "bytes", "cache-control": "private, max-age=3600" });
    return createReadStream(absolute).pipe(response);
  }
  const start = Number(range[1]);
  const end = range[2] ? Math.min(Number(range[2]), stat.size - 1) : stat.size - 1;
  if (start >= stat.size || end < start) {
    response.writeHead(416, { "content-range": `bytes */${stat.size}` });
    return response.end();
  }
  response.writeHead(206, {
    "content-type": "video/mp4",
    "content-length": end - start + 1,
    "content-range": `bytes ${start}-${end}/${stat.size}`,
    "accept-ranges": "bytes",
    "cache-control": "private, max-age=3600"
  });
  createReadStream(absolute, { start, end }).pipe(response);
}

export async function startServer({ port = Number(process.env.PORT ?? 4173), seed, recordingsRoot = DEFAULT_RECORDINGS_ROOT, recordingManifestPath = RECORDING_MANIFEST } = {}) {
  const store = new TodoStore(seed);
  const server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url, "http://localhost");

      if (url.pathname === "/api/todos" && request.method === "GET") {
        return json(response, 200, { items: store.list(url.searchParams.get("filter") ?? "all"), summary: store.summary() });
      }
      if (url.pathname === "/api/todos" && request.method === "POST") {
        const todo = store.create((await bodyJson(request)).title);
        return json(response, 201, { todo, summary: store.summary() });
      }
      const todoMatch = url.pathname.match(/^\/api\/todos\/([^/]+)$/);
      if (todoMatch && request.method === "PATCH") {
        const todo = store.toggle(decodeURIComponent(todoMatch[1]));
        return json(response, 200, { todo, summary: store.summary() });
      }
      if (todoMatch && request.method === "DELETE") {
        const todo = store.remove(decodeURIComponent(todoMatch[1]));
        return json(response, 200, { todo, summary: store.summary() });
      }
      if (url.pathname === "/api/verification" && request.method === "GET") {
        const report = await runVerification({ mode: "implementation" });
        return json(response, 200, report);
      }
      if (url.pathname === "/api/verify" && request.method === "POST") {
        const report = await runVerification({ mode: "implementation", reportPath: "artifacts/verification-report.json" });
        return json(response, report.verdict === "pass" ? 200 : 422, report);
      }
      if (url.pathname === "/api/artifacts" && request.method === "GET") {
        const artifacts = await Promise.all(ARTIFACT_PATHS.map(readArtifact));
        return json(response, 200, { artifacts });
      }
      if (url.pathname === "/api/recordings" && request.method === "GET") {
        const recordings = await loadRecordingManifest(recordingManifestPath);
        return json(response, 200, { recordings: recordings.map(publicRecording) });
      }
      const recordingMatch = url.pathname.match(/^\/api\/recordings\/([^/]+)$/);
      if (recordingMatch && request.method === "GET") {
        const recordings = await loadRecordingManifest(recordingManifestPath);
        const recording = recordings.find((item) => item.id === decodeURIComponent(recordingMatch[1]));
        if (!recording) return json(response, 404, { error: "Recording not found." });
        return await sendRecording(request, response, recording, recordingsRoot);
      }

      const requested = url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      const absolute = path.resolve(PUBLIC_ROOT, requested);
      if (!absolute.startsWith(`${PUBLIC_ROOT}${path.sep}`)) return json(response, 404, { error: "Not found." });
      const content = await fs.readFile(absolute);
      const extension = path.extname(absolute);
      const contentTypes = { ".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".svg": "image/svg+xml" };
      response.writeHead(200, { "content-type": contentTypes[extension] ?? "application/octet-stream" });
      response.end(content);
    } catch (error) {
      const status = error.statusCode ?? (error instanceof SyntaxError ? 400 : error.code === "ENOENT" ? 404 : 500);
      json(response, status, { error: error.message });
    }
  });

  await new Promise((resolve, reject) => {
    const onError = (error) => reject(error);
    server.once("error", onError);
    server.listen(port, "127.0.0.1", () => {
      server.off("error", onError);
      resolve();
    });
  });
  return { server, port: server.address().port, store };
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  const { port } = await startServer();
  console.log(`OpenSpec showcase running at http://localhost:${port}`);
}
