# Tasks — telegram-integration

Scope: **native** → all implementation via `generic_agent`.  
Nested generic Task fan-out: `composer-2.5` (`agents.planning_agent.subagent`, `same_of_agent: true`).  
**Compatibility:** do not regress Cursor IDE Runtime, C4/C5/C6/C7/C8 harness Execute, checksum upgrade, dual-entry, timers (ADR-058), or deterministic engine. Follow golang-tui Elm Architecture on existing `github.com/charmbracelet/bubbletea` (no v2 migration).  
**Divergence:** idea-file “Runtime owns Bot API from every TUI” is superseded by daemon + IPC (ADR-059/060).

**Parallelism legend:** **PARALLEL** = concurrent Task subagents after deps met; **SERIES** = ordered.

PRD traceability: [PRD-C09-001](../../docs/product/PRD-C09-001-telegram-integration.md); ADR: [ADR-C09-002](../../docs/architecture/ADR-C09-002-telegram-integration.md); UI: [UI-C09-001](../../docs/product/UI-C09-001-telegram-integration.md).

---

## 1. Conversation service extraction — SERIES (foundation)

Extract UI-independent conversation core before Telegram wiring (ADR-061).

- [x] 1.1 Create `internal/conversation` with `Service`, session/context types, and a narrow `Notifier` interface for cycle/stage/approval/error/final events. Document public API in package doc (design D3)
- [x] 1.2 Move harness dispatch, slash-vs-text routing, and Execute lifecycle hooks from `internal/tui/conversation.go` into the service without behavior change. Keep Bubble Tea rendering in TUI. Colocated characterization tests: existing slash commands and free-text turns still reach the same adapter paths (spec `conversation-service` R1)
- [x] 1.3 Wire TUI to adapt Bubble Tea messages into `conversation.Service` methods; preserve transcript roles, wait spinner, Esc cancel, and multi-Execute multiplexing (spec `conversation-service` R2)
- [x] 1.4 Subscribe cycle engine events to `Notifier` so downstream transports can filter notifications without importing harness types (spec `conversation-service` R3; PRD-C09-001 §3.3)

## 2. Hero log rotation — PARALLEL with §3 after §1.1

- [x] 2.1 Add rotating writer helper (10 MB × 10 files) with shared credential redaction. Unit tests for rotation trigger and redaction patterns (spec `hero-log-rotation`; ADR-064)
- [x] 2.2 Point TUI slog from `.workflow-hero/tui.log` to `.workflow-hero/logs/tui.log`. Upgrade/install: safe one-time migration when legacy file exists (PRD-C09-001 §3.5 AC7)
- [x] 2.3 Daemon global log under `~/.workflow-hero/logs/telegram-daemon.log` using the same rotation policy (design D6)
- [x] 2.4 Extend `envhygiene` managed `.gitignore` for `.workflow-hero/logs/` without overwriting user entries; update envhygiene tests (spec `env-hygiene`)

## 3. Plugin distribution + CLI — PARALLEL with §2 after §1.1

- [x] 3.1 Define plugin manifest layout under `~/.workflow-hero/plugins/telegram/` (version, daemon path, protocol version). Colocated load/save tests (spec `telegram-plugin`)
- [x] 3.2 Implement `hero plugin install|uninstall|list telegram` and embed/extract platform daemon artifact from release layout. Integration test with `t.TempDir()` (spec `cli-deterministic-command-suite`; ADR-059)
- [x] 3.3 Extend `scripts/release.sh` and release contract tests for daemon artifacts per GOOS/GOARCH (spec `asset-bootstrap-and-layout`)
- [x] 3.4 Doctor/status: plugin installed, daemon binary present, version compatibility warning (spec `telegram-plugin` R2)

## 4. Credential vault — SERIES after §3.2

- [x] 4.1 Define injectable `vault.Store` with `Store/Load/Clear/Available`; fake in-memory impl for tests (spec `telegram-vault`; ADR-062)
- [x] 4.2 Linux Secret Service + macOS Keychain backends; unsupported platform returns explicit error (no silent env fallback)
- [x] 4.3 Redaction helpers: never log or return token/chat id through errors, JSON debug, or Settings (golden tests)

## 5. IPC protocol — SERIES after §3.1

- [x] 5.1 Specify and implement framed JSON protocol (`protocol_version`, register/unregister/ack/outbound/inbound/event/error). Client and server share `internal/telegram/ipc` types (design D2; spec `telegram-ipc`)
- [x] 5.2 OS-user-private socket with `0600` perms; reject wrong UID and incompatible protocol version. Colocated socket permission tests
- [x] 5.3 Daemon-authoritative instance registration and suffix allocation (`base`, `_2+`, `free_N`) with SQLite transaction. Concurrent registration test (spec `telegram-ipc` R2; PRD-C09-001 §3.2)
- [x] 5.4 TUI IPC client: connect/register/unregister/reconnect in `tea.Cmd`; push `tea.Msg` without blocking Update (spec `telegram-tui` R4; ADR-053)

## 6. Daemon core — SERIES after §4 and §5

- [x] 6.1 Daemon process entry: injectable Bot API, clock, vault, IPC listener, SQLite store. Lifecycle: start on first client, exit after last unregister (spec `telegram-daemon` R1)
- [x] 6.2 Long-poll Bot API receive loop with `update_id` de-duplication and idempotent persistence (spec `telegram-daemon` R5)
- [x] 6.3 Pairing: generate 10-minute single-use code, validate `/start <code>` or bare code, bind one chat, clear pending on cancel/timeout (PRD-C09-001 §3.4; spec `telegram-daemon` R6)
- [x] 6.4 Inbound routing: require `<address>:` prefix; slash → forward as command; plain → `inbound` to registered TUI; malformed/unknown → generic response without project disclosure (spec `telegram-daemon` R3)
- [x] 6.5 Pending queue: store when target offline; 24-hour expiry; deliver on reconnect; `ack_delivery` state machine (`pending`→`delivered`→`processed`/`cancelled`/`expired`) (spec `telegram-daemon` R4; PRD-C09-001 §3.3)
- [x] 6.6 Daemon-owned `/telegram-cancel-pending`: cancel pending rows only; never invoke `/hero-cancel` or harness interrupt (spec `telegram-daemon` R7)
- [x] 6.7 Outbound notifications from TUI `outbound` IPC: prefix source abbreviation; filter out stream/thinking/tool noise (PRD-C09-001 §3.3; UI-C09-001 §4)
- [x] 6.8 Registration announcements and test message actions from Settings (PRD-C09-001 §3.2)

## 7. TUI Settings + pairing — PARALLEL with §8 after §5.4

- [x] 7.1 Settings Telegram section when plugin installed: installed/version, daemon connection, configured state, editable project abbreviation, display-only live suffix (UI-C09-001 §1)
- [x] 7.2 Pair/Replace/Clear/Test actions with state gating; not-installed copy points to `hero plugin install telegram`
- [x] 7.3 Pairing modal: step instructions, visible countdown, waiting/success/expiry/cancel; Esc invalidates code; never render token/chat id (UI-C09-001 §2)
- [x] 7.4 Daemon outage: Chat + Settings retry messaging; bounded backoff restart; success `✓ Telegram daemon reconnected` (UI-C09-001 §1)

## 8. TUI transcript + notifications — PARALLEL with §7 after §1.4 and §5.4

- [x] 8.1 Render Telegram-originated input with `← [Telegram · <address>]` without changing harness speaker headers (UI-C09-001 §3)
- [x] 8.2 Render outbound Telegram replies with `→ [Telegram · <address>]` after harness completion; slash commands keep command result rendering (UI-C09-001 §3)
- [x] 8.3 Wire `Notifier` to IPC outbound for cycle/stage/approval/error/final only; pending/expired/cancel notices as muted informational lines (UI-C09-001 §§3–4)

## 9. End-to-end + regression — PARALLEL tracks then SERIES lock

### 9A. Architecture overview — PARALLEL

- [x] 9A.1 Update `docs/architecture/architecture-overview.md`: daemon, IPC, conversation service, vault, log paths (AGENTS.md rule)

### 9B. Regression — PARALLEL

- [x] 9B.1 Existing TUI Chat, Execute, Config, timers, free chat, and Cursor IDE asset tests remain green; fix only intentional breaks from conversation extraction

### 9C. Context landing — PARALLEL

- [x] 9C.1 Update `context/current-state.md` and append `context/context-log.md` when implementation lands

### 9D. Integration lock — SERIES after §6–§8 and 9A–9C

- [x] 9D.1 Temp-dir integration: install plugin → pair (fake vault/Bot API) → two TUIs get `proj`/`proj_2` → addressed command + text → offline queue → cancel-pending → notification filter → `go test ./...` green
- [x] 9D.2 Verify `hero cycle openspec-change telegram-integration` persisted on active cycle (ADR-023)

---

## Parallel groups (orchestrator fan-out)

| Group | When | Tasks | Agent |
|---|---|---|---|
| Foundation | start | §1 SERIES | `generic_agent` |
| A | after 1.1 | 2.1–2.4 ‖ 3.1–3.4 | `generic_agent` |
| B | after 5.4 + 1.4 | 7.1–7.4 ‖ 8.1–8.3 | `generic_agent` |
| C | after §6–§8 | 9A.1, 9B.1, 9C.1 | `generic_agent` |

Hard series spine: **1 → (2 ‖ 3) → 4 → 5 → 6 → (7 ‖ 8) → (9A ‖ 9B ‖ 9C) → 9D**.

**Task count:** 39 checklist items across §1–§9.
