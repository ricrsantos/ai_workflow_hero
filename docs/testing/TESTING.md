# Testing — AI Workflow Hero

> Test strategy and commands for the Hero CLI repository.

## Test command

```bash
go test ./...
```

Run from the repository root after any code change. All tests must pass before marking work complete.

## Strategy

- **Unit tests**: colocated `*_test.go` in each `internal/<feature>/` package; same package; test behavior, not implementation details.
- **Golden tests**: template rendering and asset output fixtures.
- **Integration tests**: compiled `hero` binary against `t.TempDir()` for `install`, `upgrade`, `uninstall`, and `doctor`. Cover install without `--tools`, `--tools` error, 1.x-style upgrade leaving OpenCode disabled, 2.4-style upgrade leaving Codex disabled, OpenCode projection on enable, and Codex projection on enable (`.codex/` from `assets/codex/`).
- **Codex adapter tests**: injectable stdio/process (no live LLM or ChatGPT account); `IsAvailable` without CLI; unauthenticated → explicit `codex login` error; incompatible/missing `app-server` → explicit error; unknown JSON-RPC event → warning (never silent); session id never resumed across harnesses; ListModels native ids; permission request mapping to `OnPermissionRequest`.
- **OpenCode adapter tests**: SSE idle/gone/busy probes; serve-restart continue of a stuck tool turn; completed-turn recovery after restart; SSE blip without process restart must not abort or re-prompt.
- **Model capability tests**: API-first model/capability discovery, adapter normalization, SQLite cache persistence, stale-cache fallback, local catalog fallback, dynamic value replacement, per-harness/model property persistence, and explicit harness rejection.
- **TUI property tests**: `/hero-model` background refresh, immediate cache/catalog rendering, boolean and multi-value property pickers, `ENTER to save`, Escape cancellation, gray unavailable labels, green configured labels, warning clearing, and responsive property-line rendering.
- **C8 TUI-direct stage Execute tests**: Planning/Judge handoff uses the stage agent pair; Implementation scope agents run concurrent Executes; nested generic Tasks chip `TASK`; named `context_agent` chips `CTX`; sibling `executeDone` does not clear the other stream; Codex/OpenCode Task start events carry `CallID` and agent/generic name.
- **Telegram plugin tests**: injected Bot API/vault/clock/IPC/process launcher; one-chat pairing and 10-minute expiry; redaction; private-socket registration; stable project/free-chat address allocation; numbered `/list` and persistent `/select n` routing, including selected-instance disconnection; TUI `/status` and project-local `0`/`1–300` minute auto-reporting; exactly-once update de-duplication; pending delivery/24-hour expiry/cancellation; daemon restart backoff; notification filtering; transcript labels; log rotation and old-path migration; plugin install/upgrade and `.gitignore` preservation. No test contacts Telegram or an OS credential vault.
- **C7 Config screen tests**: conditional Config navigation (active-cycle only), round-trip YAML golden fixtures preserving comments/order/unknown keys/`workflow_rules`, managed-field merge after parallel edits, atomic-write failure safety, field-level validation, responsive form states, dirty-exit confirmation, read-only execution state, harness/model/property filtering, Save/Save and start routing, cycle synchronization, completed-stage protection, and explicit failed-stage retry with counter reset and preserved events/metrics.
- **Dependencies**: prefer real filesystem and `embed.FS` over mocks; keep tests deterministic and fast.

## Coverage areas

| Area | Packages / paths |
|---|---|
| Install / upgrade / uninstall | `internal/install`, `internal/upgrade`, `internal/uninstall` |
| Doctor / status | `internal/doctor`, `internal/status` |
| Cycle / store / engine | `internal/cycle`, `internal/store`, `internal/engine` |
| TUI / harness | `internal/tui`, `internal/harness`, `internal/adapters/cursor`, `internal/adapters/opencode`, `internal/adapters/codex` |
| Model properties / metadata cache | `internal/tui`, `internal/harness`, `internal/harnessmgr`, `internal/store`, `internal/adapters/cursor`, `internal/adapters/opencode`, `internal/adapters/codex`, `assets/models/` |
| Templates / assets | `internal/common/template`, `assets/` |
| Release contract | `scripts/release_test.go` |

## CI

No CI workflow is required for V1 (manual release via `scripts/release.sh`). CI/CD automation is deferred to V2 (see PRD §2.3).
