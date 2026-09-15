import assert from "node:assert/strict";
import test from "node:test";
import { TodoStore, TodoNotFoundError, TodoValidationError } from "../src/todo-store.mjs";

function store() {
  return new TodoStore([
    { id: "todo-1", title: "Open", completed: false },
    { id: "todo-2", title: "Done", completed: true }
  ]);
}

// @verifies scn.todo.31ac09e7f482
test("filters tasks by completion state", () => {
  assert.deepEqual(store().list("open").map((todo) => todo.title), ["Open"]);
  assert.deepEqual(store().list("done").map((todo) => todo.title), ["Done"]);
  assert.equal(store().list("all").length, 2);
});

// @verifies scn.todo.f98c1437a6d2
test("creates a trimmed open task", () => {
  const todos = store();
  const created = todos.create("  Review evidence  ");
  assert.deepEqual(created, { id: "todo-3", title: "Review evidence", completed: false });
  assert.equal(todos.summary().open, 2);
});

// @verifies scn.todo.4bd8e1603ca9
test("rejects blank and overlong task text", () => {
  const todos = store();
  assert.throws(() => todos.create("   "), TodoValidationError);
  assert.throws(() => todos.create("x".repeat(121)), TodoValidationError);
  assert.equal(todos.summary().total, 2);
});

// @verifies scn.todo.678c20e4b91f
test("toggles a task and updates the summary", () => {
  const todos = store();
  assert.equal(todos.toggle("todo-1").completed, true);
  assert.deepEqual(todos.summary(), { total: 2, open: 0, done: 2 });
  assert.equal(todos.toggle("todo-1").completed, false);
});

// @verifies scn.todo.d23a76bf490e
test("removes only the selected task", () => {
  const todos = store();
  assert.equal(todos.remove("todo-1").title, "Open");
  assert.deepEqual(todos.list().map((todo) => todo.id), ["todo-2"]);
  assert.throws(() => todos.remove("missing"), TodoNotFoundError);
});
