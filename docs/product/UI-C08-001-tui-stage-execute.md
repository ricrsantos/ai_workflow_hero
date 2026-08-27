# UI-C08-001 — TUI-Direct Stage Execute

> Cycle C08 terminal UX. Extends [UI-C03-001](UI-C03-001-tui-harness-autonomy.md) conversation streaming and [UI-C04-001](UI-C04-001-tui-multi-harness.md) speaker labels.

## 1. Scope

In scope: Chat transcript labels for TUI-direct stage agents, Task launch lines, navbar `TASK` chips, parallel Implementation speakers, wait spinner while any Execute is live.

Out of scope: Cursor IDE chrome, Config screen, free-chat Execute routing.

## 2. Speaker headers

Each TUI-direct Execute and each attributed nested block uses:

```text
[PLAN - grok-4.6-high · cursor]
[GEN - gpt-5.6-terra · codex]
[JUDG - opencode-go/deepseek-v4-pro · opencode]
[TASK - composer-2.5 · cursor]
[CTX - opencode-go/deepseek-v4-flash · opencode]
```

The pair is the agent YAML pair (or the Task `model` argument when present). It must not fall back to the orchestrator pair while a child Execute is live.

## 3. Task launch line

When a Task starts, Chat inserts a muted tool line **before** swallowing the event. Example:

```text
[PLAN - grok-4.6-high · cursor]
→ Task planning_agent
```

Generic nested Tasks use the `TASK` header. Completion may omit a second tool line when nested text already arrived (existing UI-C03 `result.content` rule).

## 4. Sidebar agents box

```text
agents: 4
BACK | FRNT | TASK | TASK
```

- Count = live Execute parents + open Task calls (named or generic).
- Generic nested Tasks clip to `TASK`.
- Named Hero agents keep existing 4-letter codes.
- `HARN` remains only for unbound freechat / unknown parent sessions, never for a nested Task chip.

## 5. Wait spinner

While any Execute is in flight, keep `Waiting for harness…` at the **end** of the transcript (under the latest child block). Hide it when the last Execute completes, fails, or is cancelled.

## 6. Parallel Implementation

Two or three in-scope implementation agents appear as separate green agent blocks in arrival order, each with its own header. Nested Task lines for a block insert immediately above that block. Completing BACK must leave the FRNT block streaming.

## 7. Messages

- `→ Planning` / `→ Implementation` / `→ QA` / `→ Judge` (and existing `→ Research`) when a handoff Execute starts.
- Fallback copy still names both harness and model (UI-C04 §6).
- Esc copy remains “cancels the running agent(s)”.

## 8. Testing UX

- Handoff goldens: Planning header uses the planning pair.
- Parallel Implementation: two headers + navbar count.
- `TASK` vs `CTX` chips.
- Sibling `executeDone` does not clear the other speaker or spinner.
