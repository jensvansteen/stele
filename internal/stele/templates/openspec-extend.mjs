// Extends a project's OpenSpec setup for Stele. Stele runs this script with Node
// and resolves `yaml` from its bundled OpenSpec, so files are edited with the
// same parser OpenSpec uses and comments survive.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

const input = JSON.parse(process.env.STELE_OPENSPEC_EXTEND ?? "{}");
const YAML = createRequire(path.join(input.openspecPackage, "package.json"))("yaml");

const APPLY_MARKER = "Stele apply steps:";
const VERIFICATION_INSTRUCTION = `Plan how Stele verifies this change:
1. Run \`stele ids --change <change>\` so every requirement and scenario has a Verification-ID.
2. Propose evidence for every scenario: one or more levels with a rationale, and an advisory placement based on the project's AGENTS.md, CLAUDE.md, skills, and existing layout. Follow the stele-plan skill.
3. Write this change's linkage-plan.json as the stele-plan skill describes, with every entry unapproved.
4. Run \`stele verify --stage proposal --change <change>\` and fix what it reports, as the stele-plan skill describes.
5. Finish with: "Plan ready. Ask me to apply the change; I'll show the levels to confirm first."
`;
const APPLY_STEPS = `${APPLY_MARKER}
1. Before the first task, confirm the verification levels with the user as the approval step in the stele-plan skill describes.
2. Add @implements and @verifies anchors while writing code and tests, as the stele-verify skill describes.
3. Run \`stele validate --change <change>\` and fix failures before reporting the change done.
`;
const APPLY_GUIDANCE = [
  "Stele: before the first task, confirm the verification levels with the user as the approval step in the stele-plan skill describes.",
  "Stele: add @implements and @verifies anchors while writing code and tests, as the stele-verify skill describes.",
  "Stele: run `stele validate --change <change>` and fix failures before reporting the change done.",
];
const ARCHIVE_REPAIR =
  "Stele: after archiving, run `stele annotate --specs --targets-from <archive-dir>` with the directory the archive created, then `stele validate --specs`.";
const ARCHIVE_GUIDANCE = ["Stele: `stele validate --change <change>` must pass before archiving.", ARCHIVE_REPAIR];
// Guidance entries of earlier Stele versions and the entries that replace them.
const REPLACED_ARCHIVE_GUIDANCE = new Map([
  ["Stele: after archiving, run `stele validate --specs`.", ARCHIVE_REPAIR],
  ["Stele: after archiving, run `stele annotate --specs`, then `stele validate --specs`.", ARCHIVE_REPAIR],
]);
const SPEC_ANNOTATION = "<!-- stele: spec v1 -->";
const SPEC_ANNOTATION_TRIGGER = /^\uFEFF?[ \t]*<!--[ \t]*stele:/u;
const VERIFICATION_RULES = [
  "Stele: follow the stele-plan skill for levels, rationale, advisory placement, and the plan format.",
];
const PLAN_TEMPLATE = `${JSON.stringify(
  { schemaVersion: 2, changeId: "<change>", scenarios: {} },
  null,
  2,
)}\n`;

const writes = [];
const notes = [];

function parse(file) {
  const doc = YAML.parseDocument(readFileSync(file, "utf8"));
  if (doc.errors.length > 0) {
    throw new Error(`${file}: ${doc.errors[0].message}`);
  }
  return doc;
}

function scalarValues(seq) {
  return seq.items.map((item) => (YAML.isScalar(item) ? item.value : item));
}

// appendMissing adds each entry that the sequence at keyPath lacks and reports
// whether anything changed.
function appendMissing(doc, keyPath, entries) {
  let seq = doc.getIn(keyPath, true);
  if (seq === undefined || seq === null || (YAML.isScalar(seq) && seq.value === null)) {
    doc.setIn(keyPath, doc.createNode([]));
    seq = doc.getIn(keyPath, true);
  }
  if (!YAML.isSeq(seq)) {
    notes.push(`openspec/config.yaml: ${keyPath.join(".")} is not a list; Stele left it unchanged.`);
    return false;
  }
  const existing = new Set(scalarValues(seq));
  let changed = false;
  for (const entry of entries) {
    if (!existing.has(entry)) {
      seq.items.push(doc.createNode(entry));
      changed = true;
    }
  }
  return changed;
}

// replaceEntries replaces older entries of the sequence at keyPath in place,
// or removes them when their replacement is already present, and reports
// whether anything changed.
function replaceEntries(doc, keyPath, replacements) {
  const seq = doc.getIn(keyPath, true);
  if (!YAML.isSeq(seq)) {
    return false;
  }
  let changed = false;
  for (const [older, newer] of replacements) {
    const index = scalarValues(seq).indexOf(older);
    if (index < 0) {
      continue;
    }
    if (scalarValues(seq).includes(newer)) {
      seq.items.splice(index, 1);
    } else {
      seq.items[index] = doc.createNode(newer);
    }
    changed = true;
  }
  return changed;
}

// annotateSpecTemplate starts the schema's specification template with the
// Stele annotation, unless it already starts with one.
function annotateSpecTemplate(schemaDir) {
  const file = path.join(schemaDir, "templates", "spec.md");
  let content;
  try {
    content = readFileSync(file, "utf8");
  } catch {
    notes.push("The stele schema has no templates/spec.md; Stele did not annotate the specification template.");
    return;
  }
  if (!SPEC_ANNOTATION_TRIGGER.test(content)) {
    writes.push([file, `${SPEC_ANNOTATION}\n${content}`]);
  }
}

function patchSchema(schemaDir) {
  const file = path.join(schemaDir, "schema.yaml");
  const doc = parse(file);
  doc.commentBefore =
    ` Stele workflow schema, forked from OpenSpec ${input.openspecVersion} spec-driven by stele ${input.steleVersion}.` +
    "\n Refresh it with `stele init --refresh-schema` after changing the OpenSpec version.";
  doc.set("name", "stele");
  doc.set("description", "Stele workflow - proposal → specs → design → verification → tasks");
  const artifacts = doc.get("artifacts");
  const ids = artifacts.items.map((item) => item.get("id"));
  if (!ids.includes("verification")) {
    const tasksIndex = ids.indexOf("tasks");
    const verification = doc.createNode({
      id: "verification",
      generates: "linkage-plan.json",
      description: "Stele verification plan: evidence per scenario, awaiting approval",
      template: "linkage-plan.json",
      instruction: VERIFICATION_INSTRUCTION,
      requires: ["specs", "design"],
    });
    artifacts.items.splice(tasksIndex < 0 ? artifacts.items.length : tasksIndex, 0, verification);
  }
  const tasks = artifacts.items.find((item) => item.get("id") === "tasks");
  if (tasks) {
    appendMissing(doc, ["artifacts", artifacts.items.indexOf(tasks), "requires"], ["verification"]);
  }
  if (!doc.get("apply")) {
    doc.set("apply", doc.createNode({ requires: ["tasks"], tracks: "tasks.md" }));
  }
  appendMissing(doc, ["apply", "requires"], ["verification"]);
  const instruction = String(doc.getIn(["apply", "instruction"]) ?? "");
  if (!instruction.includes(APPLY_MARKER)) {
    doc.setIn(["apply", "instruction"], `${instruction.trimEnd()}\n\n${APPLY_STEPS}`.trimStart());
  }
  writes.push([file, doc.toString()]);
  writes.push([path.join(schemaDir, "templates", "linkage-plan.json"), PLAN_TEMPLATE]);
  annotateSpecTemplate(schemaDir);
}

function mergeConfig(file) {
  const doc = parse(file);
  let changed = false;
  const schema = doc.get("schema");
  if (schema === "spec-driven") {
    doc.set("schema", "stele");
    changed = true;
  } else if (schema !== "stele") {
    notes.push(
      `openspec/config.yaml selects the "${schema}" schema; Stele kept it. Set schema: stele to plan new changes with the Stele workflow.`,
    );
  }
  changed = appendMissing(doc, ["operations", "apply", "guidance"], APPLY_GUIDANCE) || changed;
  changed = replaceEntries(doc, ["operations", "archive", "guidance"], REPLACED_ARCHIVE_GUIDANCE) || changed;
  changed = appendMissing(doc, ["operations", "archive", "guidance"], ARCHIVE_GUIDANCE) || changed;
  changed = appendMissing(doc, ["rules", "verification"], VERIFICATION_RULES) || changed;
  if (changed) {
    writes.push([file, doc.toString()]);
  }
  return changed;
}

try {
  const openspecDir = path.join(input.root, "openspec");
  if (input.patchSchema) {
    patchSchema(path.join(openspecDir, "schemas", "stele"));
  }
  const configChanged = mergeConfig(path.join(openspecDir, "config.yaml"));
  for (const [file, content] of writes) {
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, content);
  }
  process.stdout.write(`${JSON.stringify({ configChanged, notes })}\n`);
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
