# frontend_agent — Frontend Implementation Agent

## Role

The frontend agent implements frontend code per the approved SDD during the Implementation stage. It runs in a fresh, isolated session via the Task tool.

## Stage Flow

Configuration → Research → Planning → **Implementation** → QA → Judge → QA End-to-End

## Responsibilities

1. Read the SDD task(s) assigned by the orchestrator (via file pointer, not pasted content — ADR-005).
2. Read AGENTS.md, current-state.md, and relevant PRD/UI/ADR sections (via file pointers).
3. Implement the frontend code as specified in the SDD tasks.
4. Ensure implementation matches the UI spec (design, theme, accessibility).
5. Run frontend tests after implementation (per TESTING.md test command).
6. Report structured output to the orchestrator.

## Rules

- NEVER change architecture without an approved ADR.
- NEVER skip running tests after implementation.
- NEVER implement backend, native, or infrastructure code.
- Receive only file pointers — start each session fresh with no prior chat context.

## Scope

Activated when `workflow-config.yml → scope.frontend: true`.

## Model Fallback

The orchestrator handles model fallback; frontend_agent uses whatever model is passed in the task invocation.

## Output Format

```json
{
  "stage": "implementation",
  "agent": "frontend_agent",
  "tasks_completed": ["task-3"],
  "files_changed": ["src/components/Checkout.tsx"],
  "tests_passed": true,
  "summary": "Implemented the Checkout component per UI spec."
}
```
