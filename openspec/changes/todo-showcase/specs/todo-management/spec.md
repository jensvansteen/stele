## Purpose

Provide a small, understandable product surface for demonstrating requirement-to-evidence tracing from an OpenSpec change through working software.

## ADDED Requirements

### Requirement: List and filter tasks
Verification-ID: req.todo.7a92c1e8b304

The application SHALL show the current task collection and allow it to be filtered by all, open, or completed state.

#### Scenario: Initial task list
Verification-ID: scn.todo.82e61fc5d9a7

- **WHEN** the workspace opens
- **THEN** the current tasks and accurate remaining count are displayed

#### Scenario: Filter by completion
Verification-ID: scn.todo.31ac09e7f482

- **WHEN** the user selects All, Open, or Done
- **THEN** only tasks matching the selected filter are displayed

### Requirement: Create a task
Verification-ID: req.todo.c6148d2fa790

The application SHALL create a task from non-empty user text and reject empty input.

#### Scenario: Add a valid task
Verification-ID: scn.todo.f98c1437a6d2

- **WHEN** the user submits trimmed non-empty task text
- **THEN** a new open task appears and the input is cleared

#### Scenario: Reject an empty task
Verification-ID: scn.todo.4bd8e1603ca9

- **WHEN** the user submits blank or whitespace-only text
- **THEN** no task is created and a validation message is shown

### Requirement: Complete or reopen a task
Verification-ID: req.todo.0fd217bc9a63

The application SHALL let the user toggle a task between open and completed states.

#### Scenario: Toggle task state
Verification-ID: scn.todo.678c20e4b91f

- **WHEN** the user activates a task checkbox
- **THEN** that task changes state and the remaining count updates

### Requirement: Delete a task
Verification-ID: req.todo.f28a3e60c145

The application SHALL let the user permanently remove a selected task from the current local session.

#### Scenario: Remove selected task
Verification-ID: scn.todo.d23a76bf490e

- **WHEN** the user activates Delete on a task
- **THEN** that task is removed while other tasks remain unchanged
