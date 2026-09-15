import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import test from "node:test";

// @verifies scn.verify.29f07d86bc4e
test("full validation command and pinned OpenSpec dependency are documented", async () => {
  const packageJson = JSON.parse(await fs.readFile("package.json", "utf8"));
  assert.equal(packageJson.dependencies["@fission-ai/openspec"], "1.13.0");
  assert.equal(packageJson.bin.stele, "./bin/stele.mjs");
  assert.match(packageJson.scripts.validate, /bin\/stele\.mjs validate/);
  assert.match(packageJson.scripts["openspec:validate"], /--strict/);
  assert.match(await fs.readFile("README.md", "utf8"), /npm run validate/);
});
