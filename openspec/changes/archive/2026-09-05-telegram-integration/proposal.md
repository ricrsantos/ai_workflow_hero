## Why

Hero users who run the TUI on a workstation want a safe, local-only remote interface to the same conversation and cycle state without exposing bot credentials to harnesses or agents. Cycle C9 adds an optional official `telegram` plugin: a per-OS-user daemon owns the Telegram Bot API; TUIs register over authenticated local IPC; pairing binds exactly one authorized chat; addressed routing prevents duplicate harness turns across concurrent project and free-chat instances (PRD-C09-001 §1–§3; ADR-059–064).

The idea file `docs/idea/v3_0/telegram_integration.md` is the product origin. On divergence it yields to [PRD-C09-001](../../docs/product/PRD-C09-001-telegram-integration.md), [UI-C09-001](../../docs/product/UI-C09-001-telegram-integration.md), and [ADR-C09-002](../../docs/architecture/ADR-C09-002-telegram-integration.md). In particular, the idea’s “Hero Runtime owns Bot API from every TUI” model is **not** in scope: only the daemon calls Telegram; TUIs use IPC push, not SQLite polling (PRD-C09-001 §3.1; ADR-060).

## What Changes

- Ship an optional **`telegram` plugin** with matching platform daemon artifacts in Hero releases; install explicitly via `hero plugin install telegram` (PRD-C09-001 §3.1; ADR-059).
- **Extract a UI-agnostic conversation service** from `internal/tui/conversation.go`; TUI and Telegram IPC ingress adapt into it; harness adapters remain unaware of Telegram (PRD-C09-001 §5; ADR-061).
- Add a **versioned OS-user IPC protocol** between daemon and TUI clients: registration, instance suffix allocation, live event push, delivery acknowledgement, reconnect, and protocol-version rejection (ADR-060).
- Implement the **local daemon**: long-poll Bot API, one authorized chat, pairing codes (10 minutes), addressed routing (`<abbrev>:` prefix), slash-command vs plain-text dispatch, durable pending queue (24 hours), daemon-owned `/telegram-cancel-pending`, cycle notification filtering, and idempotent update handling (PRD-C09-001 §3.2–§3.4; ADR-063).
- Store **bot token and chat id only in the OS credential vault**; SQLite holds non-sensitive config, queue, and audit state; all logs and UI redact secrets (PRD-C09-001 §3.4; ADR-062).
- Extend the **TUI**: Settings Telegram section, pairing modal, transcript origin/destination labels, daemon reconnect UX, and IPC client lifecycle with bounded backoff (UI-C09-001; PRD-C09-001 §4).
- **Migrate and rotate logs**: project TUI log → `.workflow-hero/logs/tui.log`; daemon global log under `~/.workflow-hero/logs/`; 10 × 10 MB rotation; install/upgrade extends managed `.gitignore` for `.workflow-hero/logs/` (PRD-C09-001 §3.5; ADR-064).

### In Scope

- Native Go: conversation extraction, plugin CLI, IPC, daemon, TUI adapter, vault abstraction, log rotation, release artifact wiring.
- Linux and macOS, `amd64` and `arm64`.
- Injectable Bot API, clock, vault, IPC transport, and process launcher for tests (PRD-C09-001 §5).
- Colocated `_test.go` per touched package; `go test ./...` green without live Telegram.

### Out of Scope

- More than one authorized chat; groups/roles; cross-machine relay; public webhooks; cloud service.
- Attachments, voice, inline keyboards, bot-command menus.
- Cancelling delivered TUI work or deleting Telegram messages.
- Moving harness execution into the daemon or Telegram-specific cycle business rules.
- Forwarding thinking/tool/stream deltas to Telegram.
- Windows CLI; Browser UI Validation; QA End-to-End (C9 scope `native` only).

### CLI vs Runtime Classification (ADR-003)

- **CLI (deterministic):** `hero plugin install|uninstall|list|doctor`, daemon binary distribution, vault read/write from daemon only, log migration on install/upgrade, IPC socket permissions.
- **Runtime (TUI):** Settings/pairing UI, IPC client registration, conversation ingress/egress labels, cycle notification subscription, bounded daemon restart in Bubble Tea `tea.Cmd`.
- **Unchanged:** Cursor IDE Runtime slash commands and orchestration assets; harness adapters do not gain Telegram types.

## Capabilities

### New Capabilities

- `conversation-service`: transport-neutral conversation/session/context service extracted from TUI; sole route to `HarnessAdapter`; cycle notifier interface (ADR-061).
- `telegram-plugin`: optional plugin install/uninstall/list/doctor; release artifact coupling; version compatibility checks (ADR-059).
- `telegram-vault`: OS credential vault abstraction for token + authorized chat id; injectable fake for tests (ADR-062).
- `telegram-ipc`: versioned local socket protocol, client auth, registration, push events, acks, incompatible-version rejection (ADR-060).
- `telegram-daemon`: Bot API client, pairing, routing, queue, notifications, daemon-owned cancel-pending, global SQLite store (ADR-059–063).
- `telegram-tui`: IPC client lifecycle, Settings section, pairing modal, transcript labels, reconnect/backoff UX (UI-C09-001).
- `hero-log-rotation`: project log path migration, size rotation, daemon global log, managed gitignore (ADR-064).

### Modified Capabilities

- `hero-tui`: Settings Telegram section visibility rules; Chat transcript remote labels; lifecycle messages in Chat (UI-C09-001 §§1–4).
- `cli-deterministic-command-suite`: new `hero plugin` subcommands; doctor coverage for plugin/daemon compatibility.
- `asset-bootstrap-and-layout`: embed/extract plugin daemon artifacts per platform in release layout.
- `env-hygiene`: extend managed `.gitignore` block for `.workflow-hero/logs/` without overwriting user rules.

## Requirement Traceability

| Approved source | Delta requirements | Implementing tasks |
|---|---|---|
| PRD-C09-001 §3.1 plugin/daemon lifecycle | telegram-plugin R1; telegram-daemon R1 | 3.1–3.4, 6.1–6.2 |
| PRD-C09-001 §3.2 instance addressing | telegram-daemon R2; telegram-ipc R2 | 5.3, 6.3–6.4 |
| PRD-C09-001 §3.3 conversation/routing/queue | conversation-service R1–R3; telegram-daemon R3–R5 | 1.1–1.4, 6.5–6.8 |
| PRD-C09-001 §3.4 pairing/vault | telegram-vault R1; telegram-daemon R6 | 4.1–4.3, 6.6 |
| PRD-C09-001 §3.5 logs | hero-log-rotation R1; env-hygiene R1 | 2.1–2.4 |
| PRD-C09-001 §4 TUI | telegram-tui R1–R4; hero-tui R1 | 7.1–7.6, 8.1–8.3 |
| PRD-C09-001 §5 NFR / testability | all specs injectable deps | 9.1, 10.1 |
| PRD-C09-001 §7 acceptance 1–8 | mapped per capability | §1–§10 |
| UI-C09-001 §§1–5 | telegram-tui; hero-tui | 7, 8 |
| ADR-059 | telegram-plugin; telegram-daemon | 3, 6 |
| ADR-060 | telegram-ipc | 5 |
| ADR-061 | conversation-service | 1 |
| ADR-062 | telegram-vault | 4 |
| ADR-063 | telegram-daemon | 6 |
| ADR-064 | hero-log-rotation; env-hygiene | 2 |

## Impact

- New packages: `internal/conversation/` (or equivalent slice), `internal/plugin/`, `internal/telegram/ipc/`, `internal/telegram/daemon/`, `internal/telegram/vault/`, `cmd/hero-telegram-daemon/` (or plugin subcommand entry).
- Heavy refactor touch: `internal/tui/conversation.go`, `internal/tui/launch.go`, `internal/common/envhygiene/`, `scripts/release.sh`, `internal/install`, `internal/upgrade`, `internal/doctor`.
- Tests: injected fakes throughout; no live Bot API or OS vault in CI.
- Implementation agent: **generic_agent** (`scope.native: true`).
- OpenSpec change: `telegram-integration`.

## Success Criteria

1. `hero plugin install telegram` installs a matching-release daemon; first configured TUI starts it; daemon exits after last client disconnects.
2. Pairing binds one chat within ten minutes; credentials never appear in UI, logs, harness context, or SQLite secrets columns.
3. Concurrent project TUIs get stable `name` / `name_2` addresses; free chat gets `free_N`; announcements and outbound messages carry source abbreviation.
4. `<addr>: /hero-status` uses TUI command path; `<addr>: text` reaches one harness turn; response appears locally and remotely.
5. Unavailable target queues 24 hours; `/telegram-cancel-pending` cancels queue only.
6. Cycle notifications delivered; intermediate harness activity not forwarded.
7. Logs rotate 10 × 10 MB; old TUI log migrates; `.workflow-hero/logs/` gitignored safely.
8. `go test ./...` passes without external Telegram or vault.
