# PRD-C08-001 — TUI-Direct Stage Execute

> Cycle C08 product requirements. Hero TUI Executes named stage agents on their YAML harness+model pair instead of waiting on orchestrator Task. Nested Task fan-out stays. Cursor IDE Runtime is unchanged.

## 1. Summary

When the orchestration agent starts an enabled stage in the Hero TUI, the TUI owns Execute for that stage’s named agent(s) on the pair in `workflow-config.yml`. Chat streams that Execute live with `[LABEL - {model} · {harness}]`. Nested Task children remain Task sessions inside the parent agent’s harness. The sidebar `agents: N` row counts Execute parents plus every open Task, including generic nested Tasks (`TASK`).

## 2. User and problem

A mixed-harness cycle (for example Codex Implementation and OpenCode Judge) currently keeps Chat on the orchestrator Execute. Nested Task work often does not stream, so the user only sees `Waiting for harness…` and a 4-letter chip without the child’s model and harness.

## 3. Goals

- Honor each named stage agent’s YAML `harness` + `model` in TUI Execute (PRD-C04).
- Stream that agent’s live thinking, tools, and text in Chat.
- Keep ADR-005 nested Task fan-out (including Implementation parallelism inside an agent).
- Show launch lines and navbar chips for named agents and generic nested Tasks.
- Run in-scope Implementation agents in parallel as separate TUI Executes.

## 4. Scope

### 4.1 TUI-direct named stage agents

After the orchestrator calls `hero stage start --name <stage>` it **stops**. It must not Task-dispatch `planning_agent`, `backend_agent`, `frontend_agent`, `generic_agent`, `qa_agent`, `judge_agent`, `browser_ui_agent`, or `end2end_qa_agent`. Research remains the existing `discover_agent` handoff.

The TUI then Executes:

| Stage | Agent(s) |
|---|---|
| Research | `discover_agent` (unchanged, interactive) |
| Planning | `planning_agent` |
| Implementation | In-scope: `backend_agent`, `frontend_agent`, `generic_agent` (parallel wave) |
| QA | `qa_agent` |
| Judge | `judge_agent` |
| Browser UI Validation | `browser_ui_agent` |
| QA End-to-End | `end2end_qa_agent` |

Each Execute uses that agent’s YAML pair (then `fallback_model`). After non-research agents finish, the TUI resumes the orchestrator with their Output Format so it can close the stage and start the next one.

### 4.2 Nested Task

Named stage agents still launch nested Task children (generic fan-out and named targets such as `context_agent`) inside their own harness. The TUI does not Execute those children as separate adapter sessions in v1. Chat shows nested activity when the parent harness forwards it; otherwise launch chip plus `result.content` on complete.

### 4.3 Implementation parallelism (v1)

The TUI starts one Execute per **in-scope** implementation agent in a single wave (`scope.backend` → BACK, `scope.frontend` → FRNT, native/script/infrastructure → GEN). Cross-agent SDD series ordering is out of scope. Each agent still serializes or fans out its own nested Tasks.

### 4.4 Navbar and Chat

- Named Hero agents keep 4-letter codes (`ORCH`, `PLAN`, `BACK`, `FRNT`, `GEN`, `QA`, `JUDG`, `CTX`, …).
- Generic nested Tasks chip as `TASK` (never `HARN`).
- `agents: N` includes every live Execute parent and every open Task call.
- Task **started** is a transcript line with `[LABEL - {model} · {harness}]`.
- `Waiting for harness…` stays while **any** Execute is live, under the active child block.

### 4.5 Concurrency

Chat may run several adapter Executes at once. Streaming stays true until the last child finishes. Esc cancels all in-flight Executes. Completing one child must not wipe sibling bubbles or their live-agent chips.

### 4.6 Non-goals

- Cursor IDE Runtime still uses Task from the orchestrator (ADR-031).
- TUI-direct Execute of nested generic Tasks.
- Parsing `tasks.md` for cross-agent series.
- Changing harness adapter transport contracts beyond Task attribution fields.

## 5. Acceptance criteria

1. After orchestrator `/hero-start` leaves Planning running, the next Execute uses `planning_agent`’s pair, not ORCH.
2. Implementation with backend+frontend scope starts two concurrent Executes; both headers show model and harness; navbar count ≥ 2.
3. Nested generic Task started increments the count with `TASK` and must not show `HARN` for that chip.
4. Nested `context_agent` chips `CTX`, not `TASK`.
5. First child `executeDone` does not clear a sibling stream.
6. Codex and OpenCode Task start events carry `CallID` plus agent or generic name.
7. Cursor IDE workflow is unchanged.
8. `go test ./...` remains green.
