# ADR-C08-001 — TUI-Direct Stage Execute

> Cycle C08 architecture decisions. Product: [PRD-C08-001](../product/PRD-C08-001-tui-stage-execute.md). UI: [UI-C08-001](../product/UI-C08-001-tui-stage-execute.md).

| # | Title | Status |
|---|---|---|
| [ADR-054](#adr-054-tui-executes-named-stage-agents-on-their-yaml-pair) | TUI Executes named stage agents on their YAML pair | Accepted |
| [ADR-055](#adr-055-implementation-scope-agents-run-as-concurrent-tui-executes) | Implementation scope agents run as concurrent TUI Executes | Accepted |
| [ADR-056](#adr-056-nested-task-stays-in-the-parent-harness-generic-tasks-chip-task) | Nested Task stays in the parent harness; generic Tasks chip TASK | Accepted |
| [ADR-057](#adr-057-chat-multiplexes-tagged-executes-until-the-last-child-finishes) | Chat multiplexes tagged Executes until the last child finishes | Accepted |
| [ADR-075](#adr-075-implementation-completion-is-gated-by-structured-reports-and-the-openspec-checklist) | Implementation completion is gated by structured reports and the OpenSpec checklist | Accepted |

**Amends:** [ADR-005](ADR.md#adr-005-subagent-invocation-via-task-tool-with-clean-sessions) (TUI named stage agents), [ADR-026](ADR-C03-001-cursor-harness-tui-autonomy.md#adr-026-tui-orchestrates-via-harness-not-chat).

## ADR-054: TUI Executes named stage agents on their YAML pair

**Context:** PRD-C04 requires the TUI to execute each agent’s `harness` + `model`. Research already does this for `discover_agent`. Other stages were still orchestrator Task, so mixed-harness YAML was ignored and Chat went idle.

**Decision:** Generalize the Research handoff. After `hero stage start`, the TUI orchestrator session stops. The TUI Executes the named stage agent(s) via `HarnessAdapter.Execute` with that agent’s YAML pair (then `fallback_model`). Cursor IDE Runtime still dispatches those agents via Task (ADR-031). Nested fan-out inside a stage agent remains Task (ADR-005).

**Consequences:**

- TUI preambles must forbid orchestrator Task dispatch of named stage agents.
- Agent prompt files are read from the harness projection (`.cursor/agents`, `.opencode/agents`, `.codex/agents`) with `.cursor/agents` as fallback.
- After non-research Executes complete, the TUI resumes the orchestrator with structured output so it can close the stage.

## ADR-055: Implementation scope agents run as concurrent TUI Executes

**Context:** Implementation may run `backend_agent`, `frontend_agent`, and `generic_agent` together. A single Execute cannot honor three harness pairs.

**Decision:** v1 starts one TUI Execute per in-scope implementation agent in one wave. Cross-agent SDD series is deferred. Each agent still uses Task for its own nested work.

**Consequences:**

- Chat must multiplex more than one adapter Execute.
- Navbar can show `BACK | FRNT | GEN` at once, plus nested `TASK` chips.

## ADR-056: Nested Task stays in the parent harness; generic Tasks chip TASK

**Context:** Nested generic Tasks were omitted from the agents box so they would not appear as `HARN`. Users still need a live count during Implementation fan-out.

**Decision:** Keep nested work as Task inside the parent Execute. Chip named Hero agents with existing 4-letter codes. Chip generic nested Tasks as `TASK`. Cursor, OpenCode, and Codex adapters MUST set `AgentName`, `CallID`, and `Phase` on Task start/complete when the harness event identifies a Task.

**Consequences:**

- `HARN` is not used for nested Task chips.
- Nested streaming remains best-effort when the parent harness forwards child deltas.

## ADR-057: Chat multiplexes tagged Executes until the last child finishes

**Context:** Conversation state assumed one `streaming` flag, one channel, and one agent bubble. Parallel Implementation would otherwise wipe the first child on `executeDone`.

**Decision:** Tag `streamDeltaMsg` and `executeDoneMsg` with an execute id. Reuse one Bubble Tea channel. `streaming` stays true until the last Execute finishes. Esc cancels every in-flight adapter session. Completing one Execute removes only that parent chip and bubble bookkeeping.

**Consequences:**

- `Update` stays non-blocking; Executes remain `tea.Cmd` / goroutine workers.
- Tests must assert sibling streams survive the first child’s `executeDone`.

## ADR-075: Implementation completion is gated by structured reports and the OpenSpec checklist

**Context:** ADR-054 resumed the orchestrator after the first TUI Execute wave,
regardless of whether an Implementation agent had completed the whole assigned
scope. Large OpenSpec changes could therefore be closed after a partial slice;
QA and Judge became an accidental scheduler for the unchecked tasks. The TUI
also treated `Escalated` as executable, so a continued agent could start before
the engine recorded the new running iteration.

**Decision:** For an OpenSpec-linked Implementation stage, the TUI reads the
linked `tasks.md`, validates exactly one canonical owner marker on every
implementation task, partitions the exact pending checklist by owner, and
injects only each agent's IDs into its assignment. The canonical markers are
`[agent:backend_agent]`, `[agent:frontend_agent]`, and
`[agent:generic_agent]`. Cross-cutting work must be decomposed into dependent
single-owner tasks. A legacy ownerless task is accepted only when exactly one
implementation agent is active and the assignment records
`ownership_validated: true`; with multiple active agents, missing, invalid, or
multiple ownership markers fail closed.

Each agent must return a structured `complete`, `partial`, or `blocked` report
whose `tasks_completed` and `tasks_remaining` contain only IDs from its
assignment. The report's required acceptance gates are
`completed_tasks_verified`, `task_ownership_respected`, and
`required_tests_passed`. Agents never edit `tasks.md` or its checkboxes. The
TUI/runtime scheduler is the sole checkbox writer and applies a completion
mark only after validating the report, ownership, gates, and verification
evidence. It may start another fresh wave in the same stage iteration only when
the checklist demonstrates progress, with a bounded no-progress guard; that
wave contains only agents with remaining assigned IDs. The orchestrator
receives permission to close Implementation only when all reports are valid and
complete, all gates pass, and the checklist has no unchecked tasks. Invalid,
empty, blocked, or no-progress results resume the orchestrator with an explicit
**do not close** instruction. Assignments and raw results are appended to
SQLite `conversation` records for post-run auditing; each assignment records
the ordered `task_ids` for its agent and wave. If Implementation starts or
restarts with no unchecked tasks, the TUI may run exactly one verification wave
with active agents to collect reports and gates. After a wave clears all
pending tasks, the scheduler rereads the checklist and closes directly without
redispatching an empty wave. `Escalated` is never executable; `/hero-continue`
must first transition the stage back to a runnable state.

If the active cycle has no linked OpenSpec checklist, the TUI fails closed and
asks the orchestrator to set `openspec_change` before retrying; it never invents
an assignment. Go extracts checklist state only; dependency reasoning and nested
Task fan-out remain with the implementation agent, preserving ADR-003.

**Consequences:**

- A successful subprocess return no longer implies stage completion.
- Additional implementation waves do not consume QA/Judge iterations.
- The no-progress guard prevents autonomous infinite Execute loops and leaves
  the running stage available for human correction.
- Mixed backend/frontend/generic waves cannot duplicate ownership or silently
  consume another agent's tasks; a missing owner with multiple active agents
  blocks before code execution.
- Agents no longer race while editing `tasks.md`; only the scheduler updates
  checkboxes after reports are validated.
- Existing harness transport interfaces and the engine stage schema are
  unchanged; audit records reuse the existing conversation table.
