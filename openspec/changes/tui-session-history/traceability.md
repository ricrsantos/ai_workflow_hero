# C16 traceability — tui-session-history

Maps PRD-C16-001 FR-01–FR-14 and §7 success criteria, UI-C16-001 §10, and ADR-091–098 to OpenSpec tasks/specs/code.

## ADR → tasks → specs → code

- ADR-091 Hero vs native identity → task-01.*, task-02.*, task-06.1 → durable-session-history, sqlite-operational-store → `internal/store/migrate.go` (v12), `internal/store/sessions.go`, `internal/conversation/session_service.go`
- ADR-092 session aggregate + events → task-01.*, task-03.* → durable-session-history, sqlite-operational-store → `internal/store/session_events.go`
- ADR-093 conversation service + async TUI → task-06.*, task-10.*, task-12.* → durable-session-history, hero-tui → `internal/conversation/session_service.go`, `internal/tui/chat_session.go`, `internal/tui/history_screen.go`, `internal/tui/chat_session_history.go`
- ADR-094 leases → task-04.1, task-13.2 → session-continuation → `internal/store/session_leases.go`, `internal/tui/chat_session_history.go` (acquire/heartbeat on History open)
- ADR-095 exact resume + fork → task-13.1, task-15.1, task-07.2 → session-continuation, harness-adapter → `SessionService.ResumeBinding` / `ForkSession`, `internal/tui/chat_session_history.go` (`tryExactHarnessResume`, fork dialog)
- ADR-096 durable assets → task-05.*, task-08.1, task-16.2 → session-asset-store → `internal/store/session_assets.go`, `internal/media/retention.go`, `internal/tui/history_screen.go` (delete UI)
- ADR-097 optional capabilities → task-07.*, task-18.1 → harness-adapter → `internal/harness/capabilities.go`, `internal/adapters/opencode/remote_history.go`, `internal/tui/session_remote.go`
- ADR-098 legacy migration → task-01.2, task-09.1 → durable-session-history, sqlite-operational-store → `internal/store/session_bindings_legacy.go`, v11→v12 fixture in `store_test.go`

## PRD FR → tasks → code

| FR | Task(s) | Primary code |
|---|---|---|
| FR-01 create on first turn | task-06.2, task-06.3 | `session_service.go` `EnsureFirstTurn`, `internal/tui/chat_session.go` `persistUserTurnBeforeExecute` |
| FR-02 incremental events + Telegram origin | task-03.*, task-12.1, task-17.1 | `session_events.go`, `chat_session.go` persist/restore, `telegram_messages.go` labels |
| FR-03 History list/search/archive | task-02.2, task-10.*, task-11.1 | `history_screen.go`, `nav_sidebar.go` |
| FR-04 rename | task-02.1, task-10.1 | `history_screen.go`, `SessionService.RenameSession` |
| FR-05 exact resume | task-13.1, task-12.2 | `chat_session_history.go` `historyOpenCmd`, `applyHeroSessionBinding` |
| FR-06 no stage mutation | task-13.3 | `chat_session_history.go` `heroSessionStageCompleted`, historical banner in Chat |
| FR-07 exclusive lease | task-04.1, task-13.2 | `session_leases.go`, `chat_session_history.go` lease acquire/heartbeat |
| FR-08 interrupt recovery | task-14.1 | `session_service.go` `MarkInterrupted`, `chat_session_history.go` `sessionRecoverCheckCmd` |
| FR-09 explicit fork | task-15.1 | `session_service.go` `ForkSession`, `history_screen.go` fork dialog |
| FR-10 archive/restore/delete + assets | task-05.*, task-16.*, task-08.1 | `history_screen.go`, `chat_session_history.go` `historyDeleteCompleteCmd` |
| FR-11 confirmed remote import | task-18.1 | `session_service.go` `ImportRemoteHistory`, `session_import.go`, `session_remote.go`, import dialog in `history_screen.go` |
| FR-12 legacy migration | task-09.1 | `session_bindings_legacy.go` |
| FR-13 /new-chat | task-12.3 | `app.go` `/new-chat`, `chat_session_test.go` |
| FR-14 async TUI I/O | task-06.1, task-10.1, task-12.1 | `tea.Cmd` paths in History/Chat session files |

## PRD §7 / UI §10 acceptance → tasks

1. Free Chat survives restart → task-12.2, task-08.1 → `chat_session_test.go`, media retention tests
2. History without cycle + keys → task-11.1, task-10.* → `nav_sidebar_test.go`, `history_screen_test.go`
3. Parallel stage rows + historical continuation → task-02.1, task-13.3 → store sessions + `heroSessionStageCompleted`
4. Telegram origin → task-17.1 → `chat_session.go` origin on user/assistant persist, `eventsToTranscript`, `telegramOriginLabel`
5. Crash mid-stream → task-14.1 → interruption events + recover check
6. Second TUI busy + stale recovery → task-13.2, task-04.1 → `ErrSessionBusy`, lease TTL tests in store
7. Delete managed vs original assets → task-16.2, task-05.* → delete dialog + `PurgeManagedAssetFiles`
8. Resize layouts → task-10.2 → `history_screen_test.go` resize/copy tests

## Gaps (non-blocking / follow-up)

- OpenCode `ReadRemoteHistory` requires a live serve; production import binds registry adapter at History open (`attachRemoteHistoryReader`).
- Cursor/Codex/Claude adapters intentionally omit `RemoteHistoryReader` (capability tests).
