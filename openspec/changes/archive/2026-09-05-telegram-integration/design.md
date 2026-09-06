## Context

Hero’s TUI currently embeds conversation lifecycle, harness dispatch, and transcript rendering in `internal/tui/conversation.go` (~3k lines). Telegram requires the same semantics from a second transport without duplicating harness rules or leaking credentials into adapters (ADR-061). Prior cycles established deterministic CLI boundaries (ADR-003), Bubble Tea Elm Architecture on existing `github.com/charmbracelet/bubbletea`, SQLite for operational cycle state, and env hygiene for `.gitignore` (C1–C8).

Authoritative sources: [PRD-C09-001](../../docs/product/PRD-C09-001-telegram-integration.md), [UI-C09-001](../../docs/product/UI-C09-001-telegram-integration.md), [ADR-C09-002](../../docs/architecture/ADR-C09-002-telegram-integration.md). Scope is **native only** → `generic_agent`. Do not migrate Bubble Tea v2.

## Goals / Non-Goals

**Goals:**

- Optional official plugin + per-user daemon (ADR-059).
- Versioned IPC with push events (ADR-060).
- Shared conversation core (ADR-061).
- Vault-isolated credentials (ADR-062).
- Deterministic addressing, queue, cancel-pending (ADR-063).
- Rotating logs + safe gitignore migration (ADR-064).
- Full testability via injection (PRD-C09-001 §5).

**Non-Goals:**

- Cloud relay, webhooks, multi-chat authorization, Windows.
- Telegram attachments/keyboards/stream mirroring.
- Harness or Cursor IDE Runtime changes.
- SQLite polling as the live transport between TUI and daemon.

## Decisions

### D1 — Plugin + daemon packaging (ADR-059)

Release artifacts include `hero-telegram-daemon` (name TBD in implementation) per `GOOS/GOARCH` alongside `hero`. The main binary embeds plugin metadata and install paths; `hero plugin install telegram` copies the daemon and registers plugin state under `~/.workflow-hero/plugins/telegram/`. Normal `hero install` does **not** enable Telegram.

Version compatibility: daemon and plugin manifest declare the Hero semver they match; doctor warns on mismatch.

Lifecycle:

```text
First configured TUI registers ──► spawn daemon if absent
Additional TUIs ──► IPC register only
Last TUI disconnects ──► daemon graceful shutdown
Unexpected daemon exit ──► TUI bounded backoff restart (transcript + technical log)
```

### D2 — IPC protocol (ADR-060)

Transport: Unix domain socket (Linux) / named pipe or UDS equivalent (macOS) under `~/.workflow-hero/run/`, mode `0600`, owned by effective UID.

Protocol: length-prefixed JSON frames (or newline-delimited JSON — pick one in implementation; document in code). Required fields on every request/response: `protocol_version`, `message_id`, `type`.

Core message types:

| Direction | Type | Purpose |
|---|---|---|
| TUI → daemon | `register` | project path, mode (cycle/free), project abbreviation, plugin version |
| TUI → daemon | `unregister` | clean disconnect |
| TUI → daemon | `ack_delivery` | idempotent delivery confirmation |
| TUI → daemon | `outbound` | harness/cycle notification to Telegram |
| daemon → TUI | `registered` | assigned address suffix, pairing state |
| daemon → TUI | `inbound` | addressed user text or slash command |
| daemon → TUI | `event` | pairing progress, daemon lifecycle, queue notices |
| daemon → TUI | `error` | non-secret failure |

Reject clients with wrong UID, incompatible `protocol_version`, or plugin/Hero version mismatch.

Instance suffix allocation is **daemon-authoritative** (atomic in daemon SQLite): base abbreviation for first live project instance; `_2`, `_3` while any sibling registered; `free_N` for free chat under the same rule.

### D3 — Conversation service extraction (ADR-061)

Introduce `internal/conversation` (name fixed in task 1.1) with:

- `Service` accepting `Input` (text, origin metadata, slash vs free text).
- Session/context types shared by cycle chat and free chat.
- Single dispatch path to `HarnessAdapter.Execute` (reuse existing routing helpers where possible).
- `Notifier` interface for cycle/stage/approval/error/final-result events consumed by TUI transcript renderer and Telegram outbound adapter.

TUI becomes a **view + adapter**: Bubble Tea messages call into `conversation.Service`; rendering stays in `internal/tui`. Telegram IPC handler enqueues the same service methods; never imports harness adapter protocol structs into daemon packages beyond what the TUI client already uses.

### D4 — Vault and pairing (ADR-062)

Vault interface: `Store(token, chatID)`, `Load()`, `Clear()`, `Available()`.

Implementations:

- Linux: Secret Service (`github.com/zalando/go-keyring` or stdlib-compatible wrapper).
- macOS: Keychain via same abstraction.

Pairing flow:

1. TUI opens modal; daemon generates single-use numeric code + 10-minute expiry (in-memory + optional daemon SQLite pending row without secrets).
2. User sends `/start <code>` or bare code to bot.
3. Daemon validates, binds `chat_id`, writes vault entry, clears pending code, notifies TUI.
4. Unauthorized inbound: generic ignore/reply with no project disclosure.

Settings never reads vault values for display—only booleans like `Configured`.

### D5 — Routing, queue, cancel-pending (ADR-063)

Inbound Telegram text MUST match `^<address>:\s*(.+)$` (case-sensitive address as allocated). Malformed or unknown address: generic response without project names.

Slash after prefix → TUI command dispatcher equivalent (`/hero-status`, etc.). Plain text → `conversation.Service` input turn.

If target TUI not connected: persist pending row in daemon SQLite with provider `update_id`, expiry `now+24h`, status `pending`. On register/reconnect, daemon pushes `inbound` once; TUI `ack_delivery` moves to `delivered` → harness path → `processed`.

`/telegram-cancel-pending` (after address prefix): daemon transitions all `pending` for that address to `cancelled`; never calls cycle cancel or harness interrupt.

Outbound notifications (daemon sends via Bot API):

- Cycle/stage start/finish, approval required, errors, final results.
- Prefix every message with `<address>:` or configured project label per PRD/UI.
- Do **not** subscribe to stream deltas, thinking, tools, or debug activities.

De-duplication: daemon records processed `update_id`; redelivered Telegram updates are no-ops.

### D6 — Logs and gitignore (ADR-064)

Project TUI slog target moves from `.workflow-hero/tui.log` to `.workflow-hero/logs/tui.log` with `lumberjack`-style rotation (10 MB × 10 files). On upgrade/install, if legacy file exists and new path absent, rename/move once.

Daemon log: `~/.workflow-hero/logs/telegram-daemon.log` (+ rotated siblings), same policy.

Shared redaction helper masks token-like and numeric chat id patterns before any log write.

`envhygiene` managed block adds `.workflow-hero/logs/` without removing user `.gitignore` lines; deprecate single-file rule where superseded.

### D7 — TUI integration (UI-C09-001)

Settings Telegram section visible only when plugin installed. States: not installed, not configured, pairing, configured, daemon disconnected/retrying.

Pairing modal: keyboard accessible, countdown, Esc cancels and invalidates code, no token/chat id rendering.

Chat transcript adds directional labels:

```text
← [Telegram · ai_workflow_2]
→ [Telegram · ai_workflow_2]
```

IPC client runs as `tea.Cmd` goroutine pushing `tea.Msg` into the program; never blocks `Update`.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Conversation refactor regressions | Characterization tests around slash dispatch + Execute routing before/after extraction |
| IPC races on concurrent TUI launch | Daemon SQLite transaction for suffix allocation; integration test with multiple temp clients |
| Vault unavailable on headless CI | Injectable fake vault; doctor surfaces explicit setup failure locally |
| Secret leakage via slog | Central redaction + golden log tests |
| Daemon orphan processes | PID file + last-client shutdown + doctor cleanup hint |

## Migration Plan

1. Land conversation extraction with TUI-only path green.
2. Add log rotation + gitignore (safe upgrade migration).
3. Add plugin CLI + daemon binary distribution.
4. Enable IPC + daemon; wire TUI client behind Settings flag.
5. Enable pairing and inbound/outbound paths.
6. Update architecture overview and context files on implementation close.

Rollback: plugin uninstall stops daemon registration; TUI continues without Telegram section; conversation service remains shared core.

## Open Questions

None blocking Planning—Research locked V0 to one chat, local daemon, and IPC push (PRD-C09-001).
