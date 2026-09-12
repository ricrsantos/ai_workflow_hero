# PRD-C16-001 — Persistent TUI Session History

> Cycle C16 product requirements. Adds durable, project-scoped conversation history to the Hero TUI. Index: [PRD.md](PRD.md). UI: [UI-C16-001](UI-C16-001-tui-session-history.md). Architecture: [ADR-C16-001](../architecture/ADR-C16-001-tui-session-history.md).

## 1. Problem

Hero currently keeps ordinary Free Chat session IDs and transcripts only for the TUI process lifetime. Cycle orchestration and named stage agents retain some harness-native session IDs in SQLite, but there is no durable, user-facing inventory of conversations, no complete transcript restoration, no naming, and no archive/delete lifecycle. A TUI restart therefore breaks the user's expectation that a conversation can be found and continued from where it stopped.

## 2. Goal

Add a persistent `History` screen where the user can find, name, resume, archive, restore, and permanently delete every conversation executed or observed by the Hero TUI, while preserving native harness continuity and keeping workflow state independent from conversation navigation.

## 3. Scope

### 3.1 Included conversations

- Free Chat sessions.
- Orchestration and Research sessions.
- Every named stage-agent session, including a separate session for each agent in a parallel stage.
- Local TUI and Telegram-originated turns in the same conversation, with origin retained.
- Sessions for active, completed, cancelled, and archived cycles.
- Existing project rows that contain usable harness-session metadata.

Only conversations executed or observed by Hero TUI are included. Chats opened directly in Cursor IDE or another harness UI are not discovered or imported in C16.

History is scoped to the current project store. Standalone `hero chat` uses its existing synthetic Free Chat root and must not aggregate conversations from unrelated projects.

### 3.2 Session creation and naming

- A durable session is created atomically when its first user message or stage-agent prompt is accepted. Empty Chat surfaces do not create history rows.
- `/new-chat` retains the current session in History and opens a new empty Chat surface.
- Free Chat receives a short deterministic title derived from its first textual message without an LLM call. An attachment-only first turn uses a deterministic media fallback title.
- Stage sessions use `C{number} · {stage} · {agent-label}`. Orchestration and Research follow the same cycle-aware convention.
- The user may rename active or archived sessions. Names need not be unique.

### 3.3 Durable transcript

- Persist every event that was visible in Chat: user messages, assistant output, harness-exposed thinking, tool activity/results, warnings, permission and question prompts/outcomes, attachment and generated-asset cards, interruptions, and Telegram origin markers.
- Never fabricate or persist private reasoning that a harness did not expose.
- Persist normalized events incrementally, in stable session order, as they arrive. A visible event must have a durable representation; persistence failure is explicit and blocks further sends until recovery or cancellation.
- Store asset references and metadata, not image bytes, in SQLite.
- Restoring a session reconstructs the full locally available transcript, scroll state defaults to the newest content, and context occupancy continues to represent the selected session rather than a cross-session total.

### 3.4 History discovery

- `History` is always present in the TUI navbar immediately after `Chat`, including when no cycle is active.
- Active sessions sort by most recent activity descending.
- Search is case-insensitive by session name only in C16.
- Each list row shows name, conversation type, harness/model, and last activity.
- A detail panel shows creation and activity timestamps, active/archived/interrupted state, cycle/stage/agent when applicable, harness/model, local transcript availability, and available actions.
- Active and archived sessions use separate views. No automatic retention limit or age-based deletion applies.

### 3.5 Resume and recovery

- Opening a session in Chat resumes the same harness-native session with its original harness and model. Saved model-property values needed for semantic continuity are restored with the session.
- Resuming an archived session first restores it to the active list.
- Continuing a completed stage-agent conversation does not reopen the stage or mutate its lifecycle merely because the conversation was resumed.
- A TUI may browse History during an Execute but may not switch, archive, restore, delete, or otherwise take ownership of a session while any current agent execution or preflight makes that action unsafe.
- Exactly one TUI instance may own a session for continuation at a time. A second instance receives a clear busy error; abandoned ownership must be recoverable after its lease becomes stale.
- If a TUI exits during a response, received events remain visible and the session is marked interrupted. On reopen, Hero checks native execution status. It reconnects to a live stream when supported and blocks new sends until completion/cancellation; otherwise it explains the adapter limitation and offers cancellation/recovery.
- If the native session or original model cannot be resumed, Hero explains why and offers an explicit fork: create a new native session using the locally available transcript as context. It never silently changes harness or model.

### 3.6 Legacy sessions and remote history

- Migration creates History entries from existing orchestration/stage session bindings when enough metadata exists. It is idempotent and never invents unavailable messages.
- A migrated entry without a local transcript is labeled `Local transcript unavailable` and remains resumable when the harness accepts its native ID.
- When an adapter can read remote session history, Hero asks for confirmation before the first import. Imported events then follow the normal local retention rules.
- A failed or unsupported remote-history read leaves local state unchanged and gives an actionable message.

### 3.7 Archive, restore, and delete

- Archive is reversible and does not end the native harness session. Archived items live in a separate History view.
- Opening an archived item restores and opens it.
- Permanently deleting a session requires explicit confirmation and removes its local transcript, metadata, ownership record, and Hero-managed asset copies.
- Original files selected by the user are external references and are never deleted.
- Hero attempts remote native-session deletion only when the adapter supports it. Local deletion remains successful when remote deletion fails; the UI warns that provider-side data may remain.
- Archiving or deleting the currently open, idle session opens a new empty Chat surface.

## 4. Functional requirements

| ID | Requirement |
|---|---|
| FR-01 | Create one durable Hero session per Free Chat, orchestration, Research, or named stage-agent conversation on first accepted turn. |
| FR-02 | Persist normalized visible transcript events incrementally with deterministic ordering and Telegram origin. |
| FR-03 | List project sessions in `History`, active first by recent activity, with a separate archived view and name search. |
| FR-04 | Rename sessions without uniqueness enforcement. |
| FR-05 | Resume the original harness-native session with the original harness/model/property snapshot. |
| FR-06 | Preserve stage lifecycle state when a completed stage conversation is resumed. |
| FR-07 | Enforce exclusive continuation ownership across TUI instances with stale-owner recovery. |
| FR-08 | Preserve interrupted transcripts and recover or explicitly cancel a still-live native execution. |
| FR-09 | Offer an explicit context fork when native continuation is unavailable; never migrate silently. |
| FR-10 | Archive, restore, and permanently delete sessions with the specified confirmation and asset rules. |
| FR-11 | Import remote transcript only after confirmation and only through an adapter capability. |
| FR-12 | Idempotently expose legacy session bindings without fabricating transcript content. |
| FR-13 | `/new-chat` keeps the prior session and resets Chat to an unpersisted empty surface. |
| FR-14 | History operations and transcript persistence never block Bubble Tea `Update`; I/O executes through commands/services. |

## 5. Non-functional requirements

- SQLite remains the sole operational source of truth. Session content is local plaintext protected by project filesystem permissions; C16 adds no application-layer encryption.
- The durable store must preserve event order across concurrent stream callbacks, reject cross-session routing, and use bounded transactions.
- Session IDs, prompts, transcript text, attachment paths, Telegram identifiers, and provider payloads must not be written to diagnostic logs.
- Long transcripts must load incrementally or by bounded pages so History and Chat remain responsive.
- All rendered widths are ANSI/rune-aware and respond to terminal resize. Narrow terminals degrade to a single-pane list/details flow without losing actions.
- Implementation must use the repository's `golang-tui` and `go-engineering` skills. Their guidance applies to Elm-style effects, responsive rendering, keymaps, package cohesion, cancellation, error handling, concurrency, security, and behavior-focused testing.
- No live harness, network, Telegram Bot API, or clock dependency is allowed in automated tests.

## 6. Out of scope

- Global cross-project history.
- Importing arbitrary Cursor IDE, OpenCode, Codex, or Claude chat catalogs.
- Full-text search over message bodies.
- Automatic summarization or LLM-generated titles.
- Automatic expiry, quotas, or retention cleanup for saved sessions.
- Application-layer transcript encryption.
- Synchronizing the same session across machines.
- Reopening workflow stages merely by opening their conversations.

## 7. Success criteria

1. A Free Chat survives TUI restart with its title, complete visible transcript, attachments, model metadata, and resumable native session.
2. Research, orchestration, and parallel stage agents appear as distinct History entries and can be continued without changing completed stage state.
3. Telegram-originated messages appear in the same transcript with their origin intact.
4. History remains responsive with a long transcript and on terminal resize; all persistence I/O stays outside `Update` and `View`.
5. Two TUI instances cannot concurrently continue the same session, and a crashed owner does not lock it forever.
6. Archive/restore is lossless. Permanent deletion removes only Hero-owned local data, never the user's source files, and reports remote deletion failure without rolling back local deletion.
7. Schema migration preserves all v11 rows, produces idempotent legacy entries, and a second open makes no further changes.
8. `go test ./...` passes with deterministic store, service, adapter-capability, migration, and TUI behavior coverage.

