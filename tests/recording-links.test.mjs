import assert from "node:assert/strict";
import test from "node:test";
import { recordingsForRequirement } from "../public/recording-links.js";

// @verifies scn.dashboard.0c4e91ab73f2
test("maps recordings to only their covered requirement scenarios", () => {
  const recordings = [
    { id: "todo", coveredScenarios: ["scn.todo.create", "scn.todo.toggle"] },
    { id: "dashboard", coveredScenarios: ["scn.dashboard.inspect"] }
  ];
  const requirement = { scenarios: [{ id: "scn.todo.create" }, { id: "scn.todo.delete" }] };

  assert.deepEqual(recordingsForRequirement(recordings, requirement), [
    { id: "todo", coveredScenarios: ["scn.todo.create", "scn.todo.toggle"], matchingScenarioIds: ["scn.todo.create"] }
  ]);
});
