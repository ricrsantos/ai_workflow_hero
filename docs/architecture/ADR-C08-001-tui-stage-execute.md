# ADR-C08-001 — TUI-Direct Stage Execute

> Cycle C08 architecture decisions. Product: [PRD-C08-001](../product/PRD-C08-001-tui-stage-execute.md). UI: [UI-C08-001](../product/UI-C08-001-tui-stage-execute.md).

| # | Title | Status |
|---|---|---|
| [ADR-054](#adr-054-tui-executes-named-stage-agents-on-their-yaml-pair) | TUI Executes named stage agents on their YAML pair | Accepted |
| [ADR-055](#adr-055-implementation-scope-agents-run-as-concurrent-tui-executes) | Implementation scope agents run as concurrent TUI Executes | Accepted |
| [ADR-056](#adr-056-nested-task-stays-in-the-parent-harness-generic-tasks-chip-task) | Nested Task stays in the parent harness; generic Tasks chip TASK | Accepted |
| [ADR-057](#adr-057-chat-multiplexes-tagged-executes-until-the-last-child-finishes) | Chat multiplexes tagged Executes until the last child finishes | Accepted |

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
