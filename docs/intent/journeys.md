# User journeys

## Journey: Plan a focused workday

**Persona:** Product engineer evaluating the showcase
**Goal:** Capture and manage a short Todo list

**Steps:**
1. Run `npm run dev` and open `http://localhost:4173`.
2. Enter “Review verification report” and press **Add task**.
3. Mark the task complete, switch between **All**, **Open**, and **Done**, then delete it.

**Expected outcome:** The UI and API stay in sync and each action gives immediate, accessible feedback.
**Claims this journey exercises:** `req.todo.7a92c1e8b304`, `req.todo.c6148d2fa790`, `req.todo.0fd217bc9a63`, `req.todo.f28a3e60c145`

## Journey: Review delivery artifacts

**Persona:** Engineering lead reviewing a proposed delivery
**Goal:** Understand what was planned and what evidence exists

**Steps:**
1. Open **Verification** in the running app.
2. Inspect the stage summary and requirement matrix.
3. Select `proposal.md`, `design.md`, `tasks.md`, or a delta spec in the artifact explorer.

**Expected outcome:** The reviewer can move from a requirement to its code/test links and inspect the source artifacts without leaving the dashboard.
**Claims this journey exercises:** `req.dashboard.9f5bca7132d8`, `req.dashboard.2e6d391a0b47`

## Journey: Re-run the guardrail

**Persona:** Engineer preparing an implementation for review
**Goal:** Confirm the current working tree has valid IDs and anchors

**Steps:**
1. Run `npm run validate` in a terminal, or click **Run validation** in the dashboard.
2. Wait for the validation steps to finish.
3. Inspect diagnostics and the requirement/scenario evidence states.

**Expected outcome:** The check reports proposal shape, real implementation links, and latest test execution separately, with failures surfaced as actionable diagnostics.
**Claims this journey exercises:** `req.verify.a18c03ef72b6`, `req.verify.b6e8f421cd09`, `req.verify.d3975ac8e142`, `req.dashboard.e14fc08b6739`

## Journey: Reproduce the showcase

**Persona:** New contributor
**Goal:** Establish that the repository works from a clean checkout

**Steps:**
1. Run `npm install`.
2. Run `npm run openspec:validate` and `npm run validate`.
3. Run `npm run dev` and exercise the two browser journeys above.

**Expected outcome:** OpenSpec validation, automated tests, and stable-ID verification pass before the UI is reviewed.
**Claims this journey exercises:** `req.verify.6b2d7904ea51`
