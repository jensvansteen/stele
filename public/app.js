import { recordingsForRequirement } from "./recording-links.js";

const state = {
  filter: "all",
  todos: [],
  summary: { total: 0, open: 0, done: 0 },
  report: null,
  artifacts: [],
  recordings: []
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: { "content-type": "application/json", ...options.headers }
  });
  const body = await response.json();
  if (!response.ok) throw Object.assign(new Error(body.error ?? "Request failed."), { body, status: response.status });
  return body;
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#039;", '"': "&quot;" })[character]);
}

function showToast(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(showToast.timeout);
  showToast.timeout = setTimeout(() => toast.classList.remove("show"), 2800);
}

function switchView(view) {
  document.body.dataset.view = view;
  $$(".view").forEach((item) => item.classList.toggle("active", item.id === `${view}-view`));
  $$(".nav-item").forEach((item) => {
    const active = item.dataset.view === view;
    item.classList.toggle("active", active);
    if (active) item.setAttribute("aria-current", "page");
    else item.removeAttribute("aria-current");
  });
  history.replaceState(null, "", `#${view}`);
  if (view === "verification" && !state.report) loadVerification();
}

async function loadTodos() {
  const body = await api(`/api/todos?filter=${state.filter}`);
  state.todos = body.items;
  state.summary = body.summary;
  renderTodos();
}

function renderTodos() {
  $("#open-count").textContent = state.summary.open;
  $("#all-badge").textContent = state.summary.total;
  const list = $("#todo-list");
  list.innerHTML = state.todos.map((todo) => `
    <li class="todo-item ${todo.completed ? "done" : ""}" data-id="${escapeHtml(todo.id)}">
      <input class="todo-check" type="checkbox" ${todo.completed ? "checked" : ""} aria-label="Mark ${escapeHtml(todo.title)} ${todo.completed ? "open" : "complete"}" />
      <span class="todo-title">${escapeHtml(todo.title)}</span>
      <button class="delete-button" type="button" aria-label="Delete ${escapeHtml(todo.title)}">Delete</button>
    </li>
  `).join("");
  $("#empty-state").hidden = state.todos.length !== 0;
}

async function submitTodo(event) {
  event.preventDefault();
  const input = $("#todo-input");
  $("#form-message").textContent = "";
  try {
    await api("/api/todos", { method: "POST", body: JSON.stringify({ title: input.value }) });
    input.value = "";
    await loadTodos();
    showToast("Task added to this session.");
  } catch (error) {
    $("#form-message").textContent = error.message;
    input.focus();
  }
}

async function handleTodoClick(event) {
  const row = event.target.closest(".todo-item");
  if (!row) return;
  if (event.target.matches(".todo-check")) {
    await api(`/api/todos/${encodeURIComponent(row.dataset.id)}`, { method: "PATCH", body: "{}" });
    await loadTodos();
  }
  if (event.target.matches(".delete-button")) {
    await api(`/api/todos/${encodeURIComponent(row.dataset.id)}`, { method: "DELETE" });
    await loadTodos();
    showToast("Task removed.");
  }
}

async function loadVerification() {
  try {
    const [report, artifactBody, recordingBody] = await Promise.all([api("/api/verification"), api("/api/artifacts"), api("/api/recordings")]);
    state.report = report;
    state.artifacts = artifactBody.artifacts;
    state.recordings = recordingBody.recordings;
    renderDashboard();
  } catch (error) {
    showToast(`Could not load verification: ${error.message}`);
  }
}

// @implements req.dashboard.9f5bca7132d8
function renderSummary() {
  const stages = [
    ["01", "Proposal", state.report.stages.proposal.status, "Requirement shape & identity"],
    ["02", "Linkage", state.report.stages.linkage.status, "Code & test anchors"],
    ["03", "Execution", state.report.stages.execution.status, "Revision-bound test result"],
    ["04", "Review", state.report.stages.review.status, "Human decision & scope"]
  ];
  $("#stage-grid").innerHTML = stages.map(([number, title, status, description]) => `
    <article class="stage-card">
      <span class="stage-number">${number}</span>
      <span class="stage-status ${escapeHtml(status)}">${escapeHtml(status)}</span>
      <h3>${title}</h3>
      <p>${description}</p>
    </article>
  `).join("");
  $("#nav-status").className = `nav-status ${state.report.verdict}`;
}

// @implements req.dashboard.2e6d391a0b47
function renderRequirements() {
  $("#matrix-count").textContent = `${state.report.summary.requirements} req · ${state.report.summary.scenarios} scn`;
  $("#requirement-list").innerHTML = state.report.requirements.map((requirement, index) => {
    const namespace = requirement.id?.split(".")[1] ?? "verify";
    const codeTarget = requirement.codeLinks[0]?.path ? `${requirement.codeLinks[0].path}:${requirement.codeLinks[0].line}` : requirement.codeLinks[0]?.target ?? "No code link";
    return `
      <article class="requirement-row ${index === 0 ? "open" : ""}">
        <button class="requirement-summary" type="button" aria-expanded="${index === 0}">
          <span class="cap-dot ${namespace}" aria-hidden="true"></span>
          <span class="requirement-name"><strong>${escapeHtml(requirement.title)}</strong><code>${escapeHtml(requirement.id)}</code></span>
          <span class="link-count">${requirement.scenarios.length} scenario${requirement.scenarios.length === 1 ? "" : "s"}</span>
          <span class="chevron" aria-hidden="true">⌄</span>
        </button>
        <div class="requirement-detail">
          <div class="anchor-line">@implements → ${escapeHtml(codeTarget)}</div>
          ${requirement.scenarios.map((scenario) => {
            const testTarget = scenario.testLinks[0]?.path ? `${scenario.testLinks[0].path}:${scenario.testLinks[0].line}` : scenario.testLinks[0]?.target ?? "No test link";
            return `
              <div class="scenario">
                <div><strong>${escapeHtml(scenario.title)}</strong><code>${escapeHtml(scenario.id)}</code></div>
                <div class="scenario-evidence">
                  <span class="status-pill ${escapeHtml(scenario.linkage)}">${escapeHtml(scenario.linkage)}</span>
                  <span class="status-pill ${escapeHtml(scenario.execution.outcome)}">${escapeHtml(scenario.execution.outcome)}</span>
                  <small>${escapeHtml(testTarget)}</small>
                </div>
              </div>`;
          }).join("")}
          ${renderRecordings(requirement)}
        </div>
      </article>`;
  }).join("");
}

// @implements req.dashboard.48d012ace679
function renderArtifacts() {
  const label = (artifact) => {
    const segments = artifact.path.split("/");
    const file = segments.at(-1);
    if (file === "spec.md") return segments.at(-2);
    return file.replace(".md", "").replace(".json", "");
  };
  $("#artifact-tabs").innerHTML = state.artifacts.map((artifact, index) => `<button class="artifact-tab ${index === 0 ? "active" : ""}" id="artifact-tab-${index}" type="button" role="tab" aria-controls="artifact-content" aria-selected="${index === 0}" tabindex="${index === 0 ? 0 : -1}" data-index="${index}">${escapeHtml(label(artifact))}</button>`).join("");
  selectArtifact(0);
}

function selectArtifact(index) {
  const artifact = state.artifacts[index];
  if (!artifact) return;
  $$(".artifact-tab").forEach((tab) => {
    const active = Number(tab.dataset.index) === index;
    tab.classList.toggle("active", active);
    tab.setAttribute("aria-selected", String(active));
    tab.tabIndex = active ? 0 : -1;
  });
  $("#artifact-content").setAttribute("aria-labelledby", `artifact-tab-${index}`);
  $("#artifact-path").textContent = artifact.path;
  $("#artifact-content").textContent = artifact.content;
}

// @implements req.dashboard.3d8a2f7c91e4
function renderRecordings(requirement) {
  const recordings = recordingsForRequirement(state.recordings, requirement);
  if (!recordings.length) return "";
  return `
    <section class="requirement-recordings" aria-label="End-to-end recording evidence">
      <div class="evidence-label"><span>▶</span> End-to-end browser evidence</div>
      ${recordings.map((recording) => `
      <article class="inline-recording">
      <div class="recording-frame">
        <video controls playsinline preload="metadata" aria-label="${escapeHtml(recording.title)} recording">
          <source src="${escapeHtml(recording.streamUrl)}" type="video/mp4" />
          Your browser does not support embedded video.
        </video>
      </div>
      <div class="recording-copy">
        <div>
          <h3>${escapeHtml(recording.title)}</h3>
          <p>${escapeHtml(recording.description)}</p>
        </div>
        <div class="recording-meta">
          <span>${recording.matchingScenarioIds.length} covered here</span>
          <a href="${escapeHtml(recording.tailnetUrl)}" target="_blank" rel="noreferrer">Direct tailnet file ↗</a>
        </div>
        <div class="recording-scenarios">${recording.matchingScenarioIds.map((id) => `<code>${escapeHtml(id)}</code>`).join("")}</div>
      </div>
      </article>`).join("")}
    </section>`;
}

function renderDashboard() {
  renderSummary();
  renderRequirements();
  renderArtifacts();
}

// @implements req.dashboard.e14fc08b6739
async function runValidation() {
  const button = $("#run-validation");
  const progress = $("#validation-progress");
  button.disabled = true;
  progress.hidden = false;
  try {
    state.report = await api("/api/verify", { method: "POST", body: "{}" });
    renderSummary();
    renderRequirements();
    const artifactBody = await api("/api/artifacts");
    state.artifacts = artifactBody.artifacts;
    renderArtifacts();
    showToast(`Validation ${state.report.verdict}: ${state.report.summary.errors} errors.`);
  } catch (error) {
    if (error.body?.requirements) {
      state.report = error.body;
      renderSummary();
      renderRequirements();
    }
    showToast(`Validation failed: ${error.message}`);
  } finally {
    button.disabled = false;
    progress.hidden = true;
  }
}

$("#today-label").textContent = new Intl.DateTimeFormat("en", { weekday: "long", month: "long", day: "numeric" }).format(new Date());
$("#todo-form").addEventListener("submit", submitTodo);
$("#todo-list").addEventListener("click", handleTodoClick);
$("#run-validation").addEventListener("click", runValidation);
$("#requirement-list").addEventListener("click", (event) => {
  const button = event.target.closest(".requirement-summary");
  if (!button) return;
  const row = button.closest(".requirement-row");
  row.classList.toggle("open");
  button.setAttribute("aria-expanded", row.classList.contains("open"));
});
$("#artifact-tabs").addEventListener("click", (event) => {
  const tab = event.target.closest(".artifact-tab");
  if (tab) selectArtifact(Number(tab.dataset.index));
});
$("#artifact-tabs").addEventListener("keydown", (event) => {
  if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
  const tabs = $$(".artifact-tab");
  if (!tabs.length) return;
  event.preventDefault();
  const current = tabs.indexOf(document.activeElement);
  const index = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : (current + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length;
  selectArtifact(Number(tabs[index].dataset.index));
  tabs[index].focus();
});
$$('[data-view]').forEach((button) => button.addEventListener("click", () => switchView(button.dataset.view)));
$$('.filter').forEach((button) => button.addEventListener("click", async () => {
  state.filter = button.dataset.filter;
  $$('.filter').forEach((item) => {
    const active = item === button;
    item.classList.toggle("active", active);
    item.setAttribute("aria-pressed", String(active));
  });
  await loadTodos();
}));

switchView(location.hash === "#verification" ? "verification" : "product");
loadTodos().catch((error) => showToast(error.message));
