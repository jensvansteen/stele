import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const ROOT = path.resolve(import.meta.dirname, "..");

test("packed package initializes a separate consumer project", async (context) => {
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
});
