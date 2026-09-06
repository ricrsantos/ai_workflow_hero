# C9 Judge loop-back — implementation gaps

Source: Judge (`judge_agent`, `opencode-go/deepseek-v4-pro`) on `openspec/changes/telegram-integration`.
`all_requirements_met: false`. No SDD ambiguity. Re-run `generic_agent` for these tasks only.

Backend packages already exist (do not reimplement unless wiring requires it):
`internal/conversation`, `internal/common/logrotate`, plugin CLI, `vault.Store`, IPC protocol, daemon lifecycle/queue/pairing.

## Required work

1. conversation-service R1 (tasks 1.2/1.3) — Wire TUI to `conversation.Service`. `internal/conversation` is unused; `internal/tui/conversation.go` still owns dispatch.
2. conversation-service R3 (task 1.4) — Subscribe cycle engine events to `Notifier`. No Notifier usage outside the package.
3. hero-log-rotation R1 (task 2.2) — Migrate TUI slog from `.workflow-hero/tui.log` to `.workflow-hero/logs/tui.log` (logrotate is daemon-only today).
4. env-hygiene R1 (task 2.4) — Extend the managed `.gitignore` block for `.workflow-hero/logs/`.
5. asset-bootstrap-and-layout R1 (task 3.3) — `scripts/release.sh` and release contract tests must include platform daemon artifacts.
6. telegram-plugin R2 (task 3.4) — Doctor/status must report plugin installed / daemon binary / version-compat.
7. telegram-ipc R3 (task 5.4) — TUI IPC client (`connect` / `register` / `unregister` / reconnect `tea.Cmd`). Zero Telegram references in `internal/tui` today.
8. telegram-tui R1/R2 (tasks 7.1–7.4) — Settings Telegram section, Pair/Replace/Clear/Test, pairing modal, daemon outage retry.
9. telegram-tui R3/R4 + hero-tui R1/R2 (tasks 8.1–8.3) — Transcript origin/destination labels, Notifier→IPC outbound wiring, reconnect copy.
10. task 9A.1 — Update `docs/architecture/architecture-overview.md` (still says planned Telegram plugin / no standalone `internal/conversation`).
11. task 9C.1 — Update `context/current-state.md` and `context/context-log.md` for landed C9 work.
12. tasks 9D.1/9D.2 — End-to-end integration lock and `hero cycle openspec-change` persistence.

SDD: `openspec/changes/telegram-integration` (39 tasks, 12 capability specs). Scope: native → `generic_agent`.
