const DEFAULT_TODOS = [
  { id: "todo-1", title: "Plan tomorrow’s priorities", completed: true },
  { id: "todo-2", title: "Send the project update", completed: false },
  { id: "todo-3", title: "Pick up groceries", completed: false }
];

export class TodoStore {
  constructor(seed = DEFAULT_TODOS) {
    this.todos = seed.map((todo) => ({ ...todo }));
    this.nextId = this.todos.reduce((max, todo) => {
      const value = Number(todo.id.split("-").at(-1));
      return Number.isFinite(value) ? Math.max(max, value + 1) : max;
    }, 1);
  }

  // @implements req.todo.7a92c1e8b304
  list(filter = "all") {
    if (filter === "open") return this.todos.filter((todo) => !todo.completed);
    if (filter === "done") return this.todos.filter((todo) => todo.completed);
    return this.todos.map((todo) => ({ ...todo }));
  }

  // @implements req.todo.c6148d2fa790
  create(rawTitle) {
    const title = String(rawTitle ?? "").trim();
    if (!title) throw new TodoValidationError("Give the task a short, useful name.");
    if (title.length > 120) throw new TodoValidationError("Keep tasks to 120 characters or fewer.");

    const todo = { id: `todo-${this.nextId++}`, title, completed: false };
    this.todos.unshift(todo);
    return { ...todo };
  }

  // @implements req.todo.0fd217bc9a63
  toggle(id) {
    const todo = this.#find(id);
    todo.completed = !todo.completed;
    return { ...todo };
  }

  // @implements req.todo.f28a3e60c145
  remove(id) {
    const index = this.todos.findIndex((todo) => todo.id === id);
    if (index === -1) throw new TodoNotFoundError(id);
    const [removed] = this.todos.splice(index, 1);
    return { ...removed };
  }

  summary() {
    return {
      total: this.todos.length,
      open: this.todos.filter((todo) => !todo.completed).length,
      done: this.todos.filter((todo) => todo.completed).length
    };
  }

  #find(id) {
    const todo = this.todos.find((candidate) => candidate.id === id);
    if (!todo) throw new TodoNotFoundError(id);
    return todo;
  }
}

export class TodoValidationError extends Error {
  statusCode = 400;
}

export class TodoNotFoundError extends Error {
  statusCode = 404;

  constructor(id) {
    super(`Task ${id} was not found.`);
  }
}
