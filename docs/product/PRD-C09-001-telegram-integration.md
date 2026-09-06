# PRD-C09-001 — Telegram Remote Interface Plugin

> Cycle C09 product requirements. Telegram is an optional, local-only remote interface for the Hero TUI. It is bidirectional, shares conversation and cycle state with the TUI, and never exposes bot credentials to a harness or agent.

## 1. Summary

Hero will ship an official optional `telegram` plugin in the same repository and release artifacts as `hero`. Installing it adds a separate local daemon that owns the Telegram Bot API connection. The daemon serves one operating-system user on one machine and communicates with all local Hero TUIs through authenticated local IPC. It is not a cloud service and does not use SQLite polling for live delivery.

Telegram and the TUI are two interfaces over the same conversation and cycle services. A Telegram slash command follows the existing TUI command flow; ordinary text is delivered to the target TUI's harness session and its response returns to Telegram. Incoming Telegram content is also shown in the target TUI transcript.

## 2. Goals

- Offer an official opt-in Telegram plugin distributed with Hero releases and installed with `hero plugin install telegram`.
- Support multiple projects and multiple TUI instances concurrently for the same local OS user.
- Route a message to exactly one TUI instance without duplicate harness turns or duplicate Telegram replies.
- Preserve TUI command semantics for Telegram-originated slash commands.
- Make pairing and daily use safe: one authorized chat, credential isolation, no sensitive values in UI or logs.
- Persist undelivered messages for 24 hours and allow remote cancellation before delivery.
- Keep both visible Chat transcripts and rotating technical logs auditable.

## 3. Scope

### 3.1 Plugin, daemon, and lifecycle

- The plugin is optional and is not installed or enabled by a normal `hero install`.
- Hero releases publish the platform-matched daemon/plugin artifact together with the main binary. The plugin version is compatible only with its matching Hero release version.
- The daemon is local to one OS user and machine. It supports every installed Hero project for that user; it is neither a project-local process nor a remote/shared service.
- The first Telegram-configured TUI starts or attaches to the daemon. The daemon stops after its last registered TUI disconnects.
- A still-connected TUI detects an unexpected daemon exit and retries startup with bounded exponential backoff. Each outage and recovery is visible in the transcript and technical logs.
- The daemon is the only process that calls the Telegram Bot API. TUIs receive live events through local IPC, not through SQLite polling.

### 3.2 Instance addressing and registration

- During first Telegram configuration for a project, the user supplies a project abbreviation. It is persisted and editable in TUI Settings.
- The first live instance of that project receives the base abbreviation. Concurrent instances receive `_2`, `_3`, and so on. Assignments remain stable while any instance for that project remains registered; after the last one closes, the next one may use the base abbreviation again.
- Free Chat instances receive `free_1`, `free_2`, and so on, under the same allocation rule.
- On registration, and after successful pairing/configuration, each instance sends an announcement to the authorized Telegram chat identifying its project/free-chat name and assigned abbreviation.
- When a live instance disconnects cleanly or its IPC socket drops, the daemon sends a compact disconnection announcement to the authorized Telegram chat before scheduling any last-client shutdown.
- All relevant outbound Telegram messages identify their source abbreviation.
- The daemon lists live instances with `/list` and persists the authorized chat's one-based `/select n` choice without storing its chat id in SQLite. Subsequent ordinary text and slash commands route to that selected live instance without an address prefix; selection is invalid while its instance is disconnected and produces an actionable error rather than a queued turn. Explicit `<abbreviation>:` input remains supported for targeted delivery and pending-queue management.

### 3.3 Conversation, commands, events, and pending messages

- A slash command after selection (or the explicit address prefix) is dispatched through the equivalent TUI command path. Ordinary text is delivered as an input turn to the target instance's `ConversationService`, then to its existing harness session. The daemon returns `OK, Received.` when it accepts live delivery to that instance.
- Telegram `/status` is TUI-owned and does not consume a harness turn. It returns `idle` when the instance is idle; an active cycle returns its cycle and current-stage state plus the TUI `Session`, `AI wk`, `AI rp`, and context-window `used/max` values; an active free-chat turn returns `Waiting for harness` with those same counters. Every Telegram request that starts or queues a harness turn also receives this status immediately.
- Project Settings persists `telegram.auto_report_minutes`: `0` disables automatic status reports and `1` through `300` sends the same status on that interval while Telegram is paired and connected.
- `<address>: /model` keeps selection remote: Telegram sends numbered harness options, then model options, then choices for every selectable model property. The final reply atomically persists the same free-chat model/properties used by local Chat.
- Harness responses and Telegram-originated inputs appear in the target TUI Chat transcript with an unambiguous Telegram origin/destination label.
- The daemon sends relevant outbound notifications only: cycle/stage start and finish, approval requests, errors, and final results. It does not forward intermediate activity, thinking, tool noise, or every stream delta.
- If the addressed instance is unavailable, the daemon stores a durable pending message. It expires after 24 hours unless delivered or cancelled.
- `<abbreviation>: /telegram-cancel-pending` is daemon-owned: it cancels every still-pending message for that instance. It never maps to `/hero-cancel`, never cancels a cycle, and never interrupts a started harness turn.
- Every inbound message has a stable provider/update id, target, timestamps, status (`pending`, `delivered`, `processed`, `cancelled`, or `expired`), and non-secret audit metadata. Delivery is idempotent.

### 3.4 Pairing and authorization

- V0 permits exactly one authorized Telegram `chat_id`.
- Settings exposes Telegram setup, pairing, test, replace, and clear actions. It never displays the configured token or chat id; it reports only state such as `Configured`.
- Pairing opens a focused TUI modal with step-by-step instructions, a single-use code, countdown, waiting state, success, expiry, and cancellation.
- The user sends `/start <code>` (or the code) to the bot. A code is valid for ten minutes; only a valid unexpired code binds the sender as the authorized chat.
- Unauthorized messages are ignored or rejected generically without project, cycle, configuration, token, or authorization disclosure.
- The bot token and authorized chat id are retained only in the operating-system credential vault. The daemon is the sole component allowed to resolve them. SQLite retains non-sensitive configuration state and durable queue/audit data only.
- Tokens and chat ids must never enter harness prompts, agent context, workflow files, UI output, logs, errors, telemetry, or diagnostic command output.

### 3.5 Logs and repository hygiene

- Project TUI technical logs move from `.workflow-hero/tui.log` to `.workflow-hero/logs/tui.log`; upgrade performs a safe migration when the old file exists.
- TUI logs rotate by size: at most ten files, each no larger than 10 MB. Telegram inputs, outputs, delivery transitions, daemon connection/restart events relevant to the project, and errors are logged with credentials masked.
- The daemon keeps its own rotating global technical log under `~/.workflow-hero/logs/` for machine-wide transport, IPC, lifecycle, and Bot API diagnostics. It uses the same 10 files × 10 MB policy and never writes credentials.
- `hero install` and `hero upgrade` extend Hero's managed `.gitignore` block to ignore `.workflow-hero/logs/` while preserving all user-owned `.gitignore` content and existing rules.

## 4. TUI requirements

- Settings gains a Telegram plugin section with installed/available/connected/configured state, project abbreviation editing, Pair/Test/Replace/Clear actions, and clear recovery guidance when the daemon cannot start.
- The Chat transcript displays remote inputs and responses without changing the existing harness speaker labels.
- Telegram lifecycle/error messages use existing Hero semantic colors/icons and remain actionable without disclosing secrets.
- The pairing modal is keyboard accessible: focusable actions, visible countdown, Escape/Cancel cleanup, and no token/chat-id rendering.

## 5. Non-functional requirements

- Linux and macOS on amd64 and arm64 only; Windows and cross-machine relay are out of scope.
- The daemon must use a local IPC endpoint with OS-user-only permissions and a versioned protocol. It must reject clients from another user and incompatible plugin versions.
- IPC registration, instance allocation, reconnect, de-duplication, delivery acknowledgement, daemon shutdown, and queue expiry must remain race-safe under concurrent TUI launches.
- The Conversation Service is UI-agnostic. Telegram-specific API, pairing, and IPC details do not leak into harness adapters or cycle business rules.
- No live Telegram account is required for automated tests. Bot API, clock, vault, process launch, and IPC transport must be injectable.

## 6. Out of scope

- More than one authorized chat, group/role authorization, cross-machine operation, public webhook hosting, or a cloud relay.
- Telegram attachments, images, voice, inline keyboards/callbacks, and bot-command discovery menus.
- Cancelling a message already delivered to a TUI or deleting a user's Telegram message.
- Replacing the TUI, moving harness execution into the daemon, or creating Telegram-specific cycle business logic.
- Sending intermediate harness/tool/thinking stream activity to Telegram.

## 7. Acceptance criteria

1. A matching-release Telegram plugin installs separately and a configured first TUI starts its daemon; the daemon stops after the last registered TUI closes.
2. Pairing binds one chat only, expires after ten minutes, masks credentials everywhere, and ignores an unauthorized chat safely.
3. Two project TUIs receive stable `name` and `name_2` addresses, while Free Chats receive `free_1`, `free_2`.
4. `/list` returns numbered live instances; `/select n` persists a selected instance; then `/hero-status` and plain text reach only that selected live harness and receive `OK, Received.` after live delivery. A disconnected selected instance returns an actionable error.
5. An unavailable target retains a message for 24 hours; `/telegram-cancel-pending` cancels its queue without changing any cycle/harness state.
6. Important cycle notifications are delivered with source identity, while intermediate activity is not.
7. Logs rotate at 10 × 10 MB, the old TUI log path migrates safely, and install/upgrade ignore the new project log directory without overwriting user `.gitignore` entries.
8. `go test ./...` passes without a live Bot API, vault, or external daemon dependency.
