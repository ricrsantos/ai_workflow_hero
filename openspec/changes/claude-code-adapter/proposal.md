## Why

Hero's TUI currently offers Cursor, OpenCode, and Codex, but users who work in Claude Code cannot assign that harness to a Hero agent while retaining Hero's normalized streaming, permissions, session, metrics, and fallback behavior. Cycle C13 adds Claude Code as an explicit, opt-in fourth TUI harness now that its headless CLI can provide a supervised stream-json execution boundary, without expanding the deterministic engine or changing Cursor IDE Runtime dispatch (PRD-C13-001 §§1–3; ADR-070–072).

## What Changes

- Add `internal/adapters/claude` for a turn-scoped `claude -p` subprocess on Claude Code 2.1.261+, including incremental NDJSON normalization, native session persistence/resume, final-result repair, usage, health, watchdog activity, and SIGINT-first cancellation (PRD-C13-001 §§2–3; ADR-071).
- Validate Claude availability and authentication at the adapter boundary; report missing, incompatible, and unauthenticated CLI states with actionable copy without starting login or storing credentials.
- Preserve `ask`, `auto-project`, and `auto-all`; validate the `ask` permission-prompt wire contract first with a fake-process/fixture spike, then use a one-time-token local/stdio bridge limited to permission requests (PRD-C13-001 §2; ADR-072).
- Extend Hero state, registry, routing, fallback, model selection, Doctor, Status, Telegram permission forwarding, and TUI identity so a cycle can mix four harnesses while preventing cross-harness session resume.
- Add embedded Claude-native commands, skills, and agent templates under `assets/claude/`, project them to `.claude/`, and manage only a marked root `CLAUDE.md` block that imports `@AGENTS.md`; upgrades and disables preserve user content and do not auto-enable Claude (PRD-C13-001 §2; ADR-073).
- Add the Claude native model catalog and C5 mapping: `ef` maps to `--effort` only when catalog-supported, while `th` and `fs` remain unavailable; unknown subscription or account pricing is never fabricated (PRD-C13-001 §2; ADR-074).
- Keep Claude TUI-only. Cursor IDE Runtime, Claude Agent SDK/Node bridges, interactive Claude TUI automation, login/API-key management, a Claude daemon, a process registry/orphan reaper, `--bare`, Windows, attachments, agent teams, and a third fallback remain out of scope (PRD-C13-001 §4; ADR-070–071).

## Capabilities

### New Capabilities

- `claude-adapter`: supervised Claude CLI execution, normalized streaming, sessions, health, cancellation, usage, and authentication/version diagnostics.
- `claude-permission-bridge`: constrained per-execution permission-prompt bridge for the `ask` profile and its failure/security behavior.
- `claude-projection`: `.claude/` assets, checksums, disabled/upgrade behavior, and marked `CLAUDE.md` ownership.
- `claude-model-catalog`: native Claude aliases/IDs, dated metadata, C5 properties, effective-model reporting, and unknown-price behavior.

### Modified Capabilities

- `harness-adapter`: add Claude as a normalized adapter with event, property, permission, health, and session-binding behavior.
- `hero-tui`: expose the fourth harness in selection, model/property pickers, labels, diagnostics, permission visibility, watchdog, and constrained rendering.
- `asset-bootstrap-and-layout`: add opt-in Claude projection and native catalog assets while preserving checksums and user-owned files.
- `cli-deterministic-command-suite`: include Claude state in install/upgrade/Doctor/Status/help diagnostics and actionable CLI/version checks.
- `harness-marker-detection`: treat an enabled Claude projection as supported while retaining warn-only behavior for an unconfigured `.claude/` marker.
- `model-property-catalog`: resolve Claude-native catalog entries and preserve zero/unset cost for unknown pricing.
- `model-property-discovery`: use local Claude catalog data and `system/init` effective model/capabilities without boot-time session creation.
- `runtime-workflow-execution`: route explicit `harness: claude` pairs through the existing TUI execution/fallback/preparation boundaries without changing Cursor Runtime behavior.

## Impact

- New adapter and tests under `internal/adapters/claude`; registry wiring in `internal/harnessmgr`; shared health timeout constants and session binding use in `internal/harness` and `internal/store`.
- Install/upgrade/uninstall, embedded assets, checksums, model catalogs, Doctor/Status, workflow configuration examples, TUI pickers/chat labels, Telegram permission forwarding, and user documentation.
- No new runtime daemon or persistent process registry; Claude child and bridge lifetime are scoped to one Execute. Tests use fake processes, NDJSON fixtures, temporary directories, injected clocks/launchers, and no live Claude account.
- All work is native scope and routes to `generic_agent`; implementation must preserve existing Cursor/OpenCode/Codex behavior and finish with `go test ./...` green.
