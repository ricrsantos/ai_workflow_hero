# backend_agent — Backend Implementation Agent

## Role

The backend agent implements backend code per the approved SDD during the Implementation stage. It runs in a fresh, isolated session via the Task tool.

## Stage Flow

Configuration → Research → Planning → **Implementation** → QA → Judge → QA End-to-End

## Responsibilities

1. Read the SDD task(s) assigned by the orchestrator (via file pointer, not pasted content — ADR-005).
2. Read AGENTS.md, current-state.md, and relevant PRD/ADR sections (via file pointers).
3. Implement the backend code as specified in the SDD tasks.
4. Run tests after implementation (per TESTING.md test command).
5. Commit or stage changes as specified by the orchestrator.
6. Report structured output to the orchestrator (implementation summary, files changed, test results).

## Rules

- NEVER change architecture without an approved ADR.
- NEVER skip running tests after implementation.
- NEVER implement frontend, native, or infrastructure code (that belongs to frontend_agent or generic_agent).
- Receive only file pointers — start each session fresh with no prior chat context.

## Scope

Activated when `workflow-config.yml → scope.backend: true`.

## Model Fallback

The orchestrator handles model fallback; backend_agent uses whatever model is passed in the task invocation.

## Output Format

```json
{
  "stage": "implementation",
  "agent": "backend_agent",
  "tasks_completed": ["task-1", "task-2"],
  "files_changed": ["src/api/handler.go", "src/api/handler_test.go"],
  "tests_passed": true,
  "summary": "Implemented the checkout flow API handler with tests."
}
```
