# ADR-C09-002 — Telegram Plugin and Local Conversation Daemon

> Cycle C09 architecture decisions. Product: [PRD-C09-001](../product/PRD-C09-001-telegram-integration.md). UI: [UI-C09-001](../product/UI-C09-001-telegram-integration.md). Status: Proposed.

| # | Decision | Status |
|---|---|---|
| ADR-059 | Telegram ships as an optional official plugin with a local daemon | Proposed |
| ADR-060 | Versioned OS-user IPC replaces multi-TUI database polling | Proposed |
| ADR-061 | Conversation core is shared; transport adapters stay at the edge | Proposed |
| ADR-062 | Pair one chat and isolate all Telegram credentials in the OS vault | Proposed |
| ADR-063 | Addressed instances, durable queue, and remote queue cancellation | Proposed |
| ADR-064 | Project and daemon rotating logs with managed ignore migration | Proposed |

## ADR-059: Telegram ships as an optional official plugin with a local daemon

**Context:** Long-polling the same bot from every TUI loses or races updates. A daemon embedded in one elected TUI makes lifecycle and recovery fragile. The product is local to one user/machine and must not require cloud infrastructure.

**Decision:** Package a `telegram` plugin and its platform-specific daemon with the matching Hero release. `hero plugin install telegram` installs it explicitly. One daemon per local user multiplexes all registered Hero TUIs, starts on first configured client, and exits after the last client disconnects. Clients perform bounded-backoff restart attempts after an unexpected exit.

**Consequences:** The main `hero` binary remains deterministic and the daemon contains no LLM reasoning. Plugin release compatibility and process ownership need explicit doctor/install/upgrade coverage. A remote daemon, webhook endpoint, and cross-machine support remain out of scope.

## ADR-060: Versioned OS-user IPC replaces multi-TUI database polling

**Context:** SQLite can provide durable queue state but cannot directly wake independent Bubble Tea processes. Polling adds latency and contention while still requiring ownership/election logic.

**Decision:** The daemon owns Bot API receive/send and exposes a versioned local IPC protocol over an OS-user-private socket. TUIs register, receive pushed events, acknowledge delivery, and reconnect. The daemon owns atomic instance suffix allocation and queue delivery. Its private SQLite store is used only for durable configuration state, update de-duplication, queue/audit records, and recovery—not for live client polling.

**Consequences:** Socket location/permissions, protocol compatibility, and client authentication become first-class platform concerns. The design preserves immediate notification while allowing durable recovery after a TUI or daemon restart.

## ADR-061: Conversation core is shared; transport adapters stay at the edge

**Context:** `internal/tui/conversation.go` currently couples conversation lifecycle to Bubble Tea. Copying it into a Telegram integration would produce diverging free-chat and cycle-chat semantics.

**Decision:** Extract UI-independent conversation service/types/session/context behind a transport-neutral interface. TUI and Telegram IPC ingress adapt messages into this service; it remains the only route to `HarnessAdapter`. Cycle status events are published through a narrow notifier interface that the TUI and Telegram adapter can subscribe to.

**Consequences:** `internal/tui` renders messages but no longer owns conversation business rules. Telegram packages cannot import harness-specific protocol types, and harness adapters never learn Telegram credentials or payload structures.

## ADR-062: Pair one chat and isolate all Telegram credentials in the OS vault

**Context:** Bot tokens grant control of the bot, while the authorized chat id identifies the sole remote principal. Both must not appear in prompts, files, logs, screenshots, or normal settings output.

**Decision:** Keep the token and authorized chat id together in an OS credential-vault entry available only to the daemon. Pairing uses a one-time ten-minute code held only for the pending operation. Project/global SQLite records only non-sensitive configuration state and queue/audit data. Settings displays state, never values.

**Consequences:** Tests use an injected vault. Unsupported/unavailable vaults fail setup explicitly; no environment-variable or plaintext fallback is silently introduced. Replacing/clearing configuration removes the vault entry and invalidates pending pairing state.

## ADR-063: Addressed instances, durable queue, and remote queue cancellation

**Context:** Multiple instances of the same project must not answer one message twice. A target can be temporarily unavailable, and the user needs a safe way to retract undelivered inputs.

**Decision:** The daemon allocates a project base abbreviation then `_2+` suffixes, retaining them until the project's final instance leaves; Free Chat uses `free_N`. It accepts only `<address>:`-prefixed input, tracks each message by provider update id and a finite delivery state, and holds unavailable-target messages for 24 hours. The daemon owns `/telegram-cancel-pending`, which moves every pending message for the address to `cancelled` without invoking the cycle command dispatcher.

**Consequences:** User messages cannot be deleted from Telegram and already delivered work cannot be rolled back. Status transitions are audit-worthy. Explicit addressing makes multi-project and multi-instance routing deterministic.

## ADR-064: Project and daemon rotating logs with managed ignore migration

**Context:** The existing TUI log is a single `.workflow-hero/tui.log`. Telegram adds per-project conversation/daemon-related diagnostics plus daemon-global transport diagnostics; unbounded files and tracked logs are unacceptable.

**Decision:** Move project log output to `.workflow-hero/logs/tui.log`, rotating after 10 MB and retaining at most ten files. Maintain an independently rotating daemon log in `~/.workflow-hero/logs/` with the same policy. Install/upgrade safely migrate the old log path and maintain a Hero-owned `.gitignore` rule for `.workflow-hero/logs/`.

**Consequences:** The project log path changes but user `.gitignore` content is preserved. Log serializers must redact tokens and chat ids before any write, including errors and debug diagnostics.
