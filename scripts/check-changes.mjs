// Runs `stele check --change <change>` with the Stele built from this commit
// for every active change, and exits with the worst code. The current
// specifications are checked by `npm run verify:self` with the published
// Stele, so this gate covers only what the published Stele cannot: changes.
import { spawnSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";

const directory = "openspec/changes";
const changes = existsSync(directory)
  ? readdirSync(directory, { withFileTypes: true })
      .filter((entry) => entry.isDirectory() && entry.name !== "archive")
      .map((entry) => entry.name)
      .sort()
  : [];
if (changes.length === 0) {
  console.log("No active changes to check.");
}
let worst = 0;
for (const change of changes) {
  const result = spawnSync("./dist/stele", ["check", "--change", change, ...process.argv.slice(2)], {
    stdio: "inherit",
  });
  worst = Math.max(worst, result.status ?? 2);
}
process.exit(worst);
